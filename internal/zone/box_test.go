package zone_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"zonebuilder/internal/zone"
)

// boxZone is a document with zone id holding one rectangle, drawn max
// corner first.
func boxZone(t *testing.T) (*zone.Document, zone.ZoneID) {
	t.Helper()
	d := zone.NewDocument()
	id := d.NewZoneID()
	apply(t, d,
		zone.CreateZone{ID: id, Name: "[zb_nave]", Type: zone.PeaceZone},
		zone.AddShape{Zone: id, Kind: zone.Rectangle, ZMin: -3660, ZMax: -3148, Points: []zone.Point{
			{X: 83800, Y: 148300, Z: -3400}, {X: 83000, Y: 147600, Z: -3410},
		}},
	)
	return d, id
}

// SetBox with no yaw keeps a rectangle, rewritten as its min corner, then
// its max corner, with the box's Z range, in 1 undo step. Catches corners
// left in drawn order, a Z range left behind, or an edit that cannot be
// undone.
func TestSetBoxRewritesRectangleAsOrderedCorners(t *testing.T) {
	d, id := boxZone(t)
	before := coords(t, d)
	apply(t, d, zone.SetBox{Zone: id, Shape: 0, Box: zone.BoxAt(83400, 147950, 1000, 800, -3700, 600)})
	want := []string{"82900 147550 -3700 -3100", "83900 148350 -3700 -3100"}
	if got := coords(t, d); !reflect.DeepEqual(got, want) {
		t.Errorf("coords after SetBox %q, want %q", got, want)
	}
	if xml := compiled(t, d)[0]; !reflect.DeepEqual(shapeBlocks(xml), []string{"rectangle 82900 147550 -3700 -3100; 83900 148350 -3700 -3100"}) {
		t.Errorf("a box with no yaw no longer compiles as a rectangle:\n%s", xml)
	}
	if !d.Undo() {
		t.Fatal("SetBox left nothing to undo")
	}
	if got := coords(t, d); !reflect.DeepEqual(got, before) {
		t.Errorf("coords after Undo %q, want %q", got, before)
	}
}

// A turned box compiles as the polygon of its 4 turned corners and stays a
// box: moving the shape moves its center, saving and loading keeps it, and
// turning it back to 0 makes it a rectangle again. Editing a vertex makes
// it a plain polygon, which undo turns back into the box. Catches a turn
// lost on save, a center left behind by a move, or a box edited into a
// shape SetBox would then rebuild from stale numbers.
func TestTurnedBoxIsAPolygonThatStaysABox(t *testing.T) {
	d, id := boxZone(t)
	turned := zone.Box{Frame: zone.Frame{CX: 83000, CY: 147000, Width: 400, Length: 200, Yaw: 90}, ZMin: -3700, ZMax: -3100}
	apply(t, d, zone.SetBox{Zone: id, Shape: 0, Box: turned})
	want := []string{"polygon 83100 146800 -3700 -3100; 83100 147200 -3700 -3100; 82900 147200 -3700 -3100; 82900 146800 -3700 -3100"}
	if got := shapeBlocks(compiled(t, d)[0]); !reflect.DeepEqual(got, want) {
		t.Errorf("turned box compiled as %q, want %q", got, want)
	}

	apply(t, d, zone.MoveShape{Zone: id, Shape: 0, DX: 10, DY: -20, DZ: 5})
	saved, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	loaded := zone.NewDocument()
	if err := json.Unmarshal(saved, loaded); err != nil {
		t.Fatal(err)
	}
	z, _ := loaded.Zone(id)
	got, ok := zone.BoxOf(z.Shapes[0])
	moved := zone.Box{Frame: zone.Frame{CX: 83010, CY: 146980, Width: 400, Length: 200, Yaw: 90}, ZMin: -3695, ZMax: -3095}
	if !ok || got != moved {
		t.Errorf("moved and reloaded box %+v (ok %v), want %+v", got, ok, moved)
	}

	apply(t, d, zone.MoveVertex{Zone: id, Shape: 0, Index: 0, Point: zone.Point{X: 83200, Y: 146700}})
	z, _ = d.Zone(id)
	if _, ok := zone.BoxOf(z.Shapes[0]); ok || z.Shapes[0].Kind != zone.Polygon {
		t.Errorf("a turned box with a moved vertex is still a box: %+v", z.Shapes[0])
	}
	if err := d.Apply(zone.SetBox{Zone: id, Shape: 0, Box: turned}); err == nil {
		t.Error("SetBox on the edited polygon succeeded, want an error")
	}
	d.Undo()
	z, _ = d.Zone(id)
	if _, ok := zone.BoxOf(z.Shapes[0]); !ok {
		t.Errorf("undoing the vertex move did not give the box back: %+v", z.Shapes[0])
	}

	turned.Yaw = 360
	apply(t, d, zone.SetBox{Zone: id, Shape: 0, Box: turned})
	z, _ = d.Zone(id)
	if s := z.Shapes[0]; s.Kind != zone.Rectangle || s.Frame != nil || len(s.Points) != 2 {
		t.Errorf("a box turned to 360° is not a rectangle again: %+v", s)
	}
}

