package coverage_test

import (
	"math"
	"slices"
	"testing"

	"zonebuilder/internal/coverage"
	"zonebuilder/internal/geom"
)

// rampHeight is the ramp's height at a grid vertex: curved along X, so
// every cell has its own slope and the floor along an edge bends at each
// cell crossing.
func rampHeight(x, y float64) float64 { return x*x/100 + 3*y }

// ramp is a floor of 100-unit quads over [0, 300]² at rampHeight, each
// split on its low-to-high diagonal, as grid does.
func ramp() floor {
	var f floor
	at := func(x, y float64) geom.Vec3 { return v(float32(x), float32(y), float32(rampHeight(x, y))) }
	for y := range 3 {
		for x := range 3 {
			x0, y0 := float64(x*100), float64(y*100)
			a, b, c, d := at(x0, y0), at(x0+100, y0), at(x0+100, y0+100), at(x0, y0+100)
			f = append(f, tri(a, b, c), tri(a, c, d))
		}
	}
	return f
}

// rampZ is the ramp's floor height at (x, y): the plane of the cell's
// triangle there.
func rampZ(x, y float64) float64 {
	x0, y0 := min(math.Floor(x/100), 2)*100, min(math.Floor(y/100), 2)*100
	dx, dy := (x-x0)/100, (y-y0)/100
	za, zb, zc, zd := rampHeight(x0, y0), rampHeight(x0+100, y0), rampHeight(x0+100, y0+100), rampHeight(x0, y0+100)
	if dy <= dx { // triangle a b c
		return za + dx*(zb-za) + dy*(zc-zb)
	}
	return za + dx*(zc-zd) + dy*(zd-za) // triangle a c d
}

// cellCrossings are the distances along p→q where it crosses the ramp's
// cell edges: the grid lines and the cells' diagonals (x − y a multiple
// of 100), strictly between its ends.
func cellCrossings(p, q coverage.Point) []float64 {
	length := math.Hypot(q.X-p.X, q.Y-p.Y)
	var ds []float64
	add := func(t float64) {
		if t > 1e-9 && t < 1-1e-9 {
			ds = append(ds, t*length)
		}
	}
	for k := -3.0; k <= 3; k++ {
		c := k * 100
		if q.X != p.X {
			add((c - p.X) / (q.X - p.X))
		}
		if q.Y != p.Y {
			add((c - p.Y) / (q.Y - p.Y))
		}
		if d := (q.X - q.Y) - (p.X - p.Y); d != 0 {
			add((c - (p.X - p.Y)) / d)
		}
	}
	slices.Sort(ds)
	return ds
}

// The floor line of each outline edge over a ramp of cells runs the whole
// edge without gaps, bends exactly where the edge crosses a cell's edge
// and has the ramp's height at every one of its points. Catches a line
// sampled at fixed steps (it misses the bends), one taken from the
// clipped pieces' vertices (Sutherland–Hodgman bridges) or edges
// renumbered by the outline's winding.
func TestEdgeFloorLineFollowsRampAtCellCrossings(t *testing.T) {
	// Clockwise on purpose: Edges keeps the outline's own order.
	o := coverage.Outline{{X: 30, Y: 40}, {X: 130, Y: 280}, {X: 270, Y: 210}, {X: 250, Y: 20}}
	r := coverage.Measure(ramp(), o, nil).Classify(-1e6, 1e6)
	if len(r.Edges) != len(o) {
		t.Fatalf("%d edges, want %d", len(r.Edges), len(o))
	}
	for i, spans := range r.Edges {
		p, q := o[i], o[(i+1)%len(o)]
		length := math.Hypot(q.X-p.X, q.Y-p.Y)
		if len(spans) == 0 {
			t.Fatalf("edge %d: no floor line", i)
		}
		var bends []float64
		at := 0.0
		for k, s := range spans {
			if !near(s.From.D, at) {
				t.Errorf("edge %d span %d starts at %v, want %v (gap or overlap)", i, k, s.From.D, at)
			}
			for _, st := range []coverage.Station{s.From, s.To} {
				x, y := p.X+(q.X-p.X)*st.D/length, p.Y+(q.Y-p.Y)*st.D/length
				if !near(st.X, x) || !near(st.Y, y) {
					t.Errorf("edge %d: station at d=%v is (%v, %v), want (%v, %v) on the edge", i, st.D, st.X, st.Y, x, y)
				}
				if z := rampZ(st.X, st.Y); math.Abs(st.Z-z) > 1e-3 {
					t.Errorf("edge %d: floor at (%v, %v) is %v, ramp is %v", i, st.X, st.Y, st.Z, z)
				}
			}
			if k > 0 {
				bends = append(bends, s.From.D)
			}
			at = s.To.D
		}
		if !near(at, length) {
			t.Errorf("edge %d: line ends at %v, want %v", i, at, length)
		}
		want := cellCrossings(p, q)
		if len(bends) != len(want) {
			t.Fatalf("edge %d: bends at %v, want the cell crossings %v", i, bends, want)
		}
		for k := range want {
			if math.Abs(bends[k]-want[k]) > 1e-6 {
				t.Errorf("edge %d: bend %d at %v, want %v", i, k, bends[k], want[k])
			}
		}
	}
}

// The floor line along the walls of a zone on a tower's floor at 15 000,
// over terrain at 0, runs only on the tower's floor; dragged down to the
// terrain, only on the terrain. Catches a line drawn on every floor under
// the outline, which stacks the tower's other storeys and the ground
// under it as rings down the tower.
func TestEdgeFloorLineLeavesOtherLayersOut(t *testing.T) {
	f := append(grid(0), bspQuad(0, 0, 300, 300, 15000)...)
	p := coverage.Measure(f, square(50, 50, 250, 250), nil)
	for _, c := range []struct {
		zmin, zmax, want float64
	}{{14744, 15256, 15000}, {-256, 256, 0}} {
		r := p.Classify(c.zmin, c.zmax)
		n := 0
		for i, spans := range r.Edges {
			for _, s := range spans {
				n++
				if s.From.Z != c.want || s.To.Z != c.want {
					t.Errorf("range %v … %v, edge %d: span at z %v … %v, want only z %v", c.zmin, c.zmax, i, s.From.Z, s.To.Z, c.want)
				}
			}
		}
		if n == 0 {
			t.Errorf("range %v … %v: no floor line, want it at z %v", c.zmin, c.zmax, c.want)
		}
	}
}
