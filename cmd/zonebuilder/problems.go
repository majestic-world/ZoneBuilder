package main

import (
	"fmt"
	"log"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// problemRows is the problem panel's rows, when the document changed
// since they were last built (ok); the problems they show are kept, which
// a click refers to.
func (e *zoneEditor) problemRows() (rows []ui.ProblemRow, ok bool) {
	if e.problemsAt == e.version+1 {
		return nil, false
	}
	e.problemsAt = e.version + 1
	e.problems = e.doc.Problems()
	rows = make([]ui.ProblemRow, len(e.problems))
	for i, p := range e.problems {
		z, _ := e.doc.Zone(p.Zone)
		name := z.Name
		if name == "" {
			name = "(sem nome)"
		}
		rows[i] = ui.ProblemRow{Zone: name, Message: p.Message}
	}
	return rows, true
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
// the camera still goes. It returns the status line.
func (e *zoneEditor) goToProblem(i int, s *scene.Scene, cam *camera.Camera) string {
	if i < 0 || i >= len(e.problems) {
		return ""
	}
	p := e.problems[i]
	z, ok := e.doc.Zone(p.Zone)
	if !ok {
		return ""
	}
	msg := z.Name + ": " + p.Message
	if e.drawing {
		msg = "Feche o polígono atual para selecionar o problema"
	} else {
		shape := max(p.Shape, 0)
		e.zone, e.shape = z.ID, shape
		e.sel = vertexRef{zone: z.ID, shape: shape, index: p.Vertex}
		e.version++
	}
	log.Printf("zona: problema em %s: %s", z.Name, p.Message)
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
		return msg + " (nada para enquadrar)"
	}
	cam.Frame(b)
	log.Printf("zona: câmera no problema: %s", formatPose(cam, s))
	return msg
}

// pointBox is the box a single server point is framed as, in s's render
// space (the same size goTo frames).
func pointBox(s *scene.Scene, p zone.Point) geom.Box {
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

// blockedStatus is the status line of a compilation the selected zones'
// problems blocked; each problem goes to the log.
func (e *zoneEditor) blockedStatus(b *zone.BlockedError) string {
	for _, p := range b.Problems {
		z, _ := e.doc.Zone(p.Zone)
		log.Printf("zona: compilação bloqueada: %s: %s", z.Name, p.Message)
	}
	return fmt.Sprintf("Compilação bloqueada, nada foi gravado: %s nas zonas selecionadas (veja Problemas)",
		count(len(b.Problems), "problema", "problemas"))
}
