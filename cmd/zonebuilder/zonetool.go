package main

import (
	"fmt"
	"image"
	"log"
	"strings"

	"gioui.org/f32"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/render"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
	"zonebuilder/internal/zonexml"
)

// closeSlop is how close, in pixels, a click must land to the first vertex
// to close the polygon.
const closeSlop = 10

// zoneEditor turns the zone controls and viewport input into zone.Document
// commands. Its zone is the selected one, which the tools add to; a tool
// takes viewport clicks while armed (see shapetool.go). The polygon tool
// adds a vertex per click on a surface, and Enter or a click on the first
// vertex closes the polygon, which sets its suggested Z range. When not
// drawing, zone and shape are the current shape the edit tools work on
// (edittool.go).
type zoneEditor struct {
	doc *zone.Document
	// zone is the selected zone; 0 before the first one is created.
	zone zone.ZoneID
	// tool is the armed tool when armed is set; banned makes its shape an
	// exclusion.
	tool   ui.Tool
	armed  bool
	banned bool
	// drawing is set while the polygon tool has a polygon open: shape.
	drawing bool
	shape   int
	// anchor is the first click of the rectangle (a corner) or circle
	// (the center) tool, when anchored; hover is the surface point under
	// the cursor, when hovering, for the preview from the anchor.
	anchor, hover      zone.Point
	anchored, hovering bool
	// version counts changes to what the overlay shows.
	version int
	editState
}

func newZoneEditor() *zoneEditor {
	return &zoneEditor{doc: zone.NewDocument(), editState: newEditState()}
}

// apply runs c and logs a failure; it reports success.
func (e *zoneEditor) apply(c zone.Command) bool {
	if err := e.doc.Apply(c); err != nil {
		log.Printf("zona: %v", err)
		return false
	}
	e.version++
	return true
}

// create starts a zone named name of type t, selects it and arms tool for
// its first shape. It returns the status line.
func (e *zoneEditor) create(name string, t zone.Type, tool ui.Tool) string {
	name = strings.TrimSpace(name)
	switch {
	case e.drawing:
		return "Feche o polígono atual antes de criar outra zona"
	case name == "":
		return "Digite o nome da zona"
	}
	id := e.doc.NewZoneID()
	if !e.apply(zone.CreateZone{ID: id, Name: name, Type: t}) {
		return "Não foi possível criar a zona"
	}
	e.zone = id
	log.Printf("zona: criada %s (%s)", name, t)
	return fmt.Sprintf("Zona %s criada. %s", name, e.arm(tool, false))
}

// points is the polygon being drawn.
func (e *zoneEditor) points() []zone.Point {
	z, _ := e.doc.Zone(e.zone)
	return z.Shapes[e.shape].Points
}

// click handles a click at viewport pixel p that picked h (ok: something
// was hit). It returns the status line, or "" when no tool is armed.
func (e *zoneEditor) click(s *scene.Scene, cam *camera.Camera, p f32.Point, viewport image.Point, h scene.Hit, ok bool) string {
	if !e.armed {
		return ""
	}
	if e.drawing {
		pts := e.points()
		if len(pts) >= 3 {
			first := renderPoint(s, pts[0])
			if x, y, vis := cam.Project(first, viewport.X, viewport.Y); vis && dist(p, f32.Pt(x, y)) <= closeSlop {
				return e.close()
			}
		}
	}
	if !ok {
		return "O clique não atingiu nenhuma superfície"
	}
	v := zone.Point{X: round(h.Pos.X), Y: round(h.Pos.Y), Z: round(h.Pos.Z)}
	switch e.tool {
	case ui.ToolRectangle:
		return e.rectangleClick(s, v)
	case ui.ToolCircle:
		return e.circleClick(s, v)
	case ui.ToolRestart, ui.ToolPKRestart:
		return e.restartClick(v)
	}
	return e.polygonClick(v)
}

// polygonClick adds vertex v to the polygon being drawn, starting the
// polygon with it if none is.
func (e *zoneEditor) polygonClick(v zone.Point) string {
	if !e.drawing {
		z, _ := e.doc.Zone(e.zone)
		if !e.apply(zone.AddShape{Zone: e.zone, Banned: e.banned, Points: []zone.Point{v}}) {
			return "Não foi possível começar o polígono"
		}
		e.drawing, e.shape = true, len(z.Shapes)
	} else if !e.apply(zone.AddVertex{Zone: e.zone, Shape: e.shape, Point: v}) {
		return "Não foi possível adicionar o vértice"
	}
	n := len(e.points())
	log.Printf("zona: vértice %d em %d %d %d", n, v.X, v.Y, v.Z)
	return fmt.Sprintf("%s: %d %d %d", count(n, "vértice", "vértices"), v.X, v.Y, v.Z)
}

// close ends the polygon being drawn with the Z range suggested from its
// vertices. It returns the status line.
func (e *zoneEditor) close() string {
	if !e.drawing {
		return ""
	}
	pts := e.points()
	if len(pts) < 3 {
		return fmt.Sprintf("O polígono tem %s; são precisos 3 para fechar", count(len(pts), "vértice", "vértices"))
	}
	zmin, zmax := zone.SuggestZRange(pts, e.margin)
	if !e.apply(zone.SetZRange{Zone: e.zone, Shape: e.shape, ZMin: zmin, ZMax: zmax}) {
		return "Não foi possível fechar o polígono"
	}
	e.drawing, e.armed = false, false
	z, _ := e.doc.Zone(e.zone)
	what := "Polígono fechado"
	if e.banned {
		what = "Exclusão fechada"
	}
	log.Printf("zona: %s: %s com %s, z %d..%d", z.Name, strings.ToLower(what), count(len(pts), "vértice", "vértices"), zmin, zmax)
	return fmt.Sprintf("%s em %s: %s, z %d … %d", what, z.Name, count(len(pts), "vértice", "vértices"), zmin, zmax)
}

