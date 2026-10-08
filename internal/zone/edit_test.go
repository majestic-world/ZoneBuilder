package zone_test

import (
	"reflect"
	"testing"

	"zonebuilder/internal/zone"
)

// compiled is the XML d compiles every zone into, as one string per file.
func compiled(t *testing.T, d *zone.Document) []string {
	t.Helper()
	files, err := d.Compile(d.ZoneIDs())
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Name + "\n" + string(f.Data)
	}
	return out
}

// coords is the loc of every compiled coords, in order.
func coords(t *testing.T, d *zone.Document) []string {
	t.Helper()
	var locs []string
	for _, f := range compiled(t, d) {
		for _, m := range coordsLoc.FindAllStringSubmatch(f, -1) {
			locs = append(locs, m[1])
		}
	}
	return locs
}

// Moving, inserting and removing vertices and moving the whole shape each
// show in the compiled coords: the edited vertex at its new x y, the
// insertion between its neighbours, and a shape move carrying the Z range
// along with the outline.
func TestVertexAndShapeEditsReachTheCompiledXML(t *testing.T) {
	d := zone.NewDocument()
	id := d.NewZoneID()
	polygonZone(t, d, id, "[giran_square]", zone.PeaceZone, -3660, -3148,
		zone.Point{X: 83000, Y: 147600, Z: -3404},
		zone.Point{X: 83800, Y: 147600, Z: -3398},
		zone.Point{X: 83800, Y: 148300, Z: -3405},
		zone.Point{X: 83000, Y: 148300, Z: -3402},
	)
	steps := []struct {
		cmd  zone.Command
		want []string
	}{
		{zone.MoveVertex{Zone: id, Shape: 0, Index: 2, Point: zone.Point{X: 83900, Y: 148400, Z: -2900}},
			[]string{"83000 147600 -3660 -3148", "83800 147600 -3660 -3148", "83900 148400 -3660 -3148", "83000 148300 -3660 -3148"}},
		{zone.InsertVertex{Zone: id, Shape: 0, Index: 1, Point: zone.Point{X: 83400, Y: 147500, Z: -3401}},
			[]string{"83000 147600 -3660 -3148", "83400 147500 -3660 -3148", "83800 147600 -3660 -3148", "83900 148400 -3660 -3148", "83000 148300 -3660 -3148"}},
		{zone.InsertVertex{Zone: id, Shape: 0, Index: 5, Point: zone.Point{X: 82900, Y: 147900, Z: -3400}},
			[]string{"83000 147600 -3660 -3148", "83400 147500 -3660 -3148", "83800 147600 -3660 -3148", "83900 148400 -3660 -3148", "83000 148300 -3660 -3148", "82900 147900 -3660 -3148"}},
		{zone.RemoveVertex{Zone: id, Shape: 0, Index: 0},
			[]string{"83400 147500 -3660 -3148", "83800 147600 -3660 -3148", "83900 148400 -3660 -3148", "83000 148300 -3660 -3148", "82900 147900 -3660 -3148"}},
		{zone.MoveShape{Zone: id, Shape: 0, DX: 100, DY: -200, DZ: 50},
			[]string{"83500 147300 -3610 -3098", "83900 147400 -3610 -3098", "84000 148200 -3610 -3098", "83100 148100 -3610 -3098", "83000 147700 -3610 -3098"}},
		{zone.SetZRange{Zone: id, Shape: 0, ZMin: -3700, ZMax: -3000},
			[]string{"83500 147300 -3700 -3000", "83900 147400 -3700 -3000", "84000 148200 -3700 -3000", "83100 148100 -3700 -3000", "83000 147700 -3700 -3000"}},
	}
	for i, s := range steps {
		apply(t, d, s.cmd)
		if got := coords(t, d); !reflect.DeepEqual(got, s.want) {
			t.Errorf("step %d (%#v): coords\n%q\nwant\n%q", i, s.cmd, got, s.want)
		}
	}
	z, _ := d.Zone(id)
	if got := z.Shapes[0].Points[2].Z; got != -2850 {
		t.Errorf("moved vertex Z after the shape move: %d, want -2850 (-2900 + 50)", got)
	}
}

// Edits that point past the shape fail and leave the document as it was:
// a vertex index out of range, an insertion beyond the end, a shape or
// zone that does not exist.
func TestVertexEditsOutOfRangeFailUnchanged(t *testing.T) {
	d := zone.NewDocument()
	id := d.NewZoneID()
	polygonZone(t, d, id, "[z]", zone.PeaceZone, -10, 10,
		zone.Point{X: 0, Y: 0}, zone.Point{X: 100, Y: 0}, zone.Point{X: 100, Y: 100})
	before := compiled(t, d)
	for _, c := range []zone.Command{
		zone.MoveVertex{Zone: id, Shape: 0, Index: 3},
		zone.MoveVertex{Zone: id, Shape: 0, Index: -1},
		zone.InsertVertex{Zone: id, Shape: 0, Index: 4},
		zone.RemoveVertex{Zone: id, Shape: 0, Index: 3},
		zone.RemoveVertex{Zone: id, Shape: 1, Index: 0},
		zone.MoveShape{Zone: id + 1, Shape: 0, DX: 5},
	} {
		if err := d.Apply(c); err == nil {
			t.Errorf("Apply(%#v) succeeded, want an error", c)
		}
	}
	if got := compiled(t, d); !reflect.DeepEqual(got, before) {
		t.Errorf("failed edits changed the document:\n%q\nwant\n%q", got, before)
	}
	if !d.Undo() {
		t.Error("Undo found nothing to undo after the zone was built")
	}
}

