package ui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/icon"
	"zonebuilder/internal/zone"
)

// ZoneRow is one zone as the zone list shows it.
type ZoneRow struct {
	ID   zone.ZoneID
	Name string
	Type zone.Type
	// Problems is how many problems the zone has.
	Problems int
	Hidden   bool
	Color    color.NRGBA
	// Note is extra state shown after the type ("desenhando").
	Note string
	// Compile reports the zone is in the compile selection.
	Compile bool
}

// ZoneList is the inspector's zone list: every zone with its name, type
// and problem count, narrowed by a name search and a type filter, with a
// show/hide toggle per zone and per type, and the rename, delete,
// duplicate and colour actions on the selected zone; plus the field that
// sends the camera to a typed x y z. Like ZonePanel it only collects
// input: Update reports what the user asked for and the window loop turns
// it into zone.Document commands and camera moves.
type ZoneList struct {
	// Rows are every zone of the document, set by the window loop each
	// frame; Selected is the selected zone's ID (0: none).
	Rows     []ZoneRow
	Selected zone.ZoneID
	// LeftOut is how many of the selected zone's shapes the viewport
	// leaves out of its footprint on the ground, set by the window loop.
	LeftOut int

	Search widget.Editor
	// TypeFilter is the shown type's index in zone.Types, -1 for all.
	TypeFilter             int
	PrevFilter, NextFilter widget.Clickable
	ToggleType             widget.Clickable

	NewName                          widget.Editor
	Rename, Delete, Duplicate, Color widget.Clickable
	// CompileAll and CompileNone put every shown zone in, or out of, the
	// compile selection.
	CompileAll, CompileNone widget.Clickable

	// GoTo holds the x y z the Go button (or Enter) flies the camera to.
	GoTo widget.Editor
	Go   widget.Clickable

	rows map[zone.ZoneID]*rowWidgets
	// named is the selection NewName was last filled for.
	named zone.ZoneID
}

type rowWidgets struct {
	pick, toggle widget.Clickable
	compile      widget.Bool
}

// Zone list requests, returned by ZoneList.Update.
type (
	// SelectZone: a row was clicked.
	SelectZone struct{ Zone zone.ZoneID }
	// HideZones: a zone's, or a type's, show/hide toggle was clicked.
	HideZones struct {
		Zones  []zone.ZoneID
		Hidden bool
	}
	// RenameZone: Rename (or Enter in NewName) with the selected zone.
	RenameZone struct {
		Zone zone.ZoneID
		Name string
	}
	// DeleteZone, DuplicateZone, CycleZoneColor: the action buttons on the
	// selected zone.
	DeleteZone     struct{ Zone zone.ZoneID }
	DuplicateZone  struct{ Zone zone.ZoneID }
	CycleZoneColor struct{ Zone zone.ZoneID }
	// GoTo: Go (or Enter in the GoTo field) with the field's text.
	GoTo struct{ Text string }
	// SelectForCompile: a zone's compile checkbox, or "todas"/"nenhuma"
	// on the shown zones, changed the compile selection.
	SelectForCompile struct {
		Zones   []zone.ZoneID
		Compile bool
	}
)

func (l *ZoneList) init() {
	l.Search.SingleLine = true
	l.NewName.SingleLine = true
	l.NewName.Submit = true
	l.GoTo.SingleLine = true
	l.GoTo.Submit = true
	l.Reset()
}

// Reset clears the search, the type filter and the per-zone state, for a
// document whose zone IDs mean other zones (an opened project). The go-to
// field keeps its text.
func (l *ZoneList) Reset() {
	l.Search.SetText("")
	l.TypeFilter = -1
	l.rows = map[zone.ZoneID]*rowWidgets{}
	l.named = 0
	l.NewName.SetText("")
}

