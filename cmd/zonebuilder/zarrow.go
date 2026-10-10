package main

import (
	"image"
	"math"
	"strconv"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// arrowGrab is how close, in pixels, a press must land to a gizmo arrow
// to grab it.
const arrowGrab = 9

// arrowReach is the world length, in server units, whose projection gives
// each arrow its screen direction and its pixels-per-unit scale.
const arrowReach = 1000

// minArrow is the shortest projection, in pixels, of arrowReach along X
// or Y that still shows that axis's arrow: below it the axis points at
// the camera and a drag along it would move the zone by leaps.
const minArrow = 4

// The gizmo's axes, in the order of ui.MoveGizmo.Arrows.
const (
	axisX = iota
	axisY
	axisZ
)

// axisArrow is one arrow of the gizmo as last laid out: its tip in
// viewport pixels, the unit screen direction of its world axis and how
// many world units a pixel along it is.
type axisArrow struct {
	ok       bool
	tip      f32.Point
	dir      f32.Point
	perPixel float32
}

// gizmo is the selected zone's move gizmo as last laid out: from base
// (the middle of the zone's top), an arrow along world X, Y and Z.
type gizmo struct {
	ok     bool
	zone   zone.ZoneID
	base   f32.Point
	arrows [3]axisArrow
}

// axisDrag is a drag of one gizmo arrow: delta is how far along axis the
// zone moves when it is released.
type axisDrag struct {
	active bool
	zone   zone.ZoneID
	axis   int
	press  f32.Point
	dir    f32.Point
	scale  float32
	delta  int
}

// layoutGizmo computes the selected zone's move gizmo, arrows length
// pixels long, for the camera, and returns how the shell draws it. There
// is none while a polygon is drawn or a tool is armed, when the zone is
// not ready, or when its top is behind the camera. An X or Y arrow that
// points at the camera is left out; the Z arrow seen from straight above
// points away from the other two, at the scale of a horizontal step.
func (e *zoneEditor) layoutGizmo(s *scene.World, cam *camera.Camera, vp image.Point, length int) ui.MoveGizmo {
	e.gizmo = gizmo{}
	z, _, top, ok := e.zoneZ()
	if !ok || s == nil || e.armed {
		return ui.MoveGizmo{}
	}
	cx, cy, ok := zoneCenter(z)
	if !ok {
		return ui.MoveGizmo{}
	}
	dx, dy, dz := e.axisDrag.offset(z.ID)
	center := zone.Point{X: cx + dx, Y: cy + dy, Z: top + dz}
	project := func(p zone.Point) (f32.Point, bool) {
		x, y, ok := cam.Project(renderPoint(s, p), vp.X, vp.Y)
		return f32.Pt(x, y), ok
	}
	base, ok := project(center)
	if !ok {
		return ui.MoveGizmo{}
	}
	g := gizmo{ok: true, zone: z.ID, base: base}
	along := func(axis int) (f32.Point, float32, bool) {
		p := center
		switch axis {
		case axisX:
			p.X += arrowReach
		case axisY:
			p.Y += arrowReach
		case axisZ:
			p.Z += arrowReach
		}
		end, ok := project(p)
		d := end.Sub(base)
		n := float32(math.Hypot(float64(d.X), float64(d.Y)))
		if !ok || n < minArrow {
			return f32.Point{}, 0, false
		}
		return d.Div(n), arrowReach / n, true
	}
	for axis := range 3 {
		if dir, perPixel, ok := along(axis); ok {
			g.arrows[axis] = axisArrow{ok: true, dir: dir, perPixel: perPixel}
		}
	}
	if !g.arrows[axisZ].ok {
		// Looking straight down the Z axis: drag away from the X and Y
		// arrows, at the scale of a horizontal step at the zone's top.
		var side f32.Point
		perPixel := float32(0)
		for _, a := range g.arrows[:axisZ] {
			if a.ok {
				side = side.Sub(a.dir)
				perPixel = a.perPixel
			}
		}
		n := float32(math.Hypot(float64(side.X), float64(side.Y)))
		if perPixel > 0 {
			dir := f32.Pt(0, -1)
			if n > 0.1 {
				dir = side.Div(n)
			}
			g.arrows[axisZ] = axisArrow{ok: true, dir: dir, perPixel: perPixel}
		}
	}
	out := ui.MoveGizmo{Visible: true, Base: base, Active: -1}
	if e.axisDrag.active {
		out.Active = e.axisDrag.axis
	}
	for axis := range g.arrows {
		a := &g.arrows[axis]
		if a.ok {
			a.tip = base.Add(a.dir.Mul(float32(length)))
			out.Arrows[axis] = ui.GizmoArrow{Visible: true, Tip: a.tip}
		}
	}
	e.gizmo = g
	return out
}

// zoneCenter is the middle of the bounding box of z's included shapes.
func zoneCenter(z zone.Zone) (x, y int, ok bool) {
	var lo, hi zone.Point
	for _, s := range z.Shapes {
		if s.Banned {
			continue
		}
		for _, p := range s.Points {
			if !ok {
				lo, hi, ok = p, p, true
				continue
			}
			lo.X, lo.Y = min(lo.X, p.X), min(lo.Y, p.Y)
			hi.X, hi.Y = max(hi.X, p.X), max(hi.Y, p.Y)
		}
	}
	return (lo.X + hi.X) / 2, (lo.Y + hi.Y) / 2, ok
}

// arrowAt is the axis of the arrow viewport pixel p lies on, the nearest
// when several are within reach.
func (g gizmo) arrowAt(p f32.Point) (int, bool) {
	if !g.ok {
		return 0, false
	}
	best, hit := float32(arrowGrab), -1
	for axis, a := range g.arrows {
		if !a.ok {
			continue
		}
		d := a.tip.Sub(g.base)
		t := ((p.X-g.base.X)*d.X + (p.Y-g.base.Y)*d.Y) / (d.X*d.X + d.Y*d.Y)
		t = max(0, min(1, t))
		// The shared base belongs to no arrow in particular.
		if t < 0.15 {
			continue
		}
		if dd := dist(p, g.base.Add(d.Mul(t))); dd <= best {
			best, hit = dd, axis
		}
	}
	return hit, hit >= 0
}

// offset is how far the drag in progress moves zone id on each axis.
func (d axisDrag) offset(id zone.ZoneID) (dx, dy, dz int) {
	if !d.active || d.zone != id {
		return 0, 0, 0
	}
	switch d.axis {
	case axisX:
		return d.delta, 0, 0
	case axisY:
		return 0, d.delta, 0
	}
	return 0, 0, d.delta
}

// axisNames name the axes in the status messages.
var axisNames = [3]string{"X", "Y", "Z"}

// gizmoEvent follows a drag of a gizmo arrow: the zone shows moved while
// the pointer moves, and the release applies the move as one command.
// Shift snaps the move to the step.
func (e *zoneEditor) gizmoEvent(ev pointer.Event) string {
	d := &e.axisDrag
	switch ev.Kind {
	case pointer.Drag:
		moved := ev.Position.Sub(d.press)
		delta := int(math.Round(float64((moved.X*d.dir.X + moved.Y*d.dir.Y) * d.scale)))
		if ev.Modifiers.Contain(key.ModShift) && e.step > 0 {
			delta = int(math.Round(float64(delta)/float64(e.step))) * e.step
		}
		if delta != d.delta {
			d.delta = delta
			e.version++
		}
		args := map[string]string{"delta": intArg(abs(delta)), "step": intArg(e.step)}
		if d.axis != axisZ {
			args["delta"], args["axis"] = signedArg(delta), axisNames[d.axis]
			return e.present(locale.Message{Key: "editor.arrow.moving", Args: args})
		}
		if delta < 0 {
			return e.present(locale.Message{Key: "editor.arrow.lowering", Args: args})
		}
		return e.present(locale.Message{Key: "editor.arrow.raising", Args: args})
	case pointer.Release:
		dx, dy, dz := d.offset(d.zone)
		*d = axisDrag{}
		e.version++
		return e.moveZone(dx, dy, dz)
	case pointer.Cancel:
		*d = axisDrag{}
		e.version++
	}
	return ""
}

// signedArg formats v with its sign, "+0" for 0.
func signedArg(v int) string {
	if v < 0 {
		return "−" + intArg(-v)
	}
	return "+" + intArg(v)
}

// grabGizmo starts a drag of the gizmo arrow ev presses on.
func (e *zoneEditor) grabGizmo(ev pointer.Event) bool {
	if ev.Kind != pointer.Press || ev.Buttons != pointer.ButtonPrimary {
		return false
	}
	axis, ok := e.gizmo.arrowAt(ev.Position)
	if !ok {
		return false
	}
	a := e.gizmo.arrows[axis]
	e.axisDrag = axisDrag{active: true, zone: e.gizmo.zone, axis: axis, press: ev.Position, dir: a.dir, scale: a.perPixel}
	e.version++
	return true
}

// heightPanel handles the floating height window's requests and fills its
// fields for the selected zone, fitting it to the floor of s by c's
// profiles. It returns the status line, "" to keep the current one.
func (e *zoneEditor) heightPanel(gtx layout.Context, p *ui.HeightPanel, s *scene.World, c *floorCoverage) string {
	var msg string
	p.ReopenRequested(gtx)
	e.step = p.StepZ()
	if p.Up.Clicked(gtx) {
		msg = e.moveZone(0, 0, e.step)
	}
	if p.Down.Clicked(gtx) {
		msg = e.moveZone(0, 0, -e.step)
	}
	if p.BaseRequested(gtx) {
		msg = e.setZoneBase(p.Base.Text())
	}
	if p.HeightRequested(gtx) {
		msg = e.setZoneHeight(p.Height.Text())
	}
	if p.FloorToGround.Clicked(gtx) {
		msg = e.zoneToGround(c, s, false)
	}
	if p.TopToGround.Clicked(gtx) {
		msg = e.zoneToGround(c, s, true)
	}
	z, base, top, ok := e.zoneZ()
	p.Zone = ""
	if ok {
		p.Zone = locale.Format(e.Language, "editor.height.zone", map[string]string{"name": z.Name, "min": intArg(base), "max": intArg(top), "height": intArg(top-base)})
	}
	k := heightKey{zone: z.ID, ok: ok, version: e.version}
	if k != e.heightFilled && !e.axisDrag.active {
		e.heightFilled = k
		if ok {
			p.Base.SetText(strconv.Itoa(base))
			p.Height.SetText(strconv.Itoa(top - base))
		}
	}
	return msg
}

// heightKey is the zone and version the height fields were last filled
// for.
type heightKey struct {
	zone    zone.ZoneID
	ok      bool
	version int
}
