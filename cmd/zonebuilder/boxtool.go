package main

import (
	"image"
	"log"
	"math"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// A box (zone.Box) is a rectangle, or a polygon of 4 corners with a Frame
// once turned, edited by its center, size, base and yaw: the box tool
// places one with a click, the edit panel types its numbers, the handles on
// its 6 faces stretch it face by face and the turn handle on its top corner
// turns it around its center.

// boxSide and boxHeight are a new box's width and length, and height.
const (
	boxSide   = 2048
	boxHeight = 1024
)

// The box faces along the box's own axes, in the order of ui.BoxHandles;
// face/2 is its axis.
const (
	faceXMax = iota
	faceXMin
	faceYMax
	faceYMin
	faceTop
	faceBottom
	// faceTurn is the turn handle, which turns the box instead of moving
	// a face.
	faceTurn
	// faceSlide is a slider of the edit panel, which sets one of the
	// box's numbers.
	faceSlide
)

// faceNames name the faces in the status messages.
var faceNames = [6]string{"+X", "−X", "+Y", "−Y", "+Z", "−Z"}

// boxGrab is how close, in pixels, a press must land to a box handle to
// grab it.
const boxGrab = 9

// turnSnap is the step, in degrees, Shift snaps a turn to.
const turnSnap = 15

// faceHandle is one box handle as last laid out: where it is and, for a
// face, the unit screen direction of its outward normal and the world
// units per pixel along it.
type faceHandle struct {
	ok       bool
	at, dir  f32.Point
	perPixel float32
}

// boxFaces are the current box's handles as last laid out.
type boxFaces struct {
	ok      bool
	box     zone.Box
	handles [6]faceHandle
	turn    faceHandle
}

// boxDrag is a drag of a handle of the box shape of zone, from the box at
// the press: of face, delta being how far it moves outward; of the turn
// handle (face faceTurn), angle0 being the world angle around the center
// the press pointed at and yaw the box's yaw so far; or of a panel slider
// (face faceSlide), to being the box so far.
type boxDrag struct {
	active bool
	zone   zone.ZoneID
	shape  int
	face   int
	press  f32.Point
	dir    f32.Point
	scale  float32
	delta  int
	angle0 float64
	yaw    float64
	from   zone.Box
	to     zone.Box
}

// shown is the box shape of zone id as the drag in progress shows it.
func (d boxDrag) shown(id zone.ZoneID, shape int) (zone.Box, bool) {
	if !d.active || d.zone != id || d.shape != shape {
		return zone.Box{}, false
	}
	switch d.face {
	case faceTurn:
		b := d.from
		b.Yaw = d.yaw
		return b, true
	case faceSlide:
		return d.to, true
	}
	return resized(d.from, d.face, d.delta), true
}

// resized is b with face moved delta outward along its normal, kept at
// least 1 wide and long and 0 tall. The opposite face stays put: exactly
// with yaw 0, within rounding when turned.
func resized(b zone.Box, face, delta int) zone.Box {
	c, s := b.Axes()
	// shift moves the center off units along the box's own X (alongX) or Y.
	shift := func(off int, alongX bool) {
		dx, dy := c, s
		if !alongX {
			dx, dy = -s, c
		}
		b.CX += int(math.Round(float64(off) * dx))
		b.CY += int(math.Round(float64(off) * dy))
	}
	switch face {
	case faceXMax, faceXMin:
		w := max(b.Width+delta, 1)
		off := w/2 - b.Width/2 // the -X side stays
		if face == faceXMin {
			off = (b.Width - b.Width/2) - (w - w/2) // the +X side stays
		}
		shift(off, true)
		b.Width = w
	case faceYMax, faceYMin:
		l := max(b.Length+delta, 1)
		off := l/2 - b.Length/2
		if face == faceYMin {
			off = (b.Length - b.Length/2) - (l - l/2)
		}
		shift(off, false)
		b.Length = l
	case faceTop:
		b.ZMax = max(b.ZMax+delta, b.ZMin)
	case faceBottom:
		b.ZMin = min(b.ZMin-delta, b.ZMax)
	}
	return b
}

// faceCenter is the middle of face of b and its outward normal, a unit
// world direction.
func faceCenter(b zone.Box, face int) (zone.Point, geom.Vec3) {
	loX, hiX, loY, hiY := -float64(b.Width/2), float64(b.Width-b.Width/2), -float64(b.Length/2), float64(b.Length-b.Length/2)
	midX, midY := (loX+hiX)/2, (loY+hiY)/2
	cz := b.ZMin + (b.ZMax-b.ZMin)/2
	c, s := b.Axes()
	switch face {
	case faceXMax:
		return b.At(hiX, midY, cz), geom.Vec3{X: float32(c), Y: float32(s)}
	case faceXMin:
		return b.At(loX, midY, cz), geom.Vec3{X: float32(-c), Y: float32(-s)}
	case faceYMax:
		return b.At(midX, hiY, cz), geom.Vec3{X: float32(-s), Y: float32(c)}
	case faceYMin:
		return b.At(midX, loY, cz), geom.Vec3{X: float32(s), Y: float32(-c)}
	case faceTop:
		return b.At(midX, midY, b.ZMax), geom.Vec3{Z: 1}
	}
	return b.At(midX, midY, b.ZMin), geom.Vec3{Z: -1}
}

// turnCorner is where b's turn handle sits: its top corner on its own +X
// +Y side.
func turnCorner(b zone.Box) zone.Point {
	return b.At(float64(b.Width-b.Width/2), float64(b.Length-b.Length/2), b.ZMax)
}

// yawArg formats a yaw in degrees for messages and the panel, with a
// point for decimals, as typed values are.
func yawArg(yaw float64) string { return strconv.FormatFloat(yaw, 'f', -1, 64) }

// boxArgs are the message arguments describing box b of zone name.
func boxArgs(name string, b zone.Box) map[string]string {
	w, l, h := b.Size()
	return map[string]string{"name": name, "x": intArg(b.CX), "y": intArg(b.CY), "width": intArg(w), "length": intArg(l), "height": intArg(h), "min": intArg(b.ZMin), "max": intArg(b.ZMax), "yaw": yawArg(b.Yaw)}
}

// boxClick adds to the selected zone a box of the default size centred on
// v, v's Z at mid-height. The Z range comes from the click, not from the
// floor: a box is meant for what floats or towers over the ground.
func (e *zoneEditor) boxClick(v zone.Point) string {
	b := zone.BoxAt(v.X, v.Y, boxSide, boxSide, v.Z-boxHeight/2, boxHeight)
	c := b.Corners()
	z, _ := e.doc.Zone(e.zone)
	if e.apply(zone.AddShape{Zone: e.zone, Kind: zone.Rectangle, Banned: e.banned, Points: []zone.Point{c[0], c[2]}, ZMin: b.ZMin, ZMax: b.ZMax}) != nil {
		return e.present(locale.Message{Key: "editor.box.add_failed"})
	}
	e.armed, e.anchored, e.hovering = false, false, false
	e.shape = len(z.Shapes)
	what := "Caixa"
	if e.banned {
		what = "Exclusão em caixa"
	}
	w, l, h := b.Size()
	log.Printf("zona: %s: %s centro %d %d, %d × %d × %d, z %d..%d", z.Name, what, v.X, v.Y, w, l, h, b.ZMin, b.ZMax)
	if e.banned {
		return e.present(locale.Message{Key: "editor.box.exclusion_done", Args: boxArgs(z.Name, b)})
	}
	return e.present(locale.Message{Key: "editor.box.done", Args: boxArgs(z.Name, b)})
}

// currentBox is the current shape when it is a box, not while a polygon is
// drawn.
func (e *zoneEditor) currentBox() (zone.Zone, bool) {
	z, sh, ok := e.currentShape()
	_, box := zone.BoxOf(sh)
	return z, ok && !e.drawing && box
}

// shownBox is shape i of zone id as a box, as the overlay draws it: as a
// box handle drag shows it, or moved by a gizmo or Ctrl shape drag.
func (e *zoneEditor) shownBox(id zone.ZoneID, i int) (zone.Box, bool) {
	if b, ok := e.boxDrag.shown(id, i); ok {
		return b, true
	}
	z, ok := e.doc.Zone(id)
	if !ok || i < 0 || i >= len(z.Shapes) {
		return zone.Box{}, false
	}
	s := z.Shapes[i]
	b, ok := zone.BoxOf(s)
	if !ok {
		return zone.Box{}, false
	}
	dx, dy, _ := e.axisDrag.offset(id)
	if d := e.drag; d.kind == dragShape && d.moved && id == e.zone && i == e.shape {
		dx, dy = dx+d.to.X-d.from.X, dy+d.to.Y-d.from.Y
	}
	b.CX, b.CY = b.CX+dx, b.CY+dy
	b.ZMin, b.ZMax = e.shownZRange(id, i, s)
	return b, true
}

// setBox makes the current box the one typed in the panel: center "x y",
// size "width length height", base "z" and yaw in degrees (a comma or a
// point for decimals).
func (e *zoneEditor) setBox(center, size, base, yaw string) string {
	if _, ok := e.currentBox(); !ok {
		return e.present(locale.Message{Key: "editor.box.select"})
	}
	c, okC := ints(center, 2)
	sz, okS := ints(size, 3)
	b, okB := ints(base, 1)
	y, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(yaw), ",", "."), 64)
	if !okC || !okS || !okB || err != nil || math.IsNaN(y) || math.IsInf(y, 0) {
		return e.present(locale.Message{Key: "editor.box.format"})
	}
	if sz[0] <= 0 || sz[1] <= 0 || sz[2] < 0 {
		return e.present(locale.Message{Key: "editor.box.size"})
	}
	box := zone.BoxAt(c[0], c[1], sz[0], sz[1], b[0], sz[2])
	box.Yaw = y
	return e.setBoxTo(box)
}

