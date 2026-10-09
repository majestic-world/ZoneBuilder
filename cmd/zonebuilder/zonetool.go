package main

import (
	"errors"
	"fmt"
	"image"
	"log"
	"strings"

	"gioui.org/f32"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
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
	// ghost is the shape placed from the anchor to hover, with its Z range
	// ghostZ, while hovering (see placed).
	ghost  []zone.Point
	ghostZ zSuggestion
	// version counts changes to what the overlay shows.
	version int
	// problems are the problems the problem panel shows (problemRows),
	// built for version problemsAt-1 (0: never built).
	problems   []zone.Problem
	problemsAt int
	// left out are the zones unchecked for compilation; every other zone,
	// new ones too, is in the compile selection.
	leftOut map[zone.ZoneID]bool
	editState
}

func newZoneEditor() *zoneEditor {
	return &zoneEditor{doc: zone.NewDocument(), editState: newEditState()}
}

// apply runs c on the document, logging a failure, and returns its error.
// Every UI request that edits a zone goes through it.
func (e *zoneEditor) apply(c zone.Command) error {
	if err := e.doc.Apply(c); err != nil {
		log.Printf("zona: %v", err)
		return err
	}
	e.version++
	return nil
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
	if e.apply(zone.CreateZone{ID: id, Name: name, Type: t}) != nil {
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
func (e *zoneEditor) click(s *scene.World, c *floorCoverage, cam *camera.Camera, p f32.Point, viewport image.Point, h scene.Hit, ok bool) string {
	if !e.armed {
		return ""
	}
	if e.drawing {
		pts := e.points()
		if len(pts) >= 3 {
			first := renderPoint(s, pts[0])
			if x, y, vis := cam.Project(first, viewport.X, viewport.Y); vis && dist(p, f32.Pt(x, y)) <= closeSlop {
				return e.close(s, c)
			}
		}
	}
	if !ok {
		return "O clique não atingiu nenhuma superfície"
	}
	v := serverPoint(h)
	switch e.tool {
	case ui.ToolRectangle:
		return e.rectangleClick(s, c, v)
	case ui.ToolCircle:
		return e.circleClick(s, c, v)
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
		if e.apply(zone.AddShape{Zone: e.zone, Banned: e.banned, Points: []zone.Point{v}}) != nil {
			return "Não foi possível começar o polígono"
		}
		e.drawing, e.shape = true, len(z.Shapes)
	} else if e.apply(zone.AddVertex{Zone: e.zone, Shape: e.shape, Point: v}) != nil {
		return "Não foi possível adicionar o vértice"
	}
	n := len(e.points())
	log.Printf("zona: vértice %d em %d %d %d", n, v.X, v.Y, v.Z)
	return fmt.Sprintf("%s: %d %d %d", inflect.Count(n, "vértice", "vértices"), v.X, v.Y, v.Z)
}

// close ends the polygon being drawn with the Z range suggested by the
// floor under its area over s, measured now, or from its vertices when the
// floor gives none (floorCoverage.suggest). It returns the status line.
func (e *zoneEditor) close(s *scene.World, c *floorCoverage) string {
	if !e.drawing {
		return ""
	}
	pts := e.points()
	if len(pts) < 3 {
		return fmt.Sprintf("O polígono tem %s; são precisos 3 para fechar", inflect.Count(len(pts), "vértice", "vértices"))
	}
	vmin, vmax := zone.SuggestZRange(pts, e.margin)
	fit := c.suggest(e, s, pts, vmin, vmax, false, true)
	if e.apply(zone.SetZRange{Zone: e.zone, Shape: e.shape, ZMin: fit.zmin, ZMax: fit.zmax}) != nil {
		return "Não foi possível fechar o polígono"
	}
	e.drawing, e.armed = false, false
	c.adopt(e, s, e.shape, pts)
	z, _ := e.doc.Zone(e.zone)
	what := "Polígono fechado"
	if e.banned {
		what = "Exclusão fechada"
	}
	log.Printf("zona: %s: %s com %s, z %d..%d %s", z.Name, strings.ToLower(what), inflect.Count(len(pts), "vértice", "vértices"), fit.zmin, fit.zmax, fit.note())
	return fmt.Sprintf("%s em %s: %s, z %d … %d %s", what, z.Name, inflect.Count(len(pts), "vértice", "vértices"), fit.zmin, fit.zmax, fit.note())
}

// selection is the zones checked for compilation, in creation order.
func (e *zoneEditor) selection() []zone.ZoneID {
	var ids []zone.ZoneID
	for _, id := range e.doc.ZoneIDs() {
		if !e.leftOut[id] {
			ids = append(ids, id)
		}
	}
	return ids
}

// selectForCompile puts ids in, or out of, the compile selection. It
// returns the status line.
func (e *zoneEditor) selectForCompile(ids []zone.ZoneID, in bool) string {
	if e.leftOut == nil {
		e.leftOut = map[zone.ZoneID]bool{}
	}
	for _, id := range ids {
		if in {
			delete(e.leftOut, id)
		} else {
			e.leftOut[id] = true
		}
	}
	n := len(e.selection())
	return fmt.Sprintf("%s de %d para compilar", inflect.Count(n, "zona selecionada", "zonas selecionadas"), len(e.doc.Zones()))
}

// compile compiles the selected zones for the XML window. Nothing comes out
// while the compilation is blocked. It returns the status line and the
// files, one per type.
func (e *zoneEditor) compile() (string, []zonexml.File) {
	sel := e.selection()
	switch {
	case e.drawing:
		return "Feche o polígono antes de compilar", nil
	case len(e.doc.Zones()) == 0:
		return "Não há zonas para compilar", nil
	case len(sel) == 0:
		return "Nenhuma zona selecionada para compilar", nil
	}
	files, err := e.doc.Compile(sel)
	if b, ok := errors.AsType[*zone.BlockedError](err); ok {
		return e.blockedStatus(b), nil
	}
	if err != nil {
		log.Printf("zona: compilação: %v", err)
		return err.Error(), nil
	}
	log.Printf("zona: compiladas %s em %s", inflect.Count(len(sel), "zona", "zonas"), inflect.Count(len(files), "arquivo", "arquivos"))
	return fmt.Sprintf("Compiladas %s em %s: copie da janela XML", inflect.Count(len(sel), "zona", "zonas"), inflect.Count(len(files), "arquivo", "arquivos")), files
}

// info is the zone panel's lines below its controls: the armed tool's
// hint. The zones themselves are in the zone list.
func (e *zoneEditor) info() []string {
	if hint := e.hint(); hint != "" {
		return []string{hint}
	}
	return nil
}

// overlay is every shape and restart point for the renderer's zone
// overlay, plus the preview of the rectangle or circle being placed.
// ground is the floor line along the current shape's walls.
func (e *zoneEditor) overlay(ground []geom.Vec3) []render.ZoneShape {
	var shapes []render.ZoneShape
	for _, z := range e.doc.Zones() {
		color := linearColor(z.DisplayColor())
		problem := len(e.doc.ZoneProblems(z.ID)) > 0
		bad := e.badVertices(z.ID)
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
			// The polygon being drawn is incomplete by nature: no flag.
			if !open {
				rs.Problem, rs.BadVertices = problem, bad[i]
			}
			if rect {
				for k, v := range rs.BadVertices {
					rs.BadVertices[k] = 2 * v // stored corner 1 is outline corner 2 (RectangleCorners)
				}
			}
			// A rectangle's edges have no midpoint handles: it stays 2
			// corners, so no vertex can be inserted into it.
			rs.Midpoints = current && !rect
			if current {
				rs.Ground = ground
			}
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
			pin := restartPin(p, restartColor)
			pin.Problem = problem
			shapes = append(shapes, pin)
		}
		for _, p := range z.PKRestartPoints {
			pin := restartPin(p, pkRestartColor)
			pin.Problem = problem
			shapes = append(shapes, pin)
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
func renderPoint(s *scene.World, p zone.Point) geom.Vec3 {
	w := scene.FromServer(serverVec(p))
	return scene.ToRender(w.Sub(s.Origin))
}
