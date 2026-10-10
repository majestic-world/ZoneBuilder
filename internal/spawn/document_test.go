package spawn_test

import (
	"reflect"
	"slices"
	"testing"

	"zonebuilder/internal/spawn"
)

// apply applies cmds in order and fails the test on the first error.
func apply(t *testing.T, d *spawn.Document, cmds ...spawn.Command) {
	t.Helper()
	for _, c := range cmds {
		if err := d.Apply(c); err != nil {
			t.Fatalf("Apply(%#v): %v", c, err)
		}
	}
}

// square is a 400×400 outline around Giran's square.
var square = []spawn.Vertex{{X: 83000, Y: 147600}, {X: 83400, Y: 147600}, {X: 83400, Y: 148000}, {X: 83000, Y: 148000}}

// generated is a document with one valid area named name whose points come
// from a SetPoints with seed 7, as the Gerar button leaves it.
func generated(t *testing.T, name string) (*spawn.Document, spawn.AreaID) {
	t.Helper()
	d := spawn.NewDocument()
	id := d.NewAreaID()
	params := spawn.DefaultParams(24)
	params.Count = 2
	apply(t, d, spawn.CreateArea{ID: id, Name: name, Outline: square, ZMin: -3600, ZMax: -3200, Params: params})
	a, _ := d.Area(id)
	apply(t, d, spawn.SetPoints{
		Area: id, Seed: 7, Fingerprint: a.Fingerprint(7),
		Points:      []spawn.Point{{X: 83100, Y: 147700, Z: -3404, Heading: 1200}, {X: 83300, Y: 147900, Z: -3398, Heading: 40000}},
		Measurement: spawn.Measurement{Known: true, FreeArea: 16384},
	})
	return d, id
}

// rules is the rules of d's problems, in order.
func rules(d *spawn.Document) []spawn.Rule {
	var out []spawn.Rule
	for _, p := range d.Problems() {
		out = append(out, p.Rule)
	}
	return out
}

// blocks reports whether d has a problem that blocks compiling.
func blocks(d *spawn.Document) bool {
	return slices.ContainsFunc(d.Problems(), spawn.Problem.Blocks)
}

// Changing any input of the distribution after SetPoints leaves the points
// stale, a problem that blocks compiling; adjusting the points by hand does
// not. Catches a fingerprint that misses an input (the XML would carry a
// distribution the user never saw) or one that counts manual adjustments
// as an input change (every drag would force a regeneration).
func TestInputChangeAfterSetPointsMarksAreaStale(t *testing.T) {
	changes := map[string]func(id spawn.AreaID, a spawn.Area) spawn.Command{
		"outline vertex": func(id spawn.AreaID, a spawn.Area) spawn.Command {
			return spawn.MoveVertex{Area: id, Index: 1, Point: spawn.Vertex{X: 83500, Y: 147600}}
		},
		"inserted vertex": func(id spawn.AreaID, a spawn.Area) spawn.Command {
			return spawn.InsertVertex{Area: id, Index: 1, Point: spawn.Vertex{X: 83200, Y: 147500}}
		},
		"moved area": func(id spawn.AreaID, a spawn.Area) spawn.Command {
			return spawn.MoveArea{Area: id, DX: 16}
		},
		"Z range": func(id spawn.AreaID, a spawn.Area) spawn.Command {
			return spawn.SetZRange{Area: id, ZMin: a.ZMin, ZMax: a.ZMax + 64}
		},
		"count": func(id spawn.AreaID, a spawn.Area) spawn.Command {
			p := a.Params
			p.Count++
			return spawn.SetParams{Area: id, Params: p}
		},
		"radius": func(id spawn.AreaID, a spawn.Area) spawn.Command {
			p := a.Params
			p.Radius += 8
			return spawn.SetParams{Area: id, Params: p}
		},
		"clearance": func(id spawn.AreaID, a spawn.Area) spawn.Command {
			p := a.Params
			p.Clearance = 48
			return spawn.SetParams{Area: id, Params: p}
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			d, id := generated(t, "giran")
			a, _ := d.Area(id)
			apply(t, d, change(id, a))
			a, _ = d.Area(id)
			if !a.Stale() {
				t.Errorf("area not stale after changing its %s", name)
			}
			if !slices.Contains(rules(d), spawn.StalePoints) || !blocks(d) {
				t.Errorf("problems %v, want a blocking StalePoints", d.Problems())
			}
		})
	}

	edits := map[string]spawn.Command{
		"moved point":   spawn.MovePoint{Area: 1, Index: 0, Point: spawn.Point{X: 83150, Y: 147750, Z: -3401, Heading: 1200}},
		"removed point": spawn.RemovePoint{Area: 1, Index: 1},
		"added point":   spawn.AddPoint{Area: 1, Point: spawn.Point{X: 83200, Y: 147800, Z: -3400, Heading: 9}},
		"renamed":       spawn.Rename{Area: 1, Name: "giran_north"},
	}
	for name, edit := range edits {
		t.Run(name, func(t *testing.T) {
			d, id := generated(t, "giran")
			if id != 1 {
				t.Fatalf("first area has ID %d, want 1", id)
			}
			apply(t, d, edit)
			if a, _ := d.Area(id); a.Stale() {
				t.Errorf("area stale after a %s", name)
			}
			if blocks(d) {
				t.Errorf("problems %v after a %s, want none blocking", d.Problems(), name)
			}
		})
	}
}

