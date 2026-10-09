package coverage_test

import (
	"testing"

	"zonebuilder/internal/coverage"
)

// Over flat terrain at 0, a range fitted from [-256, 256] with a margin of
// 256 takes in a mezzanine 200 above that top, ending at 712, while a roof
// 3000 above that top is the 1 layer left out, at its Z. Catches a rule
// that lets every BSP or mesh floor pull the range (a roof or a tree
// canopy stretches a street zone), or one that drops BSP and mesh
// altogether.
func TestFarRoofLeftOutNearMezzaninePulls(t *testing.T) {
	f := append(grid(0), bspQuad(0, 0, 100, 300, 256+200)...)
	f = append(f, bspQuad(200, 0, 300, 300, 256+3000)...)
	zmin, zmax, g := coverage.Measure(f, square(0, 0, 300, 300), nil).Fit(-256, 256, 256, coverage.BothSides)
	if !g.Measured {
		t.Fatal("no ground counted under the square")
	}
	if g.Max.Z != 456 || g.Min.Z != 0 || zmin != -256 || zmax != 712 {
		t.Errorf("ground %v … %v fitted to %d … %d, want 0 … 456 (the mezzanine) fitted to -256 … 712", g.Min.Z, g.Max.Z, zmin, zmax)
	}
	if len(g.Others) != 1 || g.Others[0].Low != 3256 || g.Others[0].High != 3256 {
		t.Errorf("other layers %v, want the roof at 3256 alone", g.Others)
	}
}

// Under a square whose corners lie on the ground at 0, a terrain peak at
// 2000.5 far from every corner sets the top fitted with a margin of 256:
// 2257, the peak rounded up plus the margin, even from a range [-2000,
// -1500] that the whole terrain lies farther than GroundReach above.
// Catches a fit taken from the ground under the vertices (the old
// "Recalcular pelo chão"), terrain left out by the reach that only BSP and
// mesh obey, or a top rounded down that leaves less than the margin above
// the peak.
func TestPeakFarFromTheVerticesSetsTheTop(t *testing.T) {
	apex := v(437, 611, 2000.5)
	a, b, c, d := v(0, 0, 0), v(1000, 0, 0), v(1000, 1000, 0), v(0, 1000, 0)
	f := floor{tri(a, b, apex), tri(b, c, apex), tri(c, d, apex), tri(d, a, apex)}
	zmin, zmax, g := coverage.Measure(f, square(0, 0, 1000, 1000), nil).Fit(-2000, -1500, 256, coverage.BothSides)
	if !g.Measured {
		t.Fatal("no ground counted under the square")
	}
	if zmin != -256 || zmax != 2257 {
		t.Errorf("fitted range %d … %d, want -256 … 2257", zmin, zmax)
	}
}

// Over flat terrain at 0 with a BSP roof at 3256 over a third of it, the
// range fitted from [-256, 256] is [-256, 256], and classified by it the
// roof is another layer: its 30 000 units² in Other, listed at its Z, and
// none of it above the top, in the highest floor, in the warnings or in
// the ruler's histogram. Catches a report that counts the layers the fit
// left out (spec D5, risks): "chão acima do topo" and a red pin on a roof
// or a floating island right after "Recalcular pelo chão".
func TestFarRoofIsAnotherLayerOfTheFittedRange(t *testing.T) {
	f := append(grid(0), bspQuad(200, 0, 300, 300, 3256)...)
	p := coverage.Measure(f, square(0, 0, 300, 300), nil)
	zmin, zmax, _ := p.Fit(-256, 256, 256, coverage.BothSides)
	if zmin != -256 || zmax != 256 {
		t.Fatalf("fitted range %d … %d, want -256 … 256", zmin, zmax)
	}
	r := p.Classify(float64(zmin), float64(zmax))
	if r.Above != 0 || !near(r.Other, 100*300) || !near(r.Inside, 300*300) {
		t.Errorf("above %v, other layers %v, inside %v; want 0, 30000, 90000", r.Above, r.Other, r.Inside)
	}
	if r.GroundMax.Z != 0 || r.TopClearance != 256 || len(r.Warnings) != 0 {
		t.Errorf("highest floor %v, top clearance %v, warnings %v; want z 0, 256, none", r.GroundMax, r.TopClearance, kinds(r.Warnings))
	}
	if len(r.Others) != 1 || r.Others[0].Low != 3256 {
		t.Errorf("other layers %v, want the roof at 3256 alone", r.Others)
	}
	if in, above, below := p.Histogram().Split(-1e5, 1e5, float64(zmin), float64(zmax)); !near(in, 300*300) || above != 0 || below != 0 {
		t.Errorf("histogram split %v/%v/%v, want 90000 inside only", in, above, below)
	}
}

// Under a square over a terrain hill rising to 600, with a BSP floor at
// 1500 over a corner of it, a range fitted from [-256, 256] tops at 856
// at first; from there the floor at 1500 lies within GroundReach, so the
// fit takes it too and tops at 1756, and the range it ends with reports
// no floor above its top. Catches a fit judged only from the range before
// it, whose own range then reaches floor above its top ("chão acima do
// topo" right after "Recalcular pelo chão").
func TestFitTakesTheFloorItsOwnRangeReaches(t *testing.T) {
	apex := v(500, 500, 600)
	a, b, c, d := v(0, 0, 0), v(1000, 0, 0), v(1000, 1000, 0), v(0, 1000, 0)
	f := append(floor{tri(a, b, apex), tri(b, c, apex), tri(c, d, apex), tri(d, a, apex)}, bspQuad(100, 100, 200, 200, 1500)...)
	p := coverage.Measure(f, square(0, 0, 1000, 1000), nil)
	zmin, zmax, g := p.Fit(-256, 256, 256, coverage.BothSides)
	if zmin != -256 || zmax != 1756 || len(g.Others) != 0 {
		t.Fatalf("fitted range %d … %d leaving out %v, want -256 … 1756 leaving out nothing", zmin, zmax, g.Others)
	}
	if r := p.Classify(float64(zmin), float64(zmax)); r.Above != 0 || r.Other != 0 || len(r.Warnings) != 0 {
		t.Errorf("above %v, other layers %v, warnings %v; want 0, 0, none", r.Above, r.Other, kinds(r.Warnings))
	}
}
