package main

import (
	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/render"
	"zonebuilder/internal/scene"
)

// populateMode is the spawn area editor ("Popular zona"): the spawn
// document and its editor, and what the renderer last got from them. It
// has nothing to compile yet.
type populateMode struct {
	spawns *spawnEditor
	// sent is set once the renderer has this mode's ground marking; grid
	// is the Ground switch it was sent for. shown is the spawns.version
	// the renderer's overlay was last built for.
	sent, grid bool
	shown      int
	// pressTaken is set when the editor used the last press (a handle
	// grabbed), so its click adds no point.
	pressTaken bool
}

func newPopulateMode(w *app.Window) *populateMode {
	return &populateMode{spawns: newSpawnEditor(w, spawnRadius()), shown: -1}
}

// status puts msg on the status line unless it is empty.
func status(ws *workspace, msg locale.Message) {
	if msg.Key != "" {
		ws.status = action(msg)
	}
}

func (p *populateMode) viewportEvent(ws *workspace, ev event.Event) bool {
	msg, used := p.spawns.viewportEvent(ws.world(), &ws.cam, ev, ws.viewport())
	status(ws, msg)
	if e, ok := ev.(pointer.Event); ok && e.Kind == pointer.Press {
		p.pressTaken = used
	}
	return used
}

func (p *populateMode) click(ws *workspace, e pointer.Event, button pointer.Buttons) {
	if button != pointer.ButtonPrimary {
		return
	}
	spawns := p.spawns
	if !spawns.armed && spawns.adding {
		if !p.pressTaken {
			status(ws, spawns.addPoint(ws.probe.click, ws.probe.clickHit))
		}
		return
	}
	status(ws, spawns.click(ws.world(), &ws.cam, e.Position, ws.viewport(), ws.probe.click, ws.probe.clickHit))
}

func (p *populateMode) key(ws *workspace, e key.Event) {
	if e.State != key.Press {
		return
	}
	switch e.Name {
	case key.NameReturn, key.NameEnter:
		status(ws, p.spawns.close(ws.world()))
	case key.NameEscape:
		status(ws, p.spawns.escape())
	}
}

func (p *populateMode) undo(ws *workspace) { ws.status = action(p.spawns.undo()) }
func (p *populateMode) redo(ws *workspace) { ws.status = action(p.spawns.redo()) }

func (p *populateMode) compile(ws *workspace) {
	ws.status = action(locale.Message{Key: "spawn.compile.empty"})
}

func (p *populateMode) update(gtx layout.Context, ws *workspace) {
	panel, spawns, w := &ws.shell.Spawn, p.spawns, ws.world()
	if t, ok := panel.Tools.Requested(gtx); ok {
		status(ws, spawns.arm(t))
	}
	a, ok := spawns.selectedArea()
	for _, req := range panel.Update(gtx, a, ok) {
		status(ws, spawns.listRequest(req, w, &ws.cam))
	}
	if i, ok := panel.Problems.Clicked(gtx); ok {
		status(ws, spawns.goToProblem(i, w, &ws.cam))
	}
}

// present fills the panel, the height window and the message card, and
// takes back the generations that finished; one running shows in the
// map section's loading line unless tiles are loading.
func (p *populateMode) present(gtx layout.Context, ws *workspace) {
	shell, spawns, w, lang := ws.shell, p.spawns, ws.world(), ws.shell.Language
	panel := &shell.Spawn
	spawns.cover.receive(spawns)
	status(ws, spawns.receive())
	status(ws, spawns.heightPanel(gtx, &shell.Height, w, lang))
	if spawns.anchored && w != nil && ws.probe.inside {
		h, ok := pickAt(w, &ws.cam, ws.probe.cursor, ws.viewport())
		status(ws, spawns.hoverAt(w, h, ok))
	} else {
		spawns.hoverAt(nil, scene.Hit{}, false)
	}
	panel.Info = spawns.info(lang)
	panel.Rows, panel.Selected = spawns.rows(lang), spawns.area
	if rows, ok := spawns.problemRows(lang); ok {
		panel.Problems.Rows = rows
	}
	panel.Tools.Armed, panel.Tools.Active = spawns.tool, spawns.armed
	info := spawns.pointsPanel(lang)
	panel.PointsNote, panel.PointStats, panel.PointWarnings = info.note, info.stats, info.warnings
	panel.Generating, panel.Adding = spawns.generating != 0, spawns.adding
	if a, ok := spawns.doc.Area(spawns.generating); ok && shell.Loading == "" {
		shell.Loading, shell.Progress = locale.Format(lang, "spawn.generate.loading", map[string]string{"name": a.Name}), 0
	}
}

func (p *populateMode) sync(r *render.Renderer, ws *workspace) {
	if on := ws.shell.Ground.On; !p.sent || p.grid != on {
		r.SetGround(render.Ground{Grid: on})
		p.sent, p.grid = true, on
	}
	if p.shown != p.spawns.version {
		r.SetZones(append(p.spawns.overlay(), p.spawns.pins()...))
		p.shown = p.spawns.version
	}
}

// forget makes the next sync send everything, and refills the height
// window's fields, which the zone mode shares.
func (p *populateMode) forget() {
	p.sent, p.shown = false, -1
	p.spawns.heightFilled = spawnHeightKey{version: -1}
}
