package main

import (
	"fmt"
	"log"
	"slices"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/coverage"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/zone"
)

// floorWarning is a floor warning of shape shape of zone zone (spec D6).
type floorWarning struct {
	zone  zone.ZoneID
	shape int
	coverage.Warning
}

// warningsKey is what every zone's floor warnings are judged from: the
// document and overlay (e's version), the world and its scenes, the
// hidden meshes and the profiles received.
type warningsKey struct {
	version    int
	world      *scene.World
	scenes     string
	hideMeshes bool
	received   int
}

// warnings are the floor warnings of every zone's included shapes over w,
// zone by zone in the document's order, and whether they changed since
// the last call. A shape is judged once its first profile is in; asking
// for every shape measures the stale ones lazily, one at a time, after
// whatever the frame asked for before. None without a world.
func (c *floorCoverage) warnings(e *zoneEditor, w *scene.World) ([]floorWarning, bool) {
	c.receive(e)
	k := warningsKey{version: e.version, world: w, received: c.received}
	if w != nil {
		k.scenes, k.hideMeshes = scenesKey(w), w.HideMeshes
	}
	if k == c.warnedAt {
		return c.warned, false
	}
	c.warnedAt = k
	var ws []floorWarning
	if w != nil {
		for _, z := range e.doc.Zones() {
			for i, sh := range z.Shapes {
				if sh.Banned || (e.drawing && z.ID == e.zone && i == e.shape) {
					continue
				}
				r, _, ok := c.shape(e, w, z.ID, i, sh)
				if !ok {
					continue
				}
				for _, wn := range r.Warnings {
					ws = append(ws, floorWarning{z.ID, i, wn})
				}
			}
		}
	}
	if slices.Equal(ws, c.warned) {
		return c.warned, false
	}
	c.warned = ws
	return ws, true
}

// warningText is w's line in the problem panel.
func warningText(w floorWarning) string {
	var s string
	switch w.Kind {
	case coverage.AboveTop:
		s = fmt.Sprintf("chão acima do topo (folga %s, %s do chão)", signed(roundF(w.Clearance)), percent(w.Share, 1))
	case coverage.BelowFloor:
		s = fmt.Sprintf("chão abaixo do piso (folga %s, %s do chão)", signed(roundF(w.Clearance)), percent(w.Share, 1))
	case coverage.TightTop:
		s = fmt.Sprintf("folga apertada no topo: %s, abaixo de %d", signed(roundF(w.Clearance)), coverage.MinClearance)
	case coverage.TightFloor:
		s = fmt.Sprintf("folga apertada no piso: %s, abaixo de %d", signed(roundF(w.Clearance)), coverage.MinClearance)
	case coverage.NoGround:
		s = fmt.Sprintf("área sem chão medido: %s u²", units(roundF(w.Area)))
	}
	return fmt.Sprintf("shape %d: %s", w.shape+1, s)
}

// goToWarning selects w's zone, with its shape as the current shape, and
// frames its worst floor, or the shape for area with no floor. While a
// polygon is being drawn the selection stays on it, but the camera still
// goes. It returns the status line.
func (e *zoneEditor) goToWarning(w floorWarning, s *scene.World, cam *camera.Camera) string {
	z, ok := e.doc.Zone(w.zone)
	if !ok || w.shape >= len(z.Shapes) {
		return ""
	}
	msg := z.Name + ": " + warningText(w)
	if e.drawing {
		msg = "Feche o polígono atual para selecionar o aviso"
	} else {
		e.zone, e.shape = z.ID, w.shape
		e.sel = vertexRef{zone: z.ID, shape: w.shape, index: -1}
		e.version++
	}
	log.Printf("zona: aviso em %s: %s", z.Name, warningText(w))
	if s == nil {
		return msg
	}
	if w.Kind == coverage.NoGround {
		b := geom.EmptyBox()
		for _, v := range z.Shapes[w.shape].Points {
			b.Include(renderPoint(s, v))
		}
		if b.Empty() {
			return msg + " (nada para enquadrar)"
		}
		cam.Frame(b)
	} else {
		cam.Frame(pointBox(s, zone.Point{X: roundF(w.At.X), Y: roundF(w.At.Y), Z: roundF(w.At.Z)}))
	}
	log.Printf("zona: câmera no aviso: %s", formatPose(cam, s))
	return msg
}
