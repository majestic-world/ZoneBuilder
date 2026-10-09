package icon

import (
	"fmt"
	"math"
	"strconv"

	"gioui.org/f32"
)

// verb is one drawing command of the pre-flattened geometry.
type verb uint8

const (
	verbMove  verb = iota // consumes 1 point
	verbLine              // consumes 1 point
	verbCube              // consumes 3 points: two controls and the end
	verbClose             // consumes no point
)

// builder appends normalised geometry (absolute move/line/cubic/close, in
// viewBox space) to shared verb and point slices.
type builder struct {
	verbs []verb
	pts   []f32.Point
}

func (b *builder) moveTo(p vec) {
	b.verbs = append(b.verbs, verbMove)
	b.pts = append(b.pts, p.f32())
}

func (b *builder) lineTo(p vec) {
	b.verbs = append(b.verbs, verbLine)
	b.pts = append(b.pts, p.f32())
}

func (b *builder) cubeTo(c1, c2, p vec) {
	b.verbs = append(b.verbs, verbCube)
	b.pts = append(b.pts, c1.f32(), c2.f32(), p.f32())
}

func (b *builder) close() {
	b.verbs = append(b.verbs, verbClose)
}

// vec is a float64 point used while parsing, so arc maths keeps precision.
type vec struct{ x, y float64 }

func (v vec) add(o vec) vec              { return vec{v.x + o.x, v.y + o.y} }
func (v vec) sub(o vec) vec              { return vec{v.x - o.x, v.y - o.y} }
func (v vec) scale(s float64) vec        { return vec{v.x * s, v.y * s} }
func (v vec) f32() f32.Point             { return f32.Pt(float32(v.x), float32(v.y)) }
func lerp(a, b vec, t float64) vec       { return a.add(b.sub(a).scale(t)) }
func reflect(ctrl, about vec) vec        { return about.add(about.sub(ctrl)) }
func quadToCube(p0, q, p vec) (vec, vec) { return lerp(p0, q, 2.0/3), lerp(p, q, 2.0/3) }

// scanner tokenises SVG number lists: path data, points and viewBox values.
// It accepts the compact syntax ("1.5.5", "-1-2", "1e-3", "011" as arc flags).
type scanner struct {
	s   string
	pos int
}

// skipSep skips whitespace and at most one comma surrounded by whitespace.
func (sc *scanner) skipSep() {
	sc.skipSpace()
	if sc.pos < len(sc.s) && sc.s[sc.pos] == ',' {
		sc.pos++
		sc.skipSpace()
	}
}

func (sc *scanner) skipSpace() {
	for sc.pos < len(sc.s) {
		switch sc.s[sc.pos] {
		case ' ', '\t', '\n', '\r', '\f':
			sc.pos++
		default:
			return
		}
	}
}

func (sc *scanner) done() bool {
	sc.skipSep()
	return sc.pos >= len(sc.s)
}

// atNumber reports whether a number starts at the current position.
func (sc *scanner) atNumber() bool {
	sc.skipSep()
	if sc.pos >= len(sc.s) {
		return false
	}
	c := sc.s[sc.pos]
	return c == '+' || c == '-' || c == '.' || (c >= '0' && c <= '9')
}

// number reads the next number.
func (sc *scanner) number() (float64, error) {
	sc.skipSep()
	start := sc.pos
	i := sc.pos
	if i < len(sc.s) && (sc.s[i] == '+' || sc.s[i] == '-') {
		i++
	}
	digits := 0
	for i < len(sc.s) && sc.s[i] >= '0' && sc.s[i] <= '9' {
		i++
		digits++
	}
	if i < len(sc.s) && sc.s[i] == '.' {
		i++
		for i < len(sc.s) && sc.s[i] >= '0' && sc.s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return 0, fmt.Errorf("expected number at offset %d in %q", start, sc.s)
	}
	// An exponent only counts if digits follow it.
	if i < len(sc.s) && (sc.s[i] == 'e' || sc.s[i] == 'E') {
		j := i + 1
		if j < len(sc.s) && (sc.s[j] == '+' || sc.s[j] == '-') {
			j++
		}
		if j < len(sc.s) && sc.s[j] >= '0' && sc.s[j] <= '9' {
			for j < len(sc.s) && sc.s[j] >= '0' && sc.s[j] <= '9' {
				j++
			}
			i = j
		}
	}
	v, err := strconv.ParseFloat(sc.s[start:i], 64)
	if err != nil {
		return 0, fmt.Errorf("bad number %q: %w", sc.s[start:i], err)
	}
	sc.pos = i
	return v, nil
}