// setBoxTo makes the current box b, in 1 undo step.
func (e *zoneEditor) setBoxTo(b zone.Box) string {
	z, ok := e.currentBox()
	if !ok {
		return e.present(locale.Message{Key: "editor.box.select"})
	}
	b.Yaw = zone.NormalYaw(b.Yaw)
	if e.apply(zone.SetBox{Zone: e.zone, Shape: e.shape, Box: b}) != nil {
		return e.present(locale.Message{Key: "editor.box.edit_failed"})
	}
	w, l, h := b.Size()
	log.Printf("zona: %s, caixa %d: centro %d %d, %d × %d × %d, z %d..%d, rotação %s°", z.Name, e.shape+1, b.CX, b.CY, w, l, h, b.ZMin, b.ZMax, yawArg(b.Yaw))
	return e.present(locale.Message{Key: "editor.box.set", Args: boxArgs(z.Name, b)})
}

// layoutBoxHandles computes the current box's handles for the camera and
// returns how the shell draws them. There are none while a tool is armed
// or a polygon drawn, when the current shape is not a box or its zone is
// hidden. A face whose normal points at the camera has no handle: a drag
// along it would move the face by leaps.
func (e *zoneEditor) layoutBoxHandles(s *scene.World, cam *camera.Camera, vp image.Point) ui.BoxHandles {
	e.boxFaces = boxFaces{}
	out := ui.BoxHandles{Active: -1}
	if s == nil || e.armed {
		return out
	}
	z, ok := e.currentBox()
	if !ok || z.Hidden {
		return out
	}
	b, ok := e.shownBox(z.ID, e.shape)
	if !ok {
		return out
	}
	project := func(p geom.Vec3) (f32.Point, bool) {
		w := scene.FromServer(p)
		x, y, ok := cam.Project(scene.ToRender(w.Sub(s.Origin)), vp.X, vp.Y)
		return f32.Pt(x, y), ok
	}
	f := boxFaces{ok: true, box: b}
	for face := range f.handles {
		c, n := faceCenter(b, face)
		at, ok := project(serverVec(c))
		if !ok {
			continue
		}
		end, ok := project(serverVec(c).Add(n.Scale(arrowReach)))
		d := end.Sub(at)
		l := float32(math.Hypot(float64(d.X), float64(d.Y)))
		if !ok || l < minArrow {
			continue
		}
		f.handles[face] = faceHandle{ok: true, at: at, dir: d.Div(l), perPixel: arrowReach / l}
		out.Visible[face], out.At[face] = true, at
	}
	if at, ok := project(serverVec(turnCorner(b))); ok {
		f.turn = faceHandle{ok: true, at: at}
		out.Turn, out.TurnAt = true, at
	}
	e.boxFaces = f
	if e.boxDrag.active {
		out.Active = e.boxDrag.face
	}
	return out
}

