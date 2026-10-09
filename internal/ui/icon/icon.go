// Package icon draws vector icons from a subset of SVG with Gio paths.
//
// Documents are parsed once into a compact, pre-flattened list of
// move/line/cubic/close commands in viewBox space; Layout only scales them and
// emits Gio clip and paint ops, so painting allocates nothing beyond the ops.
package icon

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Icon is a parsed SVG drawn with Gio paths.
type Icon struct {
	// StrokeScale multiplies every stroke width (1 = as in the file).
	StrokeScale float32

	vbMin, vbSize f32.Point
	verbs         []verb
	pts           []f32.Point
	shapes        []shape
}

// paintKind selects how a fill or stroke is painted.
type paintKind uint8

const (
	paintNone paintKind = iota
	paintColor
	paintCurrent // the colour passed to Layout
	paintGradient
)

// stop is a gradient colour stop. The alpha of color includes stop-opacity;
// for a currentColor stop only that alpha is meaningful.
type stop struct {
	offset  float32
	color   color.NRGBA
	current bool
}

// paintSpec is a resolved fill or stroke paint.
type paintSpec struct {
	kind    paintKind
	color   color.NRGBA // paintColor
	opacity float32     // fill/stroke-opacity times the accumulated opacity
	// g1 and g2 span offsets 0 and 1 of a gradient, in viewBox space.
	g1, g2 f32.Point
	stops  []stop
}

// shape is one drawable element: a range of the icon's geometry and its paints.
type shape struct {
	v0, v1      int32 // verb range
	p0          int32 // index of the first point
	strokeWidth float32
	fill        paintSpec
	stroke      paintSpec
}

// xform maps viewBox coordinates to pixels.
type xform struct {
	s   float32
	off f32.Point
}

func (x xform) pt(p f32.Point) f32.Point {
	return f32.Pt(p.X*x.s+x.off.X, p.Y*x.s+x.off.Y)
}

// Layout paints the icon scaled uniformly from its viewBox into a size×size dp
// square at the current origin (centred if the viewBox is not square) and
// returns that square. currentColor resolves to c (alpha of c multiplies as
// well).
func (ic *Icon) Layout(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	px := gtx.Dp(size)
	dims := layout.Dimensions{Size: image.Pt(px, px)}
	if ic == nil || len(ic.shapes) == 0 || px <= 0 {
		return dims
	}
	fpx := float32(px)
	s := fpx / max(ic.vbSize.X, ic.vbSize.Y)
	x := xform{
		s: s,
		off: f32.Pt(
			(fpx-ic.vbSize.X*s)/2-ic.vbMin.X*s,
			(fpx-ic.vbSize.Y*s)/2-ic.vbMin.Y*s,
		),
	}
	for i := range ic.shapes {
		sh := &ic.shapes[i]
		fill := sh.fill.kind != paintNone
		width := sh.strokeWidth * ic.StrokeScale * s
		stroke := sh.stroke.kind != paintNone && width > 0
		if !fill && !stroke {
			continue
		}
		spec := ic.path(gtx.Ops, sh, x)
		if fill {
			ic.paint(gtx.Ops, clip.Outline{Path: spec}.Op(), &sh.fill, x, fpx, c)
		}
		if stroke {
			ic.paint(gtx.Ops, clip.Stroke{Path: spec, Width: width}.Op(), &sh.stroke, x, fpx, c)
		}
	}
	return dims
}

// path records the geometry of sh in pixel space.
func (ic *Icon) path(ops *op.Ops, sh *shape, x xform) clip.PathSpec {
	var p clip.Path
	p.Begin(ops)
	pts := ic.pts[sh.p0:]
	j := 0
	for _, v := range ic.verbs[sh.v0:sh.v1] {
		switch v {
		case verbMove:
			p.MoveTo(x.pt(pts[j]))
			j++
		case verbLine:
			p.LineTo(x.pt(pts[j]))
			j++
		case verbCube:
			p.CubeTo(x.pt(pts[j]), x.pt(pts[j+1]), x.pt(pts[j+2]))
			j += 3
		case verbClose:
			p.Close()
		}
	}
	return p.End()
}

// paint fills the clip area cl with ps. size is the icon square in pixels.
func (ic *Icon) paint(ops *op.Ops, cl clip.Op, ps *paintSpec, x xform, size float32, c color.NRGBA) {
	switch ps.kind {
	case paintColor:
		paint.FillShape(ops, withAlpha(ps.color, ps.opacity), cl)
	case paintCurrent:
		paint.FillShape(ops, withAlpha(c, ps.opacity), cl)
	case paintGradient:
		defer cl.Push(ops).Pop()
		fillGradient(ops, ps, x, size, c)
	}
}

// withAlpha multiplies the alpha of col by a.
func withAlpha(col color.NRGBA, a float32) color.NRGBA {
	if a < 1 {
		col.A = uint8(float32(col.A)*max(a, 0) + 0.5)
	}
	return col
}

// stopColor resolves a gradient stop for painting.
func stopColor(s *stop, opacity float32, c color.NRGBA) color.NRGBA {
	if s.current {
		c.A = uint8(float32(c.A)*float32(s.color.A)/0xff + 0.5)
		return withAlpha(c, opacity)
	}
	return withAlpha(s.color, opacity)
}

