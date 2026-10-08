// Package ui holds the Gio side of the Zone Builder window: the viewport
// widget that reserves the area the 3D renderer draws into and receives its
// pointer input, and the panels drawn on top.
package ui

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// Viewport is the area of the window the 3D scene is drawn into. It paints
// nothing: the renderer fills the area before Gio draws, and any Gio content
// laid out after it (as a sibling, not inside its clip) lands on top and
// takes the pointer events over its own bounds.
type Viewport struct {
	size image.Point
}

// Update returns the next pointer event that hit the viewport, with Position
// relative to the viewport's top-left corner. Call it until it returns
// false, before Layout, the way Gio widgets are driven.
func (v *Viewport) Update(gtx layout.Context) (pointer.Event, bool) {
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target:  v,
			Kinds:   pointer.Press | pointer.Release | pointer.Drag | pointer.Move | pointer.Scroll,
			ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20},
		})
		if !ok {
			return pointer.Event{}, false
		}
		if pe, ok := ev.(pointer.Event); ok {
			return pe, true
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
