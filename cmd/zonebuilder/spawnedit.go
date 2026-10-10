package main

import (
	"image"
	"log"
	"strconv"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/coverage"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/spawn"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// selectedArea is the selected area, the one the list highlights and the
// properties edit; false when none is.
func (e *spawnEditor) selectedArea() (spawn.Area, bool) {
	if e.area == 0 {
		return spawn.Area{}, false
	}
	return e.doc.Area(e.area)
}

// current is the selected area once closed: the one the vertex edits and
// the height window work on.
func (e *spawnEditor) current() (spawn.Area, bool) {
	a, ok := e.selectedArea()
	if !ok || e.drawing {
		return spawn.Area{}, false
	}
	return a, true
}

// selected is the selected vertex of the current area.
func (e *spawnEditor) selected() (int, bool) {
	a, ok := e.current()
	if !ok || e.sel < 0 || e.sel >= len(a.Outline) {
		return -1, false
	}
	return e.sel, true
}

// shownPoints is a's outline with its handles' Z, as the overlay draws it:
// moved by the drag in progress.
func (e *spawnEditor) shownPoints(a spawn.Area) []zone.Point {
	pts := e.points(a)
	d := e.drag
	if d.kind == dragNone || !d.moved || a.ID != e.area {
		return pts
	}
	v, ok := e.selected()
	if !ok {
		return pts
	}
	if d.kind == dragVertex {
		pts[v] = d.to
		return pts
	}
	for i := range pts {
		pts[i].X += d.to.X - d.from.X
		pts[i].Y += d.to.Y - d.from.Y
		pts[i].Z += d.to.Z - d.from.Z
	}
	return pts
}

// shownZRange is a's Z range as the overlay draws it while an area drag
// moves it.
func (e *spawnEditor) shownZRange(a spawn.Area) (int, int) {
	d := e.drag
	if d.kind != dragShape || !d.moved || a.ID != e.area {
		return a.ZMin, a.ZMax
	}
	return a.ZMin + d.to.Z - d.from.Z, a.ZMax + d.to.Z - d.from.Z
}

// sync drops UI state that an Undo or Redo left pointing at nothing: the
// selection and the polygon being drawn of an area that is gone.
func (e *spawnEditor) sync() {
	if _, ok := e.doc.Area(e.area); !ok {
		if e.drawing {
			e.drawing, e.armed = false, false
		}
		e.area, e.sel = 0, -1
	}
	e.drag = drag{}
}

func (e *spawnEditor) undo() locale.Message {
	if !e.doc.Undo() {
		return locale.Message{Key: "spawn.undo.empty"}
	}
	e.version++
	e.edits++
	e.sync()
	log.Printf("spawn: desfeito")
	return locale.Message{Key: "spawn.undo.done"}
}

func (e *spawnEditor) redo() locale.Message {
	if !e.doc.Redo() {
		return locale.Message{Key: "spawn.redo.empty"}
	}
	e.version++
	e.edits++
	e.sync()
	log.Printf("spawn: refeito")
	return locale.Message{Key: "spawn.redo.done"}
}

// viewportEvent handles the editing input of the viewport: Ctrl+Z/Ctrl+Y,
// Delete, PageUp/PageDown, and presses and drags on vertex and edge
// midpoint handles. It reports whether it used ev (which then must not
// move the camera) and the status message, empty to keep the current one.
func (e *spawnEditor) viewportEvent(w *scene.World, cam *camera.Camera, ev event.Event, vp image.Point) (locale.Message, bool) {
	switch ev := ev.(type) {
	case key.Event:
		undo, redo := historyKey(ev)
		switch {
		case undo:
			return e.undo(), true
		case redo:
			return e.redo(), true
		case ev.State != key.Press:
			return locale.Message{}, ev.Name == "Z" || ev.Name == "Y" || ev.Name == key.NameDeleteForward || ev.Name == key.NamePageUp || ev.Name == key.NamePageDown
		}
		switch ev.Name {
		case key.NameDeleteForward:
			return e.removeVertex(), true
		case key.NamePageUp:
			return e.shiftArea(e.step), true
		case key.NamePageDown:
			return e.shiftArea(-e.step), true
		}
	case pointer.Event:
		if w == nil || e.drawing || e.armed {
			return locale.Message{}, false
		}
		if e.drag.kind != dragNone {
			return e.dragEvent(w, cam, ev, vp), true
		}
		if ev.Kind == pointer.Press && ev.Buttons == pointer.ButtonPrimary {
			return e.press(w, cam, ev, vp)
		}
	}
	return locale.Message{}, false
}

// press grabs the vertex handle under ev (Ctrl grabs its whole area) or,
// on the current area's edge midpoint handle, inserts a vertex there and
// grabs it.
func (e *spawnEditor) press(w *scene.World, cam *camera.Camera, ev pointer.Event, vp image.Point) (locale.Message, bool) {
	best := float32(grabSlop)
	var hit spawn.AreaID
	index := -1
	for _, a := range e.doc.Areas() {
		if a.Hidden {
			continue
		}
		for i, p := range e.points(a) {
			if d, ok := screenDist(w, cam, p, ev.Position, vp); ok && d <= best {
				best, hit, index = d, a.ID, i
			}
		}
	}
	if hit != 0 {
		e.area, e.sel = hit, index
		e.version++
		a, _ := e.doc.Area(hit)
		p := e.points(a)[index]
		e.drag = drag{kind: dragVertex, press: ev.Position, from: p}
		if ev.Modifiers.Contain(key.ModCtrl) {
			e.drag.kind = dragShape
			return locale.Message{Key: "spawn.drag.area", Args: map[string]string{"name": a.Name}}, true
		}
		return locale.Message{Key: "spawn.drag.vertex", Args: map[string]string{"index": intArg(index + 1), "name": a.Name, "x": intArg(p.X), "y": intArg(p.Y), "z": intArg(p.Z)}}, true
	}
	if a, ok := e.current(); ok && len(a.Outline) >= 2 {
		pts := e.points(a)
		for i := range pts {
			if d, ok := screenDist(w, cam, midpoint(pts, i), ev.Position, vp); ok && d <= grabSlop {
				msg := e.insertAfter(i)
				if v, ok := e.selected(); ok && v == i+1 {
					e.drag = drag{kind: dragVertex, press: ev.Position, from: midpoint(pts, i)}
				}
				return msg, true
			}
		}
	}
	return locale.Message{}, false
}

// dragEvent follows a drag: the grabbed vertex snaps to the surface under
// the cursor, and the release applies the move as one command.
func (e *spawnEditor) dragEvent(w *scene.World, cam *camera.Camera, ev pointer.Event, vp image.Point) locale.Message {
	switch ev.Kind {
	case pointer.Drag:
		if !e.drag.moved && dist(ev.Position, e.drag.press) <= clickSlop {
			return locale.Message{}
		}
		if h, ok := pickAt(w, cam, ev.Position, vp); ok {
			e.drag.to, e.drag.moved = serverPoint(h), true
			e.version++
			return locale.Message{Key: "spawn.drag.drop", Args: pointArgs(e.drag.to)}
		}
		return locale.Message{Key: "editor.click.no_surface"}
	case pointer.Release:
		d := e.drag
		e.drag = drag{}
		e.version++
		v, ok := e.selected()
		if !d.moved || d.to == d.from || !ok {
			return locale.Message{}
		}
		if d.kind == dragShape {
			return e.moveArea(d.to.X-d.from.X, d.to.Y-d.from.Y, d.to.Z-d.from.Z)
		}
		return e.moveVertex(v, d.to)
	case pointer.Cancel:
		e.drag = drag{}
		e.version++
	}
	return locale.Message{}
}

func (e *spawnEditor) moveVertex(v int, p zone.Point) locale.Message {
	if e.apply(spawn.MoveVertex{Area: e.area, Index: v, Point: spawn.Vertex{X: p.X, Y: p.Y}}) != nil {
		return locale.Message{Key: "editor.vertex.move_failed"}
	}
	e.remember(e.area, p)
	log.Printf("spawn: vértice %d movido para %d %d %d", v+1, p.X, p.Y, p.Z)
	return locale.Message{Key: "editor.vertex.moved", Args: map[string]string{"index": intArg(v + 1), "x": intArg(p.X), "y": intArg(p.Y), "z": intArg(p.Z)}}
}

// moveArea moves the current area by dx dy, and its range and handles by
// dz.
func (e *spawnEditor) moveArea(dx, dy, dz int) locale.Message {
	a, ok := e.current()
	if !ok {
		return locale.Message{Key: "spawn.area.move_select"}
	}
	if e.translate(a, dx, dy, dz) != nil {
		return locale.Message{Key: "spawn.area.move_failed"}
	}
	return locale.Message{Key: "spawn.area.moved", Args: map[string]string{"name": a.Name, "x": intArg(dx), "y": intArg(dy), "z": intArg(dz)}}
}

// translate applies the move of a by dx dy dz, its handles along.
func (e *spawnEditor) translate(a spawn.Area, dx, dy, dz int) error {
	pts := e.points(a)
	if err := e.apply(spawn.MoveArea{Area: a.ID, DX: dx, DY: dy, DZ: dz}); err != nil {
		return err
	}
	for _, p := range pts {
		e.remember(a.ID, zone.Point{X: p.X + dx, Y: p.Y + dy, Z: p.Z + dz})
	}
	log.Printf("spawn: %s movida por %d %d %d", a.Name, dx, dy, dz)
	return nil
}

// shiftArea raises (dz > 0) or lowers the current area.
func (e *spawnEditor) shiftArea(dz int) locale.Message {
	a, ok := e.current()
	switch {
	case !ok:
		return locale.Message{Key: "spawn.area.move_select"}
	case dz == 0:
		return locale.Message{}
	}
	if e.translate(a, 0, 0, dz) != nil {
		return locale.Message{Key: "spawn.area.move_failed"}
	}
	args := map[string]string{"name": a.Name, "delta": intArg(abs(dz)), "min": intArg(a.ZMin + dz), "max": intArg(a.ZMax + dz)}
	if dz < 0 {
		return locale.Message{Key: "editor.zone.lowered", Args: args}
	}
	return locale.Message{Key: "editor.zone.raised", Args: args}
}

// insertAfter inserts a vertex in the middle of the current area's edge
// from vertex i to the next one and selects it.
func (e *spawnEditor) insertAfter(i int) locale.Message {
	a, ok := e.current()
	if !ok || len(a.Outline) < 2 {
		return locale.Message{Key: "spawn.vertex.select_area"}
	}
	p := midpoint(e.points(a), i)
	if e.apply(spawn.InsertVertex{Area: a.ID, Index: i + 1, Point: spawn.Vertex{X: p.X, Y: p.Y}}) != nil {
		return locale.Message{Key: "editor.vertex.insert_failed"}
	}
	e.remember(a.ID, p)
	e.sel = i + 1
	log.Printf("spawn: vértice %d inserido em %d %d %d", i+2, p.X, p.Y, p.Z)
	return locale.Message{Key: "editor.vertex.inserted", Args: map[string]string{"index": intArg(i + 2), "x": intArg(p.X), "y": intArg(p.Y), "z": intArg(p.Z)}}
}

func (e *spawnEditor) removeVertex() locale.Message {
	v, ok := e.selected()
	if !ok {
		return locale.Message{Key: "editor.vertex.select_remove"}
	}
	if e.apply(spawn.RemoveVertex{Area: e.area, Index: v}) != nil {
		return locale.Message{Key: "editor.vertex.remove_failed"}
	}
	a, _ := e.doc.Area(e.area)
	e.sel = min(v, len(a.Outline)-1)
	log.Printf("spawn: vértice %d apagado", v+1)
	return locale.Message{Key: "editor.vertex.removed", Count: len(a.Outline), Plural: true, Args: map[string]string{"index": intArg(v + 1)}}
}

// heightPanel handles the floating height window's requests and fills it
// for the current area: its range, and the floor under it over w with the
// ruler. It returns the status message.
func (e *spawnEditor) heightPanel(gtx layout.Context, p *ui.HeightPanel, w *scene.World, lang locale.Language) locale.Message {
	var msg locale.Message
	p.ReopenRequested(gtx)
	e.step = p.StepZ()
	if p.Up.Clicked(gtx) {
		msg = e.shiftArea(e.step)
	}
	if p.Down.Clicked(gtx) {
		msg = e.shiftArea(-e.step)
	}
	if p.BaseRequested(gtx) {
		msg = e.setBase(p.Base.Text())
	}
	if p.HeightRequested(gtx) {
		msg = e.setHeight(p.Height.Text())
	}
	if p.FloorToGround.Clicked(gtx) {
		msg = e.toGround(w, false)
	}
	if p.TopToGround.Clicked(gtx) {
		msg = e.toGround(w, true)
	}
	a, ok := e.current()
	p.Zone, p.Coverage, p.Ruler = "", "", ui.Ruler{}
	if !ok {
		e.heightFilled = spawnHeightKey{version: -1}
		return msg
	}
	zmin, zmax := e.shownZRange(a)
	p.Zone = locale.Format(lang, "spawn.height.area", map[string]string{"name": a.Name, "min": intArg(zmin), "max": intArg(zmax), "height": intArg(zmax - zmin)})
	if k := (spawnHeightKey{area: a.ID, ok: true, version: e.version}); k != e.heightFilled {
		e.heightFilled = k
		p.Base.SetText(strconv.Itoa(a.ZMin))
		p.Height.SetText(strconv.Itoa(a.ZMax - a.ZMin))
	}
	if w == nil {
		return msg
	}
	r, measuring, rok := e.cover.report(e, w, areaRef(a.ID), e.shownPoints(a), zmin, zmax)
	if !rok {
		if measuring {
			p.Coverage = locale.Text(lang, "spawn.height.measuring")
		}
		return msg
	}
	p.Coverage = areaSummary(lang, r, measuring)
	if part, ok := e.cover.rulerPart(areaRef(a.ID), w, zmin, zmax); ok && r.Measured {
		p.Ruler = rulerOf([]rulerPart{part}, r, areaNRGBA(a.ID), lang)
	}
	return msg
}

// areaSummary is the height window's floor summary of an area's report r.
func areaSummary(lang locale.Language, r coverage.Report, measuring bool) string {
	title := locale.Text(lang, "spawn.height.coverage")
	if measuring {
		title = locale.Text(lang, "spawn.height.coverage_measuring")
	}
	lines := []string{title}
	if r.Measured {
		lines = append(lines,
			locale.Format(lang, "spawn.height.ground", map[string]string{
				"low": measureNumber(lang, roundF(r.GroundMin.Z)), "high": measureNumber(lang, roundF(r.GroundMax.Z)),
			}),
			locale.Format(lang, "coverage.zone.clearances", map[string]string{
				"floor": clearance(lang, r.FloorClearance), "top": clearance(lang, r.TopClearance),
			}),
		)
	} else if r.Excluded > 0 || r.Other > 0 {
		lines = append(lines, locale.Text(lang, "coverage.no_included_ground"))
	} else {
		lines = append(lines, locale.Text(lang, "spawn.height.no_ground"))
	}
	total := r.Total()
	lines = append(lines, locale.Format(lang, "coverage.zone.shares", map[string]string{
		"coverage": share(lang, r.Coverage()), "above": percent(lang, r.Above, total),
		"below": percent(lang, r.Below, total), "missing": percent(lang, r.NoGround, total),
	}))
	return strings.Join(lines, "\n")
}

// setBase moves the current area so its zmin lands at text's z.
func (e *spawnEditor) setBase(text string) locale.Message {
	v, ok := ints(text, 1)
	if !ok {
		return locale.Message{Key: "editor.zone.base_integer"}
	}
	a, ok := e.current()
	if !ok {
		return locale.Message{Key: "spawn.area.move_select"}
	}
	return e.shiftArea(v[0] - a.ZMin)
}

// setHeight makes the current area text's height tall, keeping its zmin.
func (e *spawnEditor) setHeight(text string) locale.Message {
	v, ok := ints(text, 1)
	if !ok || v[0] < 0 {
		return locale.Message{Key: "editor.zone.height_integer"}
	}
	a, ok := e.current()
	if !ok {
		return locale.Message{Key: "spawn.area.move_select"}
	}
	if e.apply(spawn.SetZRange{Area: a.ID, ZMin: a.ZMin, ZMax: a.ZMin + v[0]}) != nil {
		return locale.Message{Key: "editor.zone.height_failed"}
	}
	log.Printf("spawn: %s com altura %d", a.Name, v[0])
	return locale.Message{Key: "editor.zone.height_done", Args: map[string]string{"name": a.Name, "height": intArg(v[0]), "min": intArg(a.ZMin), "max": intArg(a.ZMin + v[0])}}
}

// toGround moves one side of the current area's range to the floor under
// it over w, by the layer rule of spec D5 from its own range: the top
// margin above its highest floor when top is set, else the floor margin
// below its lowest.
func (e *spawnEditor) toGround(w *scene.World, top bool) locale.Message {
	sides, side := coverage.FloorSide, locale.Message{Key: "editor.ground.floor_side"}
	if top {
		sides, side = coverage.TopSide, locale.Message{Key: "editor.ground.top_side"}
	}
	a, ok := e.current()
	switch {
	case w == nil:
		return locale.Message{Key: "editor.ground.open_first"}
	case !ok:
		return locale.Message{Key: "spawn.area.move_select"}
	}
	p := e.cover.profile(e, w, areaRef(a.ID), e.points(a), true)
	if p == nil {
		return locale.Message{Key: "spawn.ground.none", Args: map[string]string{"name": a.Name}, Parts: map[string]locale.Message{"side": side}}
	}
	zmin, zmax, g := p.Fit(a.ZMin, a.ZMax, spawnZMargin, sides)
	if !g.Measured || zmin > zmax {
		return locale.Message{Key: "spawn.ground.none", Args: map[string]string{"name": a.Name}, Parts: map[string]locale.Message{"side": side}}
	}
	if e.apply(spawn.SetZRange{Area: a.ID, ZMin: zmin, ZMax: zmax}) != nil {
		return locale.Message{Key: "spawn.ground.failed"}
	}
	log.Printf("spawn: %s ao chão, faixa %d..%d", a.Name, zmin, zmax)
	return locale.Message{Key: "spawn.ground.done", Args: map[string]string{"name": a.Name, "min": intArg(zmin), "max": intArg(zmax)}, Parts: map[string]locale.Message{"side": side}}
}

// rows is every area as the area list shows it.
func (e *spawnEditor) rows(lang locale.Language) []ui.AreaRow {
	counts := map[spawn.AreaID]int{}
	for _, p := range e.doc.Problems() {
		if p.Blocks() {
			counts[p.Area]++
		}
	}
	areas := e.doc.Areas()
	rows := make([]ui.AreaRow, len(areas))
	for i, a := range areas {
		npc := locale.Text(lang, "spawn.row.no_npc")
		if a.Params.NPCID > 0 {
			npc = locale.Format(lang, "spawn.row.npc", map[string]string{"id": strconv.Itoa(a.Params.NPCID)})
		}
		detail := locale.Format(lang, "spawn.row.detail", map[string]string{"npc": npc, "monsters": locale.Plural(lang, "spawn.count.monsters", a.Params.Count, nil)})
		if e.drawing && a.ID == e.area {
			detail = locale.Text(lang, "editor.row.drawing")
		}
		rows[i] = ui.AreaRow{ID: a.ID, Name: a.Name, Detail: detail, Problems: counts[a.ID], Hidden: a.Hidden, Color: areaNRGBA(a.ID)}
	}
	return rows
}

// listRequest carries out one area panel request; w and cam are the open
// map (nil when none) and its camera. It returns the status message.
func (e *spawnEditor) listRequest(req any, w *scene.World, cam *camera.Camera) locale.Message {
	switch r := req.(type) {
	case ui.SelectArea:
		return e.selectArea(r.Area, w, cam)
	case ui.HideArea:
		a, ok := e.doc.Area(r.Area)
		if !ok || e.apply(spawn.SetHidden{Areas: []spawn.AreaID{r.Area}, Hidden: r.Hidden}) != nil {
			return locale.Message{Key: "spawn.area.visibility_failed"}
		}
		if r.Hidden {
			return locale.Message{Key: "spawn.area.hidden", Args: map[string]string{"name": a.Name}}
		}
		return locale.Message{Key: "spawn.area.shown", Args: map[string]string{"name": a.Name}}
	case ui.DuplicateArea:
		return e.duplicate(r.Area)
	case ui.DeleteArea:
		return e.deleteArea(r.Area)
	case ui.SetAreaField:
		return e.setField(r.Area, r.Field, r.Text)
	}
	return locale.Message{}
}

// selectArea selects area id and frames it. While a polygon is being
// drawn the selection stays on its area, but the camera still goes.
func (e *spawnEditor) selectArea(id spawn.AreaID, w *scene.World, cam *camera.Camera) locale.Message {
	a, ok := e.doc.Area(id)
	if !ok {
		return locale.Message{}
	}
	msg := locale.Message{Key: "spawn.area.selected", Args: map[string]string{"name": a.Name}}
	if e.drawing && id != e.area {
		msg = locale.Message{Key: "spawn.area.select_drawing"}
	} else {
		e.area, e.sel = id, -1
		e.version++
	}
	if w == nil {
		return msg
	}
	b := areaBox(w, e.points(a))
	if b.Empty() {
		return msg
	}
	cam.Frame(b)
	log.Printf("spawn: câmera em %s: %s", a.Name, formatPose(cam, w))
	return msg
}

func (e *spawnEditor) duplicate(id spawn.AreaID) locale.Message {
	a, ok := e.doc.Area(id)
	switch {
	case !ok:
		return locale.Message{}
	case e.drawing:
		return locale.Message{Key: "spawn.area.duplicate_drawing"}
	}
	dup := e.doc.NewAreaID()
	if e.apply(spawn.DuplicateArea{Area: id, ID: dup}) != nil {
		return locale.Message{Key: "spawn.area.duplicate_failed"}
	}
	for _, p := range e.points(a) {
		e.remember(dup, p)
	}
	e.area, e.sel = dup, -1
	c, _ := e.doc.Area(dup)
	log.Printf("spawn: %s duplicada como %s", a.Name, c.Name)
	return locale.Message{Key: "spawn.area.duplicated", Args: map[string]string{"old": a.Name, "name": c.Name}}
}

func (e *spawnEditor) deleteArea(id spawn.AreaID) locale.Message {
	a, ok := e.doc.Area(id)
	if !ok {
		return locale.Message{}
	}
	if e.apply(spawn.DeleteArea{Area: id}) != nil {
		return locale.Message{Key: "spawn.area.delete_failed"}
	}
	if e.area == id {
		e.sync()
	}
	log.Printf("spawn: %s apagada", a.Name)
	return locale.Message{Key: "spawn.area.deleted", Args: map[string]string{"name": a.Name}}
}

// fieldMessage names field f in messages.
func fieldMessage(f ui.AreaField) locale.Message {
	switch f {
	case ui.AreaNPC:
		return locale.Message{Key: "spawn.field.npc"}
	case ui.AreaCount:
		return locale.Message{Key: "spawn.field.count"}
	case ui.AreaRespawn:
		return locale.Message{Key: "spawn.field.respawn"}
	case ui.AreaRespawnRand:
		return locale.Message{Key: "spawn.field.respawn_rand"}
	case ui.AreaRadius:
		return locale.Message{Key: "spawn.field.radius"}
	case ui.AreaClearance:
		return locale.Message{Key: "spawn.field.clearance"}
	}
	return locale.Message{Key: "spawn.field.name"}
}

// setField sets area id's property f from the panel's text: the name as
// typed (trimmed), every other one an integer. Values the server rejects
// are applied and show as problems.
func (e *spawnEditor) setField(id spawn.AreaID, f ui.AreaField, text string) locale.Message {
	a, ok := e.doc.Area(id)
	if !ok {
		return locale.Message{}
	}
	text = strings.TrimSpace(text)
	if f == ui.AreaName {
		if text == a.Name {
			return locale.Message{}
		}
		if e.apply(spawn.Rename{Area: id, Name: text}) != nil {
			return locale.Message{Key: "spawn.area.rename_failed"}
		}
		log.Printf("spawn: %s renomeada para %s", a.Name, text)
		return locale.Message{Key: "spawn.area.renamed", Args: map[string]string{"old": a.Name, "name": text}}
	}
	n, err := strconv.Atoi(text)
	if err != nil {
		return locale.Message{Key: "spawn.field.integer", Parts: map[string]locale.Message{"field": fieldMessage(f)}}
	}
	p := a.Params
	switch f {
	case ui.AreaNPC:
		p.NPCID = n
	case ui.AreaCount:
		p.Count = n
	case ui.AreaRespawn:
		p.Respawn = n
	case ui.AreaRespawnRand:
		p.RespawnRand = n
	case ui.AreaRadius:
		p.Radius = n
	case ui.AreaClearance:
		p.Clearance = n
	}
	if p == a.Params {
		return locale.Message{}
	}
	if e.apply(spawn.SetParams{Area: id, Params: p}) != nil {
		return locale.Message{Key: "spawn.field.failed"}
	}
	log.Printf("spawn: %s: %s = %d", a.Name, fieldMessage(f).Render(locale.PtBR), n)
	return locale.Message{Key: "spawn.field.set", Args: map[string]string{"name": a.Name, "value": intArg(n)}, Parts: map[string]locale.Message{"field": fieldMessage(f)}}
}

// problemRows rebuilds the problem panel's rows when the document or the
// language changed; the kept problems keep the row indices of goToProblem.
func (e *spawnEditor) problemRows(lang locale.Language) ([]ui.ProblemRow, bool) {
	if e.problemsAt == e.version+1 && e.problemsLang == lang {
		return nil, false
	}
	e.problemsAt, e.problemsLang = e.version+1, lang
	e.problems = e.doc.Problems()
	rows := make([]ui.ProblemRow, len(e.problems))
	for i, p := range e.problems {
		rows[i] = ui.ProblemRow{Zone: e.areaName(p.Area, lang), Message: p.Text(lang), Warning: !p.Blocks()}
	}
	return rows, true
}

// areaName is area id's name as the problem panel shows it.
func (e *spawnEditor) areaName(id spawn.AreaID, lang locale.Language) string {
	if a, _ := e.doc.Area(id); strings.TrimSpace(a.Name) != "" {
		return a.Name
	}
	return locale.Text(lang, "spawn.problem.unnamed")
}

// goToProblem selects the area of the problem panel's row i, with the
// problem's vertex selected, and frames where the problem is: the point,
// the vertex, else the area. While a polygon is being drawn the selection
// stays on it, but the camera still goes.
func (e *spawnEditor) goToProblem(i int, w *scene.World, cam *camera.Camera) locale.Message {
	if i < 0 || i >= len(e.problems) {
		return locale.Message{}
	}
	p := e.problems[i]
	a, ok := e.doc.Area(p.Area)
	if !ok {
		return locale.Message{}
	}
	msg := locale.Message{Key: "spawn.problem.selected", Args: map[string]string{"name": a.Name}, Parts: map[string]locale.Message{"problem": p.Message()}}
	if e.drawing {
		msg = locale.Message{Key: "spawn.area.select_drawing"}
	} else {
		e.area, e.sel = a.ID, p.Vertex
		e.version++
	}
	log.Printf("spawn: problema em %s: %s", a.Name, p.Text(locale.PtBR))
	if w == nil {
		return msg
	}
	pts := e.points(a)
	var b geom.Box
	switch {
	case p.Point >= 0 && p.Point < len(a.Points):
		pt := a.Points[p.Point]
		b = pointBox(w, zone.Point{X: pt.X, Y: pt.Y, Z: pt.Z})
	case p.Vertex >= 0 && p.Vertex < len(pts):
		b = pointBox(w, pts[p.Vertex])
	default:
		b = areaBox(w, pts)
	}
	if b.Empty() {
		return msg
	}
	cam.Frame(b)
	log.Printf("spawn: câmera no problema: %s", formatPose(cam, w))
	return msg
}

// areaBox is the box an outline pts is framed as, in w's render space: a
// lone vertex is framed as a point (pointBox); empty for no vertex.
func areaBox(w *scene.World, pts []zone.Point) geom.Box {
	if len(pts) == 1 {
		return pointBox(w, pts[0])
	}
	b := geom.EmptyBox()
	for _, p := range pts {
		b.Include(renderPoint(w, p))
	}
	return b
}