// A rectangle stays its 2 corners: inserting, removing or appending a
// vertex fails and leaves the document as it was, while moving a corner or
// the whole rectangle shows in its compiled 2 coords. Catches a rectangle
// edited into 1 or 3 corners, which the ZoneParser cannot read.
func TestRectangleEditsKeepTwoCorners(t *testing.T) {
	d := zone.NewDocument()
	id := d.NewZoneID()
	apply(t, d,
		zone.CreateZone{ID: id, Name: "[zb_rect]", Type: zone.PeaceZone},
		zone.AddShape{Zone: id, Kind: zone.Rectangle, Banned: true, ZMin: -3700, ZMax: -3100, Points: []zone.Point{
			{X: 82000, Y: 148000, Z: -3467}, {X: 82800, Y: 148700, Z: -3404},
		}},
		// The exclusion needs an included shape to cut from, or the zone
		// has a problem and does not compile.
		zone.AddShape{Zone: id, ZMin: -3700, ZMax: -3100, Points: []zone.Point{
			{X: 81000, Y: 147000}, {X: 84000, Y: 147000}, {X: 84000, Y: 150000},
		}},
	)
	before := compiled(t, d)
	for _, c := range []zone.Command{
		zone.InsertVertex{Zone: id, Shape: 0, Index: 1, Point: zone.Point{X: 82400, Y: 148000}},
		zone.InsertVertex{Zone: id, Shape: 0, Index: 2, Point: zone.Point{X: 82400, Y: 148000}},
		zone.RemoveVertex{Zone: id, Shape: 0, Index: 1},
		zone.AddVertex{Zone: id, Shape: 0, Point: zone.Point{X: 82400, Y: 148000}},
	} {
		if err := d.Apply(c); err == nil {
			t.Errorf("Apply(%#v) on a rectangle succeeded, want an error", c)
		}
	}
	if got := compiled(t, d); !reflect.DeepEqual(got, before) {
		t.Errorf("failed rectangle edits changed the document:\n%q\nwant\n%q", got, before)
	}

	apply(t, d, zone.MoveVertex{Zone: id, Shape: 0, Index: 1, Point: zone.Point{X: 82900, Y: 148600, Z: -3400}})
	apply(t, d, zone.MoveShape{Zone: id, Shape: 0, DX: 100, DY: 100, DZ: 10})
	z, _ := d.Zone(id)
	if s := z.Shapes[0]; s.Kind != zone.Rectangle || len(s.Points) != 2 ||
		s.Points[0] != (zone.Point{X: 82100, Y: 148100, Z: -3457}) || s.Points[1] != (zone.Point{X: 83000, Y: 148700, Z: -3390}) ||
		s.ZMin != -3690 || s.ZMax != -3090 {
		t.Errorf("rectangle after moving corner 2 and the shape: %+v", s)
	}
	// The banned rectangle still compiles as the 4-corner banned_polygon of
	// its moved corners; the included polygon is untouched.
	want := []string{"82100 148100 -3690 -3090", "83000 148100 -3690 -3090", "83000 148700 -3690 -3090", "82100 148700 -3690 -3090",
		"81000 147000 -3700 -3100", "84000 147000 -3700 -3100", "84000 150000 -3700 -3100"}
	if got := coords(t, d); !reflect.DeepEqual(got, want) {
		t.Errorf("coords %q, want %q", got, want)
	}
}

