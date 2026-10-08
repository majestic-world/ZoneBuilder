package scene

import (
	"fmt"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/texture"
	"zonebuilder/internal/unreal"
)

// addTerrain adds the terrain of map m (tile t), if it has one: one batch
// per drawable layer and its height field. Port of UE2-Studio's
// add_terrain and TerrainGrid::new, except that a broken TerrainScale is
// placed by MapX/MapY instead of being dropped (UE2-Studio draws no terrain
// then), and that a terrain with no drawable layer is drawn untextured
// rather than not at all. A terrain whose heightmap is absent (no
// TerrainMap, or a package the client lacks) becomes a warning, as in
// UE2-Studio.
func (s *Scene) addTerrain(ld *loader, m *l2pkg.Package, t Tile) error {
	i := unreal.FindTerrainInfo(m)
	if i < 0 {
		return nil
	}
	info, err := unreal.ReadTerrainInfo(m, i)
	if err != nil {
		return err
	}
	if info.TerrainMap == 0 {
		s.Warnings = append(s.Warnings, t.Name()+": sem terreno: o TerrainInfo não tem TerrainMap")
		return nil
	}
	hm, err := info.Heightmap(ld.c, m)
	if l2pkg.IsMissing(err) {
		s.Warnings = append(s.Warnings, fmt.Sprintf("%s: sem terreno: %v", t.Name(), err))
		return nil
	}
	if err != nil {
		return err
	}
	heights, err := hm.Heights()
	if err != nil {
		return err
	}
	w, h := hm.USize, hm.VSize
	if w < 2 || h < 2 {
		return fmt.Errorf("heightmap de %d×%d: o terreno precisa de pelo menos 2×2", w, h)
	}
	ter := Terrain{
		Tile:           t,
		Width:          w,
		Height:         h,
		Heights:        heights,
		Position:       vec(info.Position(w, h)),
		Scale:          vec(info.Scale()),
		QuadVisibility: info.QuadVisibilityBitmap,
		EdgeTurn:       info.EdgeTurnBitmap,
		FallbackScale:  info.BrokenScale(),
	}
	// The grid every layer shares: positions, mask UVs spanning the tile,
	// and the triangles. Only the texture UV differs per layer.
	grid := make([]Vertex, 0, w*h)
	ter.Bounds = geom.EmptyBox()
	for y := range h {
		for x := range w {
			p := ter.Vertex(x, y)
			grid = append(grid, Vertex{Pos: p, MaskUV: [2]float32{float32(x) / float32(w-1), float32(y) / float32(h-1)}})
			ter.Bounds.Include(p)
		}
	}
	var indices []uint32
	for y := range h - 1 {
		for x := range w - 1 {
			if tris, ok := ter.cell(x, y); ok {
				indices = append(indices, tris[:]...)
			}
		}
	}
	for k := range info.Layers {
		l := &info.Layers[k]
		tex, mask, err := layerBitmaps(ld, m, l)
		if err != nil {
			s.Warnings = append(s.Warnings, fmt.Sprintf("%s: camada %d do terreno não desenhada: %v", t.Name(), k, err))
			continue
		}
		// The first layer is the opaque base; its alpha map is never read
		// (UE2-Studio's fs_main), so the batch keeps no mask.
		mode := TerrainLayer
		if k == 0 {
			mode, mask = Opaque, nil
		}
		uv := l.UVMapping()
		verts := make([]Vertex, len(grid))
		for i, v := range grid {
			v.UV = uv(i%w, i/w)
			verts[i] = v
		}
		ter.Layers = append(ter.Layers, len(s.Batches))
		s.Batches = append(s.Batches, Batch{Mode: mode, Texture: tex, Mask: mask, Vertices: verts, Indices: indices, Bounds: ter.Bounds})
	}
	if len(ter.Layers) == 0 {
		ter.Layers = append(ter.Layers, len(s.Batches))
		s.Batches = append(s.Batches, Batch{Mode: Opaque, Vertices: grid, Indices: indices, Bounds: ter.Bounds})
	}
	ter.Batch = ter.Layers[0]
	s.Terrains = append(s.Terrains, ter)
	return nil
}

// layerBitmaps are the texture and alpha map a terrain layer of map m is
// drawn with; a nil mask is a layer with no alpha map, which covers the
// whole terrain (UE2-Studio's 1×1 white mask). A layer that has an alpha
// map but cannot read or decode it is not drawn, as in UE2-Studio.
func layerBitmaps(ld *loader, m *l2pkg.Package, l *unreal.TerrainLayer) (tex, mask *texture.Texture, err error) {
	if tex, err = ld.texture(m, l.Texture); err != nil {
		return nil, nil, fmt.Errorf("textura: %w", err)
	}
	if l.AlphaMap != 0 {
		if mask, err = ld.texture(m, l.AlphaMap); err != nil {
			return nil, nil, fmt.Errorf("alpha map: %w", err)
		}
	}
	return tex, mask, nil
}

// Vertex is the world position of grid sample (x, y).
func (t *Terrain) Vertex(x, y int) geom.Vec3 {
	g := geom.Vec3{X: float32(x), Y: float32(y), Z: float32(t.Heights[y*t.Width+x])}
	return g.Mul(t.Scale).Add(t.Position)
}

// cell returns the two triangles (vertex indices into the terrain batch)
// of grid cell (x, y), whose corners are samples (x, y) to (x+1, y+1); an
// invisible quad has none. The split diagonal follows EdgeTurn.
func (t *Terrain) cell(x, y int) ([6]uint32, bool) {
	q := x + y*t.Width // stride is the vertex width, not Width-1
	if !bit(t.QuadVisibility, q) {
		return [6]uint32{}, false
	}
	a, b := uint32(q), uint32(q+1)
	c, d := uint32(q+1+t.Width), uint32(q+t.Width)
	if !bit(t.EdgeTurn, q) {
		return [6]uint32{a, b, c, a, c, d}, true
	}
	return [6]uint32{d, a, b, d, b, c}, true
}

// bit reads bit i of an LSB-first bitmap; past its end reads false, since
// Unreal omits trailing all-zero bytes.
func bit(b []byte, i int) bool {
	return i/8 < len(b) && b[i/8]&(1<<(i%8)) != 0
}

func vec(v [3]float32) geom.Vec3 { return geom.Vec3{X: v[0], Y: v[1], Z: v[2]} }