// Each input the server loses the whole file over (or that would compile a
// distribution the user never meant) blocks compiling, while the
// distribution's warnings do not. Catches a rule left out of Problems, a
// rule reported as a warning, or a warning that blocks compiling the points
// that did fit.
func TestProblemsBlockAndWarningsDoNot(t *testing.T) {
	params := func(edit func(*spawn.Params)) func(*spawn.Document, spawn.AreaID) spawn.Command {
		return func(d *spawn.Document, id spawn.AreaID) spawn.Command {
			a, _ := d.Area(id)
			p := a.Params
			edit(&p)
			return spawn.SetParams{Area: id, Params: p}
		}
	}
	blocking := []struct {
		name string
		want spawn.Rule
		edit func(*spawn.Document, spawn.AreaID) spawn.Command
	}{
		{"count 0", spawn.InvalidCount, params(func(p *spawn.Params) { p.Count = 0 })},
		{"empty name", spawn.EmptyName, func(_ *spawn.Document, id spawn.AreaID) spawn.Command {
			return spawn.Rename{Area: id, Name: " "}
		}},
		{"repeated name", spawn.DuplicateName, func(d *spawn.Document, id spawn.AreaID) spawn.Command {
			other := d.NewAreaID()
			a, _ := d.Area(id)
			return spawn.CreateArea{ID: other, Name: a.Name, Outline: a.Outline, ZMin: a.ZMin, ZMax: a.ZMax, Params: a.Params}
		}},
		{"no points", spawn.NoPoints, func(_ *spawn.Document, id spawn.AreaID) spawn.Command {
			return spawn.Batch{spawn.RemovePoint{Area: id, Index: 1}, spawn.RemovePoint{Area: id, Index: 0}}
		}},
		{"2 vertices", spawn.TooFewVertices, func(_ *spawn.Document, id spawn.AreaID) spawn.Command {
			return spawn.Batch{spawn.RemoveVertex{Area: id, Index: 3}, spawn.RemoveVertex{Area: id, Index: 2}}
		}},
		{"self-intersection", spawn.SelfIntersection, func(_ *spawn.Document, id spawn.AreaID) spawn.Command {
			// Swapping 2 corners of the square makes a bow tie.
			return spawn.Batch{
				spawn.MoveVertex{Area: id, Index: 2, Point: square[3]},
				spawn.MoveVertex{Area: id, Index: 3, Point: square[2]},
			}
		}},
		{"zmin above zmax", spawn.InvertedZRange, func(_ *spawn.Document, id spawn.AreaID) spawn.Command {
			return spawn.SetZRange{Area: id, ZMin: -3200, ZMax: -3600}
		}},
		{"point east of the world", spawn.OutOfBounds, func(_ *spawn.Document, id spawn.AreaID) spawn.Command {
			return spawn.AddPoint{Area: id, Point: spawn.Point{X: 229376, Y: 147800, Z: -3400, Heading: 1}}
		}},
		{"point south of the world", spawn.OutOfBounds, func(_ *spawn.Document, id spawn.AreaID) spawn.Command {
			return spawn.MovePoint{Area: id, Index: 0, Point: spawn.Point{X: 83100, Y: 294912, Z: -3400, Heading: 1}}
		}},
	}
	for _, c := range blocking {
		t.Run(c.name, func(t *testing.T) {
			d, id := generated(t, "giran")
			if blocks(d) {
				t.Fatalf("valid area has blocking problems %v", d.Problems())
			}
			apply(t, d, c.edit(d, id))
			i := slices.IndexFunc(d.Problems(), func(p spawn.Problem) bool { return p.Rule == c.want })
			if i < 0 || !d.Problems()[i].Blocks() {
				t.Errorf("problems %v, want a blocking rule %d", d.Problems(), c.want)
			}
		})
	}

	t.Run("warnings", func(t *testing.T) {
		d, id := generated(t, "giran")
		a, _ := d.Area(id)
		apply(t, d, spawn.SetPoints{
			Area: id, Seed: 8, Fingerprint: a.Fingerprint(8), Points: a.Points[:1],
			Warnings: []spawn.Warning{{Rule: spawn.FitsOnly, Placed: 1, Requested: 2}, {Rule: spawn.NoFreeCell}},
		})
		if got, want := rules(d), []spawn.Rule{spawn.FitsOnly, spawn.NoFreeCell}; !slices.Equal(got, want) {
			t.Errorf("problems %v, want the 2 warnings", d.Problems())
		}
		if p := d.Problems()[0]; p.Numbers[0] != 1 || p.Numbers[1] != 2 {
			t.Errorf("FitsOnly numbers %v, want K=1 N=2", p.Numbers)
		}
		if blocks(d) {
			t.Errorf("warnings %v block compiling", d.Problems())
		}
	})
}

