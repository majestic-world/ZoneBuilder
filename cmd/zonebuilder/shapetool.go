package main

import (
	"fmt"
	"log"
	"math"
	"slices"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/render"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// Overlay colours (linear RGB): exclusions, restart points and player-killer
// restart points stand apart from the zones' included shapes.
var (
	bannedColor    = [3]float32{0.1, 0.45, 1}
	restartColor   = [3]float32{0.1, 0.9, 0.2}
	pkRestartColor = [3]float32{0.9, 0.05, 0.6}
)

const (
	// minCircleRadius is the smallest circle the tool takes, in server
	// units: below it the polygon's rounded vertices start to merge.
	minCircleRadius = 32
	// restartPinHeight is how far above its point a restart point's pin
	// reaches in the overlay.
	restartPinHeight = 160
	// groundAbove and groundReach bound the vertical pick that drops a
	// generated vertex (a circle vertex, a rectangle's other corners) onto
	// the surface under it: it starts groundAbove over the clicked height
	// and takes a hit no further than groundReach from it, so a roof or a
	// cave floor far from the drawing level is not picked.
	groundAbove = zone.DefaultZMargin
	groundReach = 1024
)

// arm makes tool take the viewport clicks for the selected zone; a shape
// tool draws an exclusion when banned is set. It returns the status line.
func (e *zoneEditor) arm(tool ui.Tool, banned bool) string {
	if e.drawing {
		return "Feche o polígono atual antes de trocar de ferramenta"
	}
	if _, ok := e.doc.Zone(e.zone); !ok {
		return "Crie uma zona antes de escolher uma ferramenta"
	}
	e.tool, e.armed, e.banned = tool, true, banned && tool.IsShape()
	e.anchored, e.hovering = false, false
	e.version++
	return e.hint()
}

// hint is the armed tool's instruction, or "" when none is armed.
func (e *zoneEditor) hint() string {
	if !e.armed {
		return ""
	}
	what := ""
	if e.banned {
		what = " (exclusão)"
	}
	switch e.tool {
	case ui.ToolRectangle:
		if e.anchored {
			return "Retângulo" + what + ": clique no canto oposto; Esc cancela"
		}
		return "Retângulo" + what + ": clique no 1º canto; Esc cancela"
	case ui.ToolCircle:
		if e.anchored {
			return "Círculo" + what + ": clique na borda para dar o raio; Esc cancela"
		}
		return "Círculo" + what + ": clique no centro; Esc cancela"
	case ui.ToolRestart:
		return "Cada clique marca um restart_point; Esc termina"
	case ui.ToolPKRestart:
		return "Cada clique marca um PKrestart_point; Esc termina"
	}
	if e.drawing {
		return "Polígono" + what + ": clique adiciona vértice; Enter ou clique no 1º vértice fecha"
	}
	return "Polígono" + what + ": clique na superfície para o 1º vértice; Esc cancela"
}

// escape puts the armed tool down, dropping a rectangle's first corner or
// a circle's center. An open polygon stays: it is in the document and
// closes with Enter. It returns the status line.
func (e *zoneEditor) escape() string {
	switch {
	case e.drawing:
		return "O polígono está aberto: Enter ou clique no 1º vértice fecha"
	case !e.armed:
		return ""
	}
	e.armed, e.anchored, e.hovering = false, false, false
	e.version++
	return "Ferramenta guardada"
}

// rectangleClick takes corner v: the first one anchors the rectangle, the
// second adds it to the selected zone with a Z range covering the surface
// under all 4 corners.
func (e *zoneEditor) rectangleClick(s *scene.Scene, v zone.Point) string {
	if !e.anchored {
		e.anchor, e.anchored = v, true
		e.version++
		return fmt.Sprintf("1º canto em %d %d %d; clique no canto oposto", v.X, v.Y, v.Z)
	}
	a := e.anchor
	if a.X == v.X || a.Y == v.Y {
		return "O retângulo precisa de largura e altura: clique noutro canto"
	}
	c := zone.RectangleCorners(a, v)
	c[1].Z = groundZ(s, c[1].X, c[1].Y, c[1].Z)
	c[3].Z = groundZ(s, c[3].X, c[3].Y, c[3].Z)
	zmin, zmax := zone.SuggestZRange(c[:], e.margin)
	z, _ := e.doc.Zone(e.zone)
	if !e.apply(zone.AddShape{Zone: e.zone, Kind: zone.Rectangle, Banned: e.banned, Points: []zone.Point{a, v}, ZMin: zmin, ZMax: zmax}) {
		return "Não foi possível adicionar o retângulo"
	}
	e.armed, e.anchored, e.hovering = false, false, false
	e.shape = len(z.Shapes)
	what := "Retângulo"
	if e.banned {
		what = "Exclusão retangular"
	}
	log.Printf("zona: %s: %s %d %d … %d %d, z %d..%d", z.Name, what, a.X, a.Y, v.X, v.Y, zmin, zmax)
	return fmt.Sprintf("%s em %s: %d %d … %d %d, z %d … %d", what, z.Name, a.X, a.Y, v.X, v.Y, zmin, zmax)
}

// circleClick takes the center, then a point on the circle: the circle
// goes into the selected zone as a polygon of zone.CircleSides vertices,
// each dropped onto the surface under it.
func (e *zoneEditor) circleClick(s *scene.Scene, v zone.Point) string {
	if !e.anchored {
		e.anchor, e.anchored = v, true
		e.version++
		return fmt.Sprintf("Centro em %d %d %d; clique na borda para dar o raio", v.X, v.Y, v.Z)
	}
	c := e.anchor
	r := int(math.Round(math.Hypot(float64(v.X-c.X), float64(v.Y-c.Y))))
	if r < minCircleRadius {
		return fmt.Sprintf("Raio de %d: o mínimo é %d; clique mais longe do centro", r, minCircleRadius)
	}
	pts := zone.CirclePoints(c, r, zone.CircleSides)
	for i := range pts {
		pts[i].Z = groundZ(s, pts[i].X, pts[i].Y, c.Z)
	}
	zmin, zmax := zone.SuggestZRange(append(slices.Clone(pts), c), e.margin)
	z, _ := e.doc.Zone(e.zone)
	if !e.apply(zone.AddShape{Zone: e.zone, Banned: e.banned, Points: pts, ZMin: zmin, ZMax: zmax}) {
		return "Não foi possível adicionar o círculo"
	}
	e.armed, e.anchored, e.hovering = false, false, false
	e.shape = len(z.Shapes)
	what := "Círculo"
	if e.banned {
		what = "Exclusão circular"
	}
	log.Printf("zona: %s: %s centro %d %d raio %d → polígono de %d vértices, z %d..%d", z.Name, what, c.X, c.Y, r, len(pts), zmin, zmax)
	return fmt.Sprintf("%s em %s: raio %d, polígono de %s, z %d … %d", what, z.Name, r, count(len(pts), "vértice", "vértices"), zmin, zmax)
}

// restartClick adds v to the selected zone's restart points (player
// killers' with the PK tool).
func (e *zoneEditor) restartClick(v zone.Point) string {
	pk := e.tool == ui.ToolPKRestart
	if !e.apply(zone.AddRestartPoint{Zone: e.zone, PK: pk, Point: v}) {
		return "Não foi possível marcar o ponto"
	}
	z, _ := e.doc.Zone(e.zone)
	name, n := "restart_point", len(z.RestartPoints)
	if pk {
		name, n = "PKrestart_point", len(z.PKRestartPoints)
	}
	log.Printf("zona: %s: %s %d em %d %d %d", z.Name, name, n, v.X, v.Y, v.Z)
	return fmt.Sprintf("%s %d de %s em %d %d %d", name, n, z.Name, v.X, v.Y, v.Z)
}

// hoverAt tracks the surface point under the cursor (h, when ok) while a
// rectangle or circle is anchored, for the preview.
func (e *zoneEditor) hoverAt(h scene.Hit, ok bool) {
	ok = ok && e.armed && e.anchored
	p := zone.Point{}
	if ok {
		p = zone.Point{X: round(h.Pos.X), Y: round(h.Pos.Y), Z: round(h.Pos.Z)}
	}
	if ok != e.hovering || p != e.hover {
		e.hover, e.hovering = p, ok
		e.version++
	}
}

// preview is the rectangle or circle being placed: from the anchor to the
// cursor, or the anchor alone while the cursor is off every surface.
func (e *zoneEditor) preview() (render.ZoneShape, bool) {
	if !e.armed || !e.anchored {
		return render.ZoneShape{}, false
	}
	z, _ := e.doc.Zone(e.zone)
	color := shapeColor(e.banned, linearColor(z.DisplayColor()))
	a := e.anchor
	if !e.hovering {
		return render.ZoneShape{Points: []geom.Vec3{serverVec(a)}, Color: color, Marked: 0}, true
	}
	h := e.hover
	var pts []zone.Point
	if e.tool == ui.ToolCircle {
		r := int(math.Round(math.Hypot(float64(h.X-a.X), float64(h.Y-a.Y))))
		pts = zone.CirclePoints(a, max(r, 1), zone.CircleSides)
	} else {
		c := zone.RectangleCorners(a, h)
		pts = c[:]
	}
	zmin, zmax := zone.SuggestZRange([]zone.Point{a, h}, zone.DefaultZMargin)
	rs := overlayShape(pts, zmin, zmax, color)
	if e.tool == ui.ToolRectangle {
		rs.Marked = 0
	}
	return rs, true
}

// shapeColor is the overlay colour of a shape: included, the zone's own
// colour (linear); banned, the one colour of exclusions.
func shapeColor(banned bool, included [3]float32) [3]float32 {
	if banned {
		return bannedColor
	}
	return included
}

// restartPin is a restart point in the overlay: a vertical line from the
// point up, with handles at both ends.
func restartPin(p zone.Point, color [3]float32) render.ZoneShape {
	top := p
	top.Z += restartPinHeight
	return render.ZoneShape{Points: []geom.Vec3{serverVec(p), serverVec(top)}, Color: color, Marked: -1}
}

func serverVec(p zone.Point) geom.Vec3 {
	return geom.Vec3{X: float32(p.X), Y: float32(p.Y), Z: float32(p.Z)}
}

// groundZ is the server Z of the surface under x y near height ref (see
// groundAbove), or ref when nothing is there.
func groundZ(s *scene.Scene, x, y, ref int) int {
	o := scene.FromServer(geom.Vec3{X: float32(x), Y: float32(y), Z: float32(ref + groundAbove)})
	h, ok := s.Pick(scene.Ray{Origin: o, Dir: geom.Vec3{Z: -1}})
	if !ok {
		return ref
	}
	z := round(h.Pos.Z)
	if z < ref-groundReach || z > ref+groundReach {
		return ref
	}
	return z
}
