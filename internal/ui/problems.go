package ui

import (
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/icon"
)

// ProblemList is the inspector's problem section: every problem of the
// document, then the floor warnings, one clickable row each. The window
// loop sets Rows whenever they change, and Clicked reports the row the
// user picked.
type ProblemList struct {
	// Rows are the problems as shown, in the document's order, then the
	// warnings.
	Rows  []ProblemRow
	picks []widget.Clickable
}

// ProblemRow is one problem: the zone's name and the message. A warning
// does not block compiling and has its own mark.
type ProblemRow struct {
	Zone, Message string
	Warning       bool
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

// problemList is the problem section of list l: the zone mode's Problems,
// or the population mode's Spawn.Problems.
func (s *Shell) problemList(l *ProblemList) []layout.FlexChild {
	if len(l.picks) < len(l.Rows) {
		l.picks = append(l.picks, make([]widget.Clickable, len(l.Rows)-len(l.picks))...)
	}
	errors := 0
	for _, r := range l.Rows {
		if !r.Warning {
			errors++
		}
	}
	children := []layout.FlexChild{
		layout.Rigid(s.section(false, icon.TriangleAlert, locale.Text(s.Language, "ui.problems.title"), problemNote(s.Language, errors, len(l.Rows)-errors))),
	}
	if errors == 0 {
		children = append(children, layout.Rigid(s.dimLabel(locale.Text(s.Language, "ui.problems.ready"))))
	}
	for i, r := range l.Rows {
		children = append(children, layout.Rigid(s.problemRow(r, &l.picks[i])))
	}
	return children
}

// problemNote counts the problems and the warnings for the section's
// header.
func problemNote(lang locale.Language, errors, warnings int) string {
	switch {
	case errors == 0 && warnings == 0:
		return locale.Text(lang, "ui.problems.zero")
	case warnings == 0:
		return locale.Plural(lang, "ui.problems.count", errors, nil)
	case errors == 0:
		return locale.Plural(lang, "ui.problems.warning_count", warnings, nil)
	}
	return locale.Format(lang, "ui.problems.mixed", map[string]string{
		"problems": locale.Plural(lang, "ui.problems.count", errors, nil),
		"warnings": locale.Plural(lang, "ui.problems.warning_count", warnings, nil),
	})
}

// problemRow is one problem, clickable: a red mark (a yellow one for a
// warning), the zone's name over the message.
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
									mark, ink := icon.TriangleAlert, errorText
									if r.Warning {
										mark, ink = icon.CircleAlert, warnText
									}
									return mark.Layout(gtx, 14, ink)
								})
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(s.text(r.Zone, bodySize, font.Medium, textColor, 1)),
									layout.Rigid(s.text(r.Message, smallSize, font.Normal, messageInk(r), 0)),
								)
							}),
						)
					})
				})
				bg := errorSoft
				switch {
				case r.Warning && pick.Hovered():
					bg = warnSoftHover
				case r.Warning:
					bg = warnSoft
				case pick.Hovered():
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

// messageInk is the colour of r's message.
func messageInk(r ProblemRow) color.NRGBA {
	if r.Warning {
		return warnText
	}
	return errorText
}

// problemCount is a zone's problem count as the zone list shows it.
func problemCount(lang locale.Language, n int) string {
	if n == 0 {
		return locale.Text(lang, "ui.zone.no_problems")
	}
	return locale.Plural(lang, "ui.zone.problem_count", n, nil)
}