// One Undo takes a whole generation back: the points, seed, fingerprint and
// warnings return to what the previous generation left, and Redo puts the
// new one back. Catches a generation recorded in several steps (the first
// Undo would leave new points with an old fingerprint, or the reverse).
func TestOneUndoRevertsAGeneration(t *testing.T) {
	d, id := generated(t, "giran")
	apply(t, d, spawn.MovePoint{Area: id, Index: 0, Point: spawn.Point{X: 83234, Y: 147765, Z: -3400, Heading: 4321}})
	before, _ := d.Area(id)
	before = spawn.CloneArea(before)
	apply(t, d, spawn.SetPoints{
		Area: id, Seed: 99, Fingerprint: before.Fingerprint(99),
		Points:      []spawn.Point{{X: 83200, Y: 147800, Z: -3400, Heading: 7}},
		Warnings:    []spawn.Warning{{Rule: spawn.FitsOnly, Placed: 1, Requested: 2}},
		Measurement: spawn.Measurement{Known: true, FreeArea: 8192},
	})
	after, _ := d.Area(id)
	after = spawn.CloneArea(after)

	if !d.Undo() {
		t.Fatal("Undo found no step")
	}
	if got, _ := d.Area(id); !reflect.DeepEqual(got, before) {
		t.Errorf("after 1 Undo:\n%#v\nwant the previous generation:\n%#v", got, before)
	}
	if !d.Redo() {
		t.Fatal("Redo found no step")
	}
	if got, _ := d.Area(id); !reflect.DeepEqual(got, after) {
		t.Errorf("after Redo:\n%#v\nwant the new generation:\n%#v", got, after)
	}
}
