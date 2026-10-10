package ui

import (
	"gioui.org/layout"
	"gioui.org/widget"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/icon"
)

// CloseMenus closes popup menus and the spawn compilation prompt:
// Esc in the viewport closes them before it does anything else.
func (s *Shell) CloseMenus() bool {
	project := s.Project.popup.Close()
	water := s.WaterMenu.Menu.Close()
	prompt := s.Mode == ModePopulate && !s.NPCIDs.Window.Closed
	s.NPCIDs.Window.Closed = true
	return project || water || prompt
}

// WaterMenu is the viewport's context menu over the water: Compilar zona
// de água.
type WaterMenu struct {
	Menu    ContextMenu
	Compile widget.Clickable
	// Disabled greys Compile out, while a polygon is open.
	Disabled bool
}

// CompileRequested reports a click on Compilar zona de água since the last
// call, closing the menu after one.
func (w *WaterMenu) CompileRequested(gtx layout.Context) bool {
	return w.Menu.Chosen(gtx, &w.Compile) && !w.Disabled
}

// waterMenu lays the water menu out while it is open.
func (s *Shell) waterMenu(gtx layout.Context) {
	w := &s.WaterMenu
	s.contextMenu(gtx, &w.Menu, MenuItem{Click: &w.Compile, Icon: icon.Droplets, Text: locale.Text(s.Language, "ui.water.compile"), Disabled: w.Disabled})
}
