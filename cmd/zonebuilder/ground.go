package main

import (
	"image"
	"math"

	"gioui.org/f32"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/render"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// minLabelEdge is how long, in viewport pixels, an edge must look for its
// length to be drawn on it: shorter ones would pile their labels up.
const minLabelEdge = 70

// groundKey is what the renderer's ground marking is built from: the
// zones' version, the selected zone and whether the Chão switch is on.
type groundKey struct {
	version int
	zone    zone.ZoneID
	on      bool
}

// ground is what the renderer marks on the ground: the terrain grid while
// grid is set (the Chão switch), and always the selected zone's
// footprint, its closed shapes as shown (a drag in progress included).
// The polygon being drawn is still open and has no footprint.
func (e *zoneEditor) ground(grid bool) render.Ground {
	g := render.Ground{Grid: grid}
	z, ok := e.doc.Zone(e.zone)
	if !ok || z.Hidden {
		return g
	}
	g.Color = linearColor(z.DisplayColor())
	for i, s := range z.Shapes {
		if e.drawing && i == e.shape {
			continue
		}
		pts := outline(s.Kind, e.shownPoints(z.ID, i, s.Points))
		if len(pts) < 3 {
			continue
		}
		zmin, zmax := e.shownZRange(z.ID, i, s)
		gs := render.GroundShape{Points: make([]geom.Vec3, len(pts)), ZMin: float32(zmin), ZMax: float32(zmax), Banned: s.Banned}
		for k, p := range pts {
			gs.Points[k] = serverVec(p)
		}
		g.Shapes = append(g.Shapes, gs)
	}
	return g
}

// outline is a shape's outline: a rectangle's 4 corners from its 2, the
// points of any other shape.
func outline(kind zone.ShapeKind, pts []zone.Point) []zone.Point {
	if kind == zone.Rectangle && len(pts) == 2 {
		c := zone.RectangleCorners(pts[0], pts[1])
		return c[:]
	}
	return pts
}

// edgeLabels are the horizontal lengths of the selected zone's edges, at
// their midpoints on the viewport of size vp seen by cam: every edge of
// its shapes, and of the polygon being drawn, that looks at least
// minLabelEdge pixels long.
func (e *zoneEditor) edgeLabels(s *scene.World, cam *camera.Camera, vp image.Point, lang locale.Language) []ui.EdgeLabel {
	z, ok := e.doc.Zone(e.zone)
	if !ok || z.Hidden {
		return nil
	}
	project := func(p zone.Point) (f32.Point, bool) {
		x, y, ok := cam.Project(renderPoint(s, p), vp.X, vp.Y)
		return f32.Pt(x, y), ok
	}
	var labels []ui.EdgeLabel
	for i, sh := range z.Shapes {
		pts := outline(sh.Kind, e.shownPoints(z.ID, i, sh.Points))
		open := e.drawing && i == e.shape
		n := len(pts)
		if open {
			n-- // no closing edge yet
		}
		if len(pts) < 2 {
			continue
		}
		for k := range n {
			a, b := pts[k], pts[(k+1)%len(pts)]
			pa, okA := project(a)
			pb, okB := project(b)
			if !okA || !okB || dist(pa, pb) < minLabelEdge {
				continue
			}
			length := math.Hypot(float64(b.X-a.X), float64(b.Y-a.Y))
			labels = append(labels, ui.EdgeLabel{At: pa.Add(pb).Mul(0.5), Text: locale.Number(lang, math.Round(length), 0)})
		}
	}
	return labels
}

// measure describes a shape's outline on the ground: the size of its
// bounding box, its area and its perimeter, in server units.
func measure(lang locale.Language, pts []zone.Point) string {
	if len(pts) < 3 {
		return ""
	}
	lo, hi := pts[0], pts[0]
	var area2, perimeter float64
	for k, a := range pts {
		b := pts[(k+1)%len(pts)]
		lo.X, lo.Y = min(lo.X, a.X), min(lo.Y, a.Y)
		hi.X, hi.Y = max(hi.X, a.X), max(hi.Y, a.Y)
		area2 += float64(a.X)*float64(b.Y) - float64(b.X)*float64(a.Y)
		perimeter += math.Hypot(float64(b.X-a.X), float64(b.Y-a.Y))
	}
	return locale.Format(lang, "editor.ground.measure", map[string]string{
		"width": locale.Number(lang, float64(hi.X-lo.X), 0), "length": locale.Number(lang, float64(hi.Y-lo.Y), 0),
		"area": locale.Number(lang, math.Round(math.Abs(area2)/2), 0), "perimeter": locale.Number(lang, math.Round(perimeter), 0),
	})
}

