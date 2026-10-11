package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/icon"
)

// WaterRow is one water volume as the water window lists it.
type WaterRow struct {
	// Tile groups the rows: a heading with it starts each run of rows of
	// one tile.
	Tile string
	// Label names the volume ("22_22 · WaterVolume0"); Detail is its top
	// in the client and on the server and whether it is exact, or why it
	// is unsupported.
	Label, Detail string
	// Checked shows the volume in the selection. A Disabled row, an
	// unsupported volume, is never toggled.
	Checked, Disabled bool
}

// WaterWindow is the zone mode's window over the loaded water volumes:
// a row per volume, grouped by tile, that toggles it in the water
// selection, with Selecionar todos and Limpar. The inspector's Volumes de
// água button opens it. Like the zone list it only collects input:
// Update reports what the user asked for, by row index, and the window
// loop applies it to the water selection.
type WaterWindow struct {
	Window FloatWindow
	// Rows are the volumes, set by the window loop, already grouped by
	// tile and in list order.
	Rows []WaterRow

	open, selectAll, clear widget.Clickable
	toggles                []widget.Clickable
}

// Water window requests, returned by WaterWindow.Update.
type (
	// ToggleWater: the row's volume goes into the water selection, or
	// out of it.
	ToggleWater struct{ Row int }
	// SelectAllWater: Selecionar todos, every supported volume.
	SelectAllWater struct{}
	// ClearWater: Limpar.
	ClearWater struct{}
)

// Open shows the window, centred when it was closed.
func (w *WaterWindow) Open() {
	if w.Window.Width == 0 {
		w.Window.Width, w.Window.Height = 440, 520
	}
	if w.Window.Closed {
		w.Window.Center()
	}
	w.Window.Closed, w.Window.Collapsed = false, false
}

// Update opens the window on a click on the inspector's button and
// returns the requests since the last call, in the order: row toggles,
// Selecionar todos, Limpar. A disabled row's clicks are dropped. Call it
// before Layout.
func (w *WaterWindow) Update(gtx layout.Context) []any {
	if w.open.Clicked(gtx) {
		w.Open()
	}
	var reqs []any
	for i := range w.toggles {
		for w.toggles[i].Clicked(gtx) {
			if i < len(w.Rows) && !w.Rows[i].Disabled {
				reqs = append(reqs, ToggleWater{Row: i})
			}
		}
	}
	if w.selectAll.Clicked(gtx) {
		reqs = append(reqs, SelectAllWater{})
	}
	if w.clear.Clicked(gtx) {
		reqs = append(reqs, ClearWater{})
	}
	return reqs
}

// Discard consumes the clicks without opening the window or replaying
// them when the editor resumes.
func (w *WaterWindow) Discard(gtx layout.Context) {
	discardClicks(gtx, &w.open, &w.selectAll, &w.clear)
	for i := range w.toggles {
		discardClicks(gtx, &w.toggles[i])
	}
}

// waterButton is the zone inspector's button that opens the water
// window, with the count of loaded volumes.
func (s *Shell) waterButton(gtx layout.Context) layout.Dimensions {
	w := &s.Water
	return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, s.fullButton(&w.open, secondaryButton, icon.Droplets, locale.Plural(s.Language, "ui.water.open", len(w.Rows), nil)))
}

// waterWindow lays the water window out over the viewport, while open.
func (s *Shell) waterWindow(gtx layout.Context) layout.Dimensions {
	w := &s.Water
	if n := len(w.Rows); len(w.toggles) < n {
		w.toggles = append(w.toggles, make([]widget.Clickable, n-len(w.toggles))...)
	}
	return w.Window.Layout(gtx, s, icon.Droplets, locale.Text(s.Language, "ui.water.title"), func(gtx layout.Context) layout.Dimensions {
		checked := 0
		for _, r := range w.Rows {
			if r.Checked {
				checked++
			}
		}
		summary := locale.Format(s.Language, "ui.water.summary", map[string]string{
			"volumes":  locale.Plural(s.Language, "ui.water.volumes", len(w.Rows), nil),
			"selected": locale.Plural(s.Language, "ui.water.selected", checked, nil),
		})
		children := []layout.FlexChild{
			layout.Rigid(s.dimLabel(summary)),
			layout.Rigid(s.spaced(buttonRow(
				s.button(&w.selectAll, secondaryButton, nil, locale.Text(s.Language, "ui.water.select_all")),
				s.button(&w.clear, secondaryButton, nil, locale.Text(s.Language, "ui.water.clear")),
			))),
		}
		if len(w.Rows) == 0 {
			children = append(children, layout.Rigid(s.dimLabel(locale.Text(s.Language, "ui.water.empty"))))
		}
		for i, r := range w.Rows {
			if i == 0 || r.Tile != w.Rows[i-1].Tile {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(6), Bottom: unit.Dp(4)}.Layout(gtx, s.text(r.Tile, smallSize, font.Medium, dimText, 1))
				}))
			}
			children = append(children, layout.Rigid(s.waterRow(r, &w.toggles[i])))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// waterRow is one volume's row: the check box, then the label over the
// detail. The whole row toggles the volume, unless it is disabled.
func (s *Shell) waterRow(r WaterRow, toggle *widget.Clickable) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		label, detail := textColor, dimText
		if r.Disabled {
			gtx = gtx.Disabled()
			label, detail = faintText, faintText
		}
		return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return toggle.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				call, content := measure(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if r.Disabled {
									size := image.Pt(gtx.Dp(16), gtx.Dp(16))
									fillRRect(gtx, size, 3, color.NRGBA{}, hairline)
									return layout.Dimensions{Size: size}
								}
								return checkMark(gtx, r.Checked, toggle.Hovered())
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
									layout.Rigid(s.text(r.Label, bodySize, font.Medium, label, 1)),
									layout.Rigid(s.text(r.Detail, captionSize, font.Normal, detail, 0)),
								)
							}),
						)
					})
				})
				switch {
				case r.Checked:
					fillRRect(gtx, content, controlRadius, accentSoft, color.NRGBA{})
				case toggle.Hovered():
					fillRRect(gtx, content, controlRadius, controlFill, color.NRGBA{})
				}
				call.Add(gtx.Ops)
				if !r.Disabled {
					pointerCursor(gtx, content)
				}
				return layout.Dimensions{Size: content}
			})
		})
	}
}
