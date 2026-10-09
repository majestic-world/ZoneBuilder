package main

import (
	"fmt"
	"image"
	"log"
	"math"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// grabSlop is how close, in pixels, a press must land to a vertex or edge
// midpoint handle to grab it.
const grabSlop = 10

// groundProbe is how far above a vertex the rays that look for the ground
// under it start, so a vertex picked on a floor finds that floor rather
// than the ceiling or roof above it.
const groundProbe = 64

// editState is the zoneEditor's editing of existing shapes: the selected
// vertex, a drag in progress and the Z margin. The current shape is
// zoneEditor.zone and .shape.
type editState struct {
	// sel is the selected vertex; it counts only while it belongs to the
	// current shape (see selected).
	sel vertexRef
	// margin is the Z margin of suggested ranges (the panel's Folga Z).
	margin int
	// step is how far Subir/Descer and PageUp/PageDown move the zone.
	step int
	drag drag
	// zdrag is a drag of the Z arrow, arrow where it was last laid out.
	zdrag zDrag
	arrow zArrow
	// heightFilled is what the height window's fields were last filled
	// for.
	heightFilled heightKey
	// filled is the selection and version the panel fields were last
	// filled for.
	filled panelKey
}

type vertexRef struct {
	zone  zone.ZoneID
	shape int
	index int
}

type panelKey struct {
	sel     vertexRef
	ok      bool
	version int
}

type dragKind int

const (
	dragNone   dragKind = iota
	dragVertex          // the selected vertex follows the surface under the cursor
	dragShape           // the whole shape follows its grabbed vertex
)

// drag is a press on a vertex handle being dragged. from is the grabbed
// vertex; to is where the surface under the cursor puts it, once the
// pointer moved past clickSlop and a surface was hit.
type drag struct {
	kind  dragKind
	press f32.Point
	from  zone.Point
	to    zone.Point
	moved bool
}

func newEditState() editState {
	return editState{sel: vertexRef{index: -1}, margin: zone.DefaultZMargin, step: ui.DefaultZStep}
}

// currentShape is the shape the panel edits, if it exists.
func (e *zoneEditor) currentShape() (zone.Zone, zone.Shape, bool) {
	z, ok := e.doc.Zone(e.zone)
	if !ok || e.shape < 0 || e.shape >= len(z.Shapes) {
		return zone.Zone{}, zone.Shape{}, false
	}
	return z, z.Shapes[e.shape], true
}

// selected is the selected vertex of the current shape.
func (e *zoneEditor) selected() (int, bool) {
	_, s, ok := e.currentShape()
	if !ok || e.drawing || e.sel.zone != e.zone || e.sel.shape != e.shape || e.sel.index < 0 || e.sel.index >= len(s.Points) {
		return -1, false
	}
	return e.sel.index, true
}

func (e *zoneEditor) selectVertex(id zone.ZoneID, shape, index int) {
	e.zone, e.shape = id, shape
	e.sel = vertexRef{zone: id, shape: shape, index: index}
	e.version++
}

// sync drops UI state that an Undo or Redo left pointing at nothing: the
// polygon being drawn, and the armed tool when its zone is gone.
func (e *zoneEditor) sync() {
	if _, _, ok := e.currentShape(); !ok {
		e.drawing = false
	}
	if _, ok := e.doc.Zone(e.zone); !ok {
		e.armed, e.anchored, e.hovering = false, false, false
	}
	e.drag = drag{}
	e.zdrag = zDrag{}
}

func (e *zoneEditor) undo() string {
	if !e.doc.Undo() {
		return "Nada para desfazer"
	}
	e.version++
	e.sync()
	log.Printf("zona: desfeito")
	return "Desfeito"
}

func (e *zoneEditor) redo() string {
	if !e.doc.Redo() {
		return "Nada para refazer"
	}
	e.version++
	e.sync()
	log.Printf("zona: refeito")
	return "Refeito"
}

// viewportEvent handles the editing input of the viewport: Ctrl+Z/Ctrl+Y,
// Delete, and presses and drags on vertex and edge midpoint handles. It
// reports whether it used ev (which then must not move the camera) and the
// status line, "" to keep the current one.
func (e *zoneEditor) viewportEvent(s *scene.World, cam *camera.Camera, ev event.Event, vp image.Point) (string, bool) {
	switch ev := ev.(type) {
	case key.Event:
		if ev.State != key.Press {
			return "", ev.Name == "Z" || ev.Name == "Y" || ev.Name == key.NameDeleteForward || ev.Name == key.NamePageUp || ev.Name == key.NamePageDown
		}
		switch {
		case ev.Name == "Z" && ev.Modifiers.Contain(key.ModShortcut|key.ModShift), ev.Name == "Y" && ev.Modifiers.Contain(key.ModShortcut):
			return e.redo(), true
		case ev.Name == "Z" && ev.Modifiers.Contain(key.ModShortcut):
			return e.undo(), true
		case ev.Name == key.NameDeleteForward:
			return e.removeVertex(), true
		case ev.Name == key.NamePageUp:
			return e.shiftZone(e.step), true
		case ev.Name == key.NamePageDown:
			return e.shiftZone(-e.step), true
		}
	case pointer.Event:
		if s == nil || e.drawing || e.armed {
			return "", false
		}
		if e.zdrag.active {
			return e.arrowEvent(ev), true
		}
		if e.drag.kind != dragNone {
			return e.dragEvent(s, cam, ev, vp), true
		}
		if e.grabArrow(ev) {
			return "Arraste a seta para subir ou descer a zona", true
		}
		if ev.Kind == pointer.Press && ev.Buttons == pointer.ButtonPrimary {
			return e.press(s, cam, ev, vp)
		}
	}
	return "", false
}

// press grabs the vertex handle under ev (Ctrl grabs its whole shape) or,
// on the current shape's edge midpoint handle, inserts a vertex there and
// grabs it.
func (e *zoneEditor) press(s *scene.World, cam *camera.Camera, ev pointer.Event, vp image.Point) (string, bool) {
	best := float32(grabSlop)
	var hit *vertexRef
	for _, z := range e.doc.Zones() {
		for si, sh := range z.Shapes {
			for i, p := range sh.Points {
				if d, ok := screenDist(s, cam, p, ev.Position, vp); ok && d <= best {
					best, hit = d, &vertexRef{zone: z.ID, shape: si, index: i}
				}
			}
		}
	}
	if hit != nil {
		e.selectVertex(hit.zone, hit.shape, hit.index)
		z, sh, _ := e.currentShape()
		p := sh.Points[hit.index]
		e.drag = drag{kind: dragVertex, press: ev.Position, from: p}
		if ev.Modifiers.Contain(key.ModCtrl) {
			e.drag.kind = dragShape
			return fmt.Sprintf("Movendo o shape %d de %s", hit.shape+1, z.Name), true
		}
		return fmt.Sprintf("Vértice %d de %s: %d %d %d", hit.index+1, z.Name, p.X, p.Y, p.Z), true
	}
	if _, sh, ok := e.currentShape(); ok && sh.Kind != zone.Rectangle && len(sh.Points) >= 2 {
		for i := range sh.Points {
			if d, ok := screenDist(s, cam, midpoint(sh.Points, i), ev.Position, vp); ok && d <= grabSlop {
				msg := e.insertAfter(i)
				if v, ok := e.selected(); ok && v == i+1 {
					e.drag = drag{kind: dragVertex, press: ev.Position, from: midpoint(sh.Points, i)}
				}
				return msg, true
			}
		}
	}
	return "", false
}

// dragEvent follows a drag: the grabbed vertex snaps to the surface under
// the cursor, and the release applies the move as one command.
func (e *zoneEditor) dragEvent(s *scene.World, cam *camera.Camera, ev pointer.Event, vp image.Point) string {
	switch ev.Kind {
	case pointer.Drag:
		if !e.drag.moved && dist(ev.Position, e.drag.press) <= clickSlop {
			return ""
		}
		if h, ok := pickAt(s, cam, ev.Position, vp); ok {
			e.drag.to, e.drag.moved = serverPoint(h), true
			e.version++
			return "Soltar em " + describeHit(h, true)
		}
		return "Nenhuma superfície sob o cursor"
	case pointer.Release:
		d := e.drag
		e.drag = drag{}
		e.version++
		if !d.moved || d.to == d.from {
			return ""
		}
		v, ok := e.selected()
		if !ok {
			return ""
		}
		if d.kind == dragShape {
			return e.moveShape(d.to.X-d.from.X, d.to.Y-d.from.Y, d.to.Z-d.from.Z)
		}
		return e.moveVertex(v, d.to)
	case pointer.Cancel:
		e.drag = drag{}
		e.version++
	}
	return ""
}

// shownPoints is pts, shape number shape of zone id, as the overlay draws
// it: moved by the drag in progress.
func (e *zoneEditor) shownPoints(id zone.ZoneID, shape int, pts []zone.Point) []zone.Point {
	if dz := e.zdrag.offset(id); dz != 0 {
		out := append([]zone.Point(nil), pts...)
		for i := range out {
			out[i].Z += dz
		}
		return out
	}
	d := e.drag
	if d.kind == dragNone || !d.moved || id != e.zone || shape != e.shape {
		return pts
	}
	v, ok := e.selected()
	if !ok {
		return pts
	}
	out := append([]zone.Point(nil), pts...)
	if d.kind == dragVertex {
		out[v] = d.to
		return out
	}
	for i := range out {
		out[i].X += d.to.X - d.from.X
		out[i].Y += d.to.Y - d.from.Y
		out[i].Z += d.to.Z - d.from.Z
	}
	return out
}

// shownZRange is the Z range of the current shape as the overlay draws it
// while a shape drag moves it.
func (e *zoneEditor) shownZRange(id zone.ZoneID, shape int, s zone.Shape) (int, int) {
	if dz := e.zdrag.offset(id); dz != 0 {
		return s.ZMin + dz, s.ZMax + dz
	}
	d := e.drag
	if d.kind != dragShape || !d.moved || id != e.zone || shape != e.shape {
		return s.ZMin, s.ZMax
	}
	return s.ZMin + d.to.Z - d.from.Z, s.ZMax + d.to.Z - d.from.Z
}

func (e *zoneEditor) moveVertex(v int, p zone.Point) string {
	if e.apply(zone.MoveVertex{Zone: e.zone, Shape: e.shape, Index: v, Point: p}) != nil {
		return "Não foi possível mover o vértice"
	}
	log.Printf("zona: vértice %d movido para %d %d %d", v+1, p.X, p.Y, p.Z)
	return fmt.Sprintf("Vértice %d em %d %d %d", v+1, p.X, p.Y, p.Z)
}

func (e *zoneEditor) moveShape(dx, dy, dz int) string {
	if e.apply(zone.MoveShape{Zone: e.zone, Shape: e.shape, DX: dx, DY: dy, DZ: dz}) != nil {
		return "Não foi possível mover o shape"
	}
	log.Printf("zona: shape %d movido por %d %d %d", e.shape+1, dx, dy, dz)
	return fmt.Sprintf("Shape movido por %d %d %d", dx, dy, dz)
}

// zoneZ is the selected zone's floor (its lowest zmin) and top (its
// highest zmax); ok is false while it has no shape or one is being drawn.
func (e *zoneEditor) zoneZ() (z zone.Zone, base, top int, ok bool) {
	z, ok = e.doc.Zone(e.zone)
	if !ok || e.drawing || len(z.Shapes) == 0 {
		return zone.Zone{}, 0, 0, false
	}
	base, top = z.Shapes[0].ZMin, z.Shapes[0].ZMax
	for _, s := range z.Shapes[1:] {
		base, top = min(base, s.ZMin), max(top, s.ZMax)
	}
	return z, base, top, true
}

// shiftZone raises (dz > 0) or lowers the selected zone, exclusions
// included.
func (e *zoneEditor) shiftZone(dz int) string {
	z, base, top, ok := e.zoneZ()
	if !ok {
		return "Selecione uma zona pronta para subir ou descer"
	}
	if dz == 0 {
		return ""
	}
	if e.apply(zone.ShiftZoneZ{Zone: z.ID, DZ: dz}) != nil {
		return "Não foi possível mover a zona"
	}
	log.Printf("zona: %s movida %+d em z, agora %d..%d", z.Name, dz, base+dz, top+dz)
	verb := "subiu"
	if dz < 0 {
		verb = "desceu"
	}
	return fmt.Sprintf("%s %s %d: z %d … %d", z.Name, verb, abs(dz), base+dz, top+dz)
}

// setZoneBase moves the selected zone so its floor lands at text's z.
func (e *zoneEditor) setZoneBase(text string) string {
	v, ok := ints(text, 1)
	if !ok {
		return "Digite a base como um número inteiro"
	}
	_, base, _, ok := e.zoneZ()
	if !ok {
		return "Selecione uma zona pronta para definir a base"
	}
	return e.shiftZone(v[0] - base)
}

// setZoneHeight makes every shape of the selected zone text's height
// tall, keeping their floors.
func (e *zoneEditor) setZoneHeight(text string) string {
	v, ok := ints(text, 1)
	if !ok || v[0] < 0 {
		return "Digite a altura como um número inteiro positivo"
	}
	z, base, _, ok := e.zoneZ()
	if !ok {
		return "Selecione uma zona pronta para definir a altura"
	}
	if e.apply(zone.SetZoneHeight{Zone: z.ID, Height: v[0]}) != nil {
		return "Não foi possível definir a altura"
	}
	log.Printf("zona: %s com altura %d", z.Name, v[0])
	return fmt.Sprintf("%s: altura %d, z %d … %d", z.Name, v[0], base, base+v[0])
}

// insertAfter inserts a vertex in the middle of the current shape's edge
// from vertex i to the next one and selects it.
func (e *zoneEditor) insertAfter(i int) string {
	_, sh, ok := e.currentShape()
	switch {
	case ok && sh.Kind == zone.Rectangle:
		return "O retângulo tem 2 cantos fixos: mova-os em vez de inserir"
	case !ok || len(sh.Points) < 2:
		return "Selecione um shape com ao menos 2 vértices"
	}
	p := midpoint(sh.Points, i)
	if e.apply(zone.InsertVertex{Zone: e.zone, Shape: e.shape, Index: i + 1, Point: p}) != nil {
		return "Não foi possível inserir o vértice"
	}
	e.selectVertex(e.zone, e.shape, i+1)
	log.Printf("zona: vértice %d inserido em %d %d %d", i+2, p.X, p.Y, p.Z)
	return fmt.Sprintf("Vértice %d inserido em %d %d %d", i+2, p.X, p.Y, p.Z)
}

func (e *zoneEditor) removeVertex() string {
	v, ok := e.selected()
	if !ok {
		return "Selecione um vértice para apagar"
	}
	if _, sh, _ := e.currentShape(); sh.Kind == zone.Rectangle {
		return "O retângulo tem 2 cantos fixos: mova-os em vez de apagar"
	}
	if e.apply(zone.RemoveVertex{Zone: e.zone, Shape: e.shape, Index: v}) != nil {
		return "Não foi possível apagar o vértice"
	}
	_, sh, _ := e.currentShape()
	e.selectVertex(e.zone, e.shape, min(v, len(sh.Points)-1))
	log.Printf("zona: vértice %d apagado", v+1)
	return fmt.Sprintf("Vértice %d apagado; restam %s", v+1, inflect.Count(len(sh.Points), "vértice", "vértices"))
}

// groundZRange sets the current shape's Z range from the ground under its
// vertices (groundUnder), then the margin below the lowest and above the
// highest ground.
func (e *zoneEditor) groundZRange(s *scene.World) string {
	_, sh, ok := e.currentShape()
	switch {
	case s == nil:
		return "Abra um mapa para achar o chão sob os vértices"
	case !ok || len(sh.Points) == 0:
		return "Selecione um shape com vértices"
	}
	pts := sh.Points
	if sh.Kind == zone.Rectangle && len(pts) == 2 {
		c := zone.RectangleCorners(pts[0], pts[1])
		pts = c[:]
	}
	ground := make([]zone.Point, 0, len(pts))
	missed := 0
	for i, p := range pts {
		z, ok := groundUnder(s, p)
		if !ok {
			log.Printf("zona: nenhum chão sob o vértice %d (%d %d)", i+1, p.X, p.Y)
			missed++
			continue
		}
		log.Printf("zona: chão sob o vértice %d (%d %d %d): z %d", i+1, p.X, p.Y, p.Z, z)
		ground = append(ground, zone.Point{X: p.X, Y: p.Y, Z: z})
	}
	if len(ground) == 0 {
		return "Nenhum vértice tem chão sob ele"
	}
	zmin, zmax := zone.SuggestZRange(ground, e.margin)
	if e.apply(zone.SetZRange{Zone: e.zone, Shape: e.shape, ZMin: zmin, ZMax: zmax}) != nil {
		return "Não foi possível definir a faixa Z"
	}
	log.Printf("zona: faixa Z pelo chão %d..%d (folga %d, %s)", zmin, zmax, e.margin, inflect.Count(missed, "vértice sem chão", "vértices sem chão"))
	msg := fmt.Sprintf("Faixa Z pelo chão: %d … %d (folga %d)", zmin, zmax, e.margin)
	if missed > 0 {
		msg += fmt.Sprintf("; %s sem chão", inflect.Count(missed, "vértice", "vértices"))
	}
	return msg
}

// groundUnder is the server Z of the ground at vertex p: the first surface
// straight down from groundProbe above it or, when nothing lies below (the
// vertex ended up under the surface, after a move onto higher ground), the
// ground it is buried under.
func groundUnder(s *scene.World, p zone.Point) (int, bool) {
	return ground(s, p, groundProbe, 0, true)
}

// panel handles the edit panel's requests and fills its fields and titles
// for the current shape and selected vertex. It returns the status line,
// "" to keep the current one.
func (e *zoneEditor) panel(gtx layout.Context, p *ui.EditPanel, s *scene.World) string {
	var msg string
	if p.Undo.Clicked(gtx) {
		msg = e.undo()
	}
	if p.Redo.Clicked(gtx) {
		msg = e.redo()
	}
	if m, err := strconv.Atoi(strings.TrimSpace(p.Margin.Text())); err == nil && m >= 0 {
		e.margin = m
	}
	if p.ZRangeRequested(gtx) {
		msg = e.setZRange(p.ZRange.Text())
	}
	if p.GroundZ.Clicked(gtx) {
		msg = e.groundZRange(s)
	}
	if p.MoveShapeRequested(gtx) {
		if d, ok := ints(p.Offset.Text(), 3); !ok {
			msg = "Digite o deslocamento como dx dy dz"
		} else if _, _, ok := e.currentShape(); !ok || e.drawing {
			msg = "Selecione um shape fechado"
		} else {
			msg = e.moveShape(d[0], d[1], d[2])
		}
	}
	if p.CoordsRequested(gtx) {
		msg = e.setCoords(p.Coords.Text())
	}
	if p.InsertAfter.Clicked(gtx) {
		if v, ok := e.selected(); ok {
			msg = e.insertAfter(v)
		} else {
			msg = "Selecione um vértice"
		}
	}
	if p.RemoveVertex.Clicked(gtx) {
		msg = e.removeVertex()
	}

	z, sh, ok := e.currentShape()
	ok = ok && !e.drawing
	v, vok := e.selected()
	p.Shape, p.Vertex = "", ""
	if ok {
		kind := shapeKind(sh)
		if sh.Kind != zone.Rectangle {
			kind += ", " + inflect.Count(len(sh.Points), "vértice", "vértices")
		}
		p.Shape = fmt.Sprintf("Shape %d de %s: %s, z %d … %d", e.shape+1, z.Name, kind, sh.ZMin, sh.ZMax)
	}
	if vok {
		p.Vertex = fmt.Sprintf("Vértice %d: x y z", v+1)
	}
	k := panelKey{sel: vertexRef{zone: e.zone, shape: e.shape, index: v}, ok: ok, version: e.version}
	if k != e.filled {
		if ok && (k.sel != e.filled.sel || !e.filled.ok) {
			p.Offset.SetText("0 0 0")
		}
		e.filled = k
		if ok {
			p.ZRange.SetText(fmt.Sprintf("%d %d", sh.ZMin, sh.ZMax))
		}
		if vok {
			pt := e.shownPoints(e.zone, e.shape, sh.Points)[v]
			p.Coords.SetText(fmt.Sprintf("%d %d %d", pt.X, pt.Y, pt.Z))
		}
	}
	return msg
}

func (e *zoneEditor) setZRange(text string) string {
	r, ok := ints(text, 2)
	switch {
	case !ok:
		return "Digite a faixa como zmin zmax"
	case e.drawing:
		return "Feche o polígono antes de editar a faixa Z"
	}
	if _, _, ok := e.currentShape(); !ok {
		return "Selecione um shape"
	}
	if e.apply(zone.SetZRange{Zone: e.zone, Shape: e.shape, ZMin: r[0], ZMax: r[1]}) != nil {
		return "Não foi possível definir a faixa Z"
	}
	log.Printf("zona: faixa Z %d..%d", r[0], r[1])
	return fmt.Sprintf("Faixa Z: %d … %d", r[0], r[1])
}

// setCoords moves the selected vertex to "x y z", or "x y" keeping its Z.
func (e *zoneEditor) setCoords(text string) string {
	v, ok := e.selected()
	if !ok {
		return "Selecione um vértice"
	}
	_, sh, _ := e.currentShape()
	p := sh.Points[v]
	if c, ok := ints(text, 3); ok {
		p = zone.Point{X: c[0], Y: c[1], Z: c[2]}
	} else if c, ok := ints(text, 2); ok {
		p.X, p.Y = c[0], c[1]
	} else {
		return "Digite as coordenadas como x y z"
	}
	return e.moveVertex(v, p)
}

// ints parses exactly n integers separated by spaces or commas.
func ints(text string, n int) ([]int, bool) {
	f := strings.FieldsFunc(text, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' })
	if len(f) != n {
		return nil, false
	}
	out := make([]int, n)
	for i, s := range f {
		v, err := strconv.Atoi(s)
		if err != nil {
			return nil, false
		}
		out[i] = v
	}
	return out, true
}

// midpoint is the middle of the edge from pts[i] to the next vertex
// (wrapping to the first), rounded.
func midpoint(pts []zone.Point, i int) zone.Point {
	a, b := pts[i], pts[(i+1)%len(pts)]
	mid := func(x, y int) int { return int(math.Round(float64(x+y) / 2)) }
	return zone.Point{X: mid(a.X, b.X), Y: mid(a.Y, b.Y), Z: mid(a.Z, b.Z)}
}

// screenDist is how far, in pixels, server point p projects from viewport
// pixel at; ok is false when p is behind the camera.
func screenDist(s *scene.World, cam *camera.Camera, p zone.Point, at f32.Point, vp image.Point) (float32, bool) {
	x, y, ok := cam.Project(renderPoint(s, p), vp.X, vp.Y)
	if !ok {
		return 0, false
	}
	return dist(at, f32.Pt(x, y)), true
}

// shapeKind names shape s's kind in the panels: "polígono" or "retângulo",
// "exclusão, " first for an exclusion.
func shapeKind(s zone.Shape) string {
	kind := "polígono"
	if s.Kind == zone.Rectangle {
		kind = "retângulo"
	}
	if s.Banned {
		kind = "exclusão, " + kind
	}
	return kind
}

// serverPoint is a picked position as a vertex.
func serverPoint(h scene.Hit) zone.Point {
	return zone.Point{X: round(h.Pos.X), Y: round(h.Pos.Y), Z: round(h.Pos.Z)}
}
