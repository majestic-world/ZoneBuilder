package ui

import (
	"cmp"
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

var (
	windowBackground = color.NRGBA{R: 0x2A, G: 0x2B, B: 0x2F, A: 0xF2}
	windowTitleBar   = color.NRGBA{R: 0x1F, G: 0x20, B: 0x23, A: 0xF2}
	windowBorder     = color.NRGBA{R: 0x4A, G: 0x4C, B: 0x52, A: 0xFF}
)

const (
	windowTitle  = unit.Dp(30)
	windowGrip   = unit.Dp(16)
	windowMinW   = unit.Dp(200)
	windowMinH   = unit.Dp(120)
	windowMargin = unit.Dp(12)
)

// FloatWindow is a window drawn over the viewport: its title bar drags it,
// the arrow on the left collapses it to the title bar, the × on the right
// closes it (Closed) and the grip in the bottom-right corner resizes it.
// It stays inside the area it is laid out in, and the pointer input over
// it never reaches what lies below.
type FloatWindow struct {
	// Closed hides the window until the caller opens it again.
	Closed    bool
	Collapsed bool
	// Width and Height are the starting size, Left and Top the starting
	// position (windowMargin when 0), in dp.
	Width, Height, Left, Top unit.Dp

	placed    bool
	pos, size image.Point
	move      gesture.Drag
	resize    gesture.Drag
	moveFrom  f32.Point
	sizeFrom  f32.Point
	collapse  widget.Clickable
	close     widget.Clickable
	list      widget.List
}

// Layout draws the window titled title with body inside, in the area of
// gtx.Constraints.Max, and handles its moving, resizing, collapsing and
// closing.
func (w *FloatWindow) Layout(gtx layout.Context, th *material.Theme, title string, body layout.Widget) layout.Dimensions {
	if w.collapse.Clicked(gtx) {
		w.Collapsed = !w.Collapsed
	}
	if w.close.Clicked(gtx) {
		w.Closed = true
	}
	if w.Closed {
		return layout.Dimensions{}
	}
	area := gtx.Constraints.Max
	titleH := gtx.Dp(windowTitle)
	if !w.placed {
		w.placed = true
		w.pos = image.Pt(gtx.Dp(cmp.Or(w.Left, windowMargin)), gtx.Dp(cmp.Or(w.Top, windowMargin)))
		w.size = image.Pt(gtx.Dp(w.Width), gtx.Dp(w.Height))
	}
	for {
		ev, ok := w.move.Update(gtx.Metric, gtx.Source, gesture.Both)
		if !ok {
			break
		}
		switch ev.Kind {
		case pointer.Press:
			w.moveFrom = ev.Position
		case pointer.Drag:
			// The title bar moves with the window, so the pointer stays at
			// the press position relative to it once the move is applied.
			w.pos = w.pos.Add(ev.Position.Sub(w.moveFrom).Round())
		}
	}
	for {
		ev, ok := w.resize.Update(gtx.Metric, gtx.Source, gesture.Both)
		if !ok {
			break
		}
		switch ev.Kind {
		case pointer.Press:
			w.sizeFrom = ev.Position
		case pointer.Drag:
			w.size = w.size.Add(ev.Position.Sub(w.sizeFrom).Round())
		}
	}
	w.size.X = max(w.size.X, gtx.Dp(windowMinW))
	w.size.Y = max(w.size.Y, gtx.Dp(windowMinH))
	w.size.X = min(w.size.X, area.X)
	w.size.Y = min(w.size.Y, area.Y)
	h := w.size.Y
	if w.Collapsed {
		h = titleH
	}
	w.pos.X = max(0, min(w.pos.X, area.X-w.size.X))
	w.pos.Y = max(0, min(w.pos.Y, area.Y-h))

	defer op.Offset(w.pos).Push(gtx.Ops).Pop()
	r := image.Rectangle{Max: image.Pt(w.size.X, h)}
	radius := gtx.Dp(4)
	paint.FillShape(gtx.Ops, windowBackground, clip.UniformRRect(r, radius).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, windowTitleBar, clip.RRect{Rect: image.Rect(0, 0, w.size.X, titleH), NW: radius, NE: radius}.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, windowBorder, clip.Stroke{Path: clip.UniformRRect(r, radius).Path(gtx.Ops), Width: 1}.Op())

	// Take every pointer event over the window, so none reaches the
	// viewport below; the controls laid out next sit on top of this area.
	block := clip.Rect(r).Push(gtx.Ops)
	event.Op(gtx.Ops, w)
	block.Pop()
	for {
		if _, ok := gtx.Event(pointer.Filter{
			Target:  w,
			Kinds:   pointer.Press | pointer.Release | pointer.Drag | pointer.Move | pointer.Scroll,
			ScrollX: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20},
			ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20},
		}); !ok {
			break
		}
	}

	bar := clip.Rect{Max: image.Pt(w.size.X, titleH)}.Push(gtx.Ops)
	w.move.Add(gtx.Ops)
	pointer.CursorAllScroll.Add(gtx.Ops)
	bar.Pop()
	w.titleBar(gtx, th, title, titleH)

	if !w.Collapsed {
		w.body(gtx, th, titleH, body)
		w.grip(gtx)
	}
	return layout.Dimensions{Size: r.Max}
}