// Update returns the requests since the last call, in the order: row
// clicks, toggles, actions, go-to. Call it before Layout.
func (l *ZoneList) Update(gtx layout.Context) []any {
	var reqs []any
	n := len(zone.Types) + 1 // the types and "all"
	for l.PrevFilter.Clicked(gtx) {
		l.TypeFilter = (l.TypeFilter+1+n-1)%n - 1
	}
	for l.NextFilter.Clicked(gtx) {
		l.TypeFilter = (l.TypeFilter+1+1)%n - 1
	}
	for _, r := range l.Rows {
		w := l.widgets(r.ID)
		if w.pick.Clicked(gtx) {
			reqs = append(reqs, SelectZone{Zone: r.ID})
		}
		if w.toggle.Clicked(gtx) {
			reqs = append(reqs, HideZones{Zones: []zone.ZoneID{r.ID}, Hidden: !r.Hidden})
		}
		w.compile.Value = r.Compile
		if w.compile.Update(gtx) {
			reqs = append(reqs, SelectForCompile{Zones: []zone.ZoneID{r.ID}, Compile: w.compile.Value})
		}
	}
	if l.ToggleType.Clicked(gtx) && l.TypeFilter >= 0 {
		if ids, hide := l.typeToggle(); len(ids) > 0 {
			reqs = append(reqs, HideZones{Zones: ids, Hidden: hide})
		}
	}
	for _, all := range []bool{true, false} {
		b := &l.CompileNone
		if all {
			b = &l.CompileAll
		}
		if b.Clicked(gtx) {
			var ids []zone.ZoneID
			for _, r := range l.shown() {
				ids = append(ids, r.ID)
			}
			if len(ids) > 0 {
				reqs = append(reqs, SelectForCompile{Zones: ids, Compile: all})
			}
		}
	}
	if submitted(gtx, &l.NewName) || l.Rename.Clicked(gtx) {
		reqs = append(reqs, RenameZone{Zone: l.Selected, Name: l.NewName.Text()})
	}
	if l.Delete.Clicked(gtx) {
		reqs = append(reqs, DeleteZone{Zone: l.Selected})
	}
	if l.Duplicate.Clicked(gtx) {
		reqs = append(reqs, DuplicateZone{Zone: l.Selected})
	}
	if l.Color.Clicked(gtx) {
		reqs = append(reqs, CycleZoneColor{Zone: l.Selected})
	}
	if submitted(gtx, &l.GoTo) || l.Go.Clicked(gtx) {
		reqs = append(reqs, GoTo{Text: l.GoTo.Text()})
	}
	return reqs
}

// submitted drains e's events and reports an Enter among them.
func submitted(gtx layout.Context, e *widget.Editor) bool {
	sub := false
	for {
		ev, ok := e.Update(gtx)
		if !ok {
			return sub
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			sub = true
		}
	}
}

func (l *ZoneList) widgets(id zone.ZoneID) *rowWidgets {
	w, ok := l.rows[id]
	if !ok {
		w = new(rowWidgets)
		l.rows[id] = w
	}
	return w
}

// filterType is the type the list is narrowed to, "" for all.
func (l *ZoneList) filterType() zone.Type {
	if l.TypeFilter < 0 {
		return ""
	}
	return zone.Types[l.TypeFilter]
}

// typeToggle is every zone of the filtered type and whether the toggle
// hides them: it does while any of them is shown.
func (l *ZoneList) typeToggle() (ids []zone.ZoneID, hide bool) {
	t := l.filterType()
	for _, r := range l.Rows {
		if r.Type == t {
			ids = append(ids, r.ID)
			hide = hide || !r.Hidden
		}
	}
	return ids, hide
}

// shown is the rows that pass the search (a case-insensitive part of the
// name) and the type filter.
func (l *ZoneList) shown() []ZoneRow {
	q := strings.ToLower(strings.TrimSpace(l.Search.Text()))
	t := l.filterType()
	var rows []ZoneRow
	for _, r := range l.Rows {
		if (t == "" || r.Type == t) && strings.Contains(strings.ToLower(r.Name), q) {
			rows = append(rows, r)
		}
	}
	return rows
}

