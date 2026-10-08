package scene

import (
	"fmt"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/unreal"
)

// BSPSurface is one drawn surface of a map's Level.Model: the fans of
// every node on it, Indices[First:First+Count] of Batches[Batch].
type BSPSurface struct {
	Tile Tile
	// Index is the surface's index in the Model's Surfs.
	Index int
	// Material is the surface's material, an object reference in the map
	// package's index space (0 = none), resolved by the textured renderer.
	Material  int32
	PolyFlags uint32
	Batch     int
	First     int
	Count     int
	// Bounds is the world AABB of the surface's vertices.
	Bounds geom.Box
}

// regionSpan is UE2-Studio's REGION_TILE_SPAN, the unit of the region
// filters.
const regionSpan = TileSpan

// addBSP adds the visible surfaces of map m's Level.Model as one batch.
// footprint is the tile's terrain footprint (world X/Y) or nil when the map
// has no terrain. Port of UE2-Studio's add_level_surfaces and
// Model::visual_surfaces: invisible, portal and backdrop surfaces are
// skipped, every node polygon is fanned (0, i-1, i), and a surface wider
// than two tiles or centred more than a tile off the terrain is dropped.
func (s *Scene) addBSP(m *l2pkg.Package, t Tile, footprint *geom.Box) error {
	li := unreal.FindLevel(m)
	if li < 0 {
		return fmt.Errorf("o mapa não tem Level")
	}
	level, err := unreal.ReadLevel(m, li)
	if err != nil {
		return err
	}
	if level.Model <= 0 || int(level.Model) > len(m.Exports) {
		return fmt.Errorf("Level.Model %d não é um export local", level.Model)
	}
	model, err := unreal.ReadModel(m, int(level.Model)-1)
	if err != nil {
		return err
	}

	// Group node polygons by surface, in node order.
	type group struct {
		verts []geom.Vec3
		idx   []uint32
	}
	groups := make(map[int]*group)
	var order []int
	err = model.VisiblePolygons(func(p unreal.Polygon) {
		g := groups[p.Surf]
		if g == nil {
			g = &group{}
			groups[p.Surf] = g
		}
		base := uint32(len(g.verts))
		for _, v := range p.Points {
			g.verts = append(g.verts, vec(v))
		}
		for i := 2; i < len(p.Points); i++ {
			g.idx = append(g.idx, base, base+uint32(i-1), base+uint32(i))
		}
	})
	if err != nil {
		return err
	}
	for k := range model.Surfs {
		if groups[k] != nil {
			order = append(order, k)
		}
	}

	batch := len(s.Batches)
	b := Batch{Mode: Opaque, Bounds: geom.EmptyBox()}
	for _, k := range order {
		g := groups[k]
		box := geom.EmptyBox()
		for _, v := range g.verts {
			box.Include(v)
		}
		if outsideRegion(box, footprint) {
			continue
		}
		base := uint32(len(b.Vertices))
		first := len(b.Indices)
		for _, v := range g.verts {
			b.Vertices = append(b.Vertices, Vertex{Pos: v})
		}
		for _, i := range g.idx {
			b.Indices = append(b.Indices, base+i)
		}
		b.Bounds.Union(box)
		surf := model.Surfs[k]
		s.BSPSurfaces = append(s.BSPSurfaces, BSPSurface{
			Tile: t, Index: k, Material: surf.Material, PolyFlags: surf.PolyFlags,
			Batch: batch, First: first, Count: len(g.idx), Bounds: box,
		})
		s.addPickable(SurfaceBSP, batch, first, len(g.idx), box)
	}
	if len(b.Indices) > 0 {
		s.Batches = append(s.Batches, b)
	}
	return nil
}

// outsideRegion reports a surface (BSP surface or mesh section, by its world
// box) that the region filters drop: wider than two tiles on X or Y (zone
// backdrop sheets, UE2-Studio exceeds_region_tile), or centred more than a
// tile outside the terrain's footprint (props parked far from the map,
// is_off_map; only when the map has a terrain, footprint non-nil).
func outsideRegion(box geom.Box, footprint *geom.Box) bool {
	size := box.Size()
	if max(size.X, size.Y) > 2*regionSpan {
		return true
	}
	if footprint == nil {
		return false
	}
	c := box.Center()
	return c.X < footprint.Min.X-regionSpan || c.X > footprint.Max.X+regionSpan ||
		c.Y < footprint.Min.Y-regionSpan || c.Y > footprint.Max.Y+regionSpan
}
