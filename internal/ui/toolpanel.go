package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/icon"
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

func (t Tool) label(language locale.Language) string {
	switch t {
	case ToolPolygon:
		return locale.Text(language, "ui.tool.polygon")
	case ToolRectangle:
		return locale.Text(language, "ui.tool.rectangle")
	case ToolCircle:
		return locale.Text(language, "ui.tool.circle")
	case ToolRestart:
		return locale.Text(language, "ui.tool.restart")
	case ToolPKRestart:
		return locale.Text(language, "ui.tool.pk_restart")
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

// Discard consumes tool requests without changing the chosen shape.
func (p *ToolPanel) Discard(gtx layout.Context) {
	for i := range p.buttons {
		discardClicks(gtx, &p.buttons[i])
	}
	discardClicks(gtx, &p.WholeTile)
	banned := p.Banned.Value
	p.Banned.Update(gtx)
	p.Banned.Value = banned
}

// icon is the tool's dock icon.
func (t Tool) icon() *icon.Icon {
	switch t {
	case ToolPolygon:
		return icon.Pentagon
	case ToolRectangle:
		return icon.Square
	case ToolCircle:
		return icon.Circle
	case ToolRestart:
		return icon.MapPin
	}
	return icon.Swords
}

// dock is the tool card down the left edge: the shape tools and Tile
// inteiro, the exclusion switch, then the restart point tools. The tools
// work on the selected zone.
func (s *Shell) dock(gtx layout.Context) layout.Dimensions {
	p := &s.Zone.Tools
	children := []layout.FlexChild{s.toolTile(p, ToolPolygon), s.toolTile(p, ToolRectangle), s.toolTile(p, ToolCircle),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.WholeTile.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return s.dockButton(gtx, icon.Grid2x2, locale.Text(s.Language, "ui.tool.whole_tile"), false, false, p.WholeTile.Hovered())
			})
		}),
		layout.Rigid(dockDivider),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.Banned.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return s.dockButton(gtx, icon.SquareDashed, locale.Text(s.Language, "ui.tool.exclusion"), false, p.Banned.Value, p.Banned.Hovered())
			})
		}),
		layout.Rigid(dockDivider),
		s.toolTile(p, ToolRestart), s.toolTile(p, ToolPKRestart),
	}
	return card(gtx, &s.dockSink, layout.UniformInset(unit.Dp(6)), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// areaDock is the population mode's tool card: the shape tools, each
// drawing a new spawn area.
func (s *Shell) areaDock(gtx layout.Context) layout.Dimensions {
	p := &s.Spawn.Tools
	return card(gtx, &s.dockSink, layout.UniformInset(unit.Dp(6)), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, s.toolTile(p, ToolPolygon), s.toolTile(p, ToolRectangle), s.toolTile(p, ToolCircle))
	})
}

// toolTile is tool t's dock tile in p, highlighted while armed.
func (s *Shell) toolTile(p *ToolPanel, t Tool) layout.FlexChild {
	c := &p.buttons[t]
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return s.dockButton(gtx, t.icon(), t.label(s.Language), p.Active && p.Armed == t, false, c.Hovered())
		})
	})
}

// dockButton is one dock tile: the icon over its name, solid violet while
// armed, tinted while switched on.
func (s *Shell) dockButton(gtx layout.Context, ic *icon.Icon, name string, armed, on, hovered bool) layout.Dimensions {
	size := image.Pt(gtx.Dp(76), gtx.Dp(54))
	bg, border, ink := color.NRGBA{}, color.NRGBA{}, dimText
	switch {
	case armed:
		bg, ink = accent, white
	case on:
		bg, border, ink = accentSoft, accent, accentText
	case hovered:
		bg, ink = controlHover, textColor
	}
	fillRRect(gtx, size, unit.Dp(6), bg, border)
	n := gtx.Dp(20)
	at(gtx, image.Pt((size.X-n)/2, gtx.Dp(9)), func(gtx layout.Context) layout.Dimensions { return ic.Layout(gtx, 20, ink) })
	call, txt := measure(gtx, s.text(name, captionSize, font.Medium, ink, 1))
	place(gtx, image.Pt(max(0, (size.X-txt.X)/2), gtx.Dp(34)), call)
	pointerCursor(gtx, size)
	return layout.Dimensions{Size: size}
}

// dockDivider is the hairline between the dock's groups.
func dockDivider(gtx layout.Context) layout.Dimensions {
	size := image.Pt(gtx.Dp(76), gtx.Dp(13))
	at(gtx, image.Pt(gtx.Dp(14), gtx.Dp(6)), func(gtx layout.Context) layout.Dimensions {
		line := image.Pt(size.X-gtx.Dp(28), gtx.Dp(1))
		paint.FillShape(gtx.Ops, hairline, clip.Rect{Max: line}.Op())
		return layout.Dimensions{Size: line}
	})
	return layout.Dimensions{Size: size}
}
