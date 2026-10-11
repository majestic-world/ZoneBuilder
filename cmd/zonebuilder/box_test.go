package main

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// boxShape is shape i of e's selected zone as a box; it fails the test
// unless that shape is a box.
func boxShape(t *testing.T, e *zoneEditor, i int) zone.Box {
	t.Helper()
	z, ok := e.doc.Zone(e.zone)
	if !ok || i >= len(z.Shapes) {
		t.Fatalf("zone %d has no shape %d", e.zone, i)
	}
	b, ok := zone.BoxOf(z.Shapes[i])
	if !ok {
		t.Fatalf("shape %d is not a box: %+v", i, z.Shapes[i])
	}
	return b
}

// placedBox is an editor whose zone [nave] holds the box a click at
// 83000 147000 -3000 places.
func placedBox(t *testing.T) (*zoneEditor, zone.Box) {
	t.Helper()
	e := newZoneEditor()
	e.Language = locale.PtBR
	e.create("[nave]", zone.PeaceZone, ui.ToolBox)
	e.boxClick(zone.Point{X: 83000, Y: 147000, Z: -3000})
	return e, boxShape(t, e, 0)
}

// A click with the box tool adds a 2048 × 2048 × 1024 rectangle centred on
// the click, the clicked Z at mid-height, puts the tool down and makes the
// box the current shape, whose fields the panel edits.
func TestBoxClickPlacesDefaultBoxCenteredOnClick(t *testing.T) {
	e, got := placedBox(t)
	want := zone.Box{Frame: zone.Frame{CX: 83000, CY: 147000, Width: 2048, Length: 2048}, ZMin: -3512, ZMax: -2488}
	if got != want {
		t.Errorf("box %+v, want %+v", got, want)
	}
	z, _ := e.doc.Zone(e.zone)
	if s := z.Shapes[0]; s.Kind != zone.Rectangle || len(s.Points) != 2 {
		t.Errorf("a new box is not a 2-corner rectangle: %+v", s)
	}
	if e.armed || e.shape != 0 {
		t.Errorf("after the click: armed %v, current shape %d; want put down, shape 0", e.armed, e.shape)
	}
}

// Dragging a face handle shows the box resized, Shift snapping to the
// step, and the release applies it as 1 undo step; a face dragged past the
// opposite one stops 1 unit from it, the opposite face staying put.
func TestBoxFaceDragSnapsAndAppliesOneStep(t *testing.T) {
	e, box := placedBox(t)
	vp := image.Pt(1280, 800)

	// Up the screen is outward for the top face, 2 units per pixel: 50 px
	// is 100 units, which Shift snaps to 2 steps of 64.
	e.boxDrag = boxDrag{active: true, zone: e.zone, shape: 0, face: faceTop, dir: f32.Pt(0, -1), scale: 2, from: box}
	e.boxDragEvent(nil, nil, pointer.Event{Kind: pointer.Drag, Position: f32.Pt(0, -50), Modifiers: key.ModShift}, vp)
	z, _ := e.doc.Zone(e.zone)
	if lo, hi := e.shownZRange(e.zone, 0, z.Shapes[0]); lo != -3512 || hi != -2360 {
		t.Errorf("shown range while dragging %d..%d, want -3512..-2360", lo, hi)
	}
	if got := boxShape(t, e, 0); got != box {
		t.Errorf("the drag changed the document before the release: %+v", got)
	}
	e.boxDragEvent(nil, nil, pointer.Event{Kind: pointer.Release}, vp)
	if got := boxShape(t, e, 0).ZMax; got != -2360 {
		t.Errorf("top after the release %d, want -2360", got)
	}
	e.undo()
	if got := boxShape(t, e, 0); got != box {
		t.Errorf("box after 1 undo %+v, want %+v", got, box)
	}

	e.boxDrag = boxDrag{active: true, zone: e.zone, shape: 0, face: faceXMax, dir: f32.Pt(1, 0), scale: 1, from: box}
	e.boxDragEvent(nil, nil, pointer.Event{Kind: pointer.Drag, Position: f32.Pt(-5000, 0)}, vp)
	e.boxDragEvent(nil, nil, pointer.Event{Kind: pointer.Release}, vp)
	got := boxShape(t, e, 0)
	if got.Width != 1 || got.Corners()[0].X != box.Corners()[0].X {
		t.Errorf("+X face dragged past −X: %+v, want width 1 with −X at %d", got, box.Corners()[0].X)
	}
}

// Releasing the turn handle turns the box into a polygon of its turned
// corners that the panel still edits as a box, in 1 undo step.
func TestBoxTurnReleaseTurnsTheBoxInOneStep(t *testing.T) {
	e, box := placedBox(t)
	e.boxDrag = boxDrag{active: true, zone: e.zone, shape: 0, face: faceTurn, yaw: 30, from: box}
	e.boxDragEvent(nil, nil, pointer.Event{Kind: pointer.Release}, image.Pt(1280, 800))
	want := box
	want.Yaw = 30
	if got := boxShape(t, e, 0); got != want {
		t.Errorf("turned box %+v, want %+v", got, want)
	}
	z, _ := e.doc.Zone(e.zone)
	if s := z.Shapes[0]; s.Kind != zone.Polygon || len(s.Points) != 4 {
		t.Errorf("a turned box is not a 4-corner polygon: %+v", s)
	}
	e.undo()
	if got := boxShape(t, e, 0); got != box {
		t.Errorf("box after 1 undo %+v, want %+v", got, box)
	}
}

// Stretching a face of a turned box moves its center half the stretch
// along the box's own axis, so the opposite face stays where it was.
// Catches a stretch along the world's X instead of the box's.
func TestResizedTurnedBoxGrowsAlongItsOwnAxis(t *testing.T) {
	b := zone.Box{Frame: zone.Frame{CX: 83000, CY: 147000, Width: 2048, Length: 1024, Yaw: 30}, ZMin: 0, ZMax: 100}
	got := resized(b, faceXMax, 100)
	// Half of 100 along (cos 30°, sin 30°) is 43.3, 25.
	if got.Width != 2148 || got.Length != 1024 || got.CX != 83043 || got.CY != 147025 {
		t.Errorf("resized %+v, want width 2148 centred at 83043 147025", got)
	}
}

// Moving a box slider of the edit panel, here by a click that comes and
// goes within a frame, applies that number to the box in 1 undo step: the
// width around the center, then the yaw, which turns the box. Catches a
// slider whose release is missed, so nothing reaches the document.
func TestBoxSliderAppliesItsNumberInOneStep(t *testing.T) {
	e, box := placedBox(t)
	p := &ui.EditPanel{}
	gtx := layout.Context{Ops: new(op.Ops)}
	e.boxSliders(gtx, p) // put the sliders on the box

	p.BoxSliders[ui.BoxSlideWidth].Value = ui.BoxSlidePos(ui.BoxSlideWidth, 10000)
	e.boxSliders(gtx, p)
	want := box
	want.Width = 10000
	if got := boxShape(t, e, 0); got != want {
		t.Errorf("box after the width slider %+v, want %+v", got, want)
	}

	p.BoxSliders[ui.BoxSlideYaw].Value = ui.BoxSlidePos(ui.BoxSlideYaw, 90)
	e.boxSliders(gtx, p)
	want.Yaw = 90
	if got := boxShape(t, e, 0); got != want {
		t.Errorf("box after the yaw slider %+v, want %+v", got, want)
	}
	e.undo()
	want.Yaw = 0
	if got := boxShape(t, e, 0); got != want {
		t.Errorf("box after 1 undo %+v, want %+v", got, want)
	}
}
