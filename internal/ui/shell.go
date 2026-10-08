package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// PanelWidth is the side panel's fixed width.
const PanelWidth = unit.Dp(300)

var panelBackground = color.NRGBA{R: 0x22, G: 0x24, B: 0x28, A: 0xFF}

// Shell arranges the window: the viewport fills the left, a fixed-width
// panel sits on the right, and a button floats over the viewport's top-left
// corner.
type Shell struct {
	Theme    *material.Theme
	Viewport Viewport
	// Pause is the floating button over the viewport.
	Pause widget.Clickable
}

// Layout lays the window out and returns the viewport rectangle in window
// pixels (origin top-left), which is where the renderer must draw. Nothing
// is painted under the viewport, so the 3D content drawn before Gio's frame
// shows through. lines fill the side panel; pauseLabel captions the button.
func (s *Shell) Layout(gtx layout.Context, pauseLabel string, lines []string) image.Rectangle {
	var vp image.Rectangle
	layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			// The viewport is the Flex's first child, so its origin is the
			// window's origin; it takes the whole slot, not the button's size.
			gtx.Constraints.Min = gtx.Constraints.Max
			dims := layout.Stack{}.Layout(gtx,
				layout.Expanded(s.Viewport.Layout),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(12)).Layout(gtx,
						material.Button(s.Theme, &s.Pause, pauseLabel).Layout)
				}),
			)
			vp = image.Rectangle{Max: s.Viewport.Size()}
			return dims
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.panel(gtx, lines)
		}),
	)
	return vp
}

func (s *Shell) panel(gtx layout.Context, lines []string) layout.Dimensions {
	w := gtx.Dp(PanelWidth)
	size := image.Point{X: w, Y: gtx.Constraints.Max.Y}
	paint.FillShape(gtx.Ops, panelBackground, clip.Rect{Max: size}.Op())
	gtx.Constraints = layout.Exact(size)
	layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(lines))
		for _, l := range lines {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(s.Theme, l)
				lbl.Color = color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
				return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, lbl.Layout)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	return layout.Dimensions{Size: size}
}
