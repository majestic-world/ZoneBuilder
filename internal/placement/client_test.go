package placement_test

import (
	"math"
	"os"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/placement"
	"zonebuilder/internal/scene"
)

// clip is the convex polygon poly cut to its part with Z ≥ z, or ≤ z
// when below is set.
func clip(poly []geom.Vec3, z float32, below bool) []geom.Vec3 {
	in := func(p geom.Vec3) bool { return p.Z == z || (p.Z > z) != below }
	var out []geom.Vec3
	for k, a := range poly {
		b := poly[(k+1)%len(poly)]
		if in(a) {
			out = append(out, a)
		}
		if in(a) != in(b) {
			s := (z - a.Z) / (b.Z - a.Z)
			out = append(out, a.Add(b.Sub(a).Scale(s)))
		}
	}
	return out
}

// reach is the distance on the X/Y plane from (x, y) to the part of
// triangle a, b, c between heights lo and hi; +Inf when no part of it is.
func reach(a, b, c geom.Vec3, lo, hi float32, x, y float64) float64 {
	poly := clip(clip([]geom.Vec3{a, b, c}, lo, false), hi, true)
	if len(poly) == 0 {
		return math.Inf(1)
	}
	d := math.Inf(1)
	left, right := false, false
	for k, p := range poly {
		q := poly[(k+1)%len(poly)]
		px, py, qx, qy := float64(p.X), float64(p.Y), float64(q.X), float64(q.Y)
		d = math.Min(d, segment(x, y, px, py, qx, qy))
		side := (qx-px)*(y-py) - (qy-py)*(x-px)
		left, right = left || side > 0, right || side < 0
	}
	if len(poly) >= 3 && left != right {
		return 0
	}
	return d
}

// A known area among the trees and rocks south-east of Giran (tile
// 22_22), on terrain, gives points that no static mesh triangle of the
// tile comes within radius + clearance of in their slice, each standing
// on the terrain or BSP floor a vertical pick finds there. The mesh
// triangles come straight from the scene's actors, not through the
// World, and the World is rebased far from the origin. Catches a World
// geometry in the wrong frame (client Z instead of server, or rebased):
// the meshes would sit beside or below where the floor puts the points.
func TestGiranPointsClearEveryMeshOfTheTile(t *testing.T) {
	root := os.Getenv("ZB_CLIENT")
	if root == "" {
		t.Skip("ZB_CLIENT not set: tests against the real client skipped")
	}
	s, err := scene.Load(root, []scene.Tile{{X: 22, Y: 22}})
	if err != nil {
		t.Fatal(err)
	}
	w := scene.NewWorld(geom.Vec3{X: 86500, Y: 162300, Z: -3500})
	w.Add(s)
	r := placement.Request{
		Outline: square(86100, 161800, 87000, 162700),
		ZMin:    -4000, ZMax: -3000,
		Count: 80, Radius: 16, Clearance: 32, Height: 80, Seed: 7,
	}
	got := placement.Distribute(w, r)
	if len(got.Points) < 40 {
		t.Fatalf("%d points, want most of %d in the clearing", len(got.Points), r.Count)
	}
	bare := scene.NewWorld(w.Origin)
	bare.Add(s)
	bare.HideMeshes = true
	// The rule is checked 1 unit inside its limits: the point moved
	// within its cell and was rounded, so its floor may differ from the
	// cell centre's by a little.
	const slack = 1
	keep := r.Radius + r.Clearance - slack
	near := 0
	for _, p := range got.Points {
		x, y, z := float64(p.X), float64(p.Y), float32(p.Z)
		h, ok := bare.Pick(scene.Ray{Origin: scene.FromServer(v(float32(x), float32(y), z+40)), Dir: v(0, 0, -1)})
		if !ok || math.Abs(float64(h.Pos.Z-z)) > 2 {
			t.Errorf("point %v not on the floor (pick %v %v)", p, ok, h.Pos)
		}
		lo, hi := z-placement.SliceBelow+slack, z+float32(r.Height)-slack
		for _, a := range s.Actors {
			if a.Hidden || a.Bounds.Max.X < float32(x-keep-64) || a.Bounds.Min.X > float32(x+keep+64) ||
				a.Bounds.Max.Y < float32(y-keep-64) || a.Bounds.Min.Y > float32(y+keep+64) {
				continue
			}
			for _, sec := range a.Sections {
				if sec.Batch < 0 {
					continue
				}
				b := &s.Batches[sec.Batch]
				idx := b.Indices[sec.First : sec.First+sec.Count]
				for k := 0; k+2 < len(idx); k += 3 {
					ta := scene.ToServer(b.Vertices[idx[k]].Pos)
					tb := scene.ToServer(b.Vertices[idx[k+1]].Pos)
					tc := scene.ToServer(b.Vertices[idx[k+2]].Pos)
					d := reach(ta, tb, tc, lo, hi, x, y)
					if d <= keep {
						t.Fatalf("point %v: mesh %s %.1f away in its slice, want > %v", p, a.Name, d, keep+slack)
					}
					if d < keep+64 {
						near++
					}
				}
			}
		}
	}
	// Without meshes close by the test would prove nothing.
	if near < 20 {
		t.Errorf("only %d mesh triangles within 64 units of the cleared line: pick an area with more meshes", near)
	}
}
