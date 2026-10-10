package main

import (
	"image"
	"log"

	"gioui.org/io/pointer"
	"gioui.org/layout"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/scene"
)

// meshHiding is the static meshes the user hid one by one, shared by the
// modes: out of the view, the picks and the floor, but still in the game
// mode's collision. The set lives for the session, keyed by tile and
// export, so a tile loaded again keeps its meshes hidden.
type meshHiding struct {
	hidden *scene.HiddenActors
	// target is the mesh the open context menu hides, label its name in
	// the list.
	target scene.ActorKey
	label  string
	// listed are the keys of the map section's rows, in row order.
	listed []scene.ActorKey
}

// rightClick opens the mesh menu when e lands on a static mesh, unless
// another viewport menu opened for the same click.
func (m *meshHiding) rightClick(ws *workspace, e pointer.Event) {
	w := ws.world()
	if w == nil || ws.shell.WaterMenu.Menu.IsOpen() {
		return
	}
	h, ok := pickAt(w, &ws.cam, e.Position, ws.viewport())
	if !ok || h.Surface != scene.SurfaceMesh {
		return
	}
	a, ok := w.Actor(h.Actor)
	if !ok {
		return
	}
	m.target, m.label = h.Actor, a.Name+" · "+h.Actor.Tile.Name()
	ws.shell.MeshMenu.Name = a.Name
	ws.shell.MeshMenu.Menu.Open(image.Pt(round(e.Position.X), round(e.Position.Y)))
}

// update hides the menu's mesh, shows the listed ones the user asked
// for, and refills the list.
func (m *meshHiding) update(gtx layout.Context, ws *workspace) {
	shell := ws.shell
	if shell.MeshMenu.HideRequested(gtx) && !m.hidden.Has(m.target) {
		name := m.label
		m.hidden = m.hidden.With(m.target, name)
		log.Printf("meshes: %s oculto (export %d)", name, m.target.Export)
		ws.status = actionArgs(locale.Message{Key: "actions.mesh.hidden"}, map[string]string{"name": name})
	}
	if i, ok := shell.HiddenMeshes.ShowRequested(gtx); ok && i < len(m.listed) {
		k := m.listed[i]
		name := m.hidden.Name(k)
		m.hidden = m.hidden.Without(k)
		log.Printf("meshes: %s visível", name)
		ws.status = actionArgs(locale.Message{Key: "actions.mesh.shown"}, map[string]string{"name": name})
	}
	if shell.HiddenMeshes.ShowAllRequested(gtx) && m.hidden.Len() > 0 {
		n := m.hidden.Len()
		m.hidden = nil
		log.Printf("meshes: %d visíveis de novo", n)
		ws.status = action(locale.Message{Key: "actions.mesh.all_shown", Count: n, Plural: true})
	}
	m.listed = m.hidden.Keys()
	rows := shell.HiddenMeshes.Rows[:0]
	for _, k := range m.listed {
		rows = append(rows, m.hidden.Name(k))
	}
	shell.HiddenMeshes.Rows = rows
}
