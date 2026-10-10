package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/icon"
)

// viewportButtons is the card of buttons floating over the scene's bottom
// right corner, left of the inspector at x ix and above the status pill
// row starting at y statusY: the populate mode's Prévia switch.
func (s *Shell) viewportButtons(gtx layout.Context, ix, statusY int) {
	if s.Mode != ModePopulate {
		return
	}
	call, size := measure(gtx, func(gtx layout.Context) layout.Dimensions {
		return card(gtx, &s.viewportSink, layout.UniformInset(unit.Dp(5)), func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(s.toggleButton(&s.Preview.button, icon.Swords, locale.Text(s.Language, "spawn.preview.button"), s.Preview.On)),
			)
		})
	})
	m := gtx.Dp(floatMargin)
	place(gtx, image.Pt(ix-m-size.X, statusY-gtx.Dp(10)-size.Y), call)
}