// sync forgets the widgets of deleted zones and fills NewName when the
// selection changes; it returns the selected row, nil for none.
func (l *ZoneList) sync() *ZoneRow {
	for id := range l.rows {
		if !l.has(id) {
			delete(l.rows, id)
		}
	}
	var selected *ZoneRow
	for i := range l.Rows {
		if l.Rows[i].ID == l.Selected {
			selected = &l.Rows[i]
		}
	}
	if l.Selected != l.named {
		l.named = l.Selected
		if selected != nil {
			l.NewName.SetText(selected.Name)
		} else {
			l.NewName.SetText("")
		}
	}
	return selected
}

// zoneList is the inspector's zone section: the search, the type filter
// and its show/hide switch, the compile selection shortcuts and a row per
// shown zone.
func (s *Shell) zoneList() []layout.FlexChild {
	l := &s.Zones
	filter := locale.Text(s.Language, "ui.zone.all_types")
	if t := l.filterType(); t != "" {
		filter = string(t)
	}
	shown := l.shown()
	children := []layout.FlexChild{
		layout.Rigid(s.section(false, icon.List, locale.Text(s.Language, "ui.zone.title"), zoneListNote(s.Language, len(l.Rows), len(shown)))),
		layout.Rigid(s.field(&l.Search, locale.Text(s.Language, "ui.zone.search"), icon.Search)),
		layout.Rigid(s.stepper(&l.PrevFilter, &l.NextFilter, nil, filter)),
	}
	if l.TypeFilter >= 0 {
		if ids, hide := l.typeToggle(); len(ids) > 0 {
			text, ic := locale.Format(s.Language, "ui.zone.show_type", map[string]string{"type": filter}), icon.Eye
			if hide {
				text, ic = locale.Format(s.Language, "ui.zone.hide_type", map[string]string{"type": filter}), icon.EyeOff
			}
			children = append(children, layout.Rigid(s.spaced(s.fullButton(&l.ToggleType, secondaryButton, ic, text))))
		}
	}
	if len(shown) > 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, s.text(locale.Text(s.Language, "ui.zone.compile"), smallSize, font.Medium, dimText, 1)),
					layout.Rigid(s.chip(&l.CompileAll, locale.Text(s.Language, "ui.zone.all"), false)),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(s.chip(&l.CompileNone, locale.Text(s.Language, "ui.zone.none"), false)),
				)
			})
		}))
	} else if len(l.Rows) == 0 {
		children = append(children, layout.Rigid(s.dimLabel(locale.Text(s.Language, "ui.zone.empty"))))
	}
	for _, r := range shown {
		children = append(children, layout.Rigid(s.zoneRow(r, l.widgets(r.ID), r.ID == l.Selected)))
	}
	return children
}

