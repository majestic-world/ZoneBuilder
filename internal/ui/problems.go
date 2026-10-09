package ui

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"zonebuilder/internal/inflect"
	"zonebuilder/internal/ui/icon"
)

// ProblemList is the inspector's problem section: every problem of the
// document, one clickable row each. The window loop sets Rows whenever the
// document changes, and Clicked reports the row the user picked.
type ProblemList struct {
	// Rows are the problems as shown, in the document's order.
	Rows  []ProblemRow
	picks []widget.Clickable
}

// ProblemRow is one problem: the zone's name and the message.
type ProblemRow struct {
	Zone, Message string
}

// Clicked reports the index in Rows of a clicked problem since the last
// call, the first if several were. Call it before Layout.
func (l *ProblemList) Clicked(gtx layout.Context) (int, bool) {
	picked, ok := 0, false
	for i := range l.picks {
		if l.picks[i].Clicked(gtx) && !ok && i < len(l.Rows) {
			picked, ok = i, true
		}
	}
	return picked, ok
}

func (s *Shell) problemList() []layout.FlexChild {
	l := &s.Problems
	if len(l.picks) < len(l.Rows) {
		l.picks = append(l.picks, make([]widget.Clickable, len(l.Rows)-len(l.picks))...)
	}
	children := []layout.FlexChild{
		layout.Rigid(s.section(false, icon.TriangleAlert, "Problemas", problemNote(len(l.Rows)))),
	}
	if len(l.Rows) == 0 {
		children = append(children, layout.Rigid(s.dimLabel("Tudo certo para compilar.")))
	}
	for i, r := range l.Rows {
		children = append(children, layout.Rigid(s.problemRow(r, &l.picks[i])))
	}
	return children
}

func problemNote(n int) string {
	if n == 0 {
		return "nenhum"
	}
	return inflect.Count(n, "problema", "problemas")
}

// problemRow is one problem, clickable: a red mark, the zone's name over
// the message.
func (s *Shell) problemRow(r ProblemRow, pick *widget.Clickable) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return pick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				call, content := measure(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: unit.Dp(1), Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return icon.TriangleAlert.Layout(gtx, 14, errorText)
								})
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(s.text(r.Zone, bodySize, font.Medium, textColor, 1)),
									layout.Rigid(s.text(r.Message, smallSize, font.Normal, errorText, 0)),
								)
							}),
						)
					})
				})
				bg := errorSoft
				if pick.Hovered() {
					bg = errorSoftHover
				}
				fillRRect(gtx, content, controlRadius, bg, color.NRGBA{})
				call.Add(gtx.Ops)
				pointerCursor(gtx, content)
				return layout.Dimensions{Size: content}
			})
		})
	}
}

// problemCount is a zone's problem count as the zone list shows it.
func problemCount(n int) string {
	if n == 0 {
		return "sem problemas"
	}
	return inflect.Count(n, "problema", "problemas")
}
