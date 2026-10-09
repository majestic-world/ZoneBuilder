package main

import (
	"log"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// problemRows rebuilds the presentation when the document, warnings or
// language changes. The retained target slices keep row indices stable.
func (e *zoneEditor) problemRows(ws []floorWarning, wsChanged bool, lang locale.Language) (rows []ui.ProblemRow, ok bool) {
	if e.problemsAt == e.version+1 && !wsChanged && e.problemsLang == string(lang) {
		return nil, false
	}
	e.problemsAt = e.version + 1
	e.problemsLang = string(lang)
	e.problems = e.doc.Problems()
	e.warnings = ws
	rows = make([]ui.ProblemRow, 0, len(e.problems)+len(ws))
	for _, p := range e.problems {
		rows = append(rows, ui.ProblemRow{Zone: e.zoneName(p.Zone, lang), Message: p.Text(lang)})
	}
	for _, w := range ws {
		rows = append(rows, ui.ProblemRow{Zone: e.zoneName(w.zone, lang), Message: warningText(w, lang), Warning: true})
	}
	return rows, true
}

// zoneName is zone id's name as the problem panel shows it.
func (e *zoneEditor) zoneName(id zone.ZoneID, lang locale.Language) string {
	z, _ := e.doc.Zone(id)
	if z.Name == "" {
		return locale.Text(lang, "zone.problem.unnamed")
	}
	return z.Name
}

// problemCounts is how many problems each zone has.
func (e *zoneEditor) problemCounts() map[zone.ZoneID]int {
	counts := map[zone.ZoneID]int{}
	for _, p := range e.doc.Problems() {
		counts[p.Zone]++
	}
	return counts
}

// goToProblem selects the zone of the problem panel's row i, with the
// problem's shape as the current shape and its vertex selected, and frames
// where the problem is: the vertex or restart point, else the shape, else
// the zone. While a polygon is being drawn the selection stays on it, but
// the camera still goes. A floor warning's row goes to it (goToWarning).
// It returns the status line.
func (e *zoneEditor) goToProblem(i int, s *scene.World, cam *camera.Camera, lang locale.Language) string {
	if n := len(e.problems); i >= n && i-n < len(e.warnings) {
		return e.goToWarning(i, e.warnings[i-n], s, cam, lang)
	}
	if i < 0 || i >= len(e.problems) {
		return ""
	}
	p := e.problems[i]
	z, ok := e.doc.Zone(p.Zone)
	if !ok {
		return ""
	}
	e.lastProblemClick, e.hasProblemClick = i, true
	e.problemClickNoFrame, e.problemClickDrawing = false, e.drawing
	msg := e.problemClickText(lang, z.Name, p.Text(lang), false)
	if !e.drawing {
		shape := max(p.Shape, 0)
		e.zone, e.shape = z.ID, shape
		e.sel = vertexRef{zone: z.ID, shape: shape, index: p.Vertex}
		e.version++
	}
	log.Printf("zona: problema em %s: %s", z.Name, p.Text(locale.PtBR))
	if s == nil {
		return msg
	}
	b := geom.EmptyBox()
	switch {
	case p.Restart >= 0:
		pts := z.RestartPoints
		if p.PK {
			pts = z.PKRestartPoints
		}
		if p.Restart < len(pts) {
			b = pointBox(s, pts[p.Restart])
		}
	case p.Shape >= 0 && p.Shape < len(z.Shapes):
		sh := z.Shapes[p.Shape]
		if p.Vertex >= 0 && p.Vertex < len(sh.Points) {
			b = pointBox(s, sh.Points[p.Vertex])
		} else {
			for _, v := range sh.Points {
				b.Include(renderPoint(s, v))
			}
		}
	default:
		for _, sh := range z.Shapes {
			for _, v := range sh.Points {
				b.Include(renderPoint(s, v))
			}
		}
		for _, v := range z.RestartPoints {
			b.Include(renderPoint(s, v))
		}
		for _, v := range z.PKRestartPoints {
			b.Include(renderPoint(s, v))
		}
	}
	if b.Empty() {
		e.problemClickNoFrame = true
		return e.problemClickText(lang, z.Name, p.Text(lang), false)
	}
	cam.Frame(b)
	log.Printf("zona: câmera no problema: %s", formatPose(cam, s))
	return msg
}

// pointBox is the box a single server point is framed as, in s's render
// space (the same size goTo frames).
func pointBox(s *scene.World, p zone.Point) geom.Box {
	c := renderPoint(s, p)
	h := geom.Vec3{X: goToHalfSize, Y: goToHalfSize, Z: goToHalfSize}
	return geom.Box{Min: c.Sub(h), Max: c.Add(h)}
}

// badVertices is, per shape of zone id, the vertices with a problem of
// their own.
func (e *zoneEditor) badVertices(id zone.ZoneID) map[int][]int {
	bad := map[int][]int{}
	for _, p := range e.doc.ZoneProblems(id) {
		if p.Shape >= 0 && p.Vertex >= 0 {
			bad[p.Shape] = append(bad[p.Shape], p.Vertex)
		}
	}
	return bad
}

// problemClickText renders the retained click result without selecting or framing again.
func (e *zoneEditor) problemClickText(lang locale.Language, name, problem string, warning bool) string {
	var msg string
	if e.problemClickDrawing {
		if warning {
			msg = locale.Text(lang, "zone.warning.select_drawing")
		} else {
			msg = locale.Text(lang, "zone.problem.select_drawing")
		}
	} else {
		msg = locale.Format(lang, "zone.problem.selected", map[string]string{"name": name, "problem": problem})
	}
	if e.problemClickNoFrame {
		return locale.Format(lang, "zone.problem.nothing_to_frame", map[string]string{"message": msg})
	}
	return msg
}

// reformatProblemClick presents the last clicked row without selecting or
// framing it again. The loop calls this only while that click owns the status.
func (e *zoneEditor) reformatProblemClick(lang locale.Language) string {
	if !e.hasProblemClick {
		return ""
	}
	i := e.lastProblemClick
	if i >= len(e.problems) {
		j := i - len(e.problems)
		if j < 0 || j >= len(e.warnings) {
			return ""
		}
		w := e.warnings[j]
		z, ok := e.doc.Zone(w.zone)
		if !ok {
			return ""
		}
		return e.problemClickText(lang, z.Name, warningText(w, lang), true)
	}
	if i < 0 || i >= len(e.problems) {
		return ""
	}
	p := e.problems[i]
	z, ok := e.doc.Zone(p.Zone)
	if !ok {
		return ""
	}
	return e.problemClickText(lang, z.Name, p.Text(lang), false)
}

// blockedStatus presents a blocked compilation; each problem goes to the log.
func (e *zoneEditor) blockedStatus(b *zone.BlockedError, lang locale.Language) string {
	for _, p := range b.Problems {
		z, _ := e.doc.Zone(p.Zone)
		log.Printf("zona: compilação bloqueada: %s: %s", z.Name, p.Text(locale.PtBR))
	}
	return blockedStatusText(b, lang)
}

// blockedStatusText re-renders a retained blocked result without recompiling.
func blockedStatusText(b *zone.BlockedError, lang locale.Language) string {
	return locale.Plural(lang, "zone.problem.blocked_status", len(b.Problems), nil)
}
