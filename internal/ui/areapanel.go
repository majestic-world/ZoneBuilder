package ui

import (
	"image/color"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/spawn"
	"zonebuilder/internal/ui/icon"
)

// AreaRow is one spawn area as the area list shows it.
type AreaRow struct {
	ID   spawn.AreaID
	Name string
	// Detail is the line under the name (count, drawing state).
	Detail string
	// Problems is how many blocking problems the area has.
	Problems int
	Hidden   bool
	Color    color.NRGBA
}

// AreaField is one of the selected area's properties in the panel.
type AreaField int

const (
	AreaName AreaField = iota
	AreaCount
	AreaRadius
	AreaClearance
	areaFields
)

// AreaPanel is the population mode's controls: the shape tools of the
// dock, the area list with its show/hide toggles, the area problems, and
// the selected area's actions and properties. Like the zone panels it only
// collects input: Update reports what the user asked for and the window
// loop turns it into spawn.Document commands and camera moves.
type AreaPanel struct {
	// Tools are the dock's polygon, rectangle and circle tools.
	Tools ToolPanel
	// Info is the armed tool's hints, shown in the message card.
	Info []string
	// Rows are every area of the document, set by the window loop each
	// frame; Selected is the selected area's ID (0: none).
	Rows     []AreaRow
	Selected spawn.AreaID
	// Problems is the areas' problem list.
	Problems ProblemList

	Duplicate, Delete widget.Clickable
	// Generate distributes the selected area's points with its seed,
	// Regenerate with another one; AddPoints toggles adding points with
	// viewport clicks.
	Generate, Regenerate, AddPoints widget.Clickable
	// PointsNote is the points section's note; PointStats are its lines
	// (free floor, spacing) and PointWarnings the ones in the warning
	// colour (stale points, the distribution's warnings). Generating
	// shows a generation running; Adding lights AddPoints.
	PointsNote                string
	PointStats, PointWarnings []string
	Generating, Adding        bool
	// XMLName is the spawn XML's file name; empty means XMLDefault (the
	// project's name), shown as its hint.
	XMLName    widget.Editor
	XMLDefault string

	fields [areaFields]areaField
	// loaded is the area the fields were last loaded for (0: none).
	loaded spawn.AreaID
	rows   map[spawn.AreaID]*areaRowWidgets
}

// areaField is one property's text field; value is the area's, as last
// loaded into the editor.
type areaField struct {
	editor  widget.Editor
	focused bool
	value   string
}

type areaRowWidgets struct {
	pick, toggle widget.Clickable
}

// Area panel requests, returned by AreaPanel.Update.
type (
	// SelectArea: a row was clicked.
	SelectArea struct{ Area spawn.AreaID }
	// HideArea: a row's show/hide toggle was clicked.
	HideArea struct {
		Area   spawn.AreaID
		Hidden bool
	}
	// DuplicateArea, DeleteArea: the actions on the selected area.
	DuplicateArea struct{ Area spawn.AreaID }
	DeleteArea    struct{ Area spawn.AreaID }
	// SetAreaField: a property field was committed (Enter, focus lost,
	// or generation requested) with text other than the area's value.
	SetAreaField struct {
		Area  spawn.AreaID
		Field AreaField
		Text  string
	}
	// GenerateArea: Gerar (Regenerate false: the area's seed) or Regerar
	// (another seed).
	GenerateArea struct {
		Area       spawn.AreaID
		Regenerate bool
	}
	// AddPoints: the Adicionar pontos toggle, On to start adding.
	AddPoints struct {
		Area spawn.AreaID
		On   bool
	}
)

func (p *AreaPanel) init() {
	for i := range p.fields {
		p.fields[i].editor.SingleLine, p.fields[i].editor.Submit = true, true
	}
	p.XMLName.SingleLine = true
	p.Reset()
}

// Reset forgets the per-area state, for a document whose area IDs mean
// other areas (an opened project).
func (p *AreaPanel) Reset() {
	p.rows = map[spawn.AreaID]*areaRowWidgets{}
	p.loaded = 0
}

// Discard consumes pending events without accepting any editor action.
func (p *AreaPanel) Discard(gtx layout.Context) {
	discardClicks(gtx, &p.Duplicate, &p.Delete, &p.Generate, &p.Regenerate, &p.AddPoints)
	for _, r := range p.Rows {
		w := p.widgets(r.ID)
		discardClicks(gtx, &w.pick, &w.toggle)
	}
	for i := range p.fields {
		f := &p.fields[i]
		discardEditorEvents(gtx, &f.editor)
		f.focused = false
	}
}