// planeAt is where the ray through viewport pixel p meets the horizontal
// plane at server height z, in server x y; ok is false when the ray runs
// along the plane or away from it.
func planeAt(s *scene.World, cam *camera.Camera, p f32.Point, vp image.Point, z int) (x, y float64, ok bool) {
	r := rayAt(s, cam, p, vp)
	zc := float64(scene.FromServer(geom.Vec3{Z: float32(z)}).Z)
	dz := float64(r.Dir.Z)
	if math.Abs(dz) < 1e-6 {
		return 0, 0, false
	}
	t := (zc - float64(r.Origin.Z)) / dz
	if t <= 0 {
		return 0, 0, false
	}
	return float64(r.Origin.X) + t*float64(r.Dir.X), float64(r.Origin.Y) + t*float64(r.Dir.Y), true
}

// turnAngle is the world angle, in degrees, around b's center of the point
// of the plane of b's top under viewport pixel p.
func turnAngle(s *scene.World, cam *camera.Camera, p f32.Point, vp image.Point, b zone.Box) (float64, bool) {
	x, y, ok := planeAt(s, cam, p, vp, b.ZMax)
	if !ok || (x == float64(b.CX) && y == float64(b.CY)) {
		return 0, false
	}
	return math.Atan2(y-float64(b.CY), x-float64(b.CX)) * 180 / math.Pi, true
}