// SetBox fails, leaving the document as it was, on a polygon that is no
// box, on a box with no width or no length, and on a box upside down.
func TestSetBoxRejectsPolygonsAndDegenerateBoxes(t *testing.T) {
	d := zone.NewDocument()
	id := d.NewZoneID()
	polygonZone(t, d, id, "[zb_poly]", zone.PeaceZone, -3700, -3100,
		zone.Point{X: 81000, Y: 147000}, zone.Point{X: 84000, Y: 147000}, zone.Point{X: 84000, Y: 150000})
	apply(t, d, zone.AddShape{Zone: id, Kind: zone.Rectangle, ZMin: -3700, ZMax: -3100, Points: []zone.Point{
		{X: 82000, Y: 148000}, {X: 82800, Y: 148700},
	}})
	before := coords(t, d)
	for _, c := range []zone.Command{
		zone.SetBox{Zone: id, Shape: 0, Box: zone.BoxAt(82000, 148000, 100, 100, -3700, 100)},
		zone.SetBox{Zone: id, Shape: 1, Box: zone.BoxAt(82400, 148350, 0, 700, -3700, 600)},
		zone.SetBox{Zone: id, Shape: 1, Box: zone.BoxAt(82400, 148350, 800, 0, -3700, 600)},
		zone.SetBox{Zone: id, Shape: 1, Box: zone.BoxAt(82400, 148350, 800, 700, -3100, -600)},
	} {
		if err := d.Apply(c); err == nil {
			t.Errorf("Apply(%#v) succeeded, want an error", c)
		}
	}
	if got := coords(t, d); !reflect.DeepEqual(got, before) {
		t.Errorf("failed SetBox changed the document:\n%q\nwant\n%q", got, before)
	}
}

// A box placed by its center keeps that center and its size through the
// rectangle it is stored as, for odd sizes and negative coordinates.
// Catches (min+max)/2, which truncates toward 0 and drifts the box by 1
// unit on every edit west of X 0.
func TestBoxCenterRoundTripsOddSizes(t *testing.T) {
	d, id := boxZone(t)
	b := zone.BoxAt(-83001, 147001, 2047, 1023, -3700, 0)
	apply(t, d, zone.SetBox{Zone: id, Shape: 0, Box: b})
	z, _ := d.Zone(id)
	if got, ok := zone.BoxOf(z.Shapes[0]); !ok || got != b {
		t.Errorf("box read back %+v, want %+v", got, b)
	}
}

// Yaws are kept to a tenth of a degree in [0, 360).
func TestNormalYaw(t *testing.T) {
	for _, c := range []struct{ in, want float64 }{
		{0, 0}, {-30, 330}, {360, 0}, {725.04, 5}, {359.96, 0}, {12.34, 12.3},
	} {
		if got := zone.NormalYaw(c.in); got != c.want {
			t.Errorf("NormalYaw(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
