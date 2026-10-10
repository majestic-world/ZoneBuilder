package main

import (
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/render"
	"zonebuilder/internal/spawn"
)

// populateMode is the spawn area editor ("Popular zona"). The project has
// no spawn areas yet: its inspector panel lists none, its history has
// nothing to undo, it has nothing to compile, and the renderer shows no
// overlay over the map, only the grid when the Ground switch is on.
type populateMode struct {
	// sent is set once the renderer has this mode's overlay; grid is the
	// Ground switch it was sent for.
	sent, grid bool
	// preview draws the monster on the points while the Prévia switch
	// is on.
	preview monsterPreview
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

func (p *populateMode) update(gtx layout.Context, ws *workspace) {
	ws.shell.Preview.Toggled(gtx)
}

func (p *populateMode) present(gtx layout.Context, ws *workspace) {
	if !ws.shell.Preview.On {
		return
	}
	if err := p.preview.advance(gtx.Now); err != nil {
		p.previewFailed(ws, err)
		return
	}
	// Wait keeps playing.
	gtx.Execute(op.InvalidateCmd{})
}

// previewPoints are the spawn points the preview draws the monster on.
func (p *populateMode) previewPoints() []spawn.Point {
	return nil
}

// previewFailed turns the preview off and reports err.
func (p *populateMode) previewFailed(ws *workspace, err error) {
	ws.shell.Preview.On = false
	ws.status = actionError(locale.Message{Key: "spawn.preview.error"}, err, nil)
}

func (p *populateMode) sync(r *render.Renderer, ws *workspace) {
	if on := ws.shell.Ground.On; !p.sent || p.grid != on {
		r.SetZones(nil)
		r.SetGround(render.Ground{Grid: on})
		p.sent, p.grid = true, on
	}
	if ws.shell.Preview.On {
		if err := p.preview.draw(r, p.previewPoints()); err != nil {
			p.previewFailed(ws, err)
		}
	}
}

func (p *populateMode) forget() { p.sent = false }
