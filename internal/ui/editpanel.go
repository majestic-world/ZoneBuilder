package ui

import (
	"strconv"

	"gioui.org/layout"
	"gioui.org/widget"

	"zonebuilder/internal/zone"
)

// EditPanel holds the side panel's editing controls: Undo and Redo, the Z
// margin, and the fields that edit the current shape and the selected
// vertex by keyboard. Like ZonePanel it only collects input; the window
// loop turns the requests into zone.Document commands and fills the fields.
type EditPanel struct {
	Undo, Redo widget.Clickable
	// Margin is how far the suggested Z range reaches past the vertices
	// or the ground under them (zone.DefaultZMargin unless changed).
	Margin widget.Editor
	// Shape titles the current shape's controls; empty hides them.
	Shape string
	// ZRange is "zmin zmax"; SetZRange (or Enter) applies it, GroundZ
	// recomputes it from the ground under the vertices.
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

func (s *Shell) editPanel() []layout.FlexChild {
	p := &s.Edit
	row := func(a, b layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx, layout.Rigid(a), layout.Rigid(b))
		})
	}
	children := []layout.FlexChild{
		row(s.button(&p.Undo, "Desfazer"), s.button(&p.Redo, "Refazer")),
		layout.Rigid(s.label("Folga Z da faixa sugerida")),
		layout.Rigid(s.field(&p.Margin, strconv.Itoa(zone.DefaultZMargin))),
	}
	if s.Height.Zone != "" && s.Height.Window.Closed {
		children = append(children, layout.Rigid(s.button(&s.Height.Reopen, "Mostrar janela de altura")))
	}
	if p.Shape != "" {
		children = append(children,
			layout.Rigid(s.label(p.Shape)),
			layout.Rigid(s.label("Faixa Z: zmin zmax")),
			layout.Rigid(s.field(&p.ZRange, "zmin zmax")),
			row(s.button(&p.SetZRange, "Aplicar faixa"), s.button(&p.GroundZ, "Recalcular pelo chão")),
			layout.Rigid(s.label("Mover shape: dx dy dz")),
			layout.Rigid(s.field(&p.Offset, "dx dy dz")),
			layout.Rigid(s.button(&p.MoveShape, "Mover shape")),
		)
	}
	if p.Vertex != "" {
		children = append(children,
			layout.Rigid(s.label(p.Vertex)),
			layout.Rigid(s.field(&p.Coords, "x y z")),
			layout.Rigid(s.button(&p.SetCoords, "Aplicar coordenadas")),
			row(s.button(&p.InsertAfter, "Inserir na aresta"), s.button(&p.RemoveVertex, "Apagar vértice")),
		)
	}
	return children
}
