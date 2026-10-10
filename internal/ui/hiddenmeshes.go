package ui

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/icon"
)

// MeshMenu is the viewport's context menu over a static mesh: Ocultar,
// for the mesh named Name.
type MeshMenu struct {
	Menu ContextMenu
	Name string
	hide widget.Clickable
}

// HideRequested reports a click on Ocultar since the last call, closing
// the menu after one.
func (m *MeshMenu) HideRequested(gtx layout.Context) bool {
	return m.Menu.Chosen(gtx, &m.hide)
}

// meshMenu lays the mesh menu out while it is open.
func (s *Shell) meshMenu(gtx layout.Context) {
	m := &s.MeshMenu
	s.contextMenu(gtx, &m.Menu, MenuItem{Click: &m.hide, Icon: icon.EyeOff, Text: locale.Format(s.Language, "ui.meshes.hide", map[string]string{"name": m.Name})})
}

// HiddenMeshes is the map section's list of the static meshes hidden one
// by one: a row per mesh, with the button that shows it again, and
// Mostrar todos.
type HiddenMeshes struct {
	// Rows name the hidden meshes, in list order.
	Rows    []string
	show    []widget.Clickable
	showAll widget.Clickable
}

// ShowRequested returns the row whose Mostrar was clicked since the last
// call.
func (h *HiddenMeshes) ShowRequested(gtx layout.Context) (int, bool) {
	for i := range h.show {
		if h.show[i].Clicked(gtx) && i < len(h.Rows) {
			return i, true
		}
	}
	return 0, false
}

// ShowAllRequested reports a click on Mostrar todos since the last call.
func (h *HiddenMeshes) ShowAllRequested(gtx layout.Context) bool {
	return h.showAll.Clicked(gtx)
}

// hiddenMeshes are the map section's rows of the hidden static meshes,
// under a label with their count; a hint on how to hide one while none
// is.
func (s *Shell) hiddenMeshes() []layout.FlexChild {
	h := &s.HiddenMeshes
	n := len(h.Rows)
	if len(h.show) < n {
		h.show = append(h.show, make([]widget.Clickable, n-len(h.show))...)
	}
	count := locale.Text(s.Language, "ui.meshes.none")
	if n > 0 {
		count = locale.Plural(s.Language, "ui.meshes.hidden_count", n, nil)
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, s.text(locale.Text(s.Language, "ui.meshes.hidden_title"), smallSize, font.Medium, dimText, 1)),
					layout.Rigid(s.text(count, smallSize, font.Normal, dimText, 1)),
				)
			})
		}),
	}
	if n == 0 {
		return append(children, layout.Rigid(s.dimLabel(locale.Text(s.Language, "ui.meshes.hidden_hint"))))
	}
	for i, name := range h.Rows {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, s.text(name, bodySize, font.Normal, textColor, 1))
					}),
					layout.Rigid(s.button(&h.show[i], ghostButton, icon.Eye, locale.Text(s.Language, "ui.meshes.show"))),
				)
			})
		}))
	}
	return append(children, layout.Rigid(s.spaced(s.fullButton(&h.showAll, secondaryButton, icon.Eye, locale.Text(s.Language, "ui.meshes.show_all")))))
}
