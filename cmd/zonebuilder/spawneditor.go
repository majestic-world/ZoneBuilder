package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"

	"gioui.org/app"
	"gioui.org/f32"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/coverage"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/model"
	"zonebuilder/internal/render"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/spawn"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// spawnRadius is a new area's default radius (spec story 21): the
// collision radius measured on the preview monster's mesh, rounded to the
// server's integer units.
func spawnRadius() int { return int(math.Round(float64(model.MonsterRadius))) }

// spawnZMargin is the margin of an area's suggested Z range, below the
// lowest and above the highest floor under it: the zone editor's default,
// as an area's range comes "do chão sob o contorno, como nas zonas"; the
// population mode has no margin field.
const spawnZMargin = zone.DefaultZMargin

// spawnEditor turns the population mode's controls and viewport input into
// spawn.Document commands, with the zone editor's tools: the polygon tool
// adds an area and a vertex per click, Enter or a click on the first
// vertex closes it; the rectangle and circle tools add an area, as a
// polygon, from 2 clicks. Every new area takes its Z range from the floor
// under its outline (floorCoverage.suggest, its own coverage). When not
// drawing, area is the selected area the vertex edits, the height window
// and the properties work on (spawnedit.go).
type spawnEditor struct {
	doc   *spawn.Document
	cover *floorCoverage
	// radius is a new area's radius: the preview monster's (spec story 21).
	radius int
	// version counts changes to what the overlay and the panel show;
	// edits counts the document's changes, for the unsaved indicator.
	version, edits int
	// area is the selected area, 0 for none; sel is its selected vertex,
	// -1 for none.
	area spawn.AreaID
	sel  int
	// tool is the armed tool when armed is set.
	tool  ui.Tool
	armed bool
	// drawing is set while the polygon tool has area open.
	drawing bool
	// anchor is the first click of the rectangle (a corner) or circle (the
	// center) tool, when anchored; hover is the surface point under the
	// cursor, when hovering; ghost is the outline placed from the anchor
	// to hover, with its Z range ghostZ.
	anchor, hover      zone.Point
	anchored, hovering bool
	ghost              []zone.Point
	ghostZ             zSuggestion
	// placing is the ID reserved for the area the rectangle or circle tool
	// places, 0 before the first anchor: its profile is measured under it.
	placing spawn.AreaID
	// known is the server Z each outline vertex was picked at (a click, a
	// drag, the ground under a generated vertex), where its handle is
	// drawn: the document keeps x y only.
	known map[areaVertex]int
	drag  drag
	// step is how far the height window's Subir/Descer and PageUp/PageDown
	// move the area.
	step int
	// heightFilled is what the height window's fields were last filled
	// for.
	heightFilled spawnHeightKey
	// problems is what the problem panel shows, built for version
	// problemsAt-1 in problemsLang.
	problems     []spawn.Problem
	problemsAt   int
	problemsLang locale.Language
	// win is woken when a generation comes back on results; generating
	// is the area one runs for, 0 when none (spawnpoints.go).
	win        *app.Window
	results    chan generation
	generating spawn.AreaID
	// free is the free floor each generation measured, by the
	// fingerprint of the inputs it read: the inspector shows the one of
	// the selected area's points.
	free map[spawn.Fingerprint]float64
	// point is the selected point, of the current area only (none when
	// its area is 0 or another); pointDrag is a press on a point being
	// dragged.
	point     areaPoint
	pointDrag pointDrag
	// adding is set while viewport clicks add points to the current area.
	adding bool
	// pointsInfo is the panel's points section, built for pointsKey.
	pointsInfo pointsInfo
	pointsKey  pointsKey
}

// areaVertex is one outline vertex of one area, as known keys it.
type areaVertex struct {
	area spawn.AreaID
	v    spawn.Vertex
}

// areaPoint is point index of area.
type areaPoint struct {
	area  spawn.AreaID
	index int
}

// spawnHeightKey is the area and version the height fields were last
// filled for.
type spawnHeightKey struct {
	area    spawn.AreaID
	ok      bool
	version int
}

