package ui

import (
	"cmp"
	"image"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"zonebuilder/internal/ui/icon"
)

const (
	windowTitle = unit.Dp(42)
	windowGrip  = unit.Dp(16)
	windowMinW  = unit.Dp(220)
	windowMinH  = unit.Dp(140)
)

// FloatWindow is a card drawn over the viewport like a window: its title
// bar drags it, the chevron on the left collapses it to the title bar, the
// × on the right closes it (Closed) and the grip in the bottom-right
// corner resizes it. It stays inside the area it is laid out in, and the
// pointer input over it never reaches what lies below.
type FloatWindow struct {
	// Closed hides the window until the caller opens it again.
	Closed    bool
	Collapsed bool
	// Width and Height are the starting size, Left and Top the starting
	// position (floatMargin when 0), in dp.
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
	sink      pointerSink
}

// Layout draws the window titled title, with ic before the title and body
// inside, in the area of gtx.Constraints.Max, and handles its moving,
// resizing, collapsing and closing.
func (w *FloatWindow) Layout(gtx layout.Context, s *Shell, ic *icon.Icon, title string, body layout.Widget) layout.Dimensions {
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
		w.pos = image.Pt(gtx.Dp(cmp.Or(w.Left, floatMargin)), gtx.Dp(cmp.Or(w.Top, floatMargin)))
		w.size = image.Pt(gtx.Dp(w.Width), gtx.Dp(w.Height))
	}
	// Queued events use the previous frame's title-bar and grip origins.
	// Keep those baselines fixed while draining the batch; updating the
	// geometry does not change coordinates already routed by Gio.
	framePos, frameSize := w.pos, w.size
	for {
		ev, ok := w.move.Update(gtx.Metric, gtx.Source, gesture.Both)
		if !ok {
			break
		}
		switch ev.Kind {
		case pointer.Press:
			w.moveFrom = ev.Position
		case pointer.Drag:
			w.pos = framePos.Add(ev.Position.Sub(w.moveFrom).Round())
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
			w.size = frameSize.Add(ev.Position.Sub(w.sizeFrom).Round())
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
	size := image.Pt(w.size.X, h)
	fillRRect(gtx, size, cardRadius, cardSurface, hairline)
	// Take every pointer event over the window, so none reaches the
	// viewport below; the controls laid out next sit on top of this area.
	w.sink.add(gtx, size)

	bar := clip.Rect{Max: image.Pt(w.size.X, titleH)}.Push(gtx.Ops)
	w.move.Add(gtx.Ops)
	pointer.CursorGrab.Add(gtx.Ops)
	bar.Pop()
	w.titleBar(gtx, s, ic, title, titleH)

	if !w.Collapsed {
		line := image.Rect(0, titleH-1, w.size.X, titleH)
		paint.FillShape(gtx.Ops, hairline, clip.Rect(line).Op())
		w.body(gtx, s, titleH, body)
		w.grip(gtx)
	}
	return layout.Dimensions{Size: size}
}

func (w *FloatWindow) titleBar(gtx layout.Context, s *Shell, ic *icon.Icon, title string, titleH int) {
	gtx.Constraints = layout.Exact(image.Pt(w.size.X, titleH))
	chevron := icon.ChevronDown
	if w.Collapsed {
		chevron = icon.ChevronRight
	}
	// The row is laid out with no minimum height, so the title is not
	// stretched to the bar and drawn at its top, and W centres it
	// vertically.
	layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, 0)
		return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(s.iconToggle(&w.collapse, chevron, false)),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(2), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ic.Layout(gtx, iconSize, accentText)
					})
				}),
				layout.Flexed(1, s.text(title, bodySize, font.SemiBold, textColor, 1)),
				layout.Rigid(s.iconToggle(&w.close, icon.X, false)),
			)
		})
	})
}

func (w *FloatWindow) body(gtx layout.Context, s *Shell, titleH int, body layout.Widget) {
	size := image.Pt(w.size.X, w.size.Y-titleH)
	defer op.Offset(image.Pt(0, titleH)).Push(gtx.Ops).Pop()
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(size)
	w.list.Axis = layout.Vertical
	s.scrollList(&w.list).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(6), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, body)
	})
}

// grip is the resize handle in the bottom-right corner: a triangle of 3
// dots.
func (w *FloatWindow) grip(gtx layout.Context) {
	g := gtx.Dp(windowGrip)
	defer op.Offset(w.size.Sub(image.Pt(g, g))).Push(gtx.Ops).Pop()
	area := clip.Rect{Max: image.Pt(g, g)}.Push(gtx.Ops)
	w.resize.Add(gtx.Ops)
	pointer.CursorNorthWestResize.Add(gtx.Ops)
	area.Pop()
	r := gtx.Dp(1.5)
	step := float32(gtx.Dp(4))
	end := float32(g - gtx.Dp(5))
	for _, d := range []f32.Point{{X: 0, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: -1}} {
		c := f32.Pt(end+d.X*step, end+d.Y*step).Round()
		dot := clip.Ellipse{Min: c.Sub(image.Pt(r, r)), Max: c.Add(image.Pt(r, r))}
		paint.FillShape(gtx.Ops, faintText, dot.Op(gtx.Ops))
	}
}