// grabBoxFace starts a drag of the box handle ev presses on.
func (e *zoneEditor) grabBoxFace(s *scene.World, cam *camera.Camera, ev pointer.Event, vp image.Point) bool {
	if ev.Kind != pointer.Press || ev.Buttons != pointer.ButtonPrimary || !e.boxFaces.ok {
		return false
	}
	best, hit := float32(boxGrab), -1
	for face, h := range e.boxFaces.handles {
		if h.ok {
			if d := dist(ev.Position, h.at); d <= best {
				best, hit = d, face
			}
		}
	}
	if h := e.boxFaces.turn; h.ok && dist(ev.Position, h.at) <= best {
		hit = faceTurn
	}
	if hit < 0 {
		return false
	}
	b := e.boxFaces.box
	if hit == faceTurn {
		a, ok := turnAngle(s, cam, ev.Position, vp, b)
		if !ok {
			return false
		}
		e.boxDrag = boxDrag{active: true, zone: e.zone, shape: e.shape, face: faceTurn, press: ev.Position, angle0: a, yaw: b.Yaw, from: b}
		e.version++
		return true
	}
	h := e.boxFaces.handles[hit]
	e.boxDrag = boxDrag{active: true, zone: e.zone, shape: e.shape, face: hit, press: ev.Position, dir: h.dir, scale: h.perPixel, from: b}
	e.version++
	return true
}

// boxGrabMessage is the status line for the box handle just grabbed.
func (e *zoneEditor) boxGrabMessage() string {
	if e.boxDrag.face == faceTurn {
		return e.present(locale.Message{Key: "editor.box.turn_hint", Args: map[string]string{"step": intArg(turnSnap)}})
	}
	return e.present(locale.Message{Key: "editor.box.drag_hint", Args: map[string]string{"face": faceNames[e.boxDrag.face], "step": intArg(e.step)}})
}

// boxDragEvent follows a drag of a box handle: the box shows resized or
// turned while the pointer moves, and the release applies it as one
// command. Shift snaps a face to the step and a turn to turnSnap degrees.
func (e *zoneEditor) boxDragEvent(s *scene.World, cam *camera.Camera, ev pointer.Event, vp image.Point) string {
	d := &e.boxDrag
	if d.face == faceSlide {
		return "" // the edit panel's slider owns this drag (boxSliders)
	}
	switch ev.Kind {
	case pointer.Drag:
		if d.face == faceTurn {
			a, ok := turnAngle(s, cam, ev.Position, vp, d.from)
			if !ok {
				return ""
			}
			yaw := d.from.Yaw + a - d.angle0
			if ev.Modifiers.Contain(key.ModShift) {
				yaw = math.Round(yaw/turnSnap) * turnSnap
			}
			if yaw = zone.NormalYaw(yaw); yaw != d.yaw {
				d.yaw = yaw
				e.version++
			}
			return e.present(locale.Message{Key: "editor.box.turning", Args: map[string]string{"yaw": yawArg(d.yaw), "step": intArg(turnSnap)}})
		}
		moved := ev.Position.Sub(d.press)
		delta := int(math.Round(float64((moved.X*d.dir.X + moved.Y*d.dir.Y) * d.scale)))
		if ev.Modifiers.Contain(key.ModShift) && e.step > 0 {
			delta = int(math.Round(float64(delta)/float64(e.step))) * e.step
		}
		if delta != d.delta {
			d.delta = delta
			e.version++
		}
		w, l, h := resized(d.from, d.face, d.delta).Size()
		return e.present(locale.Message{Key: "editor.box.resizing", Args: map[string]string{"face": faceNames[d.face], "delta": signedArg(delta), "width": intArg(w), "length": intArg(l), "height": intArg(h), "step": intArg(e.step)}})
	case pointer.Release:
		b, _ := d.shown(d.zone, d.shape)
		moved := d.delta != 0 || (d.face == faceTurn && d.yaw != d.from.Yaw)
		*d = boxDrag{}
		e.version++
		if !moved {
			return ""
		}
		return e.setBoxTo(b)
	case pointer.Cancel:
		*d = boxDrag{}
		e.version++
	}
	return ""
}

