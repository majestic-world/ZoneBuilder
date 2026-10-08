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
	"zonebuilder/internal/zone"
	"zonebuilder/internal/zonexml"
)

// closeSlop is how close, in pixels, a click must land to the first vertex
// to close the polygon.
const closeSlop = 10

// zoneColor is the overlay colour of every zone (linear RGB) until zones get
// a colour per type.
var zoneColor = [3]float32{1, 0.35, 0.05}

// zoneEditor turns the zone controls and viewport input into zone.Document
// commands. The polygon tool is active while drawing is set: each click on
// a surface adds a vertex at the picked point, and Enter or a click on the
// first vertex closes the polygon, which sets its suggested Z range.
type zoneEditor struct {
	doc     *zone.Document
	drawing bool
	zone    zone.ZoneID
	shape   int
	// version counts changes to what the overlay shows.
	version int
}

func newZoneEditor() *zoneEditor { return &zoneEditor{doc: zone.NewDocument()} }

// apply runs c and logs a failure; it reports success.
func (e *zoneEditor) apply(c zone.Command) bool {
	if err := e.doc.Apply(c); err != nil {
		log.Printf("zona: %v", err)
		return false
	}
	e.version++
	return true
}

// create starts a zone named name of type t and its polygon. It returns the
// status line.
func (e *zoneEditor) create(name string, t zone.Type) string {
	name = strings.TrimSpace(name)
	switch {
	case e.drawing:
		return "Feche o polígono atual antes de criar outra zona"
	case name == "":
		return "Digite o nome da zona"
	}
	id := e.doc.NewZoneID()
	if !e.apply(zone.CreateZone{ID: id, Name: name, Type: t}) || !e.apply(zone.AddShape{Zone: id}) {
		return "Não foi possível criar a zona"
	}
	e.drawing, e.zone, e.shape = true, id, 0
	log.Printf("zona: criada %s (%s)", name, t)
	return fmt.Sprintf("Zona %s criada: clique na superfície para adicionar vértices; Enter ou clique no 1º vértice fecha", name)
}

// points is the polygon being drawn.
func (e *zoneEditor) points() []zone.Point {
	z, _ := e.doc.Zone(e.zone)
	return z.Shapes[e.shape].Points
}

// click handles a click at viewport pixel p that picked h (ok: something
// was hit). It returns the status line, or "" when the tool is idle.
func (e *zoneEditor) click(s *scene.Scene, cam *camera.Camera, p f32.Point, viewport image.Point, h scene.Hit, ok bool) string {
	if !e.drawing {
		return ""
	}
	pts := e.points()
	if len(pts) >= 3 {
		first := renderPoint(s, pts[0])
		if x, y, vis := cam.Project(first, viewport.X, viewport.Y); vis && dist(p, f32.Pt(x, y)) <= closeSlop {
			return e.close()
		}
	}
	if !ok {
		return "O clique não atingiu nenhuma superfície"
	}
	v := zone.Point{X: round(h.Pos.X), Y: round(h.Pos.Y), Z: round(h.Pos.Z)}
	if !e.apply(zone.AddVertex{Zone: e.zone, Shape: e.shape, Point: v}) {
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
	zmin, zmax := zone.SuggestZRange(pts, zone.DefaultZMargin)
	if !e.apply(zone.SetZRange{Zone: e.zone, Shape: e.shape, ZMin: zmin, ZMax: zmax}) {
		return "Não foi possível fechar o polígono"
	}
	e.drawing = false
	z, _ := e.doc.Zone(e.zone)
	log.Printf("zona: %s fechada com %s, z %d..%d", z.Name, count(len(pts), "vértice", "vértices"), zmin, zmax)
	return fmt.Sprintf("Zona %s fechada: %s, z %d … %d", z.Name, count(len(pts), "vértice", "vértices"), zmin, zmax)
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

// info is the zone panel's lines: the zones and the tool hint.
func (e *zoneEditor) info() []string {
	zones := e.doc.Zones()
	lines := []string{count(len(zones), "zona", "zonas")}
	for _, z := range zones {
		line := fmt.Sprintf("%s (%s)", z.Name, z.Type)
		for _, s := range z.Shapes {
			line += fmt.Sprintf(": %s, z %d … %d", count(len(s.Points), "vértice", "vértices"), s.ZMin, s.ZMax)
		}
		if e.drawing && z.ID == e.zone {
			line = fmt.Sprintf("%s (%s): desenhando, %s", z.Name, z.Type, count(len(e.points()), "vértice", "vértices"))
		}
		lines = append(lines, line)
	}
	if e.drawing {
		lines = append(lines, "Clique adiciona vértice; Enter ou clique no 1º vértice fecha")
	}
	return lines
}

// overlay is every shape for the renderer's zone overlay.
func (e *zoneEditor) overlay() []render.ZoneShape {
	var shapes []render.ZoneShape
	for _, z := range e.doc.Zones() {
		for i, s := range z.Shapes {
			open := e.drawing && z.ID == e.zone && i == e.shape
			rs := render.ZoneShape{
				Points: make([]geom.Vec3, len(s.Points)),
				ZMin:   float32(s.ZMin),
				ZMax:   float32(s.ZMax),
				Closed: !open,
				Color:  zoneColor,
				Marked: -1,
			}
			if open {
				rs.Marked = 0
			}
			for k, p := range s.Points {
				rs.Points[k] = geom.Vec3{X: float32(p.X), Y: float32(p.Y), Z: float32(p.Z)}
			}
			shapes = append(shapes, rs)
		}
	}
	return shapes
}

// renderPoint is server point p in s's rebased render space, where the
// camera lives.
func renderPoint(s *scene.Scene, p zone.Point) geom.Vec3 {
	w := scene.FromServer(geom.Vec3{X: float32(p.X), Y: float32(p.Y), Z: float32(p.Z)})
	return scene.ToRender(w.Sub(s.Origin))
}
