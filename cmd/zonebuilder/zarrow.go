package main

import (
	"fmt"
	"image"
	"math"
	"strconv"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// arrowGrab is how close, in pixels, a press must land to the Z arrow to
// grab it.
const arrowGrab = 9

// arrowReach is the world height, in server units, whose projection gives
// the arrow its screen direction and its pixels-per-unit scale.
const arrowReach = 1000

// zArrow is the selected zone's Z arrow as last laid out: from base (the
// middle of the zone's top) to tip, in viewport pixels, with the unit
// screen direction of world +Z and how many world units a pixel along it
// is.
type zArrow struct {
	ok        bool
	zone      zone.ZoneID
	base, tip f32.Point
	dir       f32.Point
	perPixel  float32
}

// zDrag is a drag of the Z arrow: dz is how far the zone moves when it is
// released.
type zDrag struct {
	active bool
	zone   zone.ZoneID
	press  f32.Point
	dir    f32.Point
	scale  float32
	dz     int
}

// layoutArrow computes the selected zone's Z arrow, length pixels long, for
// the camera, and returns how the shell draws it. There is none while a
// polygon is drawn or a tool is armed, when the zone is not ready, or when
// its top is behind the camera.
func (e *zoneEditor) layoutArrow(s *scene.World, cam *camera.Camera, vp image.Point, length int) ui.ZArrow {
	e.arrow = zArrow{}
	z, _, top, ok := e.zoneZ()
	if !ok || s == nil || e.armed {
		return ui.ZArrow{}
	}
	cx, cy, ok := zoneCenter(z)
	if !ok {
		return ui.ZArrow{}
	}
	top += e.zdrag.offset(z.ID)
	project := func(p zone.Point) (f32.Point, bool) {
		x, y, ok := cam.Project(renderPoint(s, p), vp.X, vp.Y)
		return f32.Pt(x, y), ok
	}
	base, ok := project(zone.Point{X: cx, Y: cy, Z: top})
	if !ok {
		return ui.ZArrow{}
	}
	up, upOK := project(zone.Point{X: cx, Y: cy, Z: top + arrowReach})
	d := up.Sub(base)
	n := float32(math.Hypot(float64(d.X), float64(d.Y)))
	dir, perPixel := d.Div(n), arrowReach/n
	if !upOK || n < 4 {
		// Looking straight down the Z axis: drag up the screen, at the
		// scale of a horizontal step at the zone's top.
		side, ok := project(zone.Point{X: cx + arrowReach, Y: cy, Z: top})
		h := side.Sub(base)
		hn := float32(math.Hypot(float64(h.X), float64(h.Y)))
		if !ok || hn < 1 {
			return ui.ZArrow{}
		}
		dir, perPixel = f32.Pt(0, -1), arrowReach/hn
	}
	e.arrow = zArrow{ok: true, zone: z.ID, base: base, tip: base.Add(dir.Mul(float32(length))), dir: dir, perPixel: perPixel}
	return ui.ZArrow{Visible: true, Base: e.arrow.base, Tip: e.arrow.tip, Active: e.zdrag.active}
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

// onArrow reports whether viewport pixel p lies on the arrow.
func (a zArrow) onArrow(p f32.Point) bool {
	if !a.ok {
		return false
	}
	d := a.tip.Sub(a.base)
	t := ((p.X-a.base.X)*d.X + (p.Y-a.base.Y)*d.Y) / (d.X*d.X + d.Y*d.Y)
	t = max(0, min(1, t))
	return dist(p, a.base.Add(d.Mul(t))) <= arrowGrab
}

// offset is how far the drag in progress moves zone id.
func (d zDrag) offset(id zone.ZoneID) int {
	if !d.active || d.zone != id {
		return 0
	}
	return d.dz
}

// arrowEvent follows a drag of the Z arrow: the zone shows moved while the
// pointer moves, and the release applies the move as one command. Shift
// snaps the move to the step.
func (e *zoneEditor) arrowEvent(ev pointer.Event) string {
	d := &e.zdrag
	switch ev.Kind {
	case pointer.Drag:
		moved := ev.Position.Sub(d.press)
		dz := int(math.Round(float64((moved.X*d.dir.X + moved.Y*d.dir.Y) * d.scale)))
		if ev.Modifiers.Contain(key.ModShift) && e.step > 0 {
			dz = int(math.Round(float64(dz)/float64(e.step))) * e.step
		}
		if dz != d.dz {
			d.dz = dz
			e.version++
		}
		verb := "Subindo"
		if dz < 0 {
			verb = "Descendo"
		}
		return fmt.Sprintf("%s a zona %d (Shift: passo de %d)", verb, abs(dz), e.step)
	case pointer.Release:
		dz := d.dz
		*d = zDrag{}
		e.version++
		return e.shiftZone(dz)
	case pointer.Cancel:
		*d = zDrag{}
		e.version++
	}
	return ""
}

// grabArrow starts a drag of the Z arrow when ev presses on it.
func (e *zoneEditor) grabArrow(ev pointer.Event) bool {
	if ev.Kind != pointer.Press || ev.Buttons != pointer.ButtonPrimary || !e.arrow.onArrow(ev.Position) {
		return false
	}
	e.zdrag = zDrag{active: true, zone: e.arrow.zone, press: ev.Position, dir: e.arrow.dir, scale: e.arrow.perPixel}
	e.version++
	return true
}

// heightPanel handles the floating height window's requests and fills its
// fields for the selected zone. It returns the status line, "" to keep the
// current one.
func (e *zoneEditor) heightPanel(gtx layout.Context, p *ui.HeightPanel) string {
	var msg string
	p.ReopenRequested(gtx)
	e.step = p.StepZ()
	if p.Up.Clicked(gtx) {
		msg = e.shiftZone(e.step)
	}
	if p.Down.Clicked(gtx) {
		msg = e.shiftZone(-e.step)
	}
	if p.BaseRequested(gtx) {
		msg = e.setZoneBase(p.Base.Text())
	}
	if p.HeightRequested(gtx) {
		msg = e.setZoneHeight(p.Height.Text())
	}
	z, base, top, ok := e.zoneZ()
	p.Zone = ""
	if ok {
		p.Zone = fmt.Sprintf("%s: piso z %d, topo z %d, altura %d", z.Name, base, top, top-base)
	}
	k := heightKey{zone: z.ID, ok: ok, version: e.version}
	if k != e.heightFilled && !e.zdrag.active {
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
