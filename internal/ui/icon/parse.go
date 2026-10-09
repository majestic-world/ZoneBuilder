package icon

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"image/color"
	"io"
	"math"
	"strconv"
	"strings"

	"gioui.org/f32"
)

// paintRef is a fill or stroke value as written, before gradient references
// are resolved. When url is set, kind and color hold the fallback paint.
type paintRef struct {
	kind  paintKind
	color color.NRGBA
	url   string
}

// style is the inherited presentation state of an element.
type style struct {
	fill, stroke               paintRef
	fillOpacity, strokeOpacity float32
	// opacity accumulates the opacity of the element and its ancestors. Group
	// opacity is approximated by multiplying it into every descendant paint.
	opacity     float32
	strokeWidth float32
}

func defaultStyle() style {
	return style{
		fill:          paintRef{kind: paintColor, color: color.NRGBA{A: 0xff}},
		stroke:        paintRef{kind: paintNone},
		fillOpacity:   1,
		strokeOpacity: 1,
		opacity:       1,
		strokeWidth:   1,
	}
}

// coord is a gradient coordinate that may be a percentage.
type coord struct {
	v   float64
	pct bool
}

// gradDef is a parsed <linearGradient>.
type gradDef struct {
	x1, y1, x2, y2 coord
	userSpace      bool
	stops          []stop
	srgb           []stop // stops subdivided for sRGB interpolation, built on demand
}

// frame is one open element on the parser stack.
type frame struct {
	st     style
	render bool     // shapes in this subtree are drawn (false inside <defs>)
	grad   *gradDef // set for <linearGradient>, to collect its stops
}

// pending remembers the unresolved paints of a shape.
type pending struct {
	fill, stroke paintRef
}

type parser struct {
	ic    *Icon
	b     builder
	grads map[string]*gradDef
	pend  []pending
}

// Parse parses an SVG document of the supported subset.
func Parse(data []byte) (*Icon, error) {
	p := parser{ic: &Icon{StrokeScale: 1}, grads: map[string]*gradDef{}}
	if err := p.parse(data); err != nil {
		return nil, fmt.Errorf("icon: %w", err)
	}
	return p.ic, nil
}

