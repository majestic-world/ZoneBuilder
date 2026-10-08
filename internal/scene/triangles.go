package scene

import (
	"math"

	"zonebuilder/internal/geom"
)

// triangleSet is a pickable run of triangles: Indices[First:First+Count]
// of s.Batches[Batch], with their world AABB for the early-out.
type triangleSet struct {
	Surface             Surface
	Batch, First, Count int
	Bounds              geom.Box
}

func (s *Scene) addPickable(surface Surface, batch, first, count int, bounds geom.Box) {
	s.pickables = append(s.pickables, triangleSet{Surface: surface, Batch: batch, First: first, Count: count, Bounds: bounds})
}

// pickTriangles updates best with the nearest triangle hit of every
// pickable set whose AABB the ray enters before best.Distance. r.Dir must
// be unit. Triangles are tested in a frame relative to the set's Bounds.Min
// to keep float precision at world coordinates.
func (s *Scene) pickTriangles(r Ray, best *Hit) {
	for k := range s.pickables {
		set := &s.pickables[k]
		entry, ok := rayBoxEntry(r, set.Bounds)
		if !ok || entry > best.Distance {
			continue
		}
		b := &s.Batches[set.Batch]
		o := r.Origin.Sub(set.Bounds.Min)
		idx := b.Indices[set.First : set.First+set.Count]
		for i := 0; i+2 < len(idx); i += 3 {
			d, ok := rayTriangle(o, r.Dir,
				b.Vertices[idx[i]].Pos.Sub(set.Bounds.Min),
				b.Vertices[idx[i+1]].Pos.Sub(set.Bounds.Min),
				b.Vertices[idx[i+2]].Pos.Sub(set.Bounds.Min))
			if ok && d < best.Distance {
				*best = Hit{Pos: ToServer(r.Origin.Add(r.Dir.Scale(d))), Distance: d, Surface: set.Surface}
			}
		}
	}
}

// rayBoxEntry is the slab test (UE2-Studio's ray_box_entry): the distance
// at which r enters b, clamped at 0 when it starts inside, false on a miss.
func rayBoxEntry(r Ray, b geom.Box) (float32, bool) {
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
		t1, t2 := (lo-o)/d, (hi-o)/d
		enter = max(enter, min(t1, t2))
		exit = min(exit, max(t1, t2))
		if enter > exit {
			return 0, false
		}
	}
	return enter, true
}
