package main

import (
	"image"
	"log"

	"gioui.org/f32"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/coverage"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/render"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// worstPinColor is the colour of the worst-point pins (spec D4e); a pin
// whose clearance is negative is drawn as a problem, in red.
var worstPinColor = [3]float32{1, 0.8, 0.15}

// worstPin is one worst point of the current shape: the highest floor
// (with the top clearance) or the lowest (with the floor clearance).
type worstPin struct {
	at    coverage.Spot
	clearance int
	alert bool // the clearance is negative
	top   bool // the highest floor, else the lowest
}

// worstPins are r's pins, the top one first; none without measured floor.
func worstPins(r coverage.Report) []worstPin {
	if !r.Measured {
		return nil
	}
	return []worstPin{
		{at: r.GroundMax, clearance: roundF(r.TopClearance), alert: roundF(r.TopClearance) < 0, top: true},
		{at: r.GroundMin, clearance: roundF(r.FloorClearance), alert: roundF(r.FloorClearance) < 0},
	}
}

// signed writes n as units does, always with its sign.
func signed(n int) string {
	if n < 0 {
		return "−" + units(-n)
	}
	return "+" + units(n)
}

func (p worstPin) label(lang locale.Language) string {
	key := "editor.pin.floor"
	if p.top {
		key = "editor.pin.top"
	}
	value := locale.Number(lang, float64(abs(p.clearance)), 0)
	if p.clearance < 0 {
		value = "−" + value
	} else {
		value = "+" + value
	}
	return locale.Format(lang, key, map[string]string{"clearance": value})
}

func pinMessage(p worstPin) locale.Message {
	key := "editor.pin.lowest"
	if p.top {
		key = "editor.pin.highest"
	}
	return locale.Message{Key: key, Args: map[string]string{"clearance": intArg(p.clearance)}}
}

func pinStatus(p worstPin, lang locale.Language) string {
	msg := pinMessage(p)
	clearance := locale.Number(lang, float64(abs(p.clearance)), 0)
	if p.clearance < 0 {
		clearance = "−" + clearance
	} else {
		clearance = "+" + clearance
	}
	msg.Args = map[string]string{"clearance": clearance}
	return msg.Render(lang)
}

// pins are the worst points of e's current shape over w, from c's report
// classified by the Z range shown this frame: the labels follow a Z drag.
// None while the selected zone is hidden, like its overlay.
func (c *floorCoverage) pins(e *zoneEditor, w *scene.World) []worstPin {
	if z, ok := e.doc.Zone(e.zone); !ok || z.Hidden {
		return nil
	}
	r, _, ok := c.current(e, w)
	if !ok {
		return nil
	}
	return worstPins(r)
}

// point is the pin's foot as a server point.
func (p worstPin) point() zone.Point {
	return zone.Point{X: roundF(p.at.X), Y: roundF(p.at.Y), Z: roundF(p.at.Z)}
}

// samePinShapes tells whether a and b draw the same overlay pins (the
// labels' numbers may differ).
func samePinShapes(a, b []worstPin) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].point() != b[i].point() || a[i].alert != b[i].alert {
			return false
		}
	}
	return true
}

// pinShapes are the overlay pins of pins, drawn like restartPin.
func pinShapes(pins []worstPin) []render.ZoneShape {
	shapes := make([]render.ZoneShape, len(pins))
	for i, p := range pins {
		shapes[i] = restartPin(p.point(), worstPinColor)
		shapes[i].Problem = p.alert
	}
	return shapes
}

// pinLabels are the labels of pins on the viewport of size vp seen by cam,
// at the top of each pin; a pin off screen has none, but keeps its index
// so PinClicked maps back to pins.
func pinLabels(pins []worstPin, s *scene.World, cam *camera.Camera, vp image.Point, lang locale.Language) []ui.EdgeLabel {
	labels := make([]ui.EdgeLabel, 0, len(pins))
	for _, p := range pins {
		top := p.point()
		top.Z += restartPinHeight
		x, y, ok := cam.Project(renderPoint(s, top), vp.X, vp.Y)
		if !ok {
			x, y = -1e6, -1e6 // off the viewport
		}
		labels = append(labels, ui.EdgeLabel{At: f32.Pt(x, y), Text: p.label(lang), Alert: p.alert})
	}
	return labels
}

// goToPin frames pin p like a problem's vertex (pointBox); it returns the
// status line.
func goToPin(p worstPin, s *scene.World, cam *camera.Camera, lang locale.Language) string {
	pt := p.point()
	msg := pinStatus(p, lang)
	if s == nil {
		return msg
	}
	cam.Frame(pointBox(s, pt))
	log.Printf("zona: câmera no pior ponto %d %d %d (%s): %s", pt.X, pt.Y, pt.Z, p.label(lang), formatPose(cam, s))
	return msg
}
