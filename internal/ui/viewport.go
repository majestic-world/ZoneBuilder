// Package ui holds the Gio side of the Zone Builder window: the viewport
// widget that reserves the area the 3D renderer draws into and receives its
// pointer and keyboard input, and the panels drawn on top.
package ui

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// viewportKeys are the keys the viewport listens to while it has focus: the
// fly keys, Enter, which closes the polygon being drawn, Delete, which
// removes the selected vertex, and Escape, which puts the armed tool down.
var viewportKeys = []key.Name{"W", "A", "S", "D", "Q", "E", key.NameShift, key.NameReturn, key.NameEnter, key.NameDeleteForward, key.NameEscape}

// shortcutKeys reach the viewport with the shortcut modifier (Ctrl) held
// unless the focused widget takes them (a text field's own Ctrl+Z): Z
// undoes, Y and Shift+Z redo.
var shortcutKeys = []key.Name{"Z", "Y"}

// Viewport is the area of the window the 3D scene is drawn into. It paints
// nothing: the renderer fills the area before Gio draws, and any Gio content
// laid out after it (as a sibling, not inside its clip) lands on top and
// takes the pointer events over its own bounds. A press inside it takes the
// keyboard focus, so the fly keys reach it.
type Viewport struct {
	size image.Point
}

// Update returns the next input event for the viewport: a pointer.Event
// with Position relative to the viewport's top-left corner, a key.Event of
// a fly key, Enter, Delete or a Ctrl shortcut, or a key.FocusEvent. Call it
// until it returns false, before Layout, the way Gio widgets are driven.
func (v *Viewport) Update(gtx layout.Context) (event.Event, bool) {
	filters := []event.Filter{
		pointer.Filter{
			Target:  v,
			Kinds:   pointer.Press | pointer.Release | pointer.Drag | pointer.Move | pointer.Scroll | pointer.Enter | pointer.Leave,
			ScrollX: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20},
			ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20},
		},
		key.FocusFilter{Target: v},
	}
	for _, n := range viewportKeys {
		filters = append(filters, key.Filter{Focus: v, Name: n, Optional: key.ModShift})
	}
	for _, n := range shortcutKeys {
		filters = append(filters, key.Filter{Name: n, Required: key.ModShortcut, Optional: key.ModShift})
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return nil, false
		}
		switch e := ev.(type) {
		case pointer.Event:
			if e.Kind == pointer.Press {
				gtx.Execute(key.FocusCmd{Tag: v})
			}
			return e, true
		case key.Event, key.FocusEvent:
			return e, true
		}
	}
}

// Layout claims all the space it is given and registers the input area.
func (v *Viewport) Layout(gtx layout.Context) layout.Dimensions {
	v.size = gtx.Constraints.Max
	defer clip.Rect{Max: v.size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, v)
	return layout.Dimensions{Size: v.size}
}

// Size is the viewport size in pixels from the last Layout.
func (v *Viewport) Size() image.Point { return v.size }