// slid is b with slider i of the edit panel at value v: the width or the
// length around the center, the height over the base, or the yaw, to a
// whole degree.
func slid(b zone.Box, i int, v float64) zone.Box {
	switch i {
	case ui.BoxSlideWidth:
		b.Width = max(int(math.Round(v)), 1)
	case ui.BoxSlideLength:
		b.Length = max(int(math.Round(v)), 1)
	case ui.BoxSlideHeight:
		b.ZMax = b.ZMin + max(int(math.Round(v)), 0)
	case ui.BoxSlideYaw:
		b.Yaw = zone.NormalYaw(math.Round(v))
	}
	return b
}

// slideValue is the number of b slider i of the edit panel shows.
func slideValue(b zone.Box, i int) float64 {
	switch i {
	case ui.BoxSlideWidth:
		return float64(b.Width)
	case ui.BoxSlideLength:
		return float64(b.Length)
	case ui.BoxSlideHeight:
		return float64(b.ZMax - b.ZMin)
	}
	return b.Yaw
}

// boxSliders follows the edit panel's box sliders: moving one shows the
// current box with that number changed, and letting go applies it as one
// command. A slider moved is one held, or one whose value is no longer the
// one this put there (a click comes and goes within a frame). Unheld
// sliders are put back on the shown box's numbers, so they follow the
// fields and the handles. It returns the status line, "" to keep it.
func (e *zoneEditor) boxSliders(gtx layout.Context, p *ui.EditPanel) string {
	p.UpdateBoxSliders(gtx)
	sliding := e.boxDrag.active && e.boxDrag.face == faceSlide
	b, ok := e.shownBox(e.zone, e.shape)
	if _, box := e.currentBox(); !ok || !box {
		if sliding {
			e.boxDrag = boxDrag{}
			e.version++
		}
		return ""
	}
	held, moved := false, -1
	for i := range p.BoxSliders {
		f := &p.BoxSliders[i]
		held = held || f.Dragging()
		if f.Dragging() || f.Value != e.slidSet[i] {
			moved = i
		}
	}
	var msg string
	if moved >= 0 && (sliding || !e.boxDrag.active) {
		if !sliding {
			e.boxDrag = boxDrag{active: true, zone: e.zone, shape: e.shape, face: faceSlide, from: b, to: b}
			sliding = true
		}
		if to := slid(e.boxDrag.to, moved, ui.BoxSlideValue(moved, p.BoxSliders[moved].Value)); to != e.boxDrag.to {
			e.boxDrag.to = to
			e.version++
		}
		w, l, h := e.boxDrag.to.Size()
		msg = e.present(locale.Message{Key: "editor.box.sliding", Args: map[string]string{"width": intArg(w), "length": intArg(l), "height": intArg(h), "yaw": yawArg(e.boxDrag.to.Yaw)}})
	}
	if sliding && !held {
		from, to := e.boxDrag.from, e.boxDrag.to
		e.boxDrag = boxDrag{}
		e.version++
		msg = ""
		if to != from {
			msg = e.setBoxTo(to)
		}
	}
	if b, ok := e.shownBox(e.zone, e.shape); ok {
		for i := range p.BoxSliders {
			if f := &p.BoxSliders[i]; !f.Dragging() {
				f.Value = ui.BoxSlidePos(i, slideValue(b, i))
			}
			e.slidSet[i] = p.BoxSliders[i].Value
		}
	}
	return msg
}
