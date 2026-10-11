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

// hiddenMeshesPanelRows is how many hidden meshes the map section lists;
// the window lists them all.
const hiddenMeshesPanelRows = 5

// HiddenMeshes is the list of the static meshes hidden one by one: in the
// map section, a row per mesh up to hiddenMeshesPanelRows, with the button
// that shows it again, Ver todos when there are more, and Mostrar todos;
// in Window, every row. Both read Rows and report the same indices.
type HiddenMeshes struct {
	// Rows name the hidden meshes, in list order.
	Rows []string
	// Window lists every hidden mesh; Ver todos opens it.
	Window FloatWindow

	show, windowShow       []widget.Clickable
	showAll, windowShowAll widget.Clickable
	viewAll                widget.Clickable
}

// Open shows the window, centred, at the size it was left at.
func (h *HiddenMeshes) Open() {
	if h.Window.Width == 0 {
		h.Window.Width, h.Window.Height = 380, 440
	}
	h.Window.Closed, h.Window.Collapsed = false, false
	h.Window.Center()
}

// ShowRequested returns the row whose Mostrar, in the map section or in
// the window, was clicked since the last call.
func (h *HiddenMeshes) ShowRequested(gtx layout.Context) (int, bool) {
	for _, show := range [...][]widget.Clickable{h.show, h.windowShow} {
		for i := range show {
			if show[i].Clicked(gtx) && i < len(h.Rows) {
				return i, true
			}
		}
	}
	return 0, false
}

// ShowAllRequested reports a click on Mostrar todos, in the map section or
// in the window, since the last call.
func (h *HiddenMeshes) ShowAllRequested(gtx layout.Context) bool {
	panel := h.showAll.Clicked(gtx)
	window := h.windowShowAll.Clicked(gtx)
	return panel || window
}

// Discard drops the window's clicks while the editor is locked, so none
// replays after it resumes.
func (h *HiddenMeshes) Discard(gtx layout.Context) {
	discardClicks(gtx, &h.windowShowAll)
	for i := range h.windowShow {
		discardClicks(gtx, &h.windowShow[i])
	}
}

// hiddenMeshes are the map section's rows of the hidden static meshes,
// under a label with their count, at most hiddenMeshesPanelRows of them
// and Ver todos for the rest; a hint on how to hide one while none is.
func (s *Shell) hiddenMeshes() []layout.FlexChild {
	h := &s.HiddenMeshes
	n := len(h.Rows)
	children := []layout.FlexChild{layout.Rigid(s.hiddenMeshesHeader)}
	if n == 0 {
		return append(children, layout.Rigid(s.dimLabel(locale.Text(s.Language, "ui.meshes.hidden_hint"))))
	}
	shown := min(n, hiddenMeshesPanelRows)
	if len(h.show) < shown {
		h.show = append(h.show, make([]widget.Clickable, shown-len(h.show))...)
	}
	children = append(children, s.hiddenMeshRows(h.Rows[:shown], h.show)...)
	if n > hiddenMeshesPanelRows {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if h.viewAll.Clicked(gtx) {
				h.Open()
			}
			return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, s.fullButton(&h.viewAll, ghostButton, nil, locale.Plural(s.Language, "ui.meshes.view_all", n, nil)))
		}))
	}
	return append(children, layout.Rigid(s.spaced(s.fullButton(&h.showAll, secondaryButton, icon.Eye, locale.Text(s.Language, "ui.meshes.show_all")))))
}

// hiddenMeshesHeader is the list's title with the count of hidden meshes.
func (s *Shell) hiddenMeshesHeader(gtx layout.Context) layout.Dimensions {
	n := len(s.HiddenMeshes.Rows)
	count := locale.Text(s.Language, "ui.meshes.none")
	if n > 0 {
		count = locale.Plural(s.Language, "ui.meshes.hidden_count", n, nil)
	}
	return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, s.text(locale.Text(s.Language, "ui.meshes.hidden_title"), smallSize, font.Medium, dimText, 1)),
			layout.Rigid(s.text(count, smallSize, font.Normal, dimText, 1)),
		)
	})
}

// hiddenMeshRows are a row per name, with its Mostrar from show.
func (s *Shell) hiddenMeshRows(names []string, show []widget.Clickable) []layout.FlexChild {
	children := make([]layout.FlexChild, 0, len(names))
	for i, name := range names {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, s.text(name, bodySize, font.Normal, textColor, 1))
					}),
					layout.Rigid(s.button(&show[i], ghostButton, icon.Eye, locale.Text(s.Language, "ui.meshes.show"))),
				)
			})
		}))
	}
	return children
}

// hiddenMeshesWindow is the floating window over every hidden mesh: the
// count on top, a row per mesh and Mostrar todos; the hint while none is.
func (s *Shell) hiddenMeshesWindow(gtx layout.Context) layout.Dimensions {
	h := &s.HiddenMeshes
	return h.Window.Layout(gtx, s, icon.EyeOff, locale.Text(s.Language, "ui.meshes.hidden_title"), func(gtx layout.Context) layout.Dimensions {
		n := len(h.Rows)
		children := []layout.FlexChild{layout.Rigid(s.hiddenMeshesHeader)}
		if n == 0 {
			children = append(children, layout.Rigid(s.dimLabel(locale.Text(s.Language, "ui.meshes.hidden_hint"))))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}
		if len(h.windowShow) < n {
			h.windowShow = append(h.windowShow, make([]widget.Clickable, n-len(h.windowShow))...)
		}
		children = append(children, s.hiddenMeshRows(h.Rows, h.windowShow)...)
		children = append(children, layout.Rigid(s.spaced(s.fullButton(&h.windowShowAll, secondaryButton, icon.Eye, locale.Text(s.Language, "ui.meshes.show_all")))))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}
