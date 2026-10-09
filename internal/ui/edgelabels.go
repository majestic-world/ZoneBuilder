package ui

import (
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// EdgeLabel is a length drawn on the viewport: Text centred on At, in
// viewport pixels.
type EdgeLabel struct {
	At   f32.Point
	Text string
}

// edgeLabels draws EdgeLabels as small tags. They take no pointer input,
// so a click on one reaches the scene under it.
func (s *Shell) edgeLabels(gtx layout.Context) {
	for _, l := range s.EdgeLabels {
		call, size := measure(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx,
				s.text(l.Text, captionSize, font.Medium, textColor, 1))
		})
		at := l.At.Round().Sub(size.Div(2))
		off := op.Offset(at).Push(gtx.Ops)
		fillRRect(gtx, size, controlRadius, cardSurface, hairline)
		call.Add(gtx.Ops)
		off.Pop()
	}
}
