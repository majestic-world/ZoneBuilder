package scene

import (
	"fmt"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/unreal"
)

// addTerrain adds the terrain of map m (tile t), if it has one: its grid
// batch and its height field. Port of UE2-Studio's add_terrain and
// TerrainGrid::new, except that a broken TerrainScale is placed by
// MapX/MapY instead of being dropped (UE2-Studio draws no terrain then).
// A terrain whose heightmap is absent (no TerrainMap, or a package the
// client lacks) becomes a warning, as in UE2-Studio.
func (s *Scene) addTerrain(c *l2pkg.Client, m *l2pkg.Package, t Tile) error {
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
	hm, err := info.Heightmap(c, m)
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
		Batch:          len(s.Batches),
		Width:          w,
		Height:         h,
		Heights:        heights,
		Position:       vec(info.Position(w, h)),
		Scale:          vec(info.Scale()),
		QuadVisibility: info.QuadVisibilityBitmap,
		EdgeTurn:       info.EdgeTurnBitmap,
		FallbackScale:  info.BrokenScale(),
	}
	b := Batch{Mode: Opaque, Vertices: make([]Vertex, 0, w*h), Bounds: geom.EmptyBox()}
	for y := range h {
		for x := range w {
			p := ter.Vertex(x, y)
			b.Vertices = append(b.Vertices, Vertex{Pos: p})
			b.Bounds.Include(p)
		}
	}
	for y := range h - 1 {
		for x := range w - 1 {
			q := x + y*w // stride is the vertex width, not w-1
			if !bit(ter.QuadVisibility, q) {
				continue
			}
			a, bb := uint32(q), uint32(q+1)
			cc, d := uint32(q+1+w), uint32(q+w)
			if !bit(ter.EdgeTurn, q) {
				b.Indices = append(b.Indices, a, bb, cc, a, cc, d)
			} else {
				b.Indices = append(b.Indices, d, a, bb, d, bb, cc)
			}
		}
	}
	ter.Bounds = b.Bounds
	s.Batches = append(s.Batches, b)
	s.Terrains = append(s.Terrains, ter)
	return nil
}

// Vertex is the world position of grid sample (x, y).
func (t *Terrain) Vertex(x, y int) geom.Vec3 {
	g := geom.Vec3{X: float32(x), Y: float32(y), Z: float32(t.Heights[y*t.Width+x])}
	return g.Mul(t.Scale).Add(t.Position)
}

// bit reads bit i of an LSB-first bitmap; past its end reads false, since
// Unreal omits trailing all-zero bytes.
func bit(b []byte, i int) bool {
	return i/8 < len(b) && b[i/8]&(1<<(i%8)) != 0
}

func vec(v [3]float32) geom.Vec3 { return geom.Vec3{X: v[0], Y: v[1], Z: v[2]} }
