package ui

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
)

// DefaultZStep is the starting Step of Up and Down.
const DefaultZStep = 64

// HeightPanel raises, lowers and sizes the selected zone from a floating
// window over the viewport. Like the other panels it only collects input;
// the window loop turns the requests into zone.Document commands and
// fills the fields.
type HeightPanel struct {
	Window FloatWindow
	// Zone describes the selected zone's floor, top and height; empty
	// hides the window.
	Zone string
	// Step is how far Up and Down (and PageUp/PageDown in the viewport)
	// raise or lower the whole zone.
	Step     widget.Editor
	Up, Down widget.Clickable
	// Base is the zone's lowest zmin; SetBase (or Enter) moves the whole
	// zone so its floor lands there.
	Base    widget.Editor
	SetBase widget.Clickable
	// Height is each shape's zmax - zmin; SetHeight (or Enter) keeps the
	// floors and moves the tops.
	Height    widget.Editor
	SetHeight widget.Clickable
	// Reopen, in the side panel, opens the window again after its × closed
	// it.
	Reopen widget.Clickable
}

func (p *HeightPanel) init() {
	for _, e := range []*widget.Editor{&p.Step, &p.Base, &p.Height} {
		e.SingleLine, e.Submit = true, true
	}
	p.Step.SetText(strconv.Itoa(DefaultZStep))
	p.Window.Width, p.Window.Height = 270, 440
}

// StepZ is Step as a positive number of units, DefaultZStep when the field
// holds anything else.
func (p *HeightPanel) StepZ() int {
	if n, err := strconv.Atoi(strings.TrimSpace(p.Step.Text())); err == nil && n > 0 {
		return n
	}
	return DefaultZStep
}

// BaseRequested reports a click on SetBase or Enter in Base.
func (p *HeightPanel) BaseRequested(gtx layout.Context) bool {
	return requested(gtx, &p.Base, &p.SetBase)
}

// HeightRequested reports a click on SetHeight or Enter in Height.
func (p *HeightPanel) HeightRequested(gtx layout.Context) bool {
	return requested(gtx, &p.Height, &p.SetHeight)
}

// ReopenRequested reports a click on Reopen and opens the window again.
// Call it before Shell.Layout: the side panel's button consumes the click
// when it is laid out.
func (p *HeightPanel) ReopenRequested(gtx layout.Context) bool {
	if !p.Reopen.Clicked(gtx) {
		return false
	}
	p.Window.Closed = false
	return true
}

// heightWindow lays the floating window out over the viewport, while a
// zone is selected and the window is open.
func (s *Shell) heightWindow(gtx layout.Context) layout.Dimensions {
	p := &s.Height
	if p.Zone == "" {
		return layout.Dimensions{}
	}
	return p.Window.Layout(gtx, s.Theme, "Altura da zona", func(gtx layout.Context) layout.Dimensions {
		row := func(a, b layout.Widget) layout.FlexChild {
			return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx, layout.Rigid(a), layout.Rigid(b))
			})
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(s.label(p.Zone)),
			layout.Rigid(s.label("Passo (PageUp/PageDown no viewport)")),
			layout.Rigid(s.field(&p.Step, strconv.Itoa(DefaultZStep))),
			row(s.button(&p.Up, "Subir"), s.button(&p.Down, "Descer")),
			layout.Rigid(s.label("Base: z do piso")),
			layout.Rigid(s.field(&p.Base, "z")),
			layout.Rigid(s.button(&p.SetBase, "Aplicar base")),
			layout.Rigid(s.label("Altura: topo = piso + altura")),
			layout.Rigid(s.field(&p.Height, "altura")),
			layout.Rigid(s.button(&p.SetHeight, "Aplicar altura")),
		)
	})
}

// ZArrow is the viewport's handle that raises and lowers the selected
// zone: an arrow from Base to Tip, in viewport pixels, pointing up the
// world's Z axis. The viewport receives the presses on it; ZArrow only
// draws it.
type ZArrow struct {
	Visible bool
	Base    f32.Point
	Tip     f32.Point
	// Active marks the arrow being dragged.
	Active bool
}

var (
	arrowColor  = color.NRGBA{R: 0x3C, G: 0x8C, B: 0xFF, A: 0xFF}
	arrowActive = color.NRGBA{R: 0xFF, G: 0xD0, B: 0x30, A: 0xFF}
)

// Layout paints the arrow: a shaft, a head at Tip and a dot at Base.
func (a ZArrow) Layout(gtx layout.Context) layout.Dimensions {
	if !a.Visible {
		return layout.Dimensions{}
	}
	c := arrowColor
	if a.Active {
		c = arrowActive
	}
	d := a.Tip.Sub(a.Base)
	n := length(d)
	if n < 1 {
		return layout.Dimensions{}
	}
	u := d.Div(n)
	side := f32.Pt(-u.Y, u.X)
	head := float32(gtx.Dp(14))
	neck := a.Tip.Sub(u.Mul(head))

	var shaft clip.Path
	shaft.Begin(gtx.Ops)
	shaft.MoveTo(a.Base)
	shaft.LineTo(neck)
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: shaft.End(), Width: float32(gtx.Dp(3))}.Op())

	var tip clip.Path
	tip.Begin(gtx.Ops)
	tip.MoveTo(a.Tip)
	tip.LineTo(neck.Add(side.Mul(head * 0.45)))
	tip.LineTo(neck.Sub(side.Mul(head * 0.45)))
	tip.Close()
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: tip.End()}.Op())

	r := float32(gtx.Dp(4))
	dot := clip.Ellipse{Min: a.Base.Sub(f32.Pt(r, r)).Round(), Max: a.Base.Add(f32.Pt(r, r)).Round()}
	paint.FillShape(gtx.Ops, c, dot.Op(gtx.Ops))
	return layout.Dimensions{}
}

func length(p f32.Point) float32 {
	return float32(math.Hypot(float64(p.X), float64(p.Y)))
}
