package coverage_test

import (
	"os"
	"testing"

	"zonebuilder/internal/coverage"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
)

// An outline drawn on the courtyard of the Giran castle (tile 22_22), a
// blocking static mesh at 3494.5 over the castle's halls (floors from
// 2246 up, and a vault at 3348), fitted from its clicked points' range
// [3238, 3750] keeps its floor within a layer gap of 3238, and classified
// by it has no floor below. The real client, named by ZB_CLIENT, is read;
// the test skips without it. Catches the halls under the courtyard pulling
// the zone down (1990 before, and 823 after "Piso ao chão").
func TestGiranCourtyardRangeStaysOnTheCourtyard(t *testing.T) {
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
	o := coverage.Outline{
		{X: 89520, Y: 160596}, {X: 89552, Y: 160532}, {X: 89936, Y: 159956}, {X: 89968, Y: 159924},
		{X: 90000, Y: 159924}, {X: 90064, Y: 159956}, {X: 90288, Y: 160180}, {X: 90352, Y: 160308},
		{X: 90384, Y: 160436}, {X: 90384, Y: 160660}, {X: 90256, Y: 160948}, {X: 90064, Y: 161140},
		{X: 89936, Y: 161204}, {X: 89808, Y: 161236}, {X: 89712, Y: 161236}, {X: 89648, Y: 161204},
		{X: 89616, Y: 161172}, {X: 89520, Y: 160692},
	}
	p := coverage.Measure(w, o, nil)
	zmin, zmax, _ := p.Fit(3494-256, 3494+256, 256, coverage.BothSides)
	if zmin < 3238-coverage.LayerGap {
		t.Fatalf("fitted floor %d, want at least %d: the courtyard's", zmin, 3238-coverage.LayerGap)
	}
	if r := p.Classify(float64(zmin), float64(zmax)); r.Below != 0 {
		t.Errorf("classified by %d … %d: %v units² of floor below, want none", zmin, zmax, r.Below)
	}
	if again, _, _ := p.Fit(zmin, zmax, 256, coverage.FloorSide); again != zmin {
		t.Errorf("piso ao chão moved the floor from %d to %d", zmin, again)
	}
}
