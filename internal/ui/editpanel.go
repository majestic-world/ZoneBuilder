package ui

import (
	"strconv"

	"gioui.org/io/key"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/widget"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/icon"
	"zonebuilder/internal/zone"
)

// EditPanel holds the inspector's editing controls: the Z margin and the
// fields that edit the current shape and the selected vertex by keyboard.
// Like ZonePanel it only collects input; the window loop turns the
// requests into zone.Document commands and fills the fields.
type EditPanel struct {
	// Margin is how far the suggested Z range reaches past the vertices
	// or the ground under them (zone.DefaultZMargin unless changed).
	Margin widget.Editor
	// FromVertices picks where a new polygon, rectangle or circle takes
	// its Z range from: the clicked points alone (zone.SuggestZRange)
	// instead of the floor under its area. PrevFrom and NextFrom step
	// between the 2. The whole tile ignores it.
	FromVertices       bool
	PrevFrom, NextFrom widget.Clickable
	// Shape titles the current shape's controls; empty hides them.
	// Measure is the shape's size on the ground under it; Coverage, one
	// line per row, is how its Z range covers the floor under it.
	Shape, Measure, Coverage string
	// ZRange is "zmin zmax"; SetZRange (or Enter) applies it, GroundZ
	// recomputes it from the floor under the shape's area.
	ZRange       widget.Editor
	SetZRange    widget.Clickable
	GroundZ      widget.Clickable
	Offset       widget.Editor // "dx dy dz" for MoveShape
	MoveShape    widget.Clickable
	// Box shows the box controls: the current shape is a box (a rectangle,
	// or a turned box). BoxCenter is "x y", BoxSize "width length height",
	// BoxBase the floor z and BoxYaw the turn in degrees; SetBox (or Enter
	// in any of them) applies all 4 at once.
	Box                                 bool
	BoxCenter, BoxSize, BoxBase, BoxYaw widget.Editor
	SetBox                              widget.Clickable
	// BoxSliders drag the box's width, length, height and yaw, in the
	// order of BoxSlideWidth...; see BoxSlideValue for their ranges.
	BoxSliders [4]widget.Float
	Vertex       string        // titles the vertex controls; empty hides them
	Coords       widget.Editor // "x y z" of the selected vertex
	SetCoords    widget.Clickable
	InsertAfter  widget.Clickable
	RemoveVertex widget.Clickable
}

func (p *EditPanel) init() {
	for _, e := range []*widget.Editor{&p.Margin, &p.ZRange, &p.Offset, &p.Coords, &p.BoxCenter, &p.BoxSize, &p.BoxBase, &p.BoxYaw} {
		e.SingleLine, e.Submit = true, true
	}
	p.Margin.SetText(strconv.Itoa(zone.DefaultZMargin))
}

// ZRangeRequested reports a click on SetZRange or Enter in ZRange.
func (p *EditPanel) ZRangeRequested(gtx layout.Context) bool {
	return requested(gtx, &p.ZRange, &p.SetZRange)
}

// MoveShapeRequested reports a click on MoveShape or Enter in Offset.
func (p *EditPanel) MoveShapeRequested(gtx layout.Context) bool {
	return requested(gtx, &p.Offset, &p.MoveShape)
}

// CoordsRequested reports a click on SetCoords or Enter in Coords.
func (p *EditPanel) CoordsRequested(gtx layout.Context) bool {
	return requested(gtx, &p.Coords, &p.SetCoords)
}

// BoxRequested reports a click on SetBox or Enter in a box field.
func (p *EditPanel) BoxRequested(gtx layout.Context) bool {
	ok := false
	for _, e := range []*widget.Editor{&p.BoxCenter, &p.BoxSize, &p.BoxBase, &p.BoxYaw} {
		if requested(gtx, e, &p.SetBox) {
			ok = true
		}
	}
	return ok
}

// The box sliders, in the order of EditPanel.BoxSliders.
const (
	BoxSlideWidth = iota
	BoxSlideLength
	BoxSlideHeight
	BoxSlideYaw
)

// boxSlideRange is each box slider's [min, max]: width and length up to a
// tile (32768), height up to half of one, yaw a whole turn in degrees.
var boxSlideRange = [4][2]float64{{1, 32768}, {1, 32768}, {0, 16384}, {0, 360}}

// BoxSlideValue is the value box slider i shows at position pos, 0 to 1.
func BoxSlideValue(i int, pos float32) float64 {
	r := boxSlideRange[i]
	return r[0] + float64(min(max(pos, 0), 1))*(r[1]-r[0])
}

// BoxSlidePos is the position, 0 to 1, of value v on box slider i; a
// value past either end sits at that end.
func BoxSlidePos(i int, v float64) float32 {
	r := boxSlideRange[i]
	return float32(min(max((v-r[0])/(r[1]-r[0]), 0), 1))
}

// UpdateBoxSliders applies the pointer input on the box sliders since the
// last call to their values.
func (p *EditPanel) UpdateBoxSliders(gtx layout.Context) {
	for i := range p.BoxSliders {
		p.BoxSliders[i].Update(gtx)
	}
}

// ApplyFrom applies clicks on the new range source's arrows since the
// last call.
func (p *EditPanel) ApplyFrom(gtx layout.Context) {
	for p.PrevFrom.Clicked(gtx) {
		p.FromVertices = !p.FromVertices
	}
	for p.NextFrom.Clicked(gtx) {
		p.FromVertices = !p.FromVertices
	}
}

