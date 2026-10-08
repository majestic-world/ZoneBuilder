package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"zonebuilder/internal/inflect"
)

// ProblemList is the side panel's problem panel: every problem of the
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
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(s.label(problemTitle(len(l.Rows)))),
	}
	for i, r := range l.Rows {
		children = append(children, layout.Rigid(s.problemRow(r, &l.picks[i])))
	}
	return children
}

func problemTitle(n int) string {
	if n == 0 {
		return "Problemas: nenhum"
	}
	return "Problemas: " + inflect.Count(n, "problema", "problemas")
}

// problemRow is one problem, clickable: the zone's name over the message.
func (s *Shell) problemRow(r ProblemRow, pick *widget.Clickable) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return material.Clickable(gtx, pick, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body2(s.Theme, r.Zone)
							lbl.Color = panelText
							lbl.MaxLines = 1
							return lbl.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							lbl := material.Caption(s.Theme, r.Message)
							lbl.Color = errorText
							return lbl.Layout(gtx)
						}),
					)
				})
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
