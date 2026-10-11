package ui

import (
	"image"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// BoxHandles are the current box's handles, in viewport pixels: a face
// handle per face, in the order +X −X +Y −Y +Z −Z along the box's own axes
// (dragging one moves that face), and the turn handle on its top corner
// (dragging it turns the box around its center). The viewport receives
// the presses; BoxHandles only draws.
type BoxHandles struct {
	Visible [6]bool
	At      [6]f32.Point
	Turn    bool
	TurnAt  f32.Point
	// Active is the face being dragged, 6 for the turn handle, -1 for
	// none.
	Active int
}

// boxTurnHandle is BoxHandles.Active while the turn handle is dragged.
const boxTurnHandle = 6

// Layout paints each visible face handle as a square in its axis colour
// (red X, green Y, blue Z, as the move gizmo) on a white border, and the
// turn handle as a violet dot ringed in white; the dragged one is yellow.
func (h BoxHandles) Layout(gtx layout.Context) layout.Dimensions {
	outer, inner := gtx.Dp(14), gtx.Dp(10)
	square := func(at f32.Point, side int) clip.Rect {
		min := at.Round().Sub(image.Pt(side/2, side/2))
		return clip.Rect{Min: min, Max: min.Add(image.Pt(side, side))}
	}
	for i, ok := range h.Visible {
		if !ok {
			continue
		}
		c := axisColors[i/2]
		if h.Active == i {
			c = arrowActive
		}
		paint.FillShape(gtx.Ops, white, square(h.At[i], outer).Op())
		paint.FillShape(gtx.Ops, c, square(h.At[i], inner).Op())
	}
	if h.Turn {
		dot := func(r int) clip.Ellipse {
			min := h.TurnAt.Round().Sub(image.Pt(r, r))
			return clip.Ellipse{Min: min, Max: min.Add(image.Pt(2*r, 2*r))}
		}
		c := accent
		if h.Active == boxTurnHandle {
			c = arrowActive
		}
		paint.FillShape(gtx.Ops, white, dot(gtx.Dp(9)).Op(gtx.Ops))
		paint.FillShape(gtx.Ops, c, dot(gtx.Dp(6)).Op(gtx.Ops))
	}
	return layout.Dimensions{}
}