// Update loads a, the selected area (ok false: none), into the property
// fields and returns the requests since the last call: row clicks and
// toggles, actions and property edits, then generation. Call it before Layout.
func (p *AreaPanel) Update(gtx layout.Context, a spawn.Area, ok bool) []any {
	var reqs []any
	for _, r := range p.Rows {
		w := p.widgets(r.ID)
		if w.pick.Clicked(gtx) {
			reqs = append(reqs, SelectArea{Area: r.ID})
		}
		if w.toggle.Clicked(gtx) {
			reqs = append(reqs, HideArea{Area: r.ID, Hidden: !r.Hidden})
		}
	}
	if !ok {
		p.loaded = 0
		return reqs
	}
	if p.Duplicate.Clicked(gtx) {
		reqs = append(reqs, DuplicateArea{Area: a.ID})
	}
	if p.Delete.Clicked(gtx) {
		reqs = append(reqs, DeleteArea{Area: a.ID})
	}
	generate := p.Generate.Clicked(gtx)
	regenerate := p.Regenerate.Clicked(gtx)
	if p.AddPoints.Clicked(gtx) {
		reqs = append(reqs, AddPoints{Area: a.ID, On: !p.Adding})
	}
	values := [areaFields]string{
		AreaName:      a.Name,
		AreaCount:     strconv.Itoa(a.Params.Count),
		AreaRadius:    strconv.Itoa(a.Params.Radius),
		AreaClearance: strconv.Itoa(a.Params.Clearance),
	}
	fresh := p.loaded != a.ID
	p.loaded = a.ID
	for i := range p.fields {
		f := &p.fields[i]
		// A field whose value did not change keeps what is being typed
		// in it, unless another area is shown.
		if fresh || f.value != values[i] {
			f.value = values[i]
			f.editor.SetText(values[i])
		}
		commit := committed(gtx, &f.editor, &f.focused)
		if (commit || generate || regenerate) && f.editor.Text() != f.value {
			reqs = append(reqs, SetAreaField{Area: a.ID, Field: AreaField(i), Text: f.editor.Text()})
		}
	}
	if generate {
		reqs = append(reqs, GenerateArea{Area: a.ID})
	}
	if regenerate {
		reqs = append(reqs, GenerateArea{Area: a.ID, Regenerate: true})
	}
	return reqs
}

func (p *AreaPanel) widgets(id spawn.AreaID) *areaRowWidgets {
	w, ok := p.rows[id]
	if !ok {
		w = new(areaRowWidgets)
		p.rows[id] = w
	}
	return w
}

// selected is the selected row, nil for none; it forgets the widgets of
// deleted areas.
func (p *AreaPanel) selected() *AreaRow {
	var sel *AreaRow
	live := make(map[spawn.AreaID]bool, len(p.Rows))
	for i := range p.Rows {
		live[p.Rows[i].ID] = true
		if p.Rows[i].ID == p.Selected {
			sel = &p.Rows[i]
		}
	}
	for id := range p.rows {
		if !live[id] {
			delete(p.rows, id)
		}
	}
	return sel
}

// areaPanel is the population mode's inspector panel, under the map
// section: the area list, the area problems and the selected area.
func (s *Shell) areaPanel() []layout.FlexChild {
	p := &s.Spawn
	note := locale.Text(s.Language, "spawn.areas.zero")
	if len(p.Rows) > 0 {
		note = locale.Plural(s.Language, "spawn.areas.count", len(p.Rows), nil)
	}
	children := []layout.FlexChild{
		layout.Rigid(s.section(false, icon.Users, locale.Text(s.Language, "spawn.areas.title"), note)),
	}
	if len(p.Rows) == 0 {
		children = append(children, layout.Rigid(s.dimLabel(locale.Text(s.Language, "spawn.areas.empty"))))
	}
	sel := p.selected()
	for _, r := range p.Rows {
		children = append(children, layout.Rigid(s.areaRow(r, p.widgets(r.ID), r.ID == p.Selected)))
	}
	children = append(children, s.problemList(&p.Problems)...)
	children = append(children,
		layout.Rigid(s.section(false, icon.CodeXML, locale.Text(s.Language, "spawn.xml.section"), "")),
		layout.Rigid(s.fieldLabel(locale.Text(s.Language, "spawn.xml.file"))),
		layout.Rigid(s.field(&p.XMLName, p.XMLDefault, nil)),
	)
	return append(children, s.selectedArea(sel)...)
}

