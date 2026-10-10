package main

import (
	"log"
	"slices"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/coverage"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/locale"
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
	scenes     uint64
	hideMeshes bool
	hidden     *scene.HiddenActors
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
		k.scenes, k.hideMeshes, k.hidden = scenesKey(w), w.HideMeshes, w.Hidden
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

// warningText presents a warning without adding it to blocking problems.
func warningText(w floorWarning, lang locale.Language) string {
	index := locale.Number(lang, float64(w.shape+1), 0)
	signedClearance := func() string {
		value := roundF(w.Clearance)
		if value < 0 {
			return "−" + locale.Number(lang, float64(-value), 0)
		}
		return "+" + locale.Number(lang, float64(value), 0)
	}
	switch w.Kind {
	case coverage.AboveTop:
		return locale.Format(lang, "zone.warning.above_top", map[string]string{
			"index": index, "clearance": signedClearance(), "share": locale.Percent(lang, w.Share, 1),
		})
	case coverage.BelowFloor:
		return locale.Format(lang, "zone.warning.below_floor", map[string]string{
			"index": index, "clearance": signedClearance(), "share": locale.Percent(lang, w.Share, 1),
		})
	case coverage.TightTop:
		return locale.Format(lang, "zone.warning.tight_top", map[string]string{
			"index": index, "clearance": signedClearance(), "minimum": locale.Number(lang, float64(coverage.MinClearance), 0),
		})
	case coverage.TightFloor:
		return locale.Format(lang, "zone.warning.tight_floor", map[string]string{
			"index": index, "clearance": signedClearance(), "minimum": locale.Number(lang, float64(coverage.MinClearance), 0),
		})
	case coverage.NoGround:
		return locale.Format(lang, "zone.warning.no_ground", map[string]string{
			"index": index, "area": locale.Number(lang, float64(roundF(w.Area)), 0),
		})
	}
	return ""
}

// goToWarning selects w's zone, with its shape as the current shape, and
// frames its worst floor, or the shape for area with no floor. While a
// polygon is being drawn the selection stays on it, but the camera still
// goes. It returns the status line.
func (e *zoneEditor) goToWarning(i int, w floorWarning, s *scene.World, cam *camera.Camera, lang locale.Language) string {
	z, ok := e.doc.Zone(w.zone)
	if !ok || w.shape >= len(z.Shapes) {
		return ""
	}
	e.lastProblemClick, e.hasProblemClick = i, true
	e.problemClickNoFrame, e.problemClickDrawing = false, e.drawing
	msg := e.problemClickText(lang, z.Name, warningText(w, lang), true)
	if !e.drawing {
		e.zone, e.shape = z.ID, w.shape
		e.sel = vertexRef{zone: z.ID, shape: w.shape, index: -1}
		e.version++
	}
	log.Printf("zona: aviso em %s: %s", z.Name, warningText(w, locale.PtBR))
	if s == nil {
		return msg
	}
	if w.Kind == coverage.NoGround {
		b := geom.EmptyBox()
		for _, v := range z.Shapes[w.shape].Points {
			b.Include(renderPoint(s, v))
		}
		if b.Empty() {
			e.problemClickNoFrame = true
			return e.problemClickText(lang, z.Name, warningText(w, lang), true)
		}
		cam.Frame(b)
	} else {
		cam.Frame(pointBox(s, zone.Point{X: roundF(w.At.X), Y: roundF(w.At.Y), Z: roundF(w.At.Z)}))
	}
	log.Printf("zona: câmera no aviso: %s", formatPose(cam, s))
	return msg
}