// flag reads an arc flag, a single '0' or '1' that needs no separator.
func (sc *scanner) flag() (bool, error) {
	sc.skipSep()
	if sc.pos < len(sc.s) {
		switch sc.s[sc.pos] {
		case '0':
			sc.pos++
			return false, nil
		case '1':
			sc.pos++
			return true, nil
		}
	}
	return false, fmt.Errorf("expected arc flag at offset %d in %q", sc.pos, sc.s)
}

// numbers reads len(dst) numbers into dst.
func (sc *scanner) numbers(dst []float64) error {
	for i := range dst {
		v, err := sc.number()
		if err != nil {
			return err
		}
		dst[i] = v
	}
	return nil
}

// parsePathData appends the geometry of an SVG path "d" attribute to b.
func parsePathData(b *builder, d string) error {
	sc := scanner{s: d}
	var (
		cur, start vec
		// ctrl is the last control point of the previous segment, used by the
		// smooth S/T commands; lastCmd tells whether it is valid for them.
		ctrl    vec
		lastCmd byte
		cmd     byte
		started bool // an M command has been seen
		n       [7]float64
	)
	for !sc.done() {
		c := sc.s[sc.pos]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			cmd = c
			sc.pos++
		} else if cmd == 0 || cmd == 'Z' || cmd == 'z' {
			// Z takes no arguments, so numbers cannot repeat it.
			return fmt.Errorf("missing command at offset %d in path data %q", sc.pos, d)
		} else if !sc.atNumber() {
			return fmt.Errorf("unexpected %q in path data %q", c, d)
		}
		if !started && cmd != 'M' && cmd != 'm' {
			return fmt.Errorf("path data must start with M: %q", d)
		}
		rel := cmd >= 'a'
		base := vec{}
		if rel {
			base = cur
		}
		switch cmd {
		case 'M', 'm':
			if err := sc.numbers(n[:2]); err != nil {
				return err
			}
			cur = base.add(vec{n[0], n[1]})
			start = cur
			b.moveTo(cur)
			started = true
			// Further coordinate pairs are implicit line commands.
			if rel {
				cmd = 'l'
			} else {
				cmd = 'L'
			}
			lastCmd = 'M'
			continue
		case 'L', 'l':
			if err := sc.numbers(n[:2]); err != nil {
				return err
			}
			cur = base.add(vec{n[0], n[1]})
			b.lineTo(cur)
		case 'H', 'h':
			if err := sc.numbers(n[:1]); err != nil {
				return err
			}
			if rel {
				cur.x += n[0]
			} else {
				cur.x = n[0]
			}
			b.lineTo(cur)
		case 'V', 'v':
			if err := sc.numbers(n[:1]); err != nil {
				return err
			}
			if rel {
				cur.y += n[0]
			} else {
				cur.y = n[0]
			}
			b.lineTo(cur)
		case 'C', 'c':
			if err := sc.numbers(n[:6]); err != nil {
				return err
			}
			c1 := base.add(vec{n[0], n[1]})
			c2 := base.add(vec{n[2], n[3]})
			p := base.add(vec{n[4], n[5]})
			b.cubeTo(c1, c2, p)
			ctrl, cur = c2, p
		case 'S', 's':
			if err := sc.numbers(n[:4]); err != nil {
				return err
			}
			c1 := cur
			if lastCmd == 'C' || lastCmd == 'S' {
				c1 = reflect(ctrl, cur)
			}
			c2 := base.add(vec{n[0], n[1]})
			p := base.add(vec{n[2], n[3]})
			b.cubeTo(c1, c2, p)
			ctrl, cur = c2, p
		case 'Q', 'q':
			if err := sc.numbers(n[:4]); err != nil {
				return err
			}
			q := base.add(vec{n[0], n[1]})
			p := base.add(vec{n[2], n[3]})
			c1, c2 := quadToCube(cur, q, p)
			b.cubeTo(c1, c2, p)
			ctrl, cur = q, p
		case 'T', 't':
			if err := sc.numbers(n[:2]); err != nil {
				return err
			}
			q := cur
			if lastCmd == 'Q' || lastCmd == 'T' {
				q = reflect(ctrl, cur)
			}
			p := base.add(vec{n[0], n[1]})
			c1, c2 := quadToCube(cur, q, p)
			b.cubeTo(c1, c2, p)
			ctrl, cur = q, p
		case 'A', 'a':
			if err := sc.numbers(n[:3]); err != nil {
				return err
			}
			large, err := sc.flag()
			if err != nil {
				return err
			}
			sweep, err := sc.flag()
			if err != nil {
				return err
			}
			if err := sc.numbers(n[3:5]); err != nil {
				return err
			}
			p := base.add(vec{n[3], n[4]})
			arcTo(b, cur, n[0], n[1], n[2], large, sweep, p)
			cur = p
		case 'Z', 'z':
			b.close()
			cur = start
		default:
			return fmt.Errorf("unsupported path command %q in %q", cmd, d)
		}
		// Normalise to upper case so S/T can test the previous command.
		lastCmd = cmd &^ 0x20
	}
	return nil
}