func (w *FloatWindow) titleBar(gtx layout.Context, th *material.Theme, title string, titleH int) {
	gtx.Constraints = layout.Exact(image.Pt(w.size.X, titleH))
	layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return w.collapse.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return iconBox(gtx, titleH, func(gtx layout.Context, c f32.Point, s float32) {
					// A triangle pointing down when open, right when
					// collapsed.
					var p clip.Path
					p.Begin(gtx.Ops)
					if w.Collapsed {
						p.MoveTo(c.Add(f32.Pt(-s*0.35, -s*0.5)))
						p.LineTo(c.Add(f32.Pt(s*0.55, 0)))
						p.LineTo(c.Add(f32.Pt(-s*0.35, s*0.5)))
					} else {
						p.MoveTo(c.Add(f32.Pt(-s*0.5, -s*0.35)))
						p.LineTo(c.Add(f32.Pt(s*0.5, -s*0.35)))
						p.LineTo(c.Add(f32.Pt(0, s*0.55)))
					}
					p.Close()
					paint.FillShape(gtx.Ops, panelText, clip.Outline{Path: p.End()}.Op())
				})
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body1(th, title)
			lbl.Color = panelText
			lbl.Alignment = text.Middle
			lbl.MaxLines = 1
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return w.close.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return iconBox(gtx, titleH, func(gtx layout.Context, c f32.Point, s float32) {
					var p clip.Path
					p.Begin(gtx.Ops)
					p.MoveTo(c.Add(f32.Pt(-s*0.45, -s*0.45)))
					p.LineTo(c.Add(f32.Pt(s*0.45, s*0.45)))
					p.MoveTo(c.Add(f32.Pt(s*0.45, -s*0.45)))
					p.LineTo(c.Add(f32.Pt(-s*0.45, s*0.45)))
					paint.FillShape(gtx.Ops, panelText, clip.Stroke{Path: p.End(), Width: float32(gtx.Dp(1.5))}.Op())
				})
			})
		}),
	)
}

// iconBox is a square of side size whose icon draw paints around its
// centre c, s being the icon's size.
func iconBox(gtx layout.Context, size int, draw func(gtx layout.Context, c f32.Point, s float32)) layout.Dimensions {
	draw(gtx, f32.Pt(float32(size)/2, float32(size)/2), float32(gtx.Dp(10)))
	return layout.Dimensions{Size: image.Pt(size, size)}
}

func (w *FloatWindow) body(gtx layout.Context, th *material.Theme, titleH int, body layout.Widget) {
	size := image.Pt(w.size.X, w.size.Y-titleH)
	defer op.Offset(image.Pt(0, titleH)).Push(gtx.Ops).Pop()
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(size)
	w.list.Axis = layout.Vertical
	layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.List(th, &w.list).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
			return body(gtx)
		})
	})
}

// grip is the resize handle in the bottom-right corner: 3 diagonal lines.
func (w *FloatWindow) grip(gtx layout.Context) {
	g := gtx.Dp(windowGrip)
	defer op.Offset(w.size.Sub(image.Pt(g, g))).Push(gtx.Ops).Pop()
	area := clip.Rect{Max: image.Pt(g, g)}.Push(gtx.Ops)
	w.resize.Add(gtx.Ops)
	pointer.CursorNorthWestResize.Add(gtx.Ops)
	area.Pop()
	var p clip.Path
	p.Begin(gtx.Ops)
	n := float32(g)
	for _, k := range []float32{0.3, 0.55, 0.8} {
		p.MoveTo(f32.Pt(n*(1-k), n-2))
		p.LineTo(f32.Pt(n-2, n*(1-k)))
	}
	paint.FillShape(gtx.Ops, dimText, clip.Stroke{Path: p.End(), Width: 1}.Op())
}