// compile writes every zone's XML into dir. It returns the status line.
func (e *zoneEditor) compile(dir string) string {
	dir = strings.TrimSpace(dir)
	switch {
	case e.drawing:
		return "Feche o polígono antes de compilar"
	case len(e.doc.Zones()) == 0:
		return "Não há zonas para compilar"
	case dir == "":
		return "Escolha a pasta de saída do XML"
	}
	files, err := e.doc.Compile(e.doc.ZoneIDs())
	if err != nil {
		return err.Error()
	}
	paths, err := zonexml.Write(dir, files)
	for _, p := range paths {
		log.Printf("zona: XML gravado em %s", p)
	}
	if err != nil {
		log.Printf("zona: compilação: %v", err)
		return err.Error()
	}
	return "XML gravado: " + strings.Join(paths, ", ")
}

// info is the zone panel's lines: the zones, their shapes and restart
// points, and the armed tool's hint.
func (e *zoneEditor) info() []string {
	zones := e.doc.Zones()
	lines := []string{count(len(zones), "zona", "zonas")}
	for _, z := range zones {
		line := fmt.Sprintf("%s (%s)", z.Name, z.Type)
		if z.ID == e.zone {
			line = "» " + line + ", selecionada"
		}
		lines = append(lines, line)
		for i, s := range z.Shapes {
			lines = append(lines, "   "+e.describeShape(z.ID, i, s))
		}
		if n := len(z.RestartPoints); n > 0 {
			lines = append(lines, "   "+count(n, "restart_point", "restart_points"))
		}
		if n := len(z.PKRestartPoints); n > 0 {
			lines = append(lines, "   "+count(n, "PKrestart_point", "PKrestart_points"))
		}
	}
	if hint := e.hint(); hint != "" {
		lines = append(lines, hint)
	}
	return lines
}

// describeShape is one info line for shape i of zone id.
func (e *zoneEditor) describeShape(id zone.ZoneID, i int, s zone.Shape) string {
	kind := "polígono"
	if s.Kind == zone.Rectangle {
		kind = "retângulo"
	}
	if s.Banned {
		kind = "exclusão, " + kind
	}
	if e.drawing && id == e.zone && i == e.shape {
		return fmt.Sprintf("%s: desenhando, %s", kind, count(len(s.Points), "vértice", "vértices"))
	}
	if s.Kind == zone.Rectangle {
		return fmt.Sprintf("%s, z %d … %d", kind, s.ZMin, s.ZMax)
	}
	return fmt.Sprintf("%s: %s, z %d … %d", kind, count(len(s.Points), "vértice", "vértices"), s.ZMin, s.ZMax)
}

// overlay is every shape and restart point for the renderer's zone
// overlay, plus the preview of the rectangle or circle being placed.
func (e *zoneEditor) overlay() []render.ZoneShape {
	var shapes []render.ZoneShape
	for _, z := range e.doc.Zones() {
		color := linearColor(z.DisplayColor())
		for i, s := range z.Shapes {
			open := e.drawing && z.ID == e.zone && i == e.shape
			if z.Hidden && !open {
				continue
			}
			current := !e.drawing && z.ID == e.zone && i == e.shape
			pts := e.shownPoints(z.ID, i, s.Points)
			zmin, zmax := e.shownZRange(z.ID, i, s)
			rect := s.Kind == zone.Rectangle && len(pts) == 2
			if rect {
				c := zone.RectangleCorners(pts[0], pts[1])
				pts = c[:]
			}
			rs := overlayShape(pts, zmin, zmax, shapeColor(s.Banned, color))
			// A rectangle's edges have no midpoint handles: it stays 2
			// corners, so no vertex can be inserted into it.
			rs.Midpoints = current && !rect
			if open {
				rs.Closed, rs.Marked = false, 0
			}
			if v, ok := e.selected(); ok && current {
				rs.Marked = v
				if rect {
					rs.Marked = 2 * v // stored corner 1 is outline corner 2 (RectangleCorners)
				}
			}
			shapes = append(shapes, rs)
		}
		if z.Hidden {
			continue
		}
		for _, p := range z.RestartPoints {
			shapes = append(shapes, restartPin(p, restartColor))
		}
		for _, p := range z.PKRestartPoints {
			shapes = append(shapes, restartPin(p, pkRestartColor))
		}
	}
	if pv, ok := e.preview(); ok {
		shapes = append(shapes, pv)
	}
	return shapes
}

// overlayShape is the closed overlay prism through pts.
func overlayShape(pts []zone.Point, zmin, zmax int, color [3]float32) render.ZoneShape {
	rs := render.ZoneShape{
		Points: make([]geom.Vec3, len(pts)),
		ZMin:   float32(zmin),
		ZMax:   float32(zmax),
		Closed: true,
		Color:  color,
		Marked: -1,
	}
	for k, p := range pts {
		rs.Points[k] = serverVec(p)
	}
	return rs
}

// renderPoint is server point p in s's rebased render space, where the
// camera lives.
func renderPoint(s *scene.Scene, p zone.Point) geom.Vec3 {
	w := scene.FromServer(serverVec(p))
	return scene.ToRender(w.Sub(s.Origin))
}
