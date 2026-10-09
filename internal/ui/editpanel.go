package ui

import (
	"strconv"

	"gioui.org/layout"
	"gioui.org/widget"

	"zonebuilder/internal/ui/icon"
	"zonebuilder/internal/zone"
)

// EditPanel holds the editing controls: the command bar's Undo and Redo,
// and in the inspector the Z margin and the fields that edit the current
// shape and the selected vertex by keyboard. Like ZonePanel it only
// collects input; the window loop turns the requests into zone.Document
// commands and fills the fields.
type EditPanel struct {
	Undo, Redo widget.Clickable
	// Margin is how far the suggested Z range reaches past the vertices
	// or the ground under them (zone.DefaultZMargin unless changed).
	Margin widget.Editor
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
	Vertex       string        // titles the vertex controls; empty hides them
	Coords       widget.Editor // "x y z" of the selected vertex
	SetCoords    widget.Clickable
	InsertAfter  widget.Clickable
	RemoveVertex widget.Clickable
}

func (p *EditPanel) init() {
	for _, e := range []*widget.Editor{&p.Margin, &p.ZRange, &p.Offset, &p.Coords} {
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

// editPanel is the inspector's last section: the Z margin, then the
// current shape's Z range and move, then the selected vertex.
func (s *Shell) editPanel() []layout.FlexChild {
	p := &s.Edit
	children := []layout.FlexChild{
		layout.Rigid(s.section(false, icon.Ruler, "Edição", "")),
		layout.Rigid(s.fieldLabel("Folga Z da faixa sugerida")),
		layout.Rigid(s.field(&p.Margin, strconv.Itoa(zone.DefaultZMargin), nil)),
	}
	if p.Shape != "" {
		children = append(children,
			layout.Rigid(s.dimLabel(p.Shape)),
			layout.Rigid(s.dimLabel(p.Measure)),
			layout.Rigid(s.dimLabel(p.Coverage)),
			layout.Rigid(s.fieldLabel("Faixa Z: zmin zmax")),
			layout.Rigid(s.fieldButton(&p.ZRange, "zmin zmax", nil, s.button(&p.SetZRange, secondaryButton, nil, "Aplicar"))),
			layout.Rigid(s.spaced(s.fullButton(&p.GroundZ, secondaryButton, icon.Mountain, "Recalcular pelo chão"))),
			layout.Rigid(s.fieldLabel("Mover shape: dx dy dz")),
			layout.Rigid(s.fieldButton(&p.Offset, "dx dy dz", nil, s.button(&p.MoveShape, secondaryButton, icon.Move, "Mover"))),
		)
	}
	if p.Vertex != "" {
		children = append(children,
			layout.Rigid(s.fieldLabel(p.Vertex)),
			layout.Rigid(s.fieldButton(&p.Coords, "x y z", nil, s.button(&p.SetCoords, secondaryButton, nil, "Aplicar"))),
			layout.Rigid(buttonRow(
				s.button(&p.InsertAfter, secondaryButton, icon.Plus, "Inserir na aresta"),
				s.button(&p.RemoveVertex, dangerButton, icon.Trash2, "Apagar vértice"),
			)),
		)
	}
	return children
}