func newSpawnEditor(w *app.Window, radius int) *spawnEditor {
	return &spawnEditor{
		doc:          spawn.NewDocument(),
		cover:        newFloorCoverage(w),
		radius:       radius,
		sel:          -1,
		known:        map[areaVertex]int{},
		step:         ui.DefaultZStep,
		heightFilled: spawnHeightKey{version: -1},
		win:          w,
		results:      make(chan generation, 1),
		free:         map[spawn.Fingerprint]float64{},
	}
}

// areaRef is area id's slot in the spawn editor's floorCoverage.
func areaRef(id spawn.AreaID) shapeRef { return shapeRef{zone: zone.ZoneID(id)} }

func (e *spawnEditor) coverageBans(shapeRef) ([]coverage.Ban, uint64) { return nil, 0 }

// keepsCoverage keeps the profiles of the areas and of the one being
// placed.
func (e *spawnEditor) keepsCoverage(ref shapeRef) bool {
	id := spawn.AreaID(ref.zone)
	_, ok := e.doc.Area(id)
	return ok || id == e.placing
}

func (e *spawnEditor) zMargin() int        { return spawnZMargin }
func (e *spawnEditor) zFromVertices() bool { return false }

// apply runs c on the document, logging a failure, and returns its error.
// Every UI request that edits an area goes through it.
func (e *spawnEditor) apply(c spawn.Command) error {
	if err := e.doc.Apply(c); err != nil {
		log.Printf("spawn: %v", err)
		return err
	}
	e.version++
	e.edits++
	return nil
}

// replace makes doc the edited document: the areas of an opened project.
// Tool, selection and drag start over. When an area was left with fewer
// than 3 vertices, it is selected with the polygon tool armed on it, so
// the user carries on drawing it. A generation still running for the old
// document is dropped when it comes back (receive).
func (e *spawnEditor) replace(doc *spawn.Document) {
	*e = spawnEditor{
		doc: doc, cover: e.cover, radius: e.radius, sel: -1, known: map[areaVertex]int{},
		step: e.step, version: e.version + 1, edits: e.edits + 1, heightFilled: spawnHeightKey{version: -1},
		win: e.win, results: e.results, free: e.free,
	}
	for _, a := range doc.Areas() {
		e.area = a.ID
		if len(a.Outline) < 3 {
			e.tool, e.armed, e.drawing = ui.ToolPolygon, true, true
			return
		}
	}
}

// newName is the name a new area id gets: area_<id>, or with a suffix no
// area has when another area took it.
func (e *spawnEditor) newName(id spawn.AreaID) string {
	name := fmt.Sprintf("area_%d", id)
	taken := map[string]bool{}
	for _, a := range e.doc.Areas() {
		taken[a.Name] = true
	}
	if !taken[name] {
		return name
	}
	return zone.CopyName(name, taken)
}

// vertexZ is the Z of vertex v of area a's handle: where it was picked,
// else the middle of the area's range.
func (e *spawnEditor) vertexZ(a spawn.Area, v spawn.Vertex) int {
	if z, ok := e.known[areaVertex{a.ID, v}]; ok {
		return z
	}
	return (a.ZMin + a.ZMax) / 2
}

// points is a's outline with its handles' Z.
func (e *spawnEditor) points(a spawn.Area) []zone.Point {
	pts := make([]zone.Point, len(a.Outline))
	for i, v := range a.Outline {
		pts[i] = zone.Point{X: v.X, Y: v.Y, Z: e.vertexZ(a, v)}
	}
	return pts
}

// remember records p as the Z of area id's vertex at p's x y.
func (e *spawnEditor) remember(id spawn.AreaID, p zone.Point) {
	e.known[areaVertex{id, spawn.Vertex{X: p.X, Y: p.Y}}] = p.Z
}

// arm makes tool take the viewport clicks: each shape tool draws a new
// area, and stops adding points. It returns the status message.
func (e *spawnEditor) arm(tool ui.Tool) locale.Message {
	if e.drawing {
		return locale.Message{Key: "spawn.tool.finish_first"}
	}
	e.tool, e.armed, e.adding = tool, true, false
	e.anchored, e.hovering = false, false
	e.version++
	return e.hintMessage()
}

