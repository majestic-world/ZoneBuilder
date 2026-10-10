package main

import (
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/render"
)

// populateMode is the spawn area editor ("Popular zona"). The project has
// no spawn areas yet: its inspector panel lists none, its history has
// nothing to undo, it has nothing to compile, and the renderer shows no
// overlay over the map, only the grid when the Ground switch is on.
type populateMode struct {
	// sent is set once the renderer has this mode's overlay; grid is the
	// Ground switch it was sent for.
	sent, grid bool
}

func (p *populateMode) viewportEvent(ws *workspace, ev event.Event) bool {
	undo, redo := historyKey(ev)
	switch {
	case undo:
		p.undo(ws)
	case redo:
		p.redo(ws)
	}
	return undo || redo
}

func (p *populateMode) click(*workspace, pointer.Event, pointer.Buttons) {}
func (p *populateMode) key(*workspace, key.Event)                        {}

func (p *populateMode) undo(ws *workspace) {
	ws.status = action(locale.Message{Key: "spawn.undo.empty"})
}

func (p *populateMode) redo(ws *workspace) {
	ws.status = action(locale.Message{Key: "spawn.redo.empty"})
}

func (p *populateMode) compile(ws *workspace) {
	ws.status = action(locale.Message{Key: "spawn.compile.empty"})
}

func (p *populateMode) update(layout.Context, *workspace)  {}
func (p *populateMode) present(layout.Context, *workspace) {}

func (p *populateMode) sync(r *render.Renderer, ws *workspace) {
	if on := ws.shell.Ground.On; !p.sent || p.grid != on {
		r.SetZones(nil)
		r.SetGround(render.Ground{Grid: on})
		p.sent, p.grid = true, on
	}
}

func (p *populateMode) forget() { p.sent = false }