// gradient is a linear gradient in pixel space, painted inside the current
// clip. Gio's LinearGradientOp has two stops and clamps beyond them, so a
// gradient with more stops is painted one stop interval at a time.
type gradient struct {
	ops     *op.Ops
	ps      *paintSpec
	c       color.NRGBA
	g1, d   f32.Point // pixel position of offset 0 and the offset-1 vector
	u, n    f32.Point // unit axis and normal
	centre  f32.Point // the icon centre projected onto the axis
	reach   float32   // distance that covers the whole icon from the axis
	extent  float32   // half-length of the band polygons along the axis
	opacity float32
}

// fillGradient paints ps inside the current clip.
func fillGradient(ops *op.Ops, ps *paintSpec, x xform, size float32, c color.NRGBA) {
	g1, g2 := x.pt(ps.g1), x.pt(ps.g2)
	d := g2.Sub(g1)
	l := float32(math.Hypot(float64(d.X), float64(d.Y)))
	stops := ps.stops
	if l < 1e-4 {
		fillColor(ops, stopColor(&stops[len(stops)-1], ps.opacity, c))
		return
	}
	g := gradient{ops: ops, ps: ps, c: c, g1: g1, d: d, opacity: ps.opacity}
	g.u = d.Div(l)
	g.n = f32.Pt(-g.u.Y, g.u.X)
	mid := f32.Pt(size/2, size/2)
	g.centre = g1.Add(g.u.Mul(dot(mid.Sub(g1), g.u)))
	// Strokes may overhang the square; twice its size is ample.
	g.reach = abs(dot(mid.Sub(g1), g.n)) + 2*size
	g.extent = 2 * size

	// Each stop interval is painted as a band clipped along the gradient axis;
	// the outermost bands extend to infinity and rely on Gio's clamping.
	// Opaque bands reach bandOverlap pixels past their end, under the next
	// band, so the next band's anti-aliased edge blends against matching
	// colour instead of the background: no seam, and a hard edge still gets a
	// clean anti-aliased transition. Every pixel is painted at most twice,
	// which keeps Gio's faint coverage noise inside curved clips from adding
	// up. Translucent bands cannot overlap without doubling their alpha, so
	// they abut exactly.
	opaque := ps.opacity >= 1
	for i := range stops {
		s := &stops[i]
		a := s.color.A
		if s.current {
			a = uint8(int(a) * int(c.A) / 0xff)
		}
		if a != 0xff {
			opaque = false
			break
		}
	}
	overlap := float32(0)
	if opaque {
		overlap = bandOverlap / l
	}
	g.bands(overlap)
}

// bandOverlap is how far, in pixels, an opaque band extends under the next.
const bandOverlap = 2

// bands paints the stop intervals of non-zero length; overlap, in offset
// units, extends each band's far end.
func (g *gradient) bands(overlap float32) {
	stops := g.ps.stops
	n := 0
	for i := range len(stops) - 1 {
		if stops[i+1].offset > stops[i].offset {
			n++
		}
	}
	if n == 0 {
		// All stops share one offset: a hard edge between the end colours.
		first, last := &stops[0], &stops[len(stops)-1]
		st := g.pushBand(0, first.offset+overlap, true, false)
		fillColor(g.ops, stopColor(first, g.opacity, g.c))
		st.Pop()
		st = g.pushBand(first.offset, 0, false, true)
		fillColor(g.ops, stopColor(last, g.opacity, g.c))
		st.Pop()
		return
	}
	k := 0
	for i := range len(stops) - 1 {
		a, b := &stops[i], &stops[i+1]
		if b.offset <= a.offset {
			continue
		}
		loInf, hiInf := k == 0, k == n-1
		k++
		if loInf && hiInf {
			g.interval(a, b)
			continue
		}
		st := g.pushBand(a.offset, b.offset+overlap, loInf, hiInf)
		g.interval(a, b)
		st.Pop()
	}
}

// interval paints the gradient between stops a and b, clamped beyond them.
func (g *gradient) interval(a, b *stop) {
	paint.LinearGradientOp{
		Stop1:  g.g1.Add(g.d.Mul(a.offset)),
		Color1: stopColor(a, g.opacity, g.c),
		Stop2:  g.g1.Add(g.d.Mul(b.offset)),
		Color2: stopColor(b, g.opacity, g.c),
	}.Add(g.ops)
	paint.PaintOp{}.Add(g.ops)
}

// pushBand clips to the part of the plane whose gradient offset lies in
// [lo, hi]; loInf and hiInf drop the respective limit.
func (g *gradient) pushBand(lo, hi float32, loInf, hiInf bool) clip.Stack {
	a := g.g1.Add(g.d.Mul(lo))
	if loInf {
		a = g.centre.Sub(g.u.Mul(g.extent))
	}
	b := g.g1.Add(g.d.Mul(hi))
	if hiInf {
		b = g.centre.Add(g.u.Mul(g.extent))
	}
	w := g.n.Mul(g.reach)
	var p clip.Path
	p.Begin(g.ops)
	p.MoveTo(a.Add(w))
	p.LineTo(b.Add(w))
	p.LineTo(b.Sub(w))
	p.LineTo(a.Sub(w))
	p.Close()
	return clip.Outline{Path: p.End()}.Op().Push(g.ops)
}

// fillColor paints the current clip with a flat colour.
func fillColor(ops *op.Ops, col color.NRGBA) {
	paint.ColorOp{Color: col}.Add(ops)
	paint.PaintOp{}.Add(ops)
}

func dot(a, b f32.Point) float32 { return a.X*b.X + a.Y*b.Y }

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
