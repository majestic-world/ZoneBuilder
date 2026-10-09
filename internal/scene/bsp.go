package scene

import (
	"fmt"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/texture"
	"zonebuilder/internal/unreal"
)

// BSPSurface is one drawn surface of a map's Level.Model: the fans of
// every node on it, Indices[First:First+Count] of Batches[Batch], the batch
// of its material.
type BSPSurface struct {
	// Index is the surface's index in the Model's Surfs.
	Index     int
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

// addBSP adds the visible surfaces of map m's Level.Model, each to the
// batch its material draws into, with texel UVs divided by the texture's
// size. footprint is the tile's terrain footprint (world X/Y) or nil when
// the map has no terrain. Port of UE2-Studio's add_level_surfaces and
// Model::visual_surfaces: invisible, portal and backdrop surfaces are
// skipped, every node polygon is fanned (0, i-1, i), a surface wider than
// two tiles or centred more than a tile off the terrain is dropped, and an
// opaque material turns Translucent on a PF_Translucent or PF_Modulated
// surface, else Masked on a PF_Masked one.
func (s *Scene) addBSP(ld *loader, m *l2pkg.Package, t Tile, footprint *geom.Box) error {
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

	for _, k := range order {
		g := groups[k]
		box := geom.EmptyBox()
		for _, v := range g.verts {
			box.Include(v)
		}
		if outsideRegion(box, footprint) {
			continue
		}
		surf := model.Surfs[k]
		mat, err := ld.material(m, surf.Material)
		if err != nil {
			return fmt.Errorf("material da superfície %d: %w", k, err)
		}
		mode := mat.mode
		if mode == Opaque && surf.PolyFlags&(unreal.PFTranslucent|unreal.PFModulated) != 0 {
			mode = Translucent
		}
		if mode == Opaque && surf.PolyFlags&unreal.PFMasked != 0 {
			mode = Masked
		}
		uv := surfaceUV(model, surf, mat.texture)
		batch := s.batch(ld, batchKey{tex: mat.texture, mode: mode, opaque: mat.vertexOpacity})
		b := &s.Batches[batch]
		base := uint32(len(b.Vertices))
		first := len(b.Indices)
		for _, v := range g.verts {
			b.Vertices = append(b.Vertices, Vertex{Pos: v, UV: uv(v), Alpha: 1})
		}
		for _, i := range g.idx {
			b.Indices = append(b.Indices, base+i)
		}
		b.Bounds.Union(box)
		s.BSPSurfaces = append(s.BSPSurfaces, BSPSurface{
			Index: k, PolyFlags: surf.PolyFlags,
			Batch: batch, First: first, Count: len(g.idx), Bounds: box,
		})
		s.addPickable(triangleSet{Surface: SurfaceBSP, Batch: batch, First: first, Count: len(g.idx), Bounds: box, Normal: vec(model.Vectors[surf.Normal])})
	}
	return nil
}

// surfaceUV maps a point of BSP surface surf to its texture coordinate:
// texels along the surface's TextureU/TextureV vectors from its Base point,
// divided by the size of tex (UE2-Studio normalize_bsp_uv); left in texels
// when tex is nil, and (0, 0) when the surface's UV basis is out of range.
func surfaceUV(model *unreal.Model, surf unreal.BSPSurf, tex *texture.Texture) func(geom.Vec3) [2]float32 {
	at := func(list [][3]float32, i int32) (geom.Vec3, bool) {
		if i < 0 || int(i) >= len(list) {
			return geom.Vec3{}, false
		}
		return vec(list[i]), true
	}
	base, okB := at(model.Points, surf.Base)
	u, okU := at(model.Vectors, surf.TextureU)
	v, okV := at(model.Vectors, surf.TextureV)
	if !okB || !okU || !okV {
		return func(geom.Vec3) [2]float32 { return [2]float32{} }
	}
	w, h := float32(1), float32(1)
	if tex != nil {
		w, h = float32(max(1, tex.Mips[0].Width)), float32(max(1, tex.Mips[0].Height))
	}
	return func(p geom.Vec3) [2]float32 {
		d := p.Sub(base)
		return [2]float32{d.Dot(u) / w, d.Dot(v) / h}
	}
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