func (e *spawnEditor) hintMessage() locale.Message {
	switch e.tool {
	case ui.ToolRectangle:
		if e.anchored {
			return locale.Message{Key: "editor.hint.rectangle_opposite"}
		}
		return locale.Message{Key: "editor.hint.rectangle_first"}
	case ui.ToolCircle:
		if e.anchored {
			return locale.Message{Key: "editor.hint.circle_radius"}
		}
		return locale.Message{Key: "editor.hint.circle_center"}
	}
	if e.drawing {
		return locale.Message{Key: "editor.hint.polygon_continue"}
	}
	return locale.Message{Key: "spawn.hint.polygon_first"}
}

// info is the armed tool's hint, or adding points', for the message card.
func (e *spawnEditor) info(lang locale.Language) []string {
	switch {
	case e.armed:
		return []string{e.hintMessage().Render(lang)}
	case e.adding:
		if a, ok := e.current(); ok {
			return []string{locale.Format(lang, "spawn.hint.add_points", map[string]string{"name": a.Name})}
		}
	}
	return nil
}

// escape puts the armed tool down, dropping a rectangle's first corner or
// a circle's center, or stops adding points. An open polygon stays: it is
// in the document and closes with Enter.
func (e *spawnEditor) escape() locale.Message {
	switch {
	case e.drawing:
		return locale.Message{Key: "editor.polygon.open"}
	case !e.armed && e.adding:
		return e.setAdding(false)
	case !e.armed:
		return locale.Message{}
	}
	e.armed, e.anchored, e.hovering = false, false, false
	e.version++
	return locale.Message{Key: "editor.tool.put_away"}
}

// click handles a click at viewport pixel p that picked h (ok: something
// was hit) over w. Nothing happens unless a tool is armed.
func (e *spawnEditor) click(w *scene.World, cam *camera.Camera, p f32.Point, vp image.Point, h scene.Hit, ok bool) locale.Message {
	if !e.armed {
		return locale.Message{}
	}
	if e.drawing {
		if a, found := e.doc.Area(e.area); found && len(a.Outline) >= 3 {
			if d, vis := screenDist(w, cam, e.points(a)[0], p, vp); vis && d <= closeSlop {
				return e.close(w)
			}
		}
	}
	if !ok {
		return locale.Message{Key: "editor.click.missed"}
	}
	v := serverPoint(h)
	switch e.tool {
	case ui.ToolRectangle:
		return e.rectangleClick(w, v)
	case ui.ToolCircle:
		return e.circleClick(w, v)
	}
	return e.polygonClick(v)
}

// polygonClick adds vertex v to the area being drawn, creating the area
// with it if none is.
func (e *spawnEditor) polygonClick(v zone.Point) locale.Message {
	xy := spawn.Vertex{X: v.X, Y: v.Y}
	if !e.drawing {
		id := e.doc.NewAreaID()
		zmin, zmax := zone.SuggestZRange([]zone.Point{v}, spawnZMargin)
		if e.apply(spawn.CreateArea{ID: id, Name: e.newName(id), Outline: []spawn.Vertex{xy}, ZMin: zmin, ZMax: zmax, Params: spawn.DefaultParams(e.radius)}) != nil {
			return locale.Message{Key: "editor.polygon.start_failed"}
		}
		e.drawing, e.area, e.sel = true, id, -1
	} else {
		a, _ := e.doc.Area(e.area)
		if e.apply(spawn.InsertVertex{Area: e.area, Index: len(a.Outline), Point: xy}) != nil {
			return locale.Message{Key: "editor.vertex.add_failed"}
		}
	}
	e.remember(e.area, v)
	a, _ := e.doc.Area(e.area)
	n := len(a.Outline)
	log.Printf("spawn: %s: vértice %d em %d %d %d", a.Name, n, v.X, v.Y, v.Z)
	return locale.Message{Key: "editor.polygon.vertex", Count: n, Plural: true, Args: pointArgs(v)}
}

// close ends the polygon being drawn with the Z range suggested by the
// floor under its outline over w, measured now, or from its vertices when
// the floor gives none.
func (e *spawnEditor) close(w *scene.World) locale.Message {
	if !e.drawing {
		return locale.Message{}
	}
	a, _ := e.doc.Area(e.area)
	pts := e.points(a)
	if len(pts) < 3 {
		return locale.Message{Key: "editor.polygon.too_few", Count: len(pts), Plural: true}
	}
	vmin, vmax := zone.SuggestZRange(pts, spawnZMargin)
	fit := e.cover.suggest(e, w, areaRef(a.ID), pts, vmin, vmax, false, true)
	if e.apply(spawn.SetZRange{Area: a.ID, ZMin: fit.zmin, ZMax: fit.zmax}) != nil {
		return locale.Message{Key: "editor.polygon.close_failed"}
	}
	e.drawing, e.armed = false, false
	log.Printf("spawn: %s: polígono fechado com %s, z %d..%d %s", a.Name, inflect.Count(len(pts), "vértice", "vértices"), fit.zmin, fit.zmax, fit.note())
	return created(a.Name, len(pts), fit)
}

