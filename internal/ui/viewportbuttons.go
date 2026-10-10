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
// row starting at y statusY: Jogar in both editing modes (Sair while
// Playing), then the populate mode's Prévia switch.
func (s *Shell) viewportButtons(gtx layout.Context, ix, statusY int) {
	call, size := measure(gtx, func(gtx layout.Context) layout.Dimensions {
		return card(gtx, &s.viewportSink, layout.UniformInset(unit.Dp(5)), func(gtx layout.Context) layout.Dimensions {
			play := s.button(&s.Play, primaryButton, icon.Gamepad2, locale.Text(s.Language, "spawn.play.button"))
			if s.Playing {
				play = s.button(&s.Play, primaryButton, icon.X, locale.Text(s.Language, "spawn.play.exit_button"))
			}
			children := []layout.FlexChild{layout.Rigid(play)}
			if s.Mode == ModePopulate {
				children = append(children,
					layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
					layout.Rigid(s.toggleButton(&s.Preview.button, icon.Swords, locale.Text(s.Language, "spawn.preview.button"), s.Preview.On)),
				)
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
	m := gtx.Dp(floatMargin)
	place(gtx, image.Pt(ix-m-size.X, statusY-gtx.Dp(10)-size.Y), call)
}
