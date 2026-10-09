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
// -1500] that the whole terrain lies farther than GroundReach above: no
// BSP or mesh floor reaches the range either, so the terrain counts by
// the fallback of the layer rule. Catches a fit taken from the ground
// under the vertices (the old "Recalcular pelo chão"), terrain left out
// with nothing else to fit to (no fallback), or a top rounded down that
// leaves less than the margin above the peak.
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
	if in, above, below := p.Histogram().Split(-1e5, 1e5, float64(zmin), float64(zmax), r.Terrain); !near(in, 300*300) || above != 0 || below != 0 {
		t.Errorf("histogram split %v/%v/%v, want 90000 inside only", in, above, below)
	}
}

// Over flat terrain at 0, a tower's BSP floors stand 600 apart from 856
// up to its roof at 3256, 3000 over the range [-256, 256]. One fit from
// that range takes the floor at 856, the only one within GroundReach, and
// tops at 1112; the roof and the floors between are left out as layers.
// Catches a fit that chains through stacked floors each within reach of
// the last (fitting again from its own range until nothing more counts),
// which pulls the tower's roof (spec D5: "o telhado de uma torre não
// estica a faixa").
func TestOneFitDoesNotClimbATowerToItsRoof(t *testing.T) {
	f := grid(0)
	for z := float32(856); z <= 3256; z += 600 {
		f = append(f, bspQuad(0, 0, 100, 300, z)...)
	}
	zmin, zmax, g := coverage.Measure(f, square(0, 0, 300, 300), nil).Fit(-256, 256, 256, coverage.BothSides)
	if zmin != -256 || zmax != 1112 || g.Max.Z != 856 {
		t.Errorf("fitted range %d … %d over floor up to %v, want -256 … 1112 over the floor at 856", zmin, zmax, g.Max.Z)
	}
	if len(g.Others) != 4 || g.Others[3].Low != 3256 {
		t.Errorf("other layers %v, want the 3 floors and the roof at 3256", g.Others)
	}
}

// A hill under a ban whose range holds it does not set the top fitted
// over flat floor at 0: 256, the margin over the flat floor, not 656 over
// the hill's apex, which it is without the ban. Catches a fit that counts
// excluded floor, so "Topo ao chão" lifts the top to a peak the zone cuts
// out.
func TestExcludedHillDoesNotSetTheFittedTop(t *testing.T) {
	o := square(0, 0, 1000, 1000)
	if _, zmax, _ := coverage.Measure(hillOnFlat(), o, []coverage.Ban{hillBan}).Fit(-100, 100, 256, coverage.TopSide); zmax != 256 {
		t.Errorf("top fitted under the ban %d, want 256", zmax)
	}
	if _, zmax, _ := coverage.Measure(hillOnFlat(), o, nil).Fit(-100, 100, 256, coverage.TopSide); zmax != 656 {
		t.Errorf("top fitted without the ban %d, want 656", zmax)
	}
}

// A tower's BSP floor at 15 000 over flat terrain at 0, fitted from
// [14744, 15256], keeps 14744 … 15256: the terrain lies far below the
// range while the tower's floor reaches it, so the terrain is another
// layer, in Other and listed at its Z, with no floor below the range and
// no warning. Catches the old rule, where the terrain always counts and
// pulls a zone drawn on a tower's top down to the ground under it.
func TestTerrainUnderATowerIsAnotherLayer(t *testing.T) {
	f := append(grid(0), bspQuad(0, 0, 300, 300, 15000)...)
	p := coverage.Measure(f, square(0, 0, 300, 300), nil)
	zmin, zmax, _ := p.Fit(14744, 15256, 256, coverage.BothSides)
	if zmin != 14744 || zmax != 15256 {
		t.Fatalf("fitted range %d … %d, want 14744 … 15256", zmin, zmax)
	}
	r := p.Classify(float64(zmin), float64(zmax))
	if r.Below != 0 || r.GroundMin.Z != 15000 || len(r.Warnings) != 0 {
		t.Errorf("below %v, lowest floor %v, warnings %v; want 0, z 15000, none", r.Below, r.GroundMin, kinds(r.Warnings))
	}
	if !near(r.Other, 300*300) || len(r.Others) != 1 || r.Others[0].Low != 0 || r.Others[0].High != 0 {
		t.Errorf("other layers %v over %v units², want the terrain at 0 over 90000", r.Others, r.Other)
	}
}

// A cave's BSP floor at −3000 under flat terrain at 0, fitted from the
// cave's range [−3256, −2744], keeps its top at −2744: the terrain, 2744
// over that top, is another layer. Catches the old rule, where the
// terrain over a cave stretches its zone's top up to the surface.
func TestTerrainOverACaveDoesNotLiftTheTop(t *testing.T) {
	f := append(grid(0), bspQuad(0, 0, 300, 300, -3000)...)
	zmin, zmax, g := coverage.Measure(f, square(0, 0, 300, 300), nil).Fit(-3256, -2744, 256, coverage.BothSides)
	if zmin != -3256 || zmax != -2744 || g.Max.Z != -3000 {
		t.Errorf("fitted range %d … %d over floor up to %v, want -3256 … -2744 over the cave at -3000", zmin, zmax, g.Max.Z)
	}
}

// Under a square whose corners lie on terrain at 0, a terrain pyramid
// climbs through a ring at 1500 to a peak at 3000, with a BSP slab at 100
// in a corner. Fitted from [−256, 256], the top is 3256, over the peak:
// the corners bring the terrain within reach, so all of it counts, though
// its triangles between the ring and the peak lie wholly farther than
// GroundReach over the range. Catches terrain judged piece by piece, which
// tops the hill at its ring (1756), instead of as one block.
func TestHillTerrainCountsAsOneBlock(t *testing.T) {
	a, b, c, d := v(0, 0, 0), v(1000, 0, 0), v(1000, 1000, 0), v(0, 1000, 0)
	ra, rb, rc, rd := v(250, 250, 1500), v(750, 250, 1500), v(750, 750, 1500), v(250, 750, 1500)
	peak := v(500, 500, 3000)
	f := floor{
		tri(a, b, rb), tri(a, rb, ra), tri(b, c, rc), tri(b, rc, rb),
		tri(c, d, rd), tri(c, rd, rc), tri(d, a, ra), tri(d, ra, rd),
		tri(ra, rb, peak), tri(rb, rc, peak), tri(rc, rd, peak), tri(rd, ra, peak),
	}
	f = append(f, bspQuad(0, 0, 100, 100, 100)...)
	if _, zmax, _ := coverage.Measure(f, square(0, 0, 1000, 1000), nil).Fit(-256, 256, 256, coverage.BothSides); zmax != 3256 {
		t.Errorf("fitted top %d, want 3256 over the peak", zmax)
	}
}