// A sequence of commands followed by as many Undo calls gives back the
// initial document, step by step in reverse; as many Redo calls reapply
// every step. A new command after an Undo drops the steps that could have
// been redone.
func TestUndoReturnsEachStepAndRedoReapplies(t *testing.T) {
	d := zone.NewDocument()
	base := d.NewZoneID()
	polygonZone(t, d, base, "[base]", zone.Water, -100, 100,
		zone.Point{X: 0, Y: 0, Z: 1}, zone.Point{X: 500, Y: 0, Z: 2}, zone.Point{X: 500, Y: 500, Z: 3})
	// Undo history from the zone above stays below this point.
	initial := zone.CloneZones(d.Zones())

	id := d.NewZoneID()
	cmds := []zone.Command{
		zone.CreateZone{ID: id, Name: "[edited]", Type: zone.PeaceZone},
		zone.AddShape{Zone: id},
		zone.AddVertex{Zone: id, Shape: 0, Point: zone.Point{X: 10, Y: 10, Z: -5}},
		zone.AddVertex{Zone: id, Shape: 0, Point: zone.Point{X: 90, Y: 10, Z: -6}},
		zone.AddVertex{Zone: id, Shape: 0, Point: zone.Point{X: 90, Y: 90, Z: -7}},
		zone.SetZRange{Zone: id, Shape: 0, ZMin: -263, ZMax: 251},
		zone.MoveVertex{Zone: base, Shape: 0, Index: 1, Point: zone.Point{X: 600, Y: -50, Z: 40}},
		zone.InsertVertex{Zone: id, Shape: 0, Index: 1, Point: zone.Point{X: 50, Y: 0, Z: -5}},
		zone.RemoveVertex{Zone: base, Shape: 0, Index: 0},
		zone.MoveShape{Zone: id, Shape: 0, DX: 1000, DY: 2000, DZ: -30},
		zone.SetZRange{Zone: base, Shape: 0, ZMin: -500, ZMax: 500},
		zone.AddShape{Zone: id, Kind: zone.Rectangle, Banned: true, ZMin: -40, ZMax: 40, Points: []zone.Point{
			{X: 20, Y: 20, Z: -5}, {X: 60, Y: 70, Z: -6},
		}},
		zone.MoveVertex{Zone: id, Shape: 1, Index: 0, Point: zone.Point{X: 25, Y: 15, Z: -4}},
		zone.AddShape{Zone: base, Banned: true, ZMin: -9, ZMax: 9, Points: []zone.Point{
			{X: 100, Y: 100}, {X: 200, Y: 100}, {X: 150, Y: 200},
		}},
		zone.AddRestartPoint{Zone: id, Point: zone.Point{X: 50, Y: 50, Z: -5}},
		zone.AddRestartPoint{Zone: id, Point: zone.Point{X: 55, Y: 50, Z: -5}},
		zone.AddRestartPoint{Zone: id, PK: true, Point: zone.Point{X: 60, Y: 60, Z: -6}},
		zone.RemoveRestartPoint{Zone: id, Index: 0},
		zone.RemoveRestartPoint{Zone: id, PK: true, Index: 0},
		zone.SetType{Zone: id, Type: zone.Swamp},
		zone.SetParam{Zone: id, Name: "playerMinLevel", Value: "20"},
		zone.SetParam{Zone: id, Name: "enabled", Value: "false"},
		zone.SetParam{Zone: base, Name: "myScriptKey", Value: "a"},
		zone.SetParam{Zone: id, Name: "playerMinLevel", Value: "25"},
		zone.RemoveParam{Zone: id, Name: "playerMinLevel"},
	}
	states := [][]zone.Zone{initial}
	for _, c := range cmds {
		apply(t, d, c)
		states = append(states, zone.CloneZones(d.Zones()))
	}
	for i := len(cmds); i > 0; i-- {
		if !d.Undo() {
			t.Fatalf("Undo of step %d (%#v) reported nothing to undo", i, cmds[i-1])
		}
		if got := d.Zones(); !reflect.DeepEqual(got, states[i-1]) {
			t.Fatalf("after undoing step %d (%#v):\n%+v\nwant\n%+v", i, cmds[i-1], got, states[i-1])
		}
	}
	if !reflect.DeepEqual(d.Zones(), initial) {
		t.Fatalf("after %d undos: %+v, want the initial %+v", len(cmds), d.Zones(), initial)
	}
	for i := 1; i <= len(cmds); i++ {
		if !d.Redo() {
			t.Fatalf("Redo of step %d (%#v) reported nothing to redo", i, cmds[i-1])
		}
		if got := d.Zones(); !reflect.DeepEqual(got, states[i]) {
			t.Fatalf("after redoing step %d (%#v):\n%+v\nwant\n%+v", i, cmds[i-1], got, states[i])
		}
	}
	if d.Redo() {
		t.Error("Redo past the last step reported a change")
	}

	// Undo twice, then a new edit: the two undone steps are gone for good.
	d.Undo()
	d.Undo()
	apply(t, d, zone.MoveVertex{Zone: id, Shape: 0, Index: 0, Point: zone.Point{X: 7, Y: 7, Z: 7}})
	if d.Redo() {
		t.Error("Redo reapplied a step after a new command following Undo")
	}
	d.Undo()
	if got := d.Zones(); !reflect.DeepEqual(got, states[len(cmds)-2]) {
		t.Errorf("undo of the new edit:\n%+v\nwant\n%+v", got, states[len(cmds)-2])
	}
}

// Undo with nothing applied reports false and changes nothing; so does a
// command that fails, which leaves no step to undo.
func TestUndoWithNothingToUndo(t *testing.T) {
	d := zone.NewDocument()
	if d.Undo() || d.Redo() {
		t.Fatal("a new document has something to undo or redo")
	}
	if err := d.Apply(zone.AddShape{Zone: 1}); err == nil {
		t.Fatal("AddShape on a missing zone succeeded")
	}
	if d.Undo() {
		t.Error("a failed command left a step to undo")
	}
}
