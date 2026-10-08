package ui

import (
	"image"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"zonebuilder/internal/zone"
)

var (
	selectedRowBackground = color.NRGBA{R: 0x3A, G: 0x4A, B: 0x66, A: 0xFF}
	hiddenRowText         = color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xFF}
)

// ZoneRow is one zone as the zone list shows it.
type ZoneRow struct {
	ID   zone.ZoneID
	Name string
	Type zone.Type
	// Problems is the problem count as shown.
	Problems string
	Hidden   bool
	Color    color.NRGBA
	// Note is extra state shown after the type ("desenhando").
	Note string
}

// ZoneList is the side panel's zone list: every zone with its name, type
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

	Search widget.Editor
	// TypeFilter is the shown type's index in zone.Types, -1 for all.
	TypeFilter             int
	PrevFilter, NextFilter widget.Clickable
	ToggleType             widget.Clickable

	NewName                          widget.Editor
	Rename, Delete, Duplicate, Color widget.Clickable

	// GoTo holds the x y z the Go button (or Enter) flies the camera to.
	GoTo widget.Editor
	Go   widget.Clickable

	rows map[zone.ZoneID]*rowWidgets
	// named is the selection NewName was last filled for.
	named zone.ZoneID
}

type rowWidgets struct {
	pick, toggle widget.Clickable
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
)

func (l *ZoneList) init() {
	l.Search.SingleLine = true
	l.NewName.SingleLine = true
	l.NewName.Submit = true
	l.GoTo.SingleLine = true
	l.GoTo.Submit = true
	l.TypeFilter = -1
	l.rows = map[zone.ZoneID]*rowWidgets{}
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
	}
	if l.ToggleType.Clicked(gtx) && l.TypeFilter >= 0 {
		if ids, hide := l.typeToggle(); len(ids) > 0 {
			reqs = append(reqs, HideZones{Zones: ids, Hidden: hide})
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

func (s *Shell) zoneList() []layout.FlexChild {
	l := &s.Zones
	// Forget the widgets of deleted zones, and fill NewName when the
	// selection changes.
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

	filter := "Todos os tipos"
	if t := l.filterType(); t != "" {
		filter = string(t)
	}
	shown := l.shown()
	children := []layout.FlexChild{
		layout.Rigid(s.label("Ir para x y z (coordenadas do servidor)")),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Start}.Layout(gtx,
				layout.Flexed(1, s.field(&l.GoTo, "83400 147943 -3400")),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(material.Button(s.Theme, &l.Go, "Ir").Layout),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		layout.Rigid(s.label(zoneListTitle(len(l.Rows), len(shown)))),
		layout.Rigid(s.field(&l.Search, "buscar por nome")),
		layout.Rigid(s.arrows(&l.PrevFilter, &l.NextFilter, filter)),
	}
	if l.TypeFilter >= 0 {
		if ids, hide := l.typeToggle(); len(ids) > 0 {
			text := "Mostrar o tipo " + filter
			if hide {
				text = "Ocultar o tipo " + filter
			}
			children = append(children, layout.Rigid(s.button(&l.ToggleType, text)))
		}
	}
	for _, r := range shown {
		children = append(children, layout.Rigid(s.zoneRow(r, l.widgets(r.ID), r.ID == l.Selected)))
	}
	if selected != nil {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(s.label("Zona selecionada: "+selected.Name)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Start}.Layout(gtx,
					layout.Flexed(1, s.field(&l.NewName, "novo nome")),
					layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
					layout.Rigid(material.Button(s.Theme, &l.Rename, "Renomear").Layout),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(s.button(&l.Duplicate, "Duplicar")),
					layout.Rigid(s.button(&l.Color, "Cor")),
					layout.Rigid(s.button(&l.Delete, "Apagar")),
				)
			}),
		)
	}
	return children
}

func (l *ZoneList) has(id zone.ZoneID) bool {
	for _, r := range l.Rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

// zoneListTitle is the list's heading: the zone count, and how many pass
// the search and filter when not all do.
func zoneListTitle(total, shown int) string {
	title := "Zonas: nenhuma"
	switch {
	case total == 1:
		title = "Zonas: 1 zona"
	case total > 1:
		title = "Zonas: " + strconv.Itoa(total) + " zonas"
	}
	if shown == total {
		return title
	}
	if shown == 1 {
		return title + " (1 exibida)"
	}
	return title + " (" + strconv.Itoa(shown) + " exibidas)"
}

// arrows is a "< text >" stepper.
func (s *Shell) arrows(prev, next *widget.Clickable, text string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(material.Button(s.Theme, prev, "<").Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body1(s.Theme, text)
						lbl.Color = panelText
						return lbl.Layout(gtx)
					})
				}),
				layout.Rigid(material.Button(s.Theme, next, ">").Layout),
			)
		})
	}
}

// zoneRow is one list row: the colour swatch, name, and type with the
// problem count, clickable to select, and the show/hide toggle.
func (s *Shell) zoneRow(r ZoneRow, w *rowWidgets, selected bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		text := panelText
		if r.Hidden {
			text = hiddenRowText
		}
		toggle := "Ocultar"
		if r.Hidden {
			toggle = "Mostrar"
		}
		detail := string(r.Type) + " · problemas: " + r.Problems
		if r.Note != "" {
			detail += " · " + r.Note
		}
		return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.Clickable(gtx, &w.pick, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Stack{}.Layout(gtx,
							layout.Expanded(func(gtx layout.Context) layout.Dimensions {
								if selected {
									paint.FillShape(gtx.Ops, selectedRowBackground, clip.Rect{Max: gtx.Constraints.Min}.Op())
								}
								return layout.Dimensions{Size: gtx.Constraints.Min}
							}),
							layout.Stacked(func(gtx layout.Context) layout.Dimensions {
								return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
										layout.Rigid(swatch(r.Color, r.Hidden)),
										layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													lbl := material.Body2(s.Theme, r.Name)
													lbl.Color = text
													lbl.MaxLines = 1
													return lbl.Layout(gtx)
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													lbl := material.Caption(s.Theme, detail)
													lbl.Color = text
													lbl.MaxLines = 1
													return lbl.Layout(gtx)
												}),
											)
										}),
									)
								})
							}),
						)
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					b := material.Button(s.Theme, &w.toggle, toggle)
					b.TextSize = unit.Sp(12)
					b.Inset = layout.UniformInset(unit.Dp(6))
					return b.Layout(gtx)
				}),
			)
		})
	}
}

// swatch is a small square of colour c, outlined only when hidden.
func swatch(c color.NRGBA, hidden bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		n := gtx.Dp(unit.Dp(12))
		sz := image.Point{X: n, Y: n}
		if hidden {
			paint.FillShape(gtx.Ops, c, clip.Stroke{Path: clip.Rect{Max: sz}.Path(), Width: float32(gtx.Dp(unit.Dp(2)))}.Op())
		} else {
			paint.FillShape(gtx.Ops, c, clip.Rect{Max: sz}.Op())
		}
		return layout.Dimensions{Size: sz}
	}
}
