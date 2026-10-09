package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// MeshToggle is the button floating in the viewport's top-right corner that
// hides the static mesh actors, leaving only the map's fixed geometry
// (terrain and BSP), as W does in UnrealEd.
type MeshToggle struct {
	// Hidden is set while the meshes are hidden.
	Hidden bool
	button widget.Clickable
}

var toggleOn = color.NRGBA{R: 0x0D, G: 0x94, B: 0x88, A: 0xF2}

// Toggled flips Hidden on a click since the last call and reports it.
func (m *MeshToggle) Toggled(gtx layout.Context) bool {
	clicked := false
	for m.button.Clicked(gtx) {
		m.Hidden = !m.Hidden
		clicked = true
	}
	return clicked
}

// meshToggle lays the button out in the top-right corner of the area of
// gtx.Constraints.Max.
func (s *Shell) meshToggle(gtx layout.Context) layout.Dimensions {
	m := &s.Meshes
	text, bg := "Ocultar static meshes", windowTitleBar
	if m.Hidden {
		text, bg = "Mostrar static meshes", toggleOn
	}
	b := material.Button(s.Theme, &m.button, text)
	b.Background = bg
	b.Color = panelText
	b.TextSize = unit.Sp(13)
	b.Inset = layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(12), Right: unit.Dp(12)}

	macro := op.Record(gtx.Ops)
	cgtx := gtx
	cgtx.Constraints.Min = image.Point{}
	dims := b.Layout(cgtx)
	call := macro.Stop()

	margin := gtx.Dp(windowMargin)
	at := image.Pt(gtx.Constraints.Max.X-dims.Size.X-margin, margin)
	defer op.Offset(at).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	return layout.Dimensions{}
}