func (p *parser) parse(data []byte) error {
	d := xml.NewDecoder(bytes.NewReader(data))
	var stack []frame
	rooted := false
	for {
		tok, err := d.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			if !rooted {
				if name != "svg" {
					return fmt.Errorf("root element is <%s>, want <svg>", name)
				}
				if err := p.viewBox(t); err != nil {
					return err
				}
				rooted = true
				st := defaultStyle()
				if err := applyStyle(&st, 1, t); err != nil {
					return err
				}
				stack = append(stack, frame{st: st, render: true})
				continue
			}
			parent := stack[len(stack)-1]
			f := frame{st: parent.st, render: parent.render}
			switch name {
			case "svg", "g", "a":
				// Nested <svg> is treated as a plain group.
			case "defs":
				f.render = false
			case "linearGradient":
				g, err := parseGradient(t)
				if err != nil {
					return err
				}
				if id, ok := attr(t, "id"); ok {
					p.grads[id] = g
				}
				f.grad = g
				f.render = false
			case "stop":
				if parent.grad == nil {
					if err := d.Skip(); err != nil {
						return err
					}
					continue
				}
				s, err := parseStop(t)
				if err != nil {
					return err
				}
				g := parent.grad
				// Offsets are clamped and never decrease (SVG 1.1 §13.2.4).
				if n := len(g.stops); n > 0 && s.offset < g.stops[n-1].offset {
					s.offset = g.stops[n-1].offset
				}
				g.stops = append(g.stops, s)
				f.render = false
			case "path", "line", "polyline", "polygon", "rect", "circle", "ellipse":
				if err := applyStyle(&f.st, parent.st.opacity, t); err != nil {
					return err
				}
				if f.render {
					if err := p.shape(t, &f.st); err != nil {
						return fmt.Errorf("<%s>: %w", name, err)
					}
				}
				// Children of shapes (<title>, <desc>, animations) are ignored.
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			default:
				// Unknown or unsupported elements are ignored with their subtree.
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			if err := applyStyle(&f.st, parent.st.opacity, t); err != nil {
				return err
			}
			stack = append(stack, f)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if !rooted {
		return errors.New("no <svg> element")
	}
	p.ic.verbs, p.ic.pts = p.b.verbs, p.b.pts
	for i := range p.ic.shapes {
		sh := &p.ic.shapes[i]
		sh.fill = p.resolve(p.pend[i].fill, sh.fill.opacity, sh)
		sh.stroke = p.resolve(p.pend[i].stroke, sh.stroke.opacity, sh)
	}
	return nil
}

// viewBox reads the root's viewBox, falling back to width and height.
func (p *parser) viewBox(t xml.StartElement) error {
	if vb, ok := attr(t, "viewBox"); ok {
		var n [4]float64
		sc := scanner{s: vb}
		if err := sc.numbers(n[:]); err != nil {
			return fmt.Errorf("viewBox: %w", err)
		}
		if n[2] <= 0 || n[3] <= 0 {
			return fmt.Errorf("viewBox %q has no area", vb)
		}
		p.ic.vbMin = f32.Pt(float32(n[0]), float32(n[1]))
		p.ic.vbSize = f32.Pt(float32(n[2]), float32(n[3]))
		return nil
	}
	w, errW := lengthAttr(t, "width", 0)
	h, errH := lengthAttr(t, "height", 0)
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return errors.New("<svg> needs a viewBox or a numeric width and height")
	}
	p.ic.vbSize = f32.Pt(float32(w), float32(h))
	return nil
}

// shape appends the geometry of a basic shape or path and records its paint.
func (p *parser) shape(t xml.StartElement, st *style) error {
	v0, p0 := len(p.b.verbs), len(p.b.pts)
	b := &p.b
	switch t.Name.Local {
	case "path":
		d, _ := attr(t, "d")
		if err := parsePathData(b, d); err != nil {
			return err
		}
	case "line":
		var n [4]float64
		if err := lengths(t, []string{"x1", "y1", "x2", "y2"}, n[:]); err != nil {
			return err
		}
		b.moveTo(vec{n[0], n[1]})
		b.lineTo(vec{n[2], n[3]})
	case "polyline", "polygon":
		pts, _ := attr(t, "points")
		sc := scanner{s: pts}
		first := true
		for sc.atNumber() {
			x, err := sc.number()
			if err != nil {
				return err
			}
			if !sc.atNumber() {
				break // an odd trailing coordinate is ignored
			}
			y, err := sc.number()
			if err != nil {
				return err
			}
			if first {
				b.moveTo(vec{x, y})
				first = false
			} else {
				b.lineTo(vec{x, y})
			}
		}
		if !first && t.Name.Local == "polygon" {
			b.close()
		}
	case "rect":
		var n [4]float64
		if err := lengths(t, []string{"x", "y", "width", "height"}, n[:]); err != nil {
			return err
		}
		rx, errX := lengthAttr(t, "rx", -1)
		ry, errY := lengthAttr(t, "ry", -1)
		if errX != nil || errY != nil {
			return errors.Join(errX, errY)
		}
		rect(b, n[0], n[1], n[2], n[3], rx, ry)
	case "circle":
		var n [3]float64
		if err := lengths(t, []string{"cx", "cy", "r"}, n[:]); err != nil {
			return err
		}
		ellipse(b, n[0], n[1], n[2], n[2])
	case "ellipse":
		var n [3]float64
		if err := lengths(t, []string{"cx", "cy", "rx"}, n[:]); err != nil {
			return err
		}
		ry, err := lengthAttr(t, "ry", -1)
		if err != nil {
			return err
		}
		if ry < 0 {
			ry = n[2] // SVG 2 "auto"
		}
		ellipse(b, n[0], n[1], n[2], ry)
	}
	if len(b.verbs) == v0 {
		return nil // nothing to draw
	}
	p.ic.shapes = append(p.ic.shapes, shape{
		v0:          int32(v0),
		v1:          int32(len(b.verbs)),
		p0:          int32(p0),
		strokeWidth: st.strokeWidth,
		// The opacities ride along until the paints are resolved.
		fill:   paintSpec{opacity: st.fillOpacity * st.opacity},
		stroke: paintSpec{opacity: st.strokeOpacity * st.opacity},
	})
	p.pend = append(p.pend, pending{fill: st.fill, stroke: st.stroke})
	return nil
}

// kappa places cubic control points that approximate a quarter ellipse.
const kappa = 0.5522847498307936

// rect appends a rectangle with optional rounded corners. Negative rx/ry mean
// "not specified" and follow the SVG auto rules.
func rect(b *builder, x, y, w, h, rx, ry float64) {
	if w <= 0 || h <= 0 {
		return
	}
	switch {
	case rx < 0 && ry < 0:
		rx, ry = 0, 0
	case rx < 0:
		rx = ry
	case ry < 0:
		ry = rx
	}
	rx = math.Min(rx, w/2)
	ry = math.Min(ry, h/2)
	if rx <= 0 || ry <= 0 {
		b.moveTo(vec{x, y})
		b.lineTo(vec{x + w, y})
		b.lineTo(vec{x + w, y + h})
		b.lineTo(vec{x, y + h})
		b.close()
		return
	}
	kx, ky := rx*kappa, ry*kappa
	r, bt := x+w, y+h
	b.moveTo(vec{x + rx, y})
	b.lineTo(vec{r - rx, y})
	b.cubeTo(vec{r - rx + kx, y}, vec{r, y + ry - ky}, vec{r, y + ry})
	b.lineTo(vec{r, bt - ry})
	b.cubeTo(vec{r, bt - ry + ky}, vec{r - rx + kx, bt}, vec{r - rx, bt})
	b.lineTo(vec{x + rx, bt})
	b.cubeTo(vec{x + rx - kx, bt}, vec{x, bt - ry + ky}, vec{x, bt - ry})
	b.lineTo(vec{x, y + ry})
	b.cubeTo(vec{x, y + ry - ky}, vec{x + rx - kx, y}, vec{x + rx, y})
	b.close()
}

// ellipse appends an axis-aligned ellipse drawn clockwise from its right end.
func ellipse(b *builder, cx, cy, rx, ry float64) {
	if rx <= 0 || ry <= 0 {
		return
	}
	kx, ky := rx*kappa, ry*kappa
	b.moveTo(vec{cx + rx, cy})
	b.cubeTo(vec{cx + rx, cy + ky}, vec{cx + kx, cy + ry}, vec{cx, cy + ry})
	b.cubeTo(vec{cx - kx, cy + ry}, vec{cx - rx, cy + ky}, vec{cx - rx, cy})
	b.cubeTo(vec{cx - rx, cy - ky}, vec{cx - kx, cy - ry}, vec{cx, cy - ry})
	b.cubeTo(vec{cx + kx, cy - ry}, vec{cx + rx, cy - ky}, vec{cx + rx, cy})
	b.close()
}

// resolve turns a paint reference into a drawable paint for shape sh.
func (p *parser) resolve(ref paintRef, opacity float32, sh *shape) paintSpec {
	ps := paintSpec{kind: ref.kind, color: ref.color, opacity: opacity}
	if ref.url == "" {
		return ps
	}
	g := p.grads[ref.url]
	if g == nil || len(g.stops) == 0 {
		return ps // fallback paint, "none" unless one was given
	}
	if len(g.stops) == 1 {
		s := g.stops[0]
		return solidStop(s, opacity)
	}
	vb := p.ic.vbSize
	var g1, g2 vec
	if g.userSpace {
		// Percentages are relative to the viewBox.
		res := func(c coord, size float32) float64 {
			if c.pct {
				return c.v * float64(size)
			}
			return c.v
		}
		g1 = vec{res(g.x1, vb.X), res(g.y1, vb.Y)}
		g2 = vec{res(g.x2, vb.X), res(g.y2, vb.Y)}
	} else {
		lo, hi, ok := p.bounds(sh)
		bw, bh := hi.x-lo.x, hi.y-lo.y
		if !ok || bw <= 0 || bh <= 0 {
			// A bounding-box gradient on a box without area paints nothing.
			return paintSpec{kind: paintNone}
		}
		u1 := vec{g.x1.v, g.y1.v}
		d := vec{g.x2.v, g.y2.v}.sub(u1)
		dd := d.x*d.x + d.y*d.y
		g1 = vec{lo.x + u1.x*bw, lo.y + u1.y*bh}
		if dd == 0 {
			g2 = g1
		} else {
			// The gradient parameter t is affine in user space: t(x) = a·(x-g1)
			// with a = (d.x/bw, d.y/bh)/|d|². Its isolines stay parallel, so the
			// equivalent user-space vector is g1 + a/|a|², which keeps the
			// skew of non-square boxes exact.
			a := vec{d.x / bw / dd, d.y / bh / dd}
			g2 = g1.add(a.scale(1 / (a.x*a.x + a.y*a.y)))
		}
	}
	if g1 == g2 {
		// SVG paints a degenerate gradient with its last stop.
		return solidStop(g.stops[len(g.stops)-1], opacity)
	}
	stops := g.stops
	if opacity >= 1 && opaqueStops(stops) {
		if g.srgb == nil {
			g.srgb = srgbStops(stops)
		}
		stops = g.srgb
	}
	return paintSpec{
		kind:    paintGradient,
		opacity: opacity,
		g1:      g1.f32(),
		g2:      g2.f32(),
		stops:   stops,
	}
}

// opaqueStops reports whether all stops are fixed, fully opaque colours.
func opaqueStops(stops []stop) bool {
	for _, s := range stops {
		if s.current || s.color.A != 0xff {
			return false
		}
	}
	return true
}

// srgbStops subdivides the intervals of opaque stops so that Gio, which
// interpolates gradients in linear light, follows SVG's sRGB interpolation.
// Each interval gets one sub-interval per 24 levels of its largest channel
// change; the piecewise-linear error then stays within a few levels.
func srgbStops(stops []stop) []stop {
	out := []stop{stops[0]}
	for i := 1; i < len(stops); i++ {
		a, b := stops[i-1], stops[i]
		if b.offset > a.offset {
			delta := max(absDiff(a.color.R, b.color.R), absDiff(a.color.G, b.color.G), absDiff(a.color.B, b.color.B))
			n := min(max((delta+23)/24, 1), 12)
			for k := 1; k < n; k++ {
				t := float32(k) / float32(n)
				out = append(out, stop{
					offset: a.offset + (b.offset-a.offset)*t,
					color: color.NRGBA{
						R: lerp8(a.color.R, b.color.R, t),
						G: lerp8(a.color.G, b.color.G, t),
						B: lerp8(a.color.B, b.color.B, t),
						A: 0xff,
					},
				})
			}
		}
		out = append(out, b)
	}
	return out
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

func lerp8(a, b uint8, t float32) uint8 {
	return uint8(float32(a) + (float32(b)-float32(a))*t + 0.5)
}

// solidStop is a flat paint with the colour of a single stop.
func solidStop(s stop, opacity float32) paintSpec {
	ps := paintSpec{kind: paintColor, color: s.color, opacity: opacity}
	if s.current {
		ps.kind = paintCurrent
		ps.opacity *= float32(s.color.A) / 0xff
	}
	return ps
}

// bounds returns the tight geometry bounding box of a shape in viewBox space.
func (p *parser) bounds(sh *shape) (lo, hi vec, ok bool) {
	lo = vec{math.Inf(1), math.Inf(1)}
	hi = vec{math.Inf(-1), math.Inf(-1)}
	add := func(v vec) {
		lo = vec{math.Min(lo.x, v.x), math.Min(lo.y, v.y)}
		hi = vec{math.Max(hi.x, v.x), math.Max(hi.y, v.y)}
	}
	pts := p.b.pts[sh.p0:]
	j := 0
	var cur vec
	for _, v := range p.b.verbs[sh.v0:sh.v1] {
		switch v {
		case verbMove, verbLine:
			cur = toVec(pts[j])
			add(cur)
			j++
		case verbCube:
			c1, c2, end := toVec(pts[j]), toVec(pts[j+1]), toVec(pts[j+2])
			add(end)
			// Add the curve's extrema, where a coordinate's derivative is zero.
			for axis := range 2 {
				for _, t := range cubicExtrema(comp(cur, axis), comp(c1, axis), comp(c2, axis), comp(end, axis)) {
					if t > 0 && t < 1 {
						add(cubicAt(cur, c1, c2, end, t))
					}
				}
			}
			cur = end
			j += 3
		}
	}
	return lo, hi, lo.x <= hi.x
}

func toVec(p f32.Point) vec { return vec{float64(p.X), float64(p.Y)} }

func comp(v vec, axis int) float64 {
	if axis == 0 {
		return v.x
	}
	return v.y
}

func cubicAt(p0, p1, p2, p3 vec, t float64) vec {
	u := 1 - t
	return p0.scale(u * u * u).add(p1.scale(3 * u * u * t)).add(p2.scale(3 * u * t * t)).add(p3.scale(t * t * t))
}

// cubicExtrema returns the parameters where the derivative of a 1-D cubic
// Bézier is zero. Unused results are NaN.
func cubicExtrema(p0, p1, p2, p3 float64) [2]float64 {
	// B'(t)/3 = a t² + b t + c.
	a := -p0 + 3*p1 - 3*p2 + p3
	b := 2 * (p0 - 2*p1 + p2)
	c := p1 - p0
	r := [2]float64{math.NaN(), math.NaN()}
	if math.Abs(a) < 1e-12 {
		if b != 0 {
			r[0] = -c / b
		}
		return r
	}
	disc := b*b - 4*a*c
	if disc < 0 {
		return r
	}
	sq := math.Sqrt(disc)
	r[0] = (-b + sq) / (2 * a)
	r[1] = (-b - sq) / (2 * a)
	return r
}

// parseGradient reads the attributes of a <linearGradient>.
func parseGradient(t xml.StartElement) (*gradDef, error) {
	g := &gradDef{x2: coord{v: 1, pct: true}}
	for _, a := range t.Attr {
		var dst *coord
		switch a.Name.Local {
		case "x1":
			dst = &g.x1
		case "y1":
			dst = &g.y1
		case "x2":
			dst = &g.x2
		case "y2":
			dst = &g.y2
		case "gradientUnits":
			g.userSpace = strings.TrimSpace(a.Value) == "userSpaceOnUse"
		}
		if dst != nil {
			v, pct, err := number(a.Value)
			if err != nil {
				return nil, fmt.Errorf("<linearGradient %s>: %w", a.Name.Local, err)
			}
			*dst = coord{v: v, pct: pct}
		}
	}
	return g, nil
}

// parseStop reads a <stop>, from attributes and its style attribute.
func parseStop(t xml.StartElement) (stop, error) {
	s := stop{color: color.NRGBA{A: 0xff}}
	alpha := float32(1)
	set := func(name, value string) error {
		value = strings.TrimSpace(value)
		switch name {
		case "offset":
			v, _, err := number(value)
			if err != nil {
				return fmt.Errorf("<stop offset>: %w", err)
			}
			s.offset = float32(math.Max(0, math.Min(1, v)))
		case "stop-color":
			ref, err := parsePaint(value)
			if err != nil || ref.url != "" || ref.kind == paintNone {
				return fmt.Errorf("<stop stop-color=%q> is not a colour", value)
			}
			s.color = ref.color
			s.current = ref.kind == paintCurrent
			if s.current {
				s.color = color.NRGBA{A: 0xff}
			}
		case "stop-opacity":
			v, err := opacityValue(value)
			if err != nil {
				return err
			}
			alpha = v
		}
		return nil
	}
	if err := eachDecl(t, set); err != nil {
		return s, err
	}
	s.color.A = uint8(float32(s.color.A)*alpha + 0.5)
	return s, nil
}

// applyStyle applies an element's presentation attributes and style
// declarations to st. parentOpacity is the accumulated opacity of the parent.
func applyStyle(st *style, parentOpacity float32, t xml.StartElement) error {
	own := float32(1)
	err := eachDecl(t, func(name, value string) error {
		value = strings.TrimSpace(value)
		if value == "inherit" {
			return nil
		}
		var err error
		switch name {
		case "fill":
			st.fill, err = parsePaint(value)
		case "stroke":
			st.stroke, err = parsePaint(value)
		case "fill-opacity":
			st.fillOpacity, err = opacityValue(value)
		case "stroke-opacity":
			st.strokeOpacity, err = opacityValue(value)
		case "opacity":
			own, err = opacityValue(value)
		case "stroke-width":
			var v float64
			v, err = parseLength(value)
			st.strokeWidth = float32(math.Max(0, v))
		}
		if err != nil {
			return fmt.Errorf("<%s %s=%q>: %w", t.Name.Local, name, value, err)
		}
		return nil
	})
	st.opacity = parentOpacity * own
	return err
}

// eachDecl calls fn for every attribute of t, then for every declaration of
// its style attribute, which takes precedence over presentation attributes.
func eachDecl(t xml.StartElement, fn func(name, value string) error) error {
	styleAttr := ""
	for _, a := range t.Attr {
		if a.Name.Local == "style" {
			styleAttr = a.Value
			continue
		}
		if err := fn(a.Name.Local, a.Value); err != nil {
			return err
		}
	}
	for decl := range strings.SplitSeq(styleAttr, ";") {
		name, value, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		if err := fn(strings.TrimSpace(name), strings.TrimSpace(value)); err != nil {
			return err
		}
	}
	return nil
}

// parsePaint parses none, currentColor, a colour, or url(#id) with an
// optional fallback colour.
func parsePaint(s string) (paintRef, error) {
	s = strings.TrimSpace(s)
	if rest, ok := strings.CutPrefix(s, "url("); ok {
		id, tail, ok := strings.Cut(rest, ")")
		if !ok {
			return paintRef{}, fmt.Errorf("unterminated url in %q", s)
		}
		id = strings.Trim(strings.TrimSpace(id), `"'`)
		id, ok = strings.CutPrefix(id, "#")
		if !ok {
			return paintRef{}, fmt.Errorf("only local url(#id) references are supported: %q", s)
		}
		ref := paintRef{kind: paintNone}
		if tail = strings.TrimSpace(tail); tail != "" {
			fb, err := parsePaint(tail)
			if err != nil {
				return paintRef{}, err
			}
			ref = fb
		}
		ref.url = id
		return ref, nil
	}
	switch strings.ToLower(s) {
	case "none", "transparent":
		return paintRef{kind: paintNone}, nil
	case "currentcolor":
		return paintRef{kind: paintCurrent}, nil
	case "black":
		return paintRef{kind: paintColor, color: color.NRGBA{A: 0xff}}, nil
	case "white":
		return paintRef{kind: paintColor, color: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}}, nil
	}
	if hex, ok := strings.CutPrefix(s, "#"); ok {
		c, err := hexColor(hex)
		if err != nil {
			return paintRef{}, err
		}
		return paintRef{kind: paintColor, color: c}, nil
	}
	return paintRef{}, fmt.Errorf("unsupported paint %q", s)
}

// hexColor parses rgb, rgba, rrggbb or rrggbbaa hex digits.
func hexColor(h string) (color.NRGBA, error) {
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("bad colour #%s", h)
	}
	nib := func(shift uint) uint8 { return uint8((v>>shift)&0xf) * 0x11 }
	byt := func(shift uint) uint8 { return uint8(v >> shift) }
	switch len(h) {
	case 3:
		return color.NRGBA{R: nib(8), G: nib(4), B: nib(0), A: 0xff}, nil
	case 4:
		return color.NRGBA{R: nib(12), G: nib(8), B: nib(4), A: nib(0)}, nil
	case 6:
		return color.NRGBA{R: byt(16), G: byt(8), B: byt(0), A: 0xff}, nil
	case 8:
		return color.NRGBA{R: byt(24), G: byt(16), B: byt(8), A: byt(0)}, nil
	}
	return color.NRGBA{}, fmt.Errorf("bad colour #%s", h)
}

