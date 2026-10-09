package ui

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// Tool is a viewport tool that turns clicks into zone elements.
type Tool int

const (
	// ToolPolygon adds a vertex per click; Enter or a click on the first
	// vertex closes the polygon.
	ToolPolygon Tool = iota
	// ToolRectangle takes 2 opposite corners.
	ToolRectangle
	// ToolCircle takes the center, then a point on the circle.
	ToolCircle
	// ToolRestart adds a restart_point per click.
	ToolRestart
	// ToolPKRestart adds a PKrestart_point per click.
	ToolPKRestart
)

// shapeTools are the tools that draw a shape, in button order.
var shapeTools = []Tool{ToolPolygon, ToolRectangle, ToolCircle}

// IsShape reports whether t draws a shape (rather than placing points).
func (t Tool) IsShape() bool { return t <= ToolCircle }

func (t Tool) label() string {
	switch t {
	case ToolPolygon:
		return "Polígono"
	case ToolRectangle:
		return "Retângulo"
	case ToolCircle:
		return "Círculo"
	case ToolRestart:
		return "Restart"
	case ToolPKRestart:
		return "PK restart"
	}
	return "?"
}

// ToolPanel holds the tool buttons. The window loop reads requests from
// it and tells it which tool is armed, which it highlights.
type ToolPanel struct {
	buttons [ToolPKRestart + 1]widget.Clickable
	// Banned makes the shape tools draw exclusions (banned_polygon).
	Banned widget.Bool
	// WholeTile adds a polygon over the whole tile in view.
	WholeTile widget.Clickable
	// Shape is the shape tool a new zone starts with: the last one chosen.
	Shape Tool
	// Armed is the tool taking viewport clicks; Active tells whether one
	// is.
	Armed  Tool
	Active bool
}

// Requested returns a tool whose button was clicked since the last call;
// choosing a shape tool also makes it the one new zones start with.
func (p *ToolPanel) Requested(gtx layout.Context) (Tool, bool) {
	var t Tool
	ok := false
	for i := range p.buttons {
		for p.buttons[i].Clicked(gtx) {
			t, ok = Tool(i), true
		}
	}
	if ok && t.IsShape() {
		p.Shape = t
	}
	return t, ok
}

var armedButton = color.NRGBA{R: 0xD0, G: 0x6A, B: 0x10, A: 0xFF}

func (s *Shell) toolButton(t Tool) layout.FlexChild {
	p := &s.Zone.Tools
	return layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			b := material.Button(s.Theme, &p.buttons[t], t.label())
			b.TextSize = unit.Sp(13)
			b.Inset = layout.UniformInset(unit.Dp(8))
			if p.Active && p.Armed == t {
				b.Background = armedButton
			}
			return b.Layout(gtx)
		})
	})
}

// toolPanel lays out the tool buttons: the shape tools, the exclusion
// switch, the restart point tools.
func (s *Shell) toolPanel() []layout.FlexChild {
	p := &s.Zone.Tools
	row := func(ts ...Tool) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, len(ts))
			for i, t := range ts {
				children[i] = s.toolButton(t)
			}
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx, children...)
			})
		})
	}
	return []layout.FlexChild{
		layout.Rigid(s.label("Ferramentas (na zona selecionada)")),
		row(shapeTools...),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				b := material.Button(s.Theme, &p.WholeTile, "Tile inteiro")
				b.TextSize = unit.Sp(13)
				b.Inset = layout.UniformInset(unit.Dp(8))
				return b.Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			cb := material.CheckBox(s.Theme, &p.Banned, "Exclusão (banned_polygon)")
			cb.Color, cb.IconColor = panelText, panelText
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, cb.Layout)
		}),
		row(ToolRestart, ToolPKRestart),
	}
}
