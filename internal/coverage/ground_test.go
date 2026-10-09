package coverage_test

import (
	"testing"

	"zonebuilder/internal/coverage"
)

// Over flat terrain at 0 with a range [-256, 256], a mezzanine 200 above
// the top pulls the top up to it, while a roof 3000 above the top does
// not and is the 1 layer left out, at its Z. Catches a rule that lets
// every BSP or mesh floor pull the range (a roof or a tree canopy
// stretches a street zone), or one that drops BSP and mesh altogether.
func TestFarRoofLeftOutNearMezzaninePulls(t *testing.T) {
	f := append(grid(0), bspQuad(0, 0, 100, 300, 256+200)...)
	f = append(f, bspQuad(200, 0, 300, 300, 256+3000)...)
	g := coverage.Measure(f, square(0, 0, 300, 300)).Ground(-256, 256)
	if !g.Measured {
		t.Fatal("no ground counted under the square")
	}
	if g.Max.Z != 456 || g.Min.Z != 0 {
		t.Errorf("ground %v … %v, want 0 … 456 (the mezzanine)", g.Min.Z, g.Max.Z)
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
	g := coverage.Measure(f, square(0, 0, 1000, 1000)).Ground(-2000, -1500)
	if !g.Measured {
		t.Fatal("no ground counted under the square")
	}
	if zmin, zmax := g.Range(256); zmin != -256 || zmax != 2257 {
		t.Errorf("fitted range %d … %d, want -256 … 2257", zmin, zmax)
	}
}