// opacityValue parses a number or percentage clamped to [0, 1].
func opacityValue(s string) (float32, error) {
	v, _, err := number(s)
	if err != nil {
		return 0, err
	}
	return float32(math.Max(0, math.Min(1, v))), nil
}

// number parses a plain number or a percentage, which is returned as a
// fraction with pct set.
func number(s string) (v float64, pct bool, err error) {
	s = strings.TrimSpace(s)
	if rest, ok := strings.CutSuffix(s, "%"); ok {
		s, pct = rest, true
	}
	v, err = strconv.ParseFloat(s, 64)
	if pct {
		v /= 100
	}
	return v, pct, err
}

// parseLength parses a user-space length, allowing a "px" suffix.
func parseLength(s string) (float64, error) {
	s = strings.TrimSuffix(strings.TrimSpace(s), "px")
	return strconv.ParseFloat(s, 64)
}

// lengthAttr returns the length attribute name of t, or def when absent.
func lengthAttr(t xml.StartElement, name string, def float64) (float64, error) {
	s, ok := attr(t, name)
	if !ok {
		return def, nil
	}
	v, err := parseLength(s)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not a length", name, s)
	}
	return v, nil
}

// lengths reads the named length attributes into dst; absent ones are 0.
func lengths(t xml.StartElement, names []string, dst []float64) error {
	for i, name := range names {
		v, err := lengthAttr(t, name, 0)
		if err != nil {
			return err
		}
		dst[i] = v
	}
	return nil
}

// attr returns the value of the attribute with the given local name.
func attr(t xml.StartElement, name string) (string, bool) {
	for _, a := range t.Attr {
		if a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}
