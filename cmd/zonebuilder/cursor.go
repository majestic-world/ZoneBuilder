package main

import (
	"fmt"
	"image"
	"log"
	"math"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"zonebuilder/internal/camera"
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
	// clicked is set after the first click; clickHit tells whether it hit.
	clicked  bool
	clickHit bool
	click    scene.Hit
}

// handle tracks one viewport pointer event and reports whether it ends a
// click: a primary-button press released without dragging.
func (c *cursorProbe) handle(e pointer.Event) bool {
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
		c.press, c.pressing = e.Position, e.Buttons == pointer.ButtonPrimary
	case pointer.Release:
		click := c.pressing && e.Buttons == 0 && dist(e.Position, c.press) <= clickSlop
		c.pressing = false
		if click {
			c.clicked = true
		}
		return click
	}
	return false
}

// status is the status bar text: the server position under the cursor and
// at the last click, rounded to whole units as //pos prints them.
func (c *cursorProbe) status(s *scene.Scene, cam *camera.Camera, viewport image.Point) string {
	if s == nil {
		return "Abra um mapa para ver as coordenadas sob o cursor"
	}
	cursor := "Cursor: fora do viewport"
	if c.inside {
		h, ok := pickAt(s, cam, c.cursor, viewport)
		cursor = "Cursor: " + describeHit(h, ok)
	}
	click := "Clique: nenhum"
	if c.clicked {
		click = "Clique: " + describeHit(c.click, c.clickHit)
	}
	return cursor + "    " + click
}

// pickAt picks s through viewport pixel p.
func pickAt(s *scene.Scene, cam *camera.Camera, p f32.Point, viewport image.Point) (scene.Hit, bool) {
	o, d := cam.Ray(p.X, p.Y, viewport.X, viewport.Y)
	return s.Pick(scene.Ray{Origin: worldPosition(s, o), Dir: scene.ToRender(d)})
}

func describeHit(h scene.Hit, ok bool) string {
	if !ok {
		return "fora do terreno"
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
