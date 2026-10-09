package ui

import (
	"gioui.org/layout"
	"gioui.org/widget"
)

// MeshToggle is the command bar's switch that hides the static mesh
// actors, leaving only the map's fixed geometry (terrain and BSP), as W
// does in UnrealEd.
type MeshToggle struct {
	// Hidden is set while the meshes are hidden.
	Hidden bool
	button widget.Clickable
}

// Toggled flips Hidden on a click since the last call and reports it.
func (m *MeshToggle) Toggled(gtx layout.Context) bool {
	clicked := false
	for m.button.Clicked(gtx) {
		m.Hidden = !m.Hidden
		clicked = true
	}
	return clicked
}
