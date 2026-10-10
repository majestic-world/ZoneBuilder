package main

import (
	"fmt"
	"image"
	"log"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/render"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
)

// workspace is what every mode shares (spec D1): the shell, with the map
// section, the viewport and the command bar; the open tiles; the camera;
// the cursor probe of the status pill; the static meshes hidden one by
// one; and the status line. The project file is the session's, and holds
// every mode's document.
type workspace struct {
	shell  *ui.Shell
	tiles  *tiles
	cam    camera.Camera
	probe  cursorProbe
	meshes meshHiding
	status actionStatus
}

// world is the open map, nil before the first tile arrives.
func (ws *workspace) world() *scene.World { return ws.tiles.world }

// viewport is the viewport's size in pixels, from the last layout.
func (ws *workspace) viewport() image.Point { return ws.shell.Viewport.Size() }

// mode is one of the window's modes. Only the active mode gets the
// viewport events, the keys, the command bar's Undo, Redo and Compile and
// the renderer's overlay, and only it polls and fills its own widgets
// (inspector panel, dock, floating windows); an inactive mode keeps its
// editor and history untouched until it is active again. The window loop
// calls, per frame and in this order: viewportEvent, click and key for each
// viewport event; undo, redo and compile on the command bar's clicks;
// update; present; then, once the shell is laid out, sync.
type mode interface {
	// viewportEvent sees each viewport event first; used keeps it from
	// the fly controls.
	viewportEvent(ws *workspace, ev event.Event) (used bool)
	// click is a click the cursor probe recognised (primary or
	// secondary button) on an open map; for the primary one, ws.probe
	// already holds what it hit.
	click(ws *workspace, e pointer.Event, button pointer.Buttons)
	// key is a viewport key event; Escape only when no menu was open to
	// close.
	key(ws *workspace, e key.Event)
	// undo and redo step the mode's own history; compile opens its XML.
	undo(ws *workspace)
	redo(ws *workspace)
	compile(ws *workspace)
	// update polls the mode's own widgets before the shared ones.
	update(gtx layout.Context, ws *workspace)
	// present fills the mode's part of the shell for this frame's layout.
	present(gtx layout.Context, ws *workspace)
	// sync sends the mode's overlay and ground marking to r when they
	// changed since the last call.
	sync(r *render.Renderer, ws *workspace)
	// forget makes the next sync send everything: the renderer lost it
	// (a new GPU context) or another mode replaced it.
	forget()
}

// modes are the window's 3 modes and the active one.
type modes struct {
	active   ui.Mode
	zones    *zoneMode
	populate *populateMode
}

func newModes(w *app.Window, active ui.Mode) *modes {
	return &modes{active: active, zones: newZoneMode(w), populate: newPopulateMode(w)}
}

// current is the active mode.
func (m *modes) current() mode {
	switch m.active {
	case ui.ModeZones:
		return m.zones
	case ui.ModePopulate:
		return m.populate
	}
	return homeMode{}
}

// set makes next the active mode. Its overlay goes to the renderer again,
// since the previous mode's replaced it.
func (m *modes) set(next ui.Mode) {
	m.active = next
	m.current().forget()
	log.Printf("modo: %s", modeName(next))
}

// forget makes every mode send its overlay again: the renderer is new.
func (m *modes) forget() {
	m.zones.forget()
	m.populate.forget()
}

// The -mode flag's values.
const (
	modeFlagZones    = "zones"
	modeFlagPopulate = "populate"
)

// parseMode reads the -mode flag: empty opens the home screen.
func parseMode(s string) (ui.Mode, error) {
	switch s {
	case "":
		return ui.ModeHome, nil
	case modeFlagZones:
		return ui.ModeZones, nil
	case modeFlagPopulate:
		return ui.ModePopulate, nil
	}
	return 0, fmt.Errorf("%q não é um modo: use %s ou %s", s, modeFlagZones, modeFlagPopulate)
}

// modeName names m in the log.
func modeName(m ui.Mode) string {
	switch m {
	case ui.ModeZones:
		return "zonas"
	case ui.ModePopulate:
		return "população"
	}
	return "início"
}

// historyKey reports whether ev is the undo shortcut (Ctrl+Z) or a redo
// one (Ctrl+Y, Ctrl+Shift+Z), pressed.
func historyKey(ev event.Event) (undo, redo bool) {
	e, ok := ev.(key.Event)
	if !ok || e.State != key.Press || !e.Modifiers.Contain(key.ModShortcut) {
		return false, false
	}
	switch {
	case e.Name == "Y", e.Name == "Z" && e.Modifiers.Contain(key.ModShift):
		return false, true
	case e.Name == "Z":
		return true, false
	}
	return false, false
}

// homeMode is the home screen: the veil covers the viewport, so it takes
// every viewport event and does nothing with it; the command bar is not
// shown.
type homeMode struct{}

func (homeMode) viewportEvent(*workspace, event.Event) bool       { return true }
func (homeMode) click(*workspace, pointer.Event, pointer.Buttons) {}
func (homeMode) key(*workspace, key.Event)                        {}
func (homeMode) undo(*workspace)                                  {}
func (homeMode) redo(*workspace)                                  {}
func (homeMode) compile(*workspace)                               {}
func (homeMode) update(layout.Context, *workspace)                {}
func (homeMode) present(layout.Context, *workspace)               {}
func (homeMode) sync(*render.Renderer, *workspace)                {}
func (homeMode) forget()                                          {}