// selectedZone is the inspector's section on the selected zone: its name,
// the duplicate, colour and delete actions, the height window, and its
// type and parameters.
func (s *Shell) selectedZone() []layout.FlexChild {
	l := &s.Zones
	selected := l.sync()
	if selected == nil {
		return []layout.FlexChild{
			layout.Rigid(s.section(false, icon.SlidersHorizontal, locale.Text(s.Language, "ui.zone.selected"), "")),
			layout.Rigid(s.dimLabel(locale.Text(s.Language, "ui.zone.select_hint"))),
		}
	}
	children := []layout.FlexChild{
		layout.Rigid(s.section(false, icon.SlidersHorizontal, locale.Text(s.Language, "ui.zone.selected"), selected.Name)),
		layout.Rigid(s.fieldLabel(locale.Text(s.Language, "ui.zone.name"))),
		layout.Rigid(s.fieldButton(&l.NewName, locale.Text(s.Language, "ui.zone.new_name"), nil, s.button(&l.Rename, primaryButton, icon.Pencil, locale.Text(s.Language, "ui.zone.rename")))),
		layout.Rigid(buttonRow(
			s.button(&l.Duplicate, secondaryButton, icon.Copy, locale.Text(s.Language, "ui.zone.duplicate")),
			s.button(&l.Color, secondaryButton, icon.Palette, locale.Text(s.Language, "ui.zone.color")),
			s.button(&l.Delete, dangerButton, icon.Trash2, locale.Text(s.Language, "ui.zone.delete")),
		)),
	}
	if l.LeftOut > 0 {
		children = append(children, layout.Rigid(s.errorLabel(locale.Plural(s.Language, "ui.zone.partial", l.LeftOut, nil))))
	}
	if s.Height.Zone != "" && s.Height.Window.Closed {
		children = append(children, layout.Rigid(s.spaced(s.fullButton(&s.Height.Reopen, secondaryButton, icon.ArrowUp, locale.Text(s.Language, "ui.zone.reopen_height")))))
	}
	return append(children, s.propertiesPanel()...)
}

func (l *ZoneList) has(id zone.ZoneID) bool {
	for _, r := range l.Rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

// zoneListNote is the zone section's note: the zone count, and how many
// pass the search and filter when not all do.
func zoneListNote(lang locale.Language, total, shown int) string {
	if total == 0 {
		return locale.Text(lang, "ui.zone.zero")
	}
	note := locale.Plural(lang, "ui.zone.count", total, nil)
	if shown == total {
		return note
	}
	return locale.Plural(lang, "ui.zone.filtered", shown, map[string]string{"total": note})
}

// zoneRow is one list row: the compile check box, then, clickable to
// select, the colour swatch, the name over the type and problem count, and
// the show/hide eye.
func (s *Shell) zoneRow(r ZoneRow, w *rowWidgets, selected bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		name, detail := textColor, dimText
		if r.Hidden {
			name, detail = faintText, faintText
		}
		problems := detail
		if r.Problems > 0 && !r.Hidden {
			problems = errorText
		}
		kind := string(r.Type)
		if r.Note != "" {
			kind += " · " + r.Note
		}
		eye := icon.Eye
		if r.Hidden {
			eye = icon.EyeOff
		}
		return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return w.compile.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return checkMark(gtx, w.compile.Value, w.compile.Hovered())
						})
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return w.pick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						call, content := measure(gtx, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min.X = gtx.Constraints.Max.X
							return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(swatch(r.Color, r.Hidden)),
									layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(s.text(r.Name, bodySize, font.Medium, name, 1)),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return layout.Flex{}.Layout(gtx,
													layout.Flexed(1, s.text(kind, captionSize, font.Normal, detail, 1)),
													layout.Rigid(s.text(problemCount(s.Language, r.Problems), captionSize, font.Medium, problems, 1)),
												)
											}),
										)
									}),
								)
							})
						})
						switch {
						case selected:
							fillRRect(gtx, content, controlRadius, accentSoft, color.NRGBA{})
						case w.pick.Hovered():
							fillRRect(gtx, content, controlRadius, controlFill, color.NRGBA{})
						}
						call.Add(gtx.Ops)
						pointerCursor(gtx, content)
						return layout.Dimensions{Size: content}
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(s.iconToggle(&w.toggle, eye, false)),
			)
		})
	}
}

// swatch is a small rounded square of colour c, outlined only when
// hidden.
func swatch(c color.NRGBA, hidden bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		n := gtx.Dp(unit.Dp(10))
		sz := image.Point{X: n, Y: n}
		rr := clip.UniformRRect(image.Rectangle{Max: sz}, gtx.Dp(3))
		if hidden {
			paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(1.5))}.Op())
		} else {
			paint.FillShape(gtx.Ops, c, rr.Op(gtx.Ops))
		}
		return layout.Dimensions{Size: sz}
	}
}
