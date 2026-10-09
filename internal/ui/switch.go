package ui

import (
	"gioui.org/layout"
	"gioui.org/widget"
)

// Switch is a command bar button that turns a view option on and off.
type Switch struct {
	On     bool
	button widget.Clickable
}

// Toggled flips On on a click since the last call and reports it.
func (s *Switch) Toggled(gtx layout.Context) bool {
	clicked := false
	for s.button.Clicked(gtx) {
		s.On = !s.On
		clicked = true
	}
	return clicked
}
