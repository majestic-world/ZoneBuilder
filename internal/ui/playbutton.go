package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/icon"
)

// playButton is the floating Jogar button at the bottom right of the scene
// area, left of the inspector (ix) and above the status pill (statusY).
func (s *Shell) playButton(gtx layout.Context, ix, statusY int) {
	m := gtx.Dp(floatMargin)
	call, size := measure(gtx, func(gtx layout.Context) layout.Dimensions {
		return card(gtx, &s.playSink, layout.UniformInset(unit.Dp(5)),
			s.button(&s.Play, primaryButton, icon.Gamepad2, locale.Text(s.Language, "spawn.play.button")))
	})
	place(gtx, image.Pt(ix-m-size.X, statusY-gtx.Dp(10)-size.Y), call)
}