// selectedArea is the section on the selected area: the duplicate and
// delete actions, the height window, its properties, then its points.
func (s *Shell) selectedArea(sel *AreaRow) []layout.FlexChild {
	p := &s.Spawn
	title := locale.Text(s.Language, "spawn.area.section")
	if sel == nil {
		return []layout.FlexChild{
			layout.Rigid(s.section(false, icon.SlidersHorizontal, title, "")),
			layout.Rigid(s.dimLabel(locale.Text(s.Language, "spawn.area.select_hint"))),
		}
	}
	field := func(f AreaField, label string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(s.fieldLabel(label)),
				layout.Rigid(s.field(&p.fields[f].editor, "", nil)),
			)
		}
	}
	pair := func(a, b layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, a),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Flexed(1, b),
			)
		})
	}
	children := []layout.FlexChild{
		layout.Rigid(s.section(false, icon.SlidersHorizontal, title, sel.Name)),
		layout.Rigid(buttonRow(
			s.button(&p.Duplicate, secondaryButton, icon.Copy, locale.Text(s.Language, "spawn.area.duplicate")),
			s.button(&p.Delete, dangerButton, icon.Trash2, locale.Text(s.Language, "spawn.area.delete")),
		)),
	}
	if s.Height.Zone != "" && s.Height.Window.Closed {
		children = append(children, layout.Rigid(s.spaced(s.fullButton(&s.Height.Reopen, secondaryButton, icon.ArrowUp, locale.Text(s.Language, "spawn.area.reopen_height")))))
	}
	children = append(children,
		layout.Rigid(field(AreaName, locale.Text(s.Language, "spawn.field.name"))),
		layout.Rigid(field(AreaCount, locale.Text(s.Language, "spawn.field.count"))),
		layout.Rigid(s.dimLabel(locale.Text(s.Language, "spawn.field.respawn_fixed"))),
		pair(field(AreaRadius, locale.Text(s.Language, "spawn.field.radius")), field(AreaClearance, locale.Text(s.Language, "spawn.field.clearance"))),
		layout.Rigid(s.dimLabel(locale.Text(s.Language, "spawn.field.hint"))),
	)
	return append(children, s.areaPoints()...)
}

// areaPoints is the section on the selected area's points: Gerar and
// Regerar, the Adicionar pontos toggle, the free floor and spacing, and
// the warnings.
func (s *Shell) areaPoints() []layout.FlexChild {
	p := &s.Spawn
	generate := locale.Text(s.Language, "spawn.points.generate")
	if p.Generating {
		generate = locale.Text(s.Language, "spawn.points.generating")
	}
	children := []layout.FlexChild{
		layout.Rigid(s.section(false, icon.MapPin, locale.Text(s.Language, "spawn.points.title"), p.PointsNote)),
		layout.Rigid(buttonRow(
			s.button(&p.Generate, primaryButton, icon.MapPin, generate),
			s.button(&p.Regenerate, secondaryButton, icon.Redo2, locale.Text(s.Language, "spawn.points.regenerate")),
		)),
		layout.Rigid(s.spaced(s.toggleButton(&p.AddPoints, icon.Plus, locale.Text(s.Language, "spawn.points.add"), p.Adding))),
	}
	for _, line := range p.PointStats {
		children = append(children, layout.Rigid(s.dimLabel(line)))
	}
	for _, line := range p.PointWarnings {
		children = append(children, layout.Rigid(s.spaced(s.text(line, smallSize, font.Normal, warnText, 0))))
	}
	return append(children, layout.Rigid(s.dimLabel(locale.Text(s.Language, "spawn.points.hint"))))
}

// areaRow is one list row, clickable to select: the colour swatch, the
// name over the detail and problem count, and the show/hide eye.
func (s *Shell) areaRow(r AreaRow, w *areaRowWidgets, selected bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		name, detail := textColor, dimText
		if r.Hidden {
			name, detail = faintText, faintText
		}
		problems := detail
		if r.Problems > 0 && !r.Hidden {
			problems = errorText
		}
		eye := icon.Eye
		if r.Hidden {
			eye = icon.EyeOff
		}
		return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
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
													layout.Flexed(1, s.text(r.Detail, captionSize, font.Normal, detail, 1)),
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