// created is the status of a new area name of n vertices with range fit.
func created(name string, n int, fit zSuggestion) locale.Message {
	return locale.Message{Key: "spawn.area.created", Count: n, Plural: true,
		Args:  map[string]string{"name": name, "min": intArg(fit.zmin), "max": intArg(fit.zmax)},
		Parts: map[string]locale.Message{"source": fit.sourceMessage()}}
}

// rectangleClick takes corner v: the first one anchors the rectangle, the
// second adds the area.
func (e *spawnEditor) rectangleClick(w *scene.World, v zone.Point) locale.Message {
	if !e.anchored {
		e.anchorAt(v)
		return locale.Message{Key: "editor.rectangle.anchor", Args: pointArgs(v)}
	}
	if a := e.anchor; a.X == v.X || a.Y == v.Y {
		return locale.Message{Key: "editor.rectangle.zero_size"}
	}
	return e.place(w, v)
}

// circleClick takes the center, then a point on the circle: the area is a
// polygon of zone.CircleSides vertices.
func (e *spawnEditor) circleClick(w *scene.World, v zone.Point) locale.Message {
	if !e.anchored {
		e.anchorAt(v)
		return locale.Message{Key: "editor.circle.anchor", Args: pointArgs(v)}
	}
	if r := radius(e.anchor, v); r < minCircleRadius {
		return locale.Message{Key: "editor.circle.small_radius", Args: map[string]string{"radius": intArg(r), "min": intArg(minCircleRadius)}}
	}
	return e.place(w, v)
}

// anchorAt takes v as the rectangle's or circle's first click, reserving
// the ID of the area it places.
func (e *spawnEditor) anchorAt(v zone.Point) {
	e.anchor, e.anchored = v, true
	if e.placing == 0 {
		e.placing = e.doc.NewAreaID()
	}
	e.version++
}

// place adds the area the armed rectangle or circle tool places from the
// anchor to v over w, and selects it.
func (e *spawnEditor) place(w *scene.World, v zone.Point) locale.Message {
	pts, fit := e.placed(w, v, true)
	id := e.placing
	outline := make([]spawn.Vertex, len(pts))
	for i, p := range pts {
		outline[i] = spawn.Vertex{X: p.X, Y: p.Y}
	}
	name := e.newName(id)
	if e.apply(spawn.CreateArea{ID: id, Name: name, Outline: outline, ZMin: fit.zmin, ZMax: fit.zmax, Params: spawn.DefaultParams(e.radius)}) != nil {
		return locale.Message{Key: "spawn.area.create_failed"}
	}
	for _, p := range pts {
		e.remember(id, p)
	}
	e.placing = 0
	e.armed, e.anchored, e.hovering = false, false, false
	e.area, e.sel = id, -1
	log.Printf("spawn: %s: %s de %s, z %d..%d %s", name, toolName(e.tool), inflect.Count(len(pts), "vértice", "vértices"), fit.zmin, fit.zmax, fit.note())
	return created(name, len(pts), fit)
}

// toolName names a shape tool in the log.
func toolName(t ui.Tool) string {
	switch t {
	case ui.ToolRectangle:
		return "retângulo"
	case ui.ToolCircle:
		return "círculo"
	}
	return "polígono"
}

// placed is the outline the armed rectangle or circle tool places from the
// anchor to v over w (shapeOutline) and its Z range: clicks (now) and the
// preview both go through it, as in the zone editor (zoneEditor.placed).
func (e *spawnEditor) placed(w *scene.World, v zone.Point, now bool) ([]zone.Point, zSuggestion) {
	pts, vertices := shapeOutline(w, e.tool, e.anchor, v)
	vmin, vmax := zone.SuggestZRange(vertices, spawnZMargin)
	return pts, e.cover.suggest(e, w, areaRef(e.placing), pts, vmin, vmax, false, now)
}

