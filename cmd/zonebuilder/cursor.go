package main

import (
	"fmt"
	"image"
	"log"
	"math"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/scene"
)

// clickSlop is how far, in pixels, the pointer may move between press and
// release and still count as a click rather than a drag to look.
const clickSlop = 3

// cursorProbe tracks the pointer over the viewport for the status bar: the
// world position under the cursor and at the last click.
type cursorProbe struct {
	cursor   f32.Point
	inside   bool
	press    f32.Point
	pressing bool
	// button is the one button the press being tracked holds.
	button pointer.Buttons
	// clicked is set after the first click; clickHit tells whether it hit.
	clicked  bool
	clickHit bool
	click    scene.Hit
}

// handle tracks one viewport pointer event and reports the button of the
// click it ends, 0 for none: a press of the primary or the secondary
// button alone, released without dragging. Pressing both (the lift)
// is no click.
func (c *cursorProbe) handle(e pointer.Event) pointer.Buttons {
	switch e.Kind {
	case pointer.Move, pointer.Drag, pointer.Enter:
		c.cursor, c.inside = e.Position, true
		if c.pressing && dist(e.Position, c.press) > clickSlop {
			c.pressing = false
		}
	case pointer.Leave, pointer.Cancel:
		c.inside, c.pressing = false, false
	case pointer.Press:
		c.cursor, c.inside = e.Position, true
		c.press, c.button = e.Position, e.Buttons
		c.pressing = e.Buttons == pointer.ButtonPrimary || e.Buttons == pointer.ButtonSecondary
	case pointer.Release:
		click := c.pressing && e.Buttons == 0 && dist(e.Position, c.press) <= clickSlop
		c.pressing = false
		if !click {
			return 0
		}
		if c.button == pointer.ButtonPrimary {
			c.clicked = true
		}
		return c.button
	}
	return 0
}

// status is the status pill's text: the server position under the cursor
// and at the last click ("" before the first), rounded to whole units as
// //pos prints them.
func (c *cursorProbe) status(s *scene.World, cam *camera.Camera, viewport image.Point, lang locale.Language) (cursor, click string) {
	if s == nil {
		return locale.Text(lang, "actions.cursor.no_map"), ""
	}
	cursor = locale.Text(lang, "actions.cursor.outside")
	if c.inside {
		h, ok := pickAt(s, cam, c.cursor, viewport)
		cursor = locale.Format(lang, "actions.cursor.current", map[string]string{"position": describeHitLang(h, ok, lang)})
	}
	if c.clicked {
		click = locale.Format(lang, "actions.cursor.last_click", map[string]string{"position": describeHitLang(c.click, c.clickHit, lang)})
	}
	return cursor, click
}

func describeHitLang(h scene.Hit, ok bool, lang locale.Language) string {
	if !ok {
		return locale.Text(lang, "actions.cursor.empty")
	}
	surface := locale.Text(lang, "actions.cursor.unknown")
	switch h.Surface {
	case scene.SurfaceTerrain:
		surface = locale.Text(lang, "actions.cursor.terrain")
	case scene.SurfaceBSP:
		surface = "BSP"
	case scene.SurfaceMesh:
		surface = locale.Text(lang, "actions.cursor.mesh")
	}
	return locale.Format(lang, "actions.cursor.position", map[string]string{
		"x": fmt.Sprint(round(h.Pos.X)), "y": fmt.Sprint(round(h.Pos.Y)),
		"z": fmt.Sprint(round(h.Pos.Z)), "surface": surface,
	})
}

// pickAt picks s through viewport pixel p.
func pickAt(s *scene.World, cam *camera.Camera, p f32.Point, viewport image.Point) (scene.Hit, bool) {
	return s.Pick(rayAt(s, cam, p, viewport))
}

// rayAt is the world ray through viewport pixel p, with a unit Dir.
func rayAt(s *scene.World, cam *camera.Camera, p f32.Point, viewport image.Point) scene.Ray {
	o, d := cam.Ray(p.X, p.Y, viewport.X, viewport.Y)
	return scene.Ray{Origin: worldPosition(s, o), Dir: scene.ToRender(d)}
}

func describeHit(h scene.Hit, ok bool) string {
	if !ok {
		return "nada sob o cursor"
	}
	return fmt.Sprintf("%d %d %d (%s)", round(h.Pos.X), round(h.Pos.Y), round(h.Pos.Z), surfaceName(h.Surface))
}

func logClick(h scene.Hit, ok bool) {
	log.Printf("clique: %s", describeHit(h, ok))
}

func surfaceName(s scene.Surface) string {
	switch s {
	case scene.SurfaceTerrain:
		return "terreno"
	case scene.SurfaceBSP:
		return "BSP"
	case scene.SurfaceMesh:
		return "static mesh"
	}
	return "superfície desconhecida"
}

func round(v float32) int { return int(math.Round(float64(v))) }

func dist(a, b f32.Point) float32 {
	d := a.Sub(b)
	return float32(math.Hypot(float64(d.X), float64(d.Y)))
}
