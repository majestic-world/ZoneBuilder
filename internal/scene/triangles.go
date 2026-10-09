package scene

import (
	"math"

	"zonebuilder/internal/geom"
)

// triangleSet is a run of batch triangles a ray can hit, picked as one unit
// behind its bounding box: a placed mesh actor, a BSP surface. Its
// triangles are Indices[First : First+Count] of Scene.Batches[Batch].
type triangleSet struct {
	Surface      Surface
	Batch        int
	First, Count int
	// Bounds is the world AABB of the set's vertices.
	Bounds geom.Box
	// Mirrored marks a mesh actor placed with a mirroring scale, whose
	// triangles wind the other way round: their right-hand normal faces
	// in instead of out.
	Mirrored bool
	// Normal is the plane normal a BSP surface's triangles share, zero for
	// a mesh: a triangle's facing then comes from its winding.
	Normal geom.Vec3
}

// addPickable registers set, a run of indices of one batch, as pickable;
// an empty set is dropped.
func (s *Scene) addPickable(set triangleSet) {
	if set.Count == 0 {
		return
	}
	s.pickables = append(s.pickables, set)
}

// pickTriangles improves best with the nearest triangle of every pickable
// set r meets closer than best.Distance, the mesh actors' sets left out
// when noMeshes is set. A set whose box the ray enters beyond the best hit
// so far is skipped (UE2-Studio's ray_box_entry early-out). r.Dir must be
// unit length.
func (s *Scene) pickTriangles(r Ray, best *Hit, noMeshes bool) {
	for i := range s.pickables {
		set := &s.pickables[i]
		if noMeshes && set.Surface == SurfaceMesh {
			continue
		}
		if entry, ok := rayBoxEntry(r, set.Bounds); !ok || entry > best.Distance {
			continue
		}
		// Test in a frame local to the box, to keep float precision away
		// from the large world offsets.
		base := set.Bounds.Min
		o := r.Origin.Sub(base)
		b := &s.Batches[set.Batch]
		idx := b.Indices[set.First : set.First+set.Count]
		for k := 0; k+2 < len(idx); k += 3 {
			d, ok := rayTriangle(o, r.Dir,
				b.Vertices[idx[k]].Pos.Sub(base),
				b.Vertices[idx[k+1]].Pos.Sub(base),
				b.Vertices[idx[k+2]].Pos.Sub(base))
			if ok && d < best.Distance {
				*best = Hit{Pos: ToServer(r.Origin.Add(r.Dir.Scale(d))), Distance: d, Surface: set.Surface, Water: b.Mode == Water}
			}
		}
	}
}

// rayBoxEntry is the distance along r at which it enters box b, clamped to
// 0 when the origin is inside; false when the ray misses the box (slab
// test, UE2-Studio's ray_box_entry).
func rayBoxEntry(r Ray, b geom.Box) (float32, bool) {
	if b.Empty() {
		return 0, false
	}
	enter, exit := float32(0), float32(math.Inf(1))
	for a := range 3 {
		o, d := r.Origin.Axis(a), r.Dir.Axis(a)
		lo, hi := b.Min.Axis(a), b.Max.Axis(a)
		if d == 0 {
			if o < lo || o > hi {
				return 0, false
			}
			continue
		}
		t0, t1 := (lo-o)/d, (hi-o)/d
		enter = max(enter, min(t0, t1))
		exit = min(exit, max(t0, t1))
		if enter > exit {
			return 0, false
		}
	}
	return enter, true
}
