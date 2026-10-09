package coverage_test

import (
	"testing"

	"zonebuilder/internal/coverage"
)

// kinds is the kinds of ws, in order.
func kinds(ws []coverage.Warning) []coverage.WarningKind {
	var k []coverage.WarningKind
	for _, w := range ws {
		k = append(k, w.Kind)
	}
	return k
}

// plateau is a flat floor at z 0 over [0, 1000]², 2 triangles, with a
// flat block of floor at height z over [x0, x1] × [y0, y1] standing on it
// as a second layer: a step of a cliff, as its own pieces.
func plateau(z float32, x0, y0, x1, y1 float32) floor {
	a, b, c, d := v(0, 0, 0), v(1000, 0, 0), v(1000, 1000, 0), v(0, 1000, 0)
	p, q, r, s := v(x0, y0, z), v(x1, y0, z), v(x1, y1, z), v(x0, y1, z)
	return floor{tri(a, b, c), tri(a, c, d), tri(p, q, r), tri(p, r, s)}
}

// A hill whose apex pierces the top warns "chão acima do topo" at the
// apex. Catches warnings that never fire, or that point anywhere but the
// highest floor.
func TestHillPiercingTheTopWarnsAtThePeak(t *testing.T) {
	apex := v(437, 611, 500)
	a, b, c, d := v(-500, -500, 0), v(1500, -500, 0), v(1500, 1500, 0), v(-500, 1500, 0)
	f := floor{tri(a, b, apex), tri(b, c, apex), tri(c, d, apex), tri(d, a, apex)}
	ws := coverage.Measure(f, square(0, 0, 1000, 1000), nil).Classify(-100, 100).Warnings
	if len(ws) != 1 || ws[0].Kind != coverage.AboveTop {
		t.Fatalf("warnings %v, want only AboveTop", kinds(ws))
	}
	if at := ws[0].At; !near(at.X, 437) || !near(at.Y, 611) || !near(at.Z, 500) {
		t.Errorf("warning at %v %v %v, want the apex 437 611 500", at.X, at.Y, at.Z)
	}
	if !near(ws[0].Clearance, -400) {
		t.Errorf("warning clearance %v, want -400", ws[0].Clearance)
	}
}

// A 1-cell sliver of a cliff (16×16 of a 1000×1000 floor) 10 units over
// the top is under both thresholds (1 % of the floor, 16 units deep) and
// warns of nothing; the same sliver 20 units over warns by depth, and a
// block 5 units over on about 4 % of the floor warns by area. Catches
// warnings on every speck of floor out of range, and thresholds that are not
// applied each on its own.
func TestCliffSliverUnderTheThresholdsDoesNotWarn(t *testing.T) {
	const top = 100
	cases := []struct {
		name string
		f    floor
		want []coverage.WarningKind
	}{
		{"lasca rasa", plateau(top+10, 500, 500, 516, 516), nil},
		{"lasca funda", plateau(top+20, 500, 500, 516, 516), []coverage.WarningKind{coverage.AboveTop}},
		{"bloco largo", plateau(top+5, 0, 0, 200, 200), []coverage.WarningKind{coverage.AboveTop}},
	}
	for _, c := range cases {
		ws := coverage.Measure(c.f, square(0, 0, 1000, 1000), nil).Classify(-100, top).Warnings
		if got := kinds(ws); len(got) != len(c.want) || (len(got) > 0 && got[0] != c.want[0]) {
			t.Errorf("%s: warnings %v, want %v", c.name, got, c.want)
		}
	}
}

// Floor below the shape's floor warns by the same rule, at the lowest
// floor. Catches a rule written for the top only.
func TestFloorUnderTheBottomWarnsAtTheLowest(t *testing.T) {
	ws := coverage.Measure(plateau(-20, 300, 300, 316, 316), square(0, 0, 1000, 1000), nil).Classify(0, 1000).Warnings
	if len(ws) != 1 || ws[0].Kind != coverage.BelowFloor {
		t.Fatalf("warnings %v, want only BelowFloor", kinds(ws))
	}
	if at := ws[0].At; !near(at.Z, -20) || at.X < 300 || at.X > 316 {
		t.Errorf("warning at %v %v %v, want on the sunken block at z -20", at.X, at.Y, at.Z)
	}
}

