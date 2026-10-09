package coverage_test

import (
	"math"
	"testing"

	"zonebuilder/internal/coverage"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
)

// floor is a synthetic floor: the triangles it holds, all terrain.
type floor []scene.FloorTriangle

func (f floor) Floor(box geom.Box, fn func(scene.FloorTriangle)) {
	for _, t := range f {
		fn(t)
	}
}

func tri(a, b, c geom.Vec3) scene.FloorTriangle {
	return scene.FloorTriangle{A: a, B: b, C: c, Surface: scene.SurfaceTerrain}
}

func v(x, y, z float32) geom.Vec3 { return geom.Vec3{X: x, Y: y, Z: z} }

// square is the outline of the axis-aligned square [x0, x1] × [y0, y1].
func square(x0, y0, x1, y1 float64) coverage.Outline {
	return coverage.Outline{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
}

func near(a, b float64) bool { return math.Abs(a-b) <= 1e-6*max(1, math.Abs(b)) }

// A pyramid whose apex lies inside a square, far from its 4 corners, has
// its apex as the highest ground under the square. Catches a measure that
// samples the ground at the outline's vertices (or on a grid) and misses a
// peak between them.
func TestPyramidApexIsTheHighestGround(t *testing.T) {
	apex := v(437, 611, 500)
	a, b, c, d := v(-500, -500, 0), v(1500, -500, 0), v(1500, 1500, 0), v(-500, 1500, 0)
	f := floor{tri(a, b, apex), tri(b, c, apex), tri(c, d, apex), tri(d, a, apex)}
	r := coverage.Measure(f, square(0, 0, 1000, 1000), nil).Classify(-100, 100)
	if !r.Measured {
		t.Fatal("no ground measured under the square")
	}
	got := r.GroundMax
	if !near(got.X, 437) || !near(got.Y, 611) || !near(got.Z, 500) {
		t.Errorf("highest ground at %v %v %v, want the apex 437 611 500", got.X, got.Y, got.Z)
	}
	if !near(r.TopClearance, 100-500) {
		t.Errorf("top clearance %v, want -400", r.TopClearance)
	}
}

// lShape is the L of the square [0, 200]² without its corner [100, 200]²:
// the arm [0, 200] × [0, 100] and the arm [0, 100] × [100, 200].
var lShape = coverage.Outline{{X: 0, Y: 0}, {X: 200, Y: 0}, {X: 200, Y: 100}, {X: 100, Y: 100}, {X: 100, Y: 200}, {X: 0, Y: 200}}

// A ramp z = x + y under the concave L, with a top at z = 150, has
// 18 750 units² of floor above the top: 10 000 in the arm along X
// (20 000 minus the trapezoid x + y ≤ 150, ∫₀¹⁰⁰ (150 − y) dy =
// 10 000) and 8 750 in the arm along Y (10 000 minus ∫₀⁵⁰ (50 − x) dx =
// 1 250). Its highest floor is 300, at the L's outer corners, not the 400
// of the notch's corner (200, 200) that the ramp's triangles also cover;
// its lowest is 0 at the origin, an outline vertex on a triangle's
// diagonal. Catches a clip that treats the outline as convex (filling the
// notch), or extremes that skip outline vertices on a triangle's edge.
func TestRampUnderConcaveOutlineAboveTopIsExact(t *testing.T) {
	ramp := func(x, y float32) geom.Vec3 { return v(x, y, x+y) }
	a, b, c, d := ramp(-100, -100), ramp(300, -100), ramp(300, 300), ramp(-100, 300)
	f := floor{tri(a, b, c), tri(a, c, d)}
	r := coverage.Measure(f, lShape, nil).Classify(-1000, 150)
	if !near(r.Above, 18750) {
		t.Errorf("area above the top %v, want 18750", r.Above)
	}
	if !near(r.Inside, 30000-18750) {
		t.Errorf("area inside %v, want 11250", r.Inside)
	}
	if !near(r.GroundMax.Z, 300) {
		t.Errorf("highest ground %v %v %v, want z 300 at an outer corner of the L", r.GroundMax.X, r.GroundMax.Y, r.GroundMax.Z)
	}
	if !near(r.GroundMin.Z, 0) {
		t.Errorf("lowest ground z %v, want 0 at the origin", r.GroundMin.Z)
	}
}

// grid is a flat floor at height z of 100-unit quads covering [0, 300]²,
// 2 triangles each, without the quads listed in holes (as {x, y} of the
// quad's low corner over 100).
func grid(z float32, holes ...[2]int) floor {
	var f floor
	for y := range 3 {
	cells:
		for x := range 3 {
			for _, h := range holes {
				if h == [2]int{x, y} {
					continue cells
				}
			}
			x0, y0 := float32(x*100), float32(y*100)
			a, b, c, d := v(x0, y0, z), v(x0+100, y0, z), v(x0+100, y0+100, z), v(x0, y0+100, z)
			f = append(f, tri(a, b, c), tri(a, c, d))
		}
	}
	return f
}

// Under a square whose floor lacks one quad, the missing quad's area is
// counted with no ground and the coverage stays below 100 %, though all
// the floor there is inside the range. Catches a coverage that only
// divides the floor found, so a hole looks covered.
func TestInvisibleQuadIsNoGround(t *testing.T) {
	r := coverage.Measure(grid(50, [2]int{1, 1}), square(50, 50, 250, 250), nil).Classify(0, 100)
	if !near(r.NoGround, 100*100) {
		t.Errorf("area with no ground %v, want 10000", r.NoGround)
	}
	if !near(r.Inside, 200*200-100*100) {
		t.Errorf("area inside %v, want 30000", r.Inside)
	}
	if c := r.Coverage(); !near(c, 0.75) {
		t.Errorf("coverage %v, want 0.75", c)
	}
}

// bspQuad is a flat BSP floor at height z over [x0, x1] × [y0, y1].
func bspQuad(x0, y0, x1, y1, z float32) floor {
	a, b, c, d := v(x0, y0, z), v(x1, y0, z), v(x1, y1, z), v(x0, y1, z)
	return floor{
		{A: a, B: b, C: c, Surface: scene.SurfaceBSP},
		{A: a, B: c, C: d, Surface: scene.SurfaceBSP},
	}
}

// A bridge at z 500 across a square of flat terrain at z 0 makes 2 layers
// under the square, and with a top at 300 its 100 × 300 deck inside the
// square is floor above the top while the terrain under it stays inside:
// the areas add up to more than the square. Catches layers counted per
// surface kind instead of per column, a bridge dropped from the extremes,
// or the terrain under a bridge taken as hidden by it.
func TestBridgeOverTerrainIsASecondLayerAboveTheTop(t *testing.T) {
	f := append(grid(0), bspQuad(100, -50, 200, 350, 500)...)
	r := coverage.Measure(f, square(0, 0, 300, 300), nil).Classify(-100, 300)
	if r.Layers != 2 {
		t.Errorf("%d layers, want 2", r.Layers)
	}
	if !near(r.Above, 100*300) {
		t.Errorf("floor above the top %v, want 30000", r.Above)
	}
	if !near(r.Inside, 300*300) || r.Below != 0 || r.NoGround != 0 {
		t.Errorf("inside %v, below %v, no ground %v; want 90000, 0, 0", r.Inside, r.Below, r.NoGround)
	}
	if r.GroundMax.Z != 500 || r.TopClearance != -200 {
		t.Errorf("highest floor %v, top clearance %v; want z 500, -200", r.GroundMax, r.TopClearance)
	}
}

// Flat terrain alone is 1 layer.
func TestTerrainAloneIsOneLayer(t *testing.T) {
	if n := coverage.Measure(grid(0), square(0, 0, 300, 300), nil).Classify(-100, 100).Layers; n != 1 {
		t.Errorf("%d layers, want 1", n)
	}
}

// A terrain hole elsewhere stays no ground when a bridge spans the square
// (the bridge's area must not make up for it), and a building floor laid
// over a hole, as the maps cut the terrain under a building, is floor and
// 1 layer. Catches no ground taken as the outline's area minus all the
// floor's area, which layers overcount.
func TestHoleIsNoGroundUnlessAFloorCoversIt(t *testing.T) {
	bridged := append(grid(0, [2]int{0, 0}), bspQuad(100, -50, 200, 350, 500)...)
	if r := coverage.Measure(bridged, square(0, 0, 300, 300), nil).Classify(-100, 600); !near(r.NoGround, 100*100) {
		t.Errorf("bridge elsewhere: no ground %v, want 10000", r.NoGround)
	}
	built := append(grid(0, [2]int{0, 0}), bspQuad(0, 0, 100, 100, 20)...)
	r := coverage.Measure(built, square(0, 0, 300, 300), nil).Classify(-100, 100)
	if r.NoGround != 0 || !near(r.Inside, 300*300) || r.Layers != 1 {
		t.Errorf("floor over the hole: no ground %v, inside %v, %d layers; want 0, 90000, 1", r.NoGround, r.Inside, r.Layers)
	}
}

// Under a square fully floored by a ramp z = x that both its top and its
// floor cut, inside + above + below add up to the square's 40 000 units²,
// and each is the strip its formula gives. Catches a piece counted in 2
// states, or lost between them, where it straddles a cut.
func TestInsideAboveBelowAddUpToTheFloor(t *testing.T) {
	ramp := func(x, y float32) geom.Vec3 { return v(x, y, x) }
	a, b, c, d := ramp(-50, -50), ramp(250, -50), ramp(250, 250), ramp(-50, 250)
	f := floor{tri(a, b, c), tri(a, c, d)}
	r := coverage.Measure(f, square(0, 0, 200, 200), nil).Classify(30, 170)
	if sum := r.Inside + r.Above + r.Below; !near(sum, 40000) {
		t.Errorf("inside %v + above %v + below %v = %v, want the floor's 40000", r.Inside, r.Above, r.Below, sum)
	}
	for _, c := range []struct {
		name      string
		got, want float64
	}{{"inside", r.Inside, 140 * 200}, {"above", r.Above, 30 * 200}, {"below", r.Below, 30 * 200}, {"no ground", r.NoGround, 0}} {
		if !near(c.got, c.want) {
			t.Errorf("%s %v, want %v", c.name, c.got, c.want)
		}
	}
}

// A floor triangle with a corner in the L's notch, at (125, 125), meets
// the L in one small triangle (150, 100), (200, 50), (200, 100) of the arm
// along X; on the plane z = x − y its lowest floor is 50 at (150, 100),
// not the 0 of the notch corner. Sutherland–Hodgman bridges the L's 2
// arms across the notch and keeps that corner on the bridge. Catches
// extremes taken from the clipped polygon's vertices.
func TestNotchCornerIsNotGround(t *testing.T) {
	plane := func(x, y float32) geom.Vec3 { return v(x, y, x-y) }
	f := floor{tri(plane(125, 125), plane(1125, -875), plane(1125, 1125))}
	r := coverage.Measure(f, lShape, nil).Classify(-1000, 1000)
	if lo := r.GroundMin; !near(lo.X, 150) || !near(lo.Y, 100) || !near(lo.Z, 50) {
		t.Errorf("lowest ground %v %v %v, want 150 100 50", lo.X, lo.Y, lo.Z)
	}
	if hi := r.GroundMax; !near(hi.Z, 150) {
		t.Errorf("highest ground z %v, want 150 at (200, 50)", hi.Z)
	}
	if !near(r.Inside, 1250) || !near(r.NoGround, 30000-1250) {
		t.Errorf("inside %v, no ground %v, want 1250 and 28750", r.Inside, r.NoGround)
	}
}

// A flat floor at z = 100 (grid) under a shape [0, 300]² whose top is at
// 50: all of it is above the top. A ban on the middle quad [100, 200]²
// reaching z = 100 puts the floor under it in "excluded" instead: exactly,
// since the ban follows the quad's edges. Catches a ban ignored, or its
// floor counted twice.
func TestBanExcludesFloorAboveTheTop(t *testing.T) {
	ban := coverage.Ban{Outline: square(100, 100, 200, 200), ZMin: 0, ZMax: 200}
	r := coverage.Measure(grid(100), square(0, 0, 300, 300), []coverage.Ban{ban}).Classify(-50, 50)
	if !near(r.Excluded, 100*100) {
		t.Errorf("excluded %v, want 10000", r.Excluded)
	}
	if !near(r.Above, 300*300-100*100) {
		t.Errorf("above %v, want 80000", r.Above)
	}
}

// The same ban with a Z range [300, 500] that does not reach the floor at
// z = 100 excludes nothing; lowered to reach it, without measuring again
// (WithBanRanges), it does. Catches a ban applied by X/Y alone.
func TestBanOutOfZRangeExcludesNothing(t *testing.T) {
	ban := coverage.Ban{Outline: square(100, 100, 200, 200), ZMin: 300, ZMax: 500}
	p := coverage.Measure(grid(100), square(0, 0, 300, 300), []coverage.Ban{ban})
	if r := p.Classify(-50, 50); r.Excluded != 0 || !near(r.Above, 300*300) {
		t.Errorf("excluded %v, above %v, want 0 and 90000", r.Excluded, r.Above)
	}
	ban.ZMin = 0
	if r := p.WithBanRanges([]coverage.Ban{ban}).Classify(-50, 50); !near(r.Excluded, 100*100) {
		t.Errorf("excluded %v after lowering the ban, want 10000", r.Excluded)
	}
}

// Under a flat floor at z = 0 missing its middle quad, with a ban off the
// quads' edges (so pieces straddle its outline), inside + above + below +
// excluded is still the measured floor and Total the outline's area.
// Catches a piece counted in 2 states, or in none.
func TestStatesAddUpWithABan(t *testing.T) {
	ban := coverage.Ban{Outline: coverage.Outline{{X: 70, Y: 20}, {X: 230, Y: 40}, {X: 180, Y: 210}}, ZMin: -10, ZMax: 10}
	r := coverage.Measure(grid(0, [2]int{1, 1}), square(10, 10, 290, 290), []coverage.Ban{ban}).Classify(5, 50)
	ground := 280.0*280 - 100*100
	if sum := r.Inside + r.Above + r.Below + r.Excluded; !near(sum, ground) {
		t.Errorf("inside %v + above %v + below %v + excluded %v = %v, want %v", r.Inside, r.Above, r.Below, r.Excluded, sum, ground)
	}
	if r.Excluded <= 0 || r.Below <= 0 {
		t.Errorf("excluded %v, below %v, want both > 0", r.Excluded, r.Below)
	}
	if !near(r.Total(), 280*280) {
		t.Errorf("total %v, want 78400", r.Total())
	}
}

// A zone of 3 shapes over a ramp z = x: A, square (0, 0)-(100, 100) in
// [0, 200], is all inside; B, square (100, 0)-(200, 100) in [150, 300],
// has its half x < 150 below its floor; C, off the floor, has no ground.
// The zone sums their areas (15 000 inside, 5 000 below, 10 000 with no
// ground), its floor spans A's lowest 0 to B's highest 200, and its
// clearances are the worst shape's: B's floor −50, A's top 100. Catches a
// zone that keeps one shape's numbers, or lets C's unmeasured zero
// extremes pull the zone's lowest floor.
func TestZoneSumsShapesAndKeepsWorstExtremes(t *testing.T) {
	ramp := func(x, y float32) geom.Vec3 { return v(x, y, x) }
	a, b, c, d := ramp(-50, -50), ramp(250, -50), ramp(250, 250), ramp(-50, 250)
	f := floor{tri(a, b, c), tri(a, c, d)}
	z := coverage.Sum(
		coverage.Measure(f, square(1000, 1000, 1100, 1100), nil).Classify(-10, 10),
		coverage.Measure(f, square(0, 0, 100, 100), nil).Classify(0, 200),
		coverage.Measure(f, square(100, 0, 200, 100), nil).Classify(150, 300),
	)
	if !z.Measured {
		t.Fatal("zone with floor under 2 shapes not measured")
	}
	for _, c := range []struct {
		name      string
		got, want float64
	}{
		{"inside", z.Inside, 15000}, {"above", z.Above, 0}, {"below", z.Below, 5000}, {"no ground", z.NoGround, 10000},
		{"lowest floor", z.GroundMin.Z, 0}, {"highest floor", z.GroundMax.Z, 200},
		{"floor clearance", z.FloorClearance, -50}, {"top clearance", z.TopClearance, 100},
	} {
		if !near(c.got, c.want) {
			t.Errorf("%s %v, want %v", c.name, c.got, c.want)
		}
	}
}

// Under a square floored by a ramp z = x − 95, the floor spreads evenly
// over Z from −95 to 105, 200 units² per unit of Z: the histogram's bins
// of 16 start at −96 (bin −6) and end at 112 (bin 6), the first holding
// the 15 units of −95..−80 (3 000), the last the 9 units of 96..105
// (1 800) and every bin between 3 200. Catches a piece put whole in the
// bin of its lowest (or average) Z, though it spans the whole ramp, and a
// bin index truncated toward 0 below Z = 0.
func TestHistogramSpreadsARampOverItsZ(t *testing.T) {
	ramp := func(x, y float32) geom.Vec3 { return v(x, y, x-95) }
	a, b, c, d := ramp(-50, -50), ramp(250, -50), ramp(250, 250), ramp(-50, 250)
	h := coverage.Measure(floor{tri(a, b, c), tri(a, c, d)}, square(0, 0, 200, 200), nil).Histogram()
	if h.First != -6 || len(h.Area) != 13 {
		t.Fatalf("bins %d..%d, want -6..6", h.First, h.First+len(h.Area)-1)
	}
	for i, got := range h.Area {
		want := 3200.0
		switch i {
		case 0:
			want = 3000
		case len(h.Area) - 1:
			want = 1800
		}
		if !near(got, want) {
			t.Errorf("bin %d: %v units², want %v", h.First+i, got, want)
		}
	}
}

// The same ramp z = x split by the range [30, 170] through the histogram
// gives the floor of each state Classify gives: 28 000 inside and 6 000
// above and below, though 30 and 170 cut bins 1 and 10 in their middle.
// Catches the ruler colouring a cut bin all in one state, or the 2
// states swapped.
func TestHistogramSplitsCutBinsByTheRange(t *testing.T) {
	ramp := func(x, y float32) geom.Vec3 { return v(x, y, x) }
	a, b, c, d := ramp(-50, -50), ramp(250, -50), ramp(250, 250), ramp(-50, 250)
	h := coverage.Measure(floor{tri(a, b, c), tri(a, c, d)}, square(0, 0, 200, 200), nil).Histogram()
	in, above, below := h.Split(-1000, 1000, 30, 170)
	for _, c := range []struct {
		name      string
		got, want float64
	}{{"inside", in, 28000}, {"above", above, 6000}, {"below", below, 6000}} {
		if !near(c.got, c.want) {
			t.Errorf("%s %v, want %v", c.name, c.got, c.want)
		}
	}
	if in, above, below := h.Split(32, 48, 30, 170); !near(in, 3200) || above != 0 || below != 0 {
		t.Errorf("bin 2 split %v/%v/%v, want 3200 inside", in, above, below)
	}
}
