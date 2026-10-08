// Package scene builds a map's renderable and pickable geometry in CPU
// memory, with no GPU involved: the Scene seam of the spec (Load; Pick
// arrives with terrain picking).
//
// Positions are absolute world coordinates in Unreal's basis (X/Y
// horizontal, Z up), which is the server's world space: tile X_Y starts at
// ((X-20)*32768, (Y-18)*32768). The renderer subtracts Origin before
// uploading, to keep float precision near the camera, and swaps Y/Z into its
// Y-up render basis (ToRender).
package scene

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/texture"
)

// TileSpan is the world size of one map tile on X and Y.
const TileSpan = 32768

// Tile names one map tile, the X_Y of Maps/X_Y.unr. Classic selects the
// X_Y_Classic variant of the same tile.
type Tile struct {
	X, Y    int
	Classic bool
}

// ParseTile reads "X_Y" or "X_Y_Classic" (any case).
func ParseTile(s string) (Tile, error) {
	parts := strings.Split(strings.TrimSpace(s), "_")
	var t Tile
	switch {
	case len(parts) == 3 && strings.EqualFold(parts[2], "Classic"):
		t.Classic = true
	case len(parts) != 2:
		return Tile{}, fmt.Errorf("tile %q: use X_Y ou X_Y_Classic", s)
	}
	var errX, errY error
	t.X, errX = strconv.Atoi(parts[0])
	t.Y, errY = strconv.Atoi(parts[1])
	if errX != nil || errY != nil {
		return Tile{}, fmt.Errorf("tile %q: X e Y devem ser números inteiros", s)
	}
	return t, nil
}

// Name is the map's package name.
func (t Tile) Name() string {
	n := fmt.Sprintf("%d_%d", t.X, t.Y)
	if t.Classic {
		n += "_Classic"
	}
	return n
}

// Origin is the world position of the tile's minimum X/Y corner.
func (t Tile) Origin() (x, y float32) {
	return float32((t.X - 20) * TileSpan), float32((t.Y - 18) * TileSpan)
}

// RenderMode is how a batch is drawn. The values run in the order the
// renderer draws their passes (UE2-Studio gpu.rs).
type RenderMode uint8

const (
	// Opaque writes depth and ignores alpha.
	Opaque RenderMode = iota
	// TerrainLayer blends a terrain layer over the layers below it by
	// Mask's R channel times the texture's alpha, depth-tested >= without
	// writing depth.
	TerrainLayer
)

// Vertex is one batch vertex.
type Vertex struct {
	// Pos is the absolute world position, Unreal basis.
	Pos geom.Vec3
	// UV is the Texture coordinate, sampled with repeat.
	UV [2]float32
	// MaskUV is the Mask coordinate, 0..1 across the mask.
	MaskUV [2]float32
}

// Batch is a run of triangles drawn with one texture, mask and render mode.
type Batch struct {
	Mode RenderMode
	// Texture is the bitmap the batch is drawn with; nil draws it
	// untextured, UE2-Studio's flat grey. Batches sharing a Texture export
	// share the pointer.
	Texture *texture.Texture
	// Mask is the coverage bitmap a TerrainLayer batch is blended by (its
	// R channel); nil covers everything.
	Mask     *texture.Texture
	Vertices []Vertex
	Indices  []uint32
	// Bounds is the world AABB of Vertices.
	Bounds geom.Box
}

// Terrain is one tile's height field, kept for picking alongside the
// batches that draw it.
type Terrain struct {
	Tile Tile
	// Batch is the index into Scene.Batches of the terrain's base: the
	// first drawn layer, or an untextured grid when no layer can be drawn.
	// Like every terrain batch, its vertex k is grid sample
	// (k % Width, k / Width).
	Batch int
	// Layers are the indices into Scene.Batches of the drawn layers, in
	// draw order (TerrainInfo.Layers order); Layers[0] is Batch. A layer
	// whose texture or alpha map cannot be drawn is left out, as in
	// UE2-Studio.
	Layers []int
	// Width and Height are the heightmap's sample counts; Heights is
	// row-major, row = y.
	Width, Height int
	Heights       []uint16
	// Position is sample (0, 0) at height 0, Scale the world size of one
	// step on each axis: vertex(x, y) = Position + (x, y, Heights[y*Width+x]) * Scale.
	Position, Scale geom.Vec3
	// QuadVisibility and EdgeTurn hold one bit per quad, index x + y*Width.
	// An invisible quad has no triangles.
	QuadVisibility, EdgeTurn []byte
	// FallbackScale is set when TerrainScale was broken and the terrain was
	// placed by MapX/MapY instead.
	FallbackScale bool
	// Bounds is the world AABB of every grid vertex.
	Bounds geom.Box
}

// Scene is everything Load built for a set of tiles.
type Scene struct {
	Batches  []Batch
	Terrains []Terrain
	// Bounds is the world AABB of every batch vertex.
	Bounds geom.Box
	// Framing is Bounds with vertex outliers trimmed, what the opening
	// camera frames.
	Framing geom.Box
	// Origin is the rebase origin, Bounds' centre: the renderer uploads
	// Pos - Origin.
	Origin geom.Vec3
	// Warnings are the parts of the tiles that could not be loaded but did
	// not stop the rest (a missing terrain package).
	Warnings []string
}

// ToRender converts an Unreal-basis vector (Z up) to the renderer's Y-up
// basis by swapping Y and Z, as UE2-Studio does. The swap is its own
// inverse.
func ToRender(v geom.Vec3) geom.Vec3 { return geom.Vec3{X: v.X, Y: v.Z, Z: v.Y} }

// Load reads the map of every tile from the client directory clientRoot
// (the folder above Maps) and builds their geometry. A tile whose map is
// missing or unreadable fails the load; a part of a map that cannot be
// loaded because its package is missing becomes a warning.
func Load(clientRoot string, tiles []Tile) (*Scene, error) {
	if len(tiles) == 0 {
		return nil, errors.New("nenhum tile para abrir")
	}
	c := l2pkg.NewClient(clientRoot)
	ld := newLoader(c)
	s := &Scene{}
	for _, t := range tiles {
		m, err := c.Package(t.Name())
		if err != nil {
			if l2pkg.IsMissing(err) {
				return nil, fmt.Errorf("o cliente não tem o mapa %s (Maps/%s.unr)", t.Name(), t.Name())
			}
			return nil, err
		}
		if err := s.addTerrain(ld, m, t); err != nil {
			return nil, fmt.Errorf("%s: terreno: %w", t.Name(), err)
		}
	}
	s.Bounds = geom.EmptyBox()
	for i := range s.Batches {
		s.Bounds.Union(s.Batches[i].Bounds)
	}
	if !s.Bounds.Empty() {
		s.Origin = s.Bounds.Center()
	}
	s.Framing = framingBounds(s.Batches, s.Bounds)
	return s, nil
}