// requested reports a click on b or Enter in e since the last call.
func requested(gtx layout.Context, e *widget.Editor, b *widget.Clickable) bool {
	ok := b.Clicked(gtx)
	for {
		ev, more := e.Update(gtx)
		if !more {
			return ok
		}
		if _, submit := ev.(widget.SubmitEvent); submit {
			ok = true
		}
	}
}

func discardClicks(gtx layout.Context, clicks ...*widget.Clickable) {
	for _, c := range clicks {
		for c.Clicked(gtx) {
		}
	}
}

// discardEditorEvents consumes late IME/keyboard/paste events without
// calling Editor.Update, which would apply edits before returning them.
// Disabled belongs to layout; it does not protect a live-source Update.
func discardEditorEvents(gtx layout.Context, editors ...*widget.Editor) {
	for _, e := range editors {
		for {
			_, ok := gtx.Event(
				key.FocusFilter{Target: e},
				key.Filter{Focus: e, Optional: key.ModCtrl | key.ModCommand | key.ModShift | key.ModAlt | key.ModSuper},
				transfer.TargetFilter{Target: e, Type: "application/text"},
			)
			if !ok {
				break
			}
		}
	}
}

// editPanel is the inspector's last section: the Z margin and the new
// range source, then the current shape's box (a rectangle's), Z range and
// move, then the selected vertex.
func (s *Shell) editPanel() []layout.FlexChild {
	p := &s.Edit
	from := locale.Text(s.Language, "ui.edit.new_range.ground")
	if p.FromVertices {
		from = locale.Text(s.Language, "ui.edit.new_range.vertices")
	}
	children := []layout.FlexChild{
		layout.Rigid(s.section(false, icon.Ruler, locale.Text(s.Language, "ui.edit.title"), "")),
		layout.Rigid(s.fieldLabel(locale.Text(s.Language, "ui.edit.margin"))),
		layout.Rigid(s.field(&p.Margin, strconv.Itoa(zone.DefaultZMargin), nil)),
		layout.Rigid(s.fieldLabel(locale.Text(s.Language, "ui.edit.new_range"))),
		layout.Rigid(s.stepper(&p.PrevFrom, &p.NextFrom, nil, from)),
	}
	if p.Shape != "" {
		children = append(children,
			layout.Rigid(s.dimLabel(p.Shape)),
			layout.Rigid(s.dimLabel(p.Measure)),
			layout.Rigid(s.dimLabel(p.Coverage)),
		)
		if p.Box {
			children = append(children,
				layout.Rigid(s.fieldLabel(locale.Text(s.Language, "ui.edit.box_center"))),
				layout.Rigid(s.field(&p.BoxCenter, "x y", nil)),
				layout.Rigid(s.fieldLabel(locale.Text(s.Language, "ui.edit.box_size"))),
				layout.Rigid(s.field(&p.BoxSize, locale.Text(s.Language, "ui.edit.box_size_placeholder"), nil)),
				layout.Rigid(s.slider(&p.BoxSliders[BoxSlideWidth], locale.Text(s.Language, "ui.edit.box_slide_width"))),
				layout.Rigid(s.slider(&p.BoxSliders[BoxSlideLength], locale.Text(s.Language, "ui.edit.box_slide_length"))),
				layout.Rigid(s.spaced(s.slider(&p.BoxSliders[BoxSlideHeight], locale.Text(s.Language, "ui.edit.box_slide_height")))),
				layout.Rigid(s.fieldLabel(locale.Text(s.Language, "ui.edit.box_base"))),
				layout.Rigid(s.field(&p.BoxBase, "z", nil)),
				layout.Rigid(s.fieldLabel(locale.Text(s.Language, "ui.edit.box_yaw"))),
				layout.Rigid(s.fieldButton(&p.BoxYaw, locale.Text(s.Language, "ui.edit.box_yaw_placeholder"), nil, s.button(&p.SetBox, secondaryButton, icon.Box, locale.Text(s.Language, "ui.edit.box_apply")))),
				layout.Rigid(s.spaced(s.slider(&p.BoxSliders[BoxSlideYaw], locale.Text(s.Language, "ui.edit.box_slide_yaw")))),
			)
		}
		children = append(children,
			layout.Rigid(s.fieldLabel(locale.Text(s.Language, "ui.edit.z_range"))),
			layout.Rigid(s.fieldButton(&p.ZRange, "zmin zmax", nil, s.button(&p.SetZRange, secondaryButton, nil, locale.Text(s.Language, "ui.edit.apply")))),
			layout.Rigid(s.spaced(s.fullButton(&p.GroundZ, secondaryButton, icon.Mountain, locale.Text(s.Language, "ui.edit.ground")))),
			layout.Rigid(s.fieldLabel(locale.Text(s.Language, "ui.edit.move_label"))),
			layout.Rigid(s.fieldButton(&p.Offset, "dx dy dz", nil, s.button(&p.MoveShape, secondaryButton, icon.Move, locale.Text(s.Language, "ui.edit.move")))),
		)
	}
	if p.Vertex != "" {
		children = append(children,
			layout.Rigid(s.fieldLabel(p.Vertex)),
			layout.Rigid(s.fieldButton(&p.Coords, "x y z", nil, s.button(&p.SetCoords, secondaryButton, nil, locale.Text(s.Language, "ui.edit.apply")))),
			layout.Rigid(buttonRow(
				s.button(&p.InsertAfter, secondaryButton, icon.Plus, locale.Text(s.Language, "ui.edit.insert")),
				s.button(&p.RemoveVertex, dangerButton, icon.Trash2, locale.Text(s.Language, "ui.edit.remove_vertex")),
			)),
		)
	}
	return children
}