// hoverAt tracks the surface point under the cursor (h, when ok) while a
// rectangle or circle is anchored and places the preview on w; while the
// preview's floor is being measured it takes the range again. It returns
// the status message when the preview's range changed.
func (e *spawnEditor) hoverAt(w *scene.World, h scene.Hit, ok bool) locale.Message {
	ok = ok && w != nil && e.armed && e.anchored
	p := zone.Point{}
	if ok {
		p = serverPoint(h)
	}
	moved := ok != e.hovering || p != e.hover
	if !moved && !(ok && e.ghostZ.from == zMeasuring) {
		return locale.Message{}
	}
	e.hover, e.hovering = p, ok
	if !ok {
		e.version++
		return locale.Message{}
	}
	fit := e.ghostZ
	if moved {
		e.ghost, fit = e.placed(w, p, false)
	} else {
		fit = e.cover.suggest(e, w, areaRef(e.placing), e.ghost, fit.vmin, fit.vmax, false, false)
	}
	if !moved && fit == e.ghostZ {
		return locale.Message{}
	}
	e.ghostZ = fit
	e.version++
	return locale.Message{Key: "editor.preview", Args: map[string]string{"min": intArg(fit.zmin), "max": intArg(fit.zmax)}, Parts: map[string]locale.Message{"hint": e.hintMessage(), "source": fit.sourceMessage()}}
}

// preview is the rectangle or circle being placed: from the anchor to the
// cursor, or the anchor alone while the cursor is off every surface.
func (e *spawnEditor) preview() (render.ZoneShape, bool) {
	if !e.armed || !e.anchored {
		return render.ZoneShape{}, false
	}
	c := linearColor(areaColor(e.placing))
	if !e.hovering {
		return render.ZoneShape{Points: []geom.Vec3{serverVec(e.anchor)}, Color: c, Marked: 0}, true
	}
	rs := overlayShape(e.ghost, e.ghostZ.zmin, e.ghostZ.zmax, c)
	if e.tool == ui.ToolRectangle {
		rs.Marked = 0
	}
	return rs, true
}

// areaColor is area id's colour in the overlay and the list: the zone
// colour palette, in turn.
func areaColor(id spawn.AreaID) zone.Color {
	return zoneColors[(max(int(id), 1)-1)%len(zoneColors)]
}

func areaNRGBA(id spawn.AreaID) color.NRGBA {
	c := areaColor(id)
	return color.NRGBA{R: c[0], G: c[1], B: c[2], A: 0xFF}
}

// outlineRules flag an area's prism in the overlay: the rest of an area's
// problems (no NPC, no points yet) are expected while it is edited and
// only show in the problem panel.
func outlineRule(r spawn.Rule) bool {
	switch r {
	case spawn.TooFewVertices, spawn.SelfIntersection, spawn.RepeatedVertex, spawn.InvertedZRange:
		return true
	}
	return false
}

// overlay is every shown area's prism for the renderer, the area being
// drawn as an open polyline, the selected one with its edge midpoint
// handles and selected vertex, plus the rectangle or circle being placed.
func (e *spawnEditor) overlay() []render.ZoneShape {
	var shapes []render.ZoneShape
	for _, a := range e.doc.Areas() {
		open := e.drawing && a.ID == e.area
		if a.Hidden && !open {
			continue
		}
		zmin, zmax := e.shownZRange(a)
		rs := overlayShape(e.shownPoints(a), zmin, zmax, linearColor(areaColor(a.ID)))
		if open {
			rs.Closed, rs.Marked = false, 0
		} else {
			for _, p := range e.doc.AreaProblems(a.ID) {
				if outlineRule(p.Rule) {
					rs.Problem = true
					if p.Vertex >= 0 {
						rs.BadVertices = append(rs.BadVertices, p.Vertex)
					}
				}
			}
		}
		if !e.drawing && a.ID == e.area {
			rs.Midpoints = true
			if v, ok := e.selected(); ok {
				rs.Marked = v
			}
		}
		shapes = append(shapes, rs)
	}
	if pv, ok := e.preview(); ok {
		shapes = append(shapes, pv)
	}
	return shapes
}
