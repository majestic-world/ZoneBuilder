package scene

import (
	"math"

	"zonebuilder/internal/geom"
)

// FloorTriangle is one triangle of floor, the surface a character can
// stand on, in server coordinates (ToServer of the geometry).
type FloorTriangle struct {
	A, B, C geom.Vec3
	Surface Surface
}

// Floor calls fn once with every floor triangle of the world whose cell
// or set box meets the X/Y of box (server coordinates; box's Z is not
// looked at), so a few triangles just outside box may come too. The floor
// is what is drawn and picked: every visible terrain quad, as its 2
// triangles split on the EdgeTurn diagonal, and nothing of an invisible
// quad; plus every BSP or static mesh triangle facing up (normal.z ≥
// floorNormalZ), the meshes left out while HideMeshes is set. Floor only
// reads the scenes: it may run off the event loop on a World that the
// loop does not Add to or Remove from meanwhile.
func (w *World) Floor(box geom.Box, fn func(FloorTriangle)) {
	for _, s := range w.scenes {
		for i := range s.Terrains {
			s.Terrains[i].floor(box, fn)
		}
		s.floor(box, fn, w.HideMeshes)
	}
}

// floorNormalZ is the least Z of a unit normal that a BSP or mesh face can
// stand on (spec "Chão"): a slope up to 60°.
const floorNormalZ = 0.5

// floor is World.Floor over the scene's pickable sets: the upward faces
// of the sets whose box meets box, the meshes' left out when noMeshes is
// set. A BSP surface faces where its plane normal does, every triangle
// alike (a sliver of its fan can wind either way); a mesh triangle where
// its right-hand normal does (out of the geometry, the maps' winding), or
// the opposite for a mirrored actor.
func (s *Scene) floor(box geom.Box, fn func(FloorTriangle), noMeshes bool) {
	for i := range s.pickables {
		set := &s.pickables[i]
		if noMeshes && set.Surface == SurfaceMesh {
			continue
		}
		if set.Bounds.Empty() || set.Bounds.Max.X < box.Min.X || set.Bounds.Min.X > box.Max.X ||
			set.Bounds.Max.Y < box.Min.Y || set.Bounds.Min.Y > box.Max.Y {
			continue
		}
		shared := set.Normal != geom.Vec3{}
		if shared && !upward(set.Normal) {
			continue
		}
		b := &s.Batches[set.Batch]
		idx := b.Indices[set.First : set.First+set.Count]
		for k := 0; k+2 < len(idx); k += 3 {
			p, q, r := b.Vertices[idx[k]].Pos, b.Vertices[idx[k+1]].Pos, b.Vertices[idx[k+2]].Pos
			if !shared {
				n := q.Sub(p).Cross(r.Sub(p))
				if set.Mirrored {
					n = n.Scale(-1)
				}
				if !upward(n) {
					continue
				}
			}
			fn(FloorTriangle{A: ToServer(p), B: ToServer(q), C: ToServer(r), Surface: set.Surface})
		}
	}
}

// upward reports a face of normal n (any length) as one to stand on.
func upward(n geom.Vec3) bool { return n.Z > 0 && n.Z >= floorNormalZ*n.Length() }

// floor is World.Floor over the terrain's grid: the cells under box.
func (t *Terrain) floor(box geom.Box, fn func(FloorTriangle)) {
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
				fn(FloorTriangle{
					A:       ToServer(t.at(tris[k])),
					B:       ToServer(t.at(tris[k+1])),
					C:       ToServer(t.at(tris[k+2])),
					Surface: SurfaceTerrain,
				})
			}
		}
	}
}

// at is the world position of batch vertex k.
func (t *Terrain) at(k uint32) geom.Vec3 {
	return t.Vertex(int(k)%t.Width, int(k)/t.Width)
}

// cellSpan is the range of cells, of n along an axis that starts at origin
// with cells of size step, under [lo, hi]; empty (first > last) when none
// is.
func cellSpan(lo, hi, origin, step float32, n int) (first, last int) {
	a, b := (lo-origin)/step, (hi-origin)/step
	if step < 0 {
		a, b = b, a
	}
	first = max(int(math.Floor(float64(a))), 0)
	last = min(int(math.Floor(float64(b))), n-1)
	return first, last
}
