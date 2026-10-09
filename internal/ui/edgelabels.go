package ui

import (
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

// EdgeLabel is a length drawn on the viewport: Text centred on At, in
// viewport pixels. Alert draws it in red.
type EdgeLabel struct {
	At    f32.Point
	Text  string
	Alert bool
}

// edgeLabels draws EdgeLabels as small tags. They take no pointer input,
// so a click on one reaches the scene under it.
func (s *Shell) edgeLabels(gtx layout.Context) {
	for _, l := range s.EdgeLabels {
		s.edgeLabel(gtx, l, nil)
	}
}

// pinLabels draws Pins as EdgeLabel tags that take clicks.
func (s *Shell) pinLabels(gtx layout.Context) {
	for len(s.pinPicks) < len(s.Pins) {
		s.pinPicks = append(s.pinPicks, widget.Clickable{})
	}
	for i, l := range s.Pins {
		s.edgeLabel(gtx, l, &s.pinPicks[i])
	}
}

// PinClicked reports the index into the Pins of the last layout whose
// label was clicked since the last call.
func (s *Shell) PinClicked(gtx layout.Context) (int, bool) {
	picked, ok := 0, false
	for i := range s.pinPicks {
		if s.pinPicks[i].Clicked(gtx) && !ok {
			picked, ok = i, true
		}
	}
	return picked, ok
}

// edgeLabel draws l as a tag centred on l.At; with pick it takes clicks.
func (s *Shell) edgeLabel(gtx layout.Context, l EdgeLabel, pick *widget.Clickable) {
	ink, fill := textColor, cardSurface
	if l.Alert {
		ink, fill = errorText, errorSoft
	}
	call, size := measure(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx,
			s.text(l.Text, captionSize, font.Medium, ink, 1))
	})
	at := l.At.Round().Sub(size.Div(2))
	off := op.Offset(at).Push(gtx.Ops)
	tag := func(gtx layout.Context) layout.Dimensions {
		fillRRect(gtx, size, controlRadius, fill, hairline)
		call.Add(gtx.Ops)
		return layout.Dimensions{Size: size}
	}
	if pick != nil {
		gtx.Constraints = layout.Exact(size)
		pick.Layout(gtx, tag)
	} else {
		tag(gtx)
	}
	off.Pop()
}