// arcTo appends an SVG elliptical arc from p0 to p as cubic Béziers,
// following the endpoint-to-centre conversion of SVG 1.1 appendix F.6.
func arcTo(b *builder, p0 vec, rx, ry, phiDeg float64, large, sweep bool, p vec) {
	if p0 == p {
		return
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	if rx == 0 || ry == 0 {
		b.lineTo(p)
		return
	}
	sinPhi, cosPhi := math.Sincos(phiDeg * math.Pi / 180)
	// Step 1: the midpoint in the ellipse's rotated frame.
	dx, dy := (p0.x-p.x)/2, (p0.y-p.y)/2
	x1 := cosPhi*dx + sinPhi*dy
	y1 := -sinPhi*dx + cosPhi*dy
	// Scale radii up when they cannot span the endpoints.
	if l := x1*x1/(rx*rx) + y1*y1/(ry*ry); l > 1 {
		s := math.Sqrt(l)
		rx, ry = rx*s, ry*s
	}
	// Step 2: the centre in the rotated frame.
	num := rx*rx*ry*ry - rx*rx*y1*y1 - ry*ry*x1*x1
	den := rx*rx*y1*y1 + ry*ry*x1*x1
	coef := 0.0
	if num > 0 && den > 0 {
		coef = math.Sqrt(num / den)
	}
	if large == sweep {
		coef = -coef
	}
	cx1 := coef * rx * y1 / ry
	cy1 := -coef * ry * x1 / rx
	// Step 3: the centre in user space.
	cx := cosPhi*cx1 - sinPhi*cy1 + (p0.x+p.x)/2
	cy := sinPhi*cx1 + cosPhi*cy1 + (p0.y+p.y)/2
	// Step 4: start angle and sweep extent.
	theta1 := math.Atan2((y1-cy1)/ry, (x1-cx1)/rx)
	theta2 := math.Atan2((-y1-cy1)/ry, (-x1-cx1)/rx)
	dtheta := theta2 - theta1
	if sweep && dtheta < 0 {
		dtheta += 2 * math.Pi
	} else if !sweep && dtheta > 0 {
		dtheta -= 2 * math.Pi
	}
	// Approximate with segments of at most 90°.
	segs := int(math.Ceil(math.Abs(dtheta)/(math.Pi/2) - 1e-9))
	if segs < 1 {
		segs = 1
	}
	step := dtheta / float64(segs)
	k := 4.0 / 3 * math.Tan(step/4)
	// point maps a unit-circle position to user space.
	point := func(cosT, sinT float64) vec {
		x, y := rx*cosT, ry*sinT
		return vec{cx + cosPhi*x - sinPhi*y, cy + sinPhi*x + cosPhi*y}
	}
	t := theta1
	sinT, cosT := math.Sincos(t)
	for i := range segs {
		t2 := t + step
		sinT2, cosT2 := math.Sincos(t2)
		c1 := point(cosT-k*sinT, sinT+k*cosT)
		c2 := point(cosT2+k*sinT2, sinT2-k*cosT2)
		to := point(cosT2, sinT2)
		if i == segs-1 {
			to = p // land exactly on the requested endpoint
		}
		b.cubeTo(c1, c2, to)
		t, sinT, cosT = t2, sinT2, cosT2
	}
}
