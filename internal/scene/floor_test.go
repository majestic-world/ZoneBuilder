package scene_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
)

// On Giran (22_22), at 1000 random columns where a vertical pick from above
// lands on the terrain, the highest floor triangle World.Floor gives under
// the column sits at the Z of that pick, within 0.01. Catches a floor that
// splits a quad on the wrong diagonal (EdgeTurn), keeps invisible quads,
// reads the heights off by a row or a column, or forgets ToServer.
func TestFloorTopMatchesTheVerticalPick(t *testing.T) {
	s := loadTile(t, "22_22")
	w := scene.NewWorld(geom.Vec3{})
	w.Add(s)
	w.HideMeshes = true
	b := s.Terrains[0].Bounds
	rng := rand.New(rand.NewPCG(22, 22))
	checked := 0
	for tries := 0; checked < 1000 && tries < 20000; tries++ {
		x := b.Min.X + rng.Float32()*(b.Max.X-b.Min.X)
		y := b.Min.Y + rng.Float32()*(b.Max.Y-b.Min.Y)
		h, ok := w.Pick(down(x, y))
		if !ok || h.Surface != scene.SurfaceTerrain {
			continue
		}
		checked++
		top, found := math.Inf(-1), false
		box := geom.Box{Min: geom.Vec3{X: x - 1, Y: y - 1}, Max: geom.Vec3{X: x + 1, Y: y + 1}}
		w.Floor(box, func(f scene.FloorTriangle) {
			if z, ok := zAt(f, float64(x), float64(y)); ok && z > top {
				top, found = z, true
			}
		})
		if !found {
			t.Errorf("column %v %v: pick hit terrain at z %v, floor has no triangle there", x, y, h.Pos.Z)
			continue
		}
		if d := math.Abs(top - float64(h.Pos.Z)); d > 0.01 {
			t.Errorf("column %v %v: floor top z %.4f, pick z %.4f (Δ %.4f)", x, y, top, h.Pos.Z, d)
		}
	}
	if checked < 1000 {
		t.Fatalf("only %d columns over the terrain", checked)
	}
}

// zAt is the Z of triangle f at column (x, y), false when the column
// misses it.
func zAt(f scene.FloorTriangle, x, y float64) (float64, bool) {
	ax, ay, az := float64(f.A.X), float64(f.A.Y), float64(f.A.Z)
	bx, by, bz := float64(f.B.X), float64(f.B.Y), float64(f.B.Z)
	cx, cy, cz := float64(f.C.X), float64(f.C.Y), float64(f.C.Z)
	det := (bx-ax)*(cy-ay) - (cx-ax)*(by-ay)
	if det == 0 {
		return 0, false
	}
	u := ((x-ax)*(cy-ay) - (cx-ax)*(y-ay)) / det
	v := ((bx-ax)*(y-ay) - (x-ax)*(by-ay)) / det
	const slack = 1e-9
	if u < -slack || v < -slack || u+v > 1+slack {
		return 0, false
	}
	return az + u*(bz-az) + v*(cz-az), true
}
