package scene

import "zonebuilder/internal/geom"

// Triangle is one triangle of the world's geometry, in server coordinates
// (ToServer of the geometry).
type Triangle struct {
	A, B, C geom.Vec3
	// Normal is the face's unit normal, out of the geometry: up for
	// terrain; the surface's plane normal for BSP, every triangle of the
	// surface alike (a sliver of its fan can wind either way); the
	// right-hand normal for a mesh (the maps' winding), or the opposite
	// for a mirrored actor. Zero for a degenerate mesh triangle.
	Normal  geom.Vec3
	Surface Surface
	// Water marks a triangle drawn in the Water pass: a water sheet, BSP
	// or mesh.
	Water bool
	// Blocks marks a triangle a walking player collides with: all terrain,
	// BSP without PolyFlags NotSolid (0x8), and the meshes whose actor's
	// collision blocks (unreal.Collision.Blocks over the class defaults).
	Blocks bool
}

// Geometry calls tri once with every triangle of the world whose terrain
// cell or set box meets the X/Y of box (server coordinates; box's Z is not
// looked at), so a few triangles just outside box may come too: every
// visible terrain quad, as its 2 triangles split on the EdgeTurn diagonal,
// and nothing of an invisible quad; every BSP triangle; every static mesh
// triangle, the meshes left out while HideMeshes is set. It then calls
// water once with every water volume whose bounds meet the X/Y of box and
// whose faces close a solid, moved to server coordinates (Faces, Planes
// and Bounds). Either callback may be nil.
//
// The order is the scenes' (the tiles' load order), then within a scene
// terrain, BSP and meshes: a caller whose result must not depend on the
// load order cannot depend on it. Geometry only reads the scenes: it may
// run off the event loop on a World that the loop does not Add to or
// Remove from meanwhile.
func (w *World) Geometry(box geom.Box, tri func(Triangle), water func(WaterVolume)) {
	if tri != nil {
		for _, s := range w.scenes {
			for i := range s.Terrains {
				s.Terrains[i].geometry(box, tri)
			}
			s.geometry(box, tri, w.HideMeshes)
		}
	}
	if water != nil {
		for _, s := range w.scenes {
			for i := range s.WaterVolumes {
				v := &s.WaterVolumes[i]
				if len(v.Planes) >= minSolidFaces && overlapsXY(v.Bounds, box) {
					water(v.server())
				}
			}
		}
	}
}

// Collision visits solid terrain, BSP and blocking actors regardless of
// rendering visibility. Geometry, Floor and Pick keep their visual semantics.
func (w *World) Collision(box geom.Box, tri func(Triangle)) {
	if tri == nil {
		return
	}
	for _, s := range w.scenes {
		for i := range s.Terrains {
			s.Terrains[i].geometry(box, tri)
		}
		s.geometry(box, func(t Triangle) {
			if t.Blocks && t.Surface != SurfaceBSP {
				tri(t)
			}
		}, false)
		for _, t := range s.hiddenCollision {
			b := geom.EmptyBox()
			b.Include(t.A)
			b.Include(t.B)
			b.Include(t.C)
			if overlapsXY(b, box) {
				tri(t)
			}
		}
	}
}

// geometry is World.Geometry over the scene's pickable sets whose box
// meets box, the meshes' left out when noMeshes is set.
func (s *Scene) geometry(box geom.Box, fn func(Triangle), noMeshes bool) {
	for i := range s.pickables {
		set := &s.pickables[i]
		if noMeshes && set.Surface == SurfaceMesh {
			continue
		}
		if set.Bounds.Empty() || !overlapsXY(set.Bounds, box) {
			continue
		}
		shared := set.Normal.Normalize()
		b := &s.Batches[set.Batch]
		water := b.Mode == Water
		idx := b.Indices[set.First : set.First+set.Count]
		for k := 0; k+2 < len(idx); k += 3 {
			p, q, r := b.Vertices[idx[k]].Pos, b.Vertices[idx[k+1]].Pos, b.Vertices[idx[k+2]].Pos
			n := shared
			if set.Normal == (geom.Vec3{}) {
				n = q.Sub(p).Cross(r.Sub(p)).Normalize()
				if set.Mirrored {
					n = n.Scale(-1)
				}
			}
			fn(Triangle{A: ToServer(p), B: ToServer(q), C: ToServer(r), Normal: n, Surface: set.Surface, Water: water, Blocks: set.Blocks})
		}
	}
}

// geometry is World.Geometry over the terrain's grid: the cells under box.
func (t *Terrain) geometry(box geom.Box, fn func(Triangle)) {
	if t.Scale.X == 0 || t.Scale.Y == 0 || t.Width < 2 || t.Height < 2 {
		return
	}
	x0, x1 := cellSpan(box.Min.X, box.Max.X, t.Position.X, t.Scale.X, t.Width-1)
	y0, y1 := cellSpan(box.Min.Y, box.Max.Y, t.Position.Y, t.Scale.Y, t.Height-1)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			tris, ok := t.cell(x, y)
			if !ok {
				continue
			}
			for k := 0; k < 6; k += 3 {
				a, b, c := t.at(tris[k]), t.at(tris[k+1]), t.at(tris[k+2])
				// A height field faces up whichever way a negative scale
				// winds it.
				n := b.Sub(a).Cross(c.Sub(a)).Normalize()
				if n.Z < 0 {
					n = n.Scale(-1)
				}
				fn(Triangle{A: ToServer(a), B: ToServer(b), C: ToServer(c), Normal: n, Surface: SurfaceTerrain, Blocks: true})
			}
		}
	}
}

// overlapsXY reports whether a and b meet on the X/Y plane.
func overlapsXY(a, b geom.Box) bool {
	return a.Max.X >= b.Min.X && a.Min.X <= b.Max.X && a.Max.Y >= b.Min.Y && a.Min.Y <= b.Max.Y
}

// server is a copy of v moved to server coordinates.
func (v *WaterVolume) server() WaterVolume {
	o := *v
	up := ToServer(geom.Vec3{}).Z
	o.Faces = make([][]geom.Vec3, len(v.Faces))
	for k, f := range v.Faces {
		o.Faces[k] = make([]geom.Vec3, len(f))
		for j, p := range f {
			o.Faces[k][j] = ToServer(p)
		}
	}
	o.Planes = make([]Plane, len(v.Planes))
	for k, p := range v.Planes {
		// N·(p + up·Z) = N·p + up·N.Z.
		o.Planes[k] = Plane{Normal: p.Normal, D: p.D + up*p.Normal.Z}
	}
	o.Bounds = geom.Box{Min: ToServer(v.Bounds.Min), Max: ToServer(v.Bounds.Max)}
	return o
}