// A clearance from 0 up to MinClearance warns "folga apertada" on its
// side; MinClearance itself does not. Catches a tight-clearance rule off
// by its bound or on the wrong side.
func TestTightClearanceWarnsUnderMinClearance(t *testing.T) {
	f := grid(50)
	o := square(0, 0, 300, 300)
	p := coverage.Measure(f, o, nil)
	if ws := p.Classify(50-coverage.MinClearance, 50+coverage.MinClearance).Warnings; len(ws) != 0 {
		t.Errorf("clearances of exactly MinClearance: warnings %v, want none", kinds(ws))
	}
	ws := p.Classify(50-coverage.MinClearance, 50+coverage.MinClearance-1).Warnings
	if len(ws) != 1 || ws[0].Kind != coverage.TightTop || !near(ws[0].Clearance, coverage.MinClearance-1) {
		t.Errorf("top clearance %d: warnings %+v, want TightTop", coverage.MinClearance-1, ws)
	}
	ws = p.Classify(50, 1000).Warnings
	if len(ws) != 1 || ws[0].Kind != coverage.TightFloor || !near(ws[0].Clearance, 0) {
		t.Errorf("floor clearance 0: warnings %+v, want TightFloor", ws)
	}
}

// An outline over a hole in the floor warns of the area with no floor.
// Catches a hole, such as a tile not loaded, passing silently.
func TestHoleWarnsNoGround(t *testing.T) {
	ws := coverage.Measure(grid(50, [2]int{1, 1}), square(0, 0, 300, 300), nil).Classify(-1000, 1000).Warnings
	if len(ws) != 1 || ws[0].Kind != coverage.NoGround || !near(ws[0].Area, 100*100) {
		t.Errorf("warnings %+v, want NoGround of 10000", ws)
	}
}

// hillOnFlat is flat floor at 0 over [-100, 1100]² around a pyramid on
// [400, 600]² with its apex at 400; hillBan is an exclusion over the hill
// whose range holds it.
func hillOnFlat() floor {
	o00, o10, o11, o01 := v(-100, -100, 0), v(1100, -100, 0), v(1100, 1100, 0), v(-100, 1100, 0)
	h00, h10, h11, h01 := v(400, 400, 0), v(600, 400, 0), v(600, 600, 0), v(400, 600, 0)
	apex := v(500, 500, 400)
	return floor{
		tri(o00, o10, h10), tri(o00, h10, h00),
		tri(o10, o11, h11), tri(o10, h11, h10),
		tri(o11, o01, h01), tri(o11, h01, h11),
		tri(o01, o00, h00), tri(o01, h00, h01),
		tri(h00, h10, apex), tri(h10, h11, apex), tri(h11, h01, apex), tri(h01, h00, apex),
	}
}

var hillBan = coverage.Ban{Outline: square(300, 300, 700, 700), ZMin: -1000, ZMax: 1000}

// A hill under an exclusion whose range holds it warns of nothing, though
// it pierces the shape's top; without the exclusion it warns. Catches
// warnings judged by the extremes of all floor, excluded included.
func TestExclusionOverAHillDoesNotWarn(t *testing.T) {
	o := square(0, 0, 1000, 1000)
	if ws := coverage.Measure(hillOnFlat(), o, []coverage.Ban{hillBan}).Classify(-100, 100).Warnings; len(ws) != 0 {
		t.Errorf("hill under the exclusion: warnings %+v, want none", ws)
	}
	if ws := coverage.Measure(hillOnFlat(), o, nil).Classify(-100, 100).Warnings; len(ws) != 1 || ws[0].Kind != coverage.AboveTop {
		t.Errorf("hill without the exclusion: warnings %v, want AboveTop", kinds(ws))
	}
}

// Under the same exclusion, the report's highest floor is the flat floor
// around the hill, at 0, with a top clearance of 100: the floor the
// warnings judge. Catches extremes and clearances taken from all floor,
// excluded included, which put a red "topo −300" pin and a "(fura)" on
// the inspector and the ruler over a hill that is not a failure (spec D1),
// where no warning points.
func TestExcludedHillIsNotTheHighestFloor(t *testing.T) {
	r := coverage.Measure(hillOnFlat(), square(0, 0, 1000, 1000), []coverage.Ban{hillBan}).Classify(-100, 100)
	if !r.Measured || r.GroundMax.Z != 0 || r.TopClearance != 100 {
		t.Errorf("highest floor %v, top clearance %v; want z 0, 100", r.GroundMax, r.TopClearance)
	}
	if r.GroundMin.Z != 0 || r.FloorClearance != 100 {
		t.Errorf("lowest floor %v, floor clearance %v; want z 0, 100", r.GroundMin, r.FloorClearance)
	}
}
