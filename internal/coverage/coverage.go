// Package coverage measures how a shape's Z range covers the floor under
// its outline (spec "Cobertura vertical", D1-D3): the lowest and highest
// floor under it, the clearance of its floor and top, and the area of
// floor inside the range, above its top, below its floor and with no floor
// at all. Everything is in server coordinates and exact: the floor's
// triangles are clipped by the outline on the X/Y plane, where their Z is
// affine, so no peak between samples is missed. No GL, no Gio.
//
// Measuring splits in 2: Measure clips the floor by the outline once into
// a Profile, which depends only on the outline and the scene; Classify
// sorts a Profile by a Z range, cheaply enough to run on every frame.
package coverage

import (
	"math"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
)

// Floor is where the floor triangles come from: scene.World, or a
// synthetic floor in tests. Floor calls fn with every floor triangle whose
// X/Y extent may meet the X/Y of box (server coordinates); extra triangles
// outside box are allowed.
type Floor interface {
	Floor(box geom.Box, fn func(scene.FloorTriangle))
}

// Point is a position on the X/Y plane, in server coordinates.
type Point struct{ X, Y float64 }

// Outline is a shape's contour on the X/Y plane, closed from the last
// point back to the first. It may wind either way and be concave.
type Outline []Point

// Spot is a point of floor, in server coordinates.
type Spot struct{ X, Y, Z float64 }

// Profile is the floor under one outline: the floor's triangles clipped by
// it, as flat pieces. It holds no Z range; Classify sorts it by one. A
// Profile is read-only once Measure returns it.
type Profile struct {
	// area is the outline's area; ground the floor area measured under it.
	area, ground float64
	// lo and hi are the lowest and highest floor; measured is false when
	// no floor lies under the outline.
	lo, hi   Spot
	measured bool
	pieces   []piece
	// verts are the pieces' vertices, piece by piece.
	verts []Spot
	// banned lists, piece by piece (piece.bans), the bans whose outline
	// holds the piece's centroid; banZ is each ban's Z range. The outline
	// test is made once by Measure, the Z test by Classify, so a ban's
	// range can change without measuring again (WithBanRanges).
	banned []int32
	banZ   [][2]float64
}

// Ban is a banned shape of the same zone: its outline and Z range. Floor
// under it and within its range is excluded, not a failure (spec D1).
type Ban struct {
	Outline    Outline
	ZMin, ZMax float64
}

// piece is a flat polygon of floor inside the outline: verts[first :
// first+n], its X/Y area and its Z extent.
type piece struct {
	first, n int32
	area     float64
	zlo, zhi float64
	// cz is the Z at the piece's centroid; bans indexes banned[bans :
	// bans+nbans].
	cz          float64
	bans, nbans int32
}

// Report is a Profile sorted by a Z range [zmin, zmax]. Areas are X/Y
// areas in server units².
type Report struct {
	// Measured is false when no floor lies under the outline; GroundMin,
	// GroundMax and the clearances are then meaningless.
	Measured bool
	// GroundMin and GroundMax are the lowest and highest floor under the
	// outline.
	GroundMin, GroundMax Spot
	// FloorClearance is GroundMin.Z − zmin, negative when floor lies
	// below the shape's floor; TopClearance is zmax − GroundMax.Z,
	// negative when floor pierces its top.
	FloorClearance, TopClearance float64
	// Inside is the floor with zmin ≤ z ≤ zmax, Above the floor over zmax
	// and Below the floor under zmin.
	Inside, Above, Below float64
	// Excluded is the floor whose piece centroid lies under a ban and
	// within its Z range; it is in none of Inside, Above, Below.
	Excluded float64
	// NoGround is the outline's area with no floor under it: an invisible
	// terrain quad, a tile not loaded, off the map.
	NoGround float64
}

// Total is the outline's area as the report splits it: the floor in each
// state plus the area with no floor.
func (r Report) Total() float64 { return r.Inside + r.Above + r.Below + r.Excluded + r.NoGround }

// Coverage is the fraction of Total that is floor inside the range, 0 for
// an empty outline. Area with no floor counts against it, never as
// covered.
func (r Report) Coverage() float64 {
	if t := r.Total(); t > 0 {
		return r.Inside / t
	}
	return 0
}

// Measure clips the floor under outline o into its Profile. An outline of
// fewer than 3 points, or of no area, has an empty profile. Each piece is
// matched to the bans whose outline holds its centroid; their Z ranges
// are applied by Classify.
func Measure(f Floor, o Outline, bans []Ban) *Profile {
	p := &Profile{}
	p.banZ = make([][2]float64, len(bans))
	for i, b := range bans {
		p.banZ[i] = [2]float64{b.ZMin, b.ZMax}
	}
	if len(o) < 3 {
		return p
	}
	o = counterClockwise(o)
	p.area = signedArea2(o) / 2
	if p.area <= 0 {
		return p
	}
	box := geom.EmptyBox()
	for _, q := range o {
		box.Include(geom.Vec3{X: float32(q.X), Y: float32(q.Y)})
	}
	box.Min.Z, box.Max.Z = 0, 0
	m := measurer{p: p, outline: o, bans: bans, lo: Point{X: float64(box.Min.X), Y: float64(box.Min.Y)}, hi: Point{X: float64(box.Max.X), Y: float64(box.Max.Y)}}
	f.Floor(box, m.triangle)
	return p
}

// WithBanRanges is p with the bans' Z ranges taken from bans (same order
// and count as given to Measure; outlines are ignored). It shares p's
// pieces, so changing a ban's range costs no new measurement.
func (p *Profile) WithBanRanges(bans []Ban) *Profile {
	if len(bans) != len(p.banZ) {
		return p
	}
	q := *p
	q.banZ = make([][2]float64, len(bans))
	for i, b := range bans {
		q.banZ[i] = [2]float64{b.ZMin, b.ZMax}
	}
	return &q
}

// measurer clips the floor's triangles by one outline into a Profile.
type measurer struct {
	p       *Profile
	outline Outline // counter-clockwise
	bans    []Ban
	lo, hi  Point // the outline's X/Y box
	// clip and spare are scratch polygons for Sutherland–Hodgman.
	clip, spare []Point
}

// triangle adds the part of t inside the outline to the profile.
func (m *measurer) triangle(t scene.FloorTriangle) {
	a, b, c := spot(t.A), spot(t.B), spot(t.C)
	if max(a.X, b.X, c.X) < m.lo.X || min(a.X, b.X, c.X) > m.hi.X ||
		max(a.Y, b.Y, c.Y) < m.lo.Y || min(a.Y, b.Y, c.Y) > m.hi.Y {
		return
	}
	area2 := (b.X-a.X)*(c.Y-a.Y) - (c.X-a.X)*(b.Y-a.Y)
	if area2 < 0 {
		b, c, area2 = c, b, -area2
	}
	if area2 <= 1e-9 {
		return // seen edge on: no area to stand on
	}
	tri := [3]Spot{a, b, c}
	if m.whollyInside(tri) {
		m.add(tri[:], area2/2)
		return
	}
	// The triangle crosses the outline: clip the outline (the subject,
	// maybe concave) by the triangle (the convex window), and put each
	// output vertex on the triangle's plane.
	m.clip = append(m.clip[:0], m.outline...)
	for k := range 3 {
		m.clip, m.spare = clipHalfPlane(m.clip, m.spare[:0], point(tri[k]), point(tri[(k+1)%3])), m.clip
		if len(m.clip) < 3 {
			return
		}
	}
	area := signedArea2(m.clip) / 2
	if area <= 1e-9 {
		return
	}
	plane := planeOf(tri)
	first := len(m.p.verts)
	for _, q := range m.clip {
		m.p.verts = append(m.p.verts, Spot{q.X, q.Y, plane(q)})
	}
	m.piece(first, area)
	// Sutherland–Hodgman joins the parts of a concave outline by zero-area
	// bridges whose corners may lie outside it: the extremes come from the
	// true corners of triangle ∩ outline instead.
	m.corners(tri, plane)
}

// whollyInside reports a triangle strictly inside the outline: its
// corners inside and off the outline, no outline vertex inside it or on
// its edges and no outline edge crossing its edges.
func (m *measurer) whollyInside(tri [3]Spot) bool {
	for _, q := range tri {
		if in, on := contains(m.outline, point(q)); !in || on {
			return false
		}
	}
	for i, q := range m.outline {
		if inTriangle(tri, q) {
			return false
		}
		r := m.outline[(i+1)%len(m.outline)]
		for k := range 3 {
			if _, ok := crossing(q, r, point(tri[k]), point(tri[(k+1)%3])); ok {
				return false
			}
		}
	}
	return true
}

// corners takes the extremes of triangle ∩ outline from its corners: the
// triangle's corners inside the outline, the outline's vertices inside
// the triangle and the crossings of their edges.
func (m *measurer) corners(tri [3]Spot, plane func(Point) float64) {
	for _, q := range tri {
		if in, on := contains(m.outline, point(q)); in || on {
			m.extreme(q)
		}
	}
	for i, q := range m.outline {
		if inTriangle(tri, q) {
			m.extreme(Spot{q.X, q.Y, plane(q)})
		}
		r := m.outline[(i+1)%len(m.outline)]
		for k := range 3 {
			if x, ok := crossing(q, r, point(tri[k]), point(tri[(k+1)%3])); ok {
				m.extreme(Spot{x.X, x.Y, plane(x)})
			}
		}
	}
}

// add adds a piece whose vertices are all corners of floor inside the
// outline.
func (m *measurer) add(vs []Spot, area float64) {
	first := len(m.p.verts)
	m.p.verts = append(m.p.verts, vs...)
	m.piece(first, area)
	for _, q := range vs {
		m.extreme(q)
	}
}

// piece records verts[first:] as a piece of the given area.
func (m *measurer) piece(first int, area float64) {
	vs := m.p.verts[first:]
	pc := piece{first: int32(first), n: int32(len(vs)), area: area, zlo: math.Inf(1), zhi: math.Inf(-1)}
	for _, q := range vs {
		pc.zlo, pc.zhi = min(pc.zlo, q.Z), max(pc.zhi, q.Z)
	}
	if len(m.bans) > 0 {
		c := centroid(vs)
		pc.cz = c.Z
		pc.bans = int32(len(m.p.banned))
		for i, b := range m.bans {
			if in, on := contains(b.Outline, point(c)); in || on {
				m.p.banned = append(m.p.banned, int32(i))
			}
		}
		pc.nbans = int32(len(m.p.banned)) - pc.bans
	}
	m.p.pieces = append(m.p.pieces, pc)
	m.p.ground += area
}

// centroid is the area centroid of flat polygon vs, with its Z on the
// polygon's plane (a fan of triangles, each weighted by its signed area).
func centroid(vs []Spot) Spot {
	var c Spot
	var w float64
	for i := 1; i+1 < len(vs); i++ {
		a, b, d := vs[0], vs[i], vs[i+1]
		s := (b.X-a.X)*(d.Y-a.Y) - (d.X-a.X)*(b.Y-a.Y)
		c.X += s * (a.X + b.X + d.X)
		c.Y += s * (a.Y + b.Y + d.Y)
		c.Z += s * (a.Z + b.Z + d.Z)
		w += s
	}
	if w == 0 {
		return vs[0]
	}
	return Spot{c.X / (3 * w), c.Y / (3 * w), c.Z / (3 * w)}
}

// excluded reports pc's centroid under one of its bans and within its Z
// range.
func (p *Profile) excluded(pc piece) bool {
	for _, i := range p.banned[pc.bans : pc.bans+pc.nbans] {
		if z := p.banZ[i]; pc.cz >= z[0] && pc.cz <= z[1] {
			return true
		}
	}
	return false
}

// extreme counts q in the lowest and highest floor.
func (m *measurer) extreme(q Spot) {
	p := m.p
	if !p.measured {
		p.lo, p.hi, p.measured = q, q, true
		return
	}
	if q.Z < p.lo.Z {
		p.lo = q
	}
	if q.Z > p.hi.Z {
		p.hi = q
	}
}

// Classify sorts the profile's floor by the Z range [zmin, zmax].
func (p *Profile) Classify(zmin, zmax float64) Report {
	r := Report{Measured: p.measured, GroundMin: p.lo, GroundMax: p.hi}
	if p.measured {
		r.FloorClearance, r.TopClearance = p.lo.Z-zmin, zmax-p.hi.Z
	}
	var cut, slab []Spot // scratch polygons
	for _, pc := range p.pieces {
		switch {
		case pc.nbans > 0 && p.excluded(pc):
			r.Excluded += pc.area
		case pc.zlo >= zmin && pc.zhi <= zmax:
			r.Inside += pc.area
		case pc.zlo > zmax:
			r.Above += pc.area
		case pc.zhi < zmin:
			r.Below += pc.area
		default:
			vs := p.verts[pc.first : pc.first+pc.n]
			cut = clipZ(vs, cut[:0], zmax, true)
			r.Above += spotArea(cut)
			cut = clipZ(vs, cut[:0], zmin, false)
			r.Below += spotArea(cut)
			cut = clipZ(vs, cut[:0], zmin, true)
			slab = clipZ(cut, slab[:0], zmax, false)
			r.Inside += spotArea(slab)
		}
	}
	r.NoGround = max(0, p.area-p.ground)
	if r.NoGround <= 1e-9*p.area {
		r.NoGround = 0
	}
	return r
}

// clipZ appends to out the part of flat polygon vs with z ≥ level (above)
// or z ≤ level (!above). The polygon is flat, so its Z is affine on X/Y
// and the cut is a straight line: the part is exact.
func clipZ(vs, out []Spot, level float64, above bool) []Spot {
	side := func(q Spot) float64 {
		if above {
			return q.Z - level
		}
		return level - q.Z
	}
	for i, q := range vs {
		r := vs[(i+1)%len(vs)]
		sq, sr := side(q), side(r)
		if sq >= 0 {
			out = append(out, q)
		}
		if (sq >= 0) != (sr >= 0) {
			t := sq / (sq - sr)
			out = append(out, Spot{q.X + t*(r.X-q.X), q.Y + t*(r.Y-q.Y), level})
		}
	}
	return out
}

// spotArea is the X/Y area of polygon vs.
func spotArea(vs []Spot) float64 {
	var a2 float64
	for i, q := range vs {
		r := vs[(i+1)%len(vs)]
		a2 += q.X*r.Y - r.X*q.Y
	}
	return math.Abs(a2) / 2
}

// clipHalfPlane appends to out the part of polygon in on the left of the
// line from a to b (Sutherland–Hodgman, one window edge).
func clipHalfPlane(in, out []Point, a, b Point) []Point {
	side := func(q Point) float64 { return (b.X-a.X)*(q.Y-a.Y) - (b.Y-a.Y)*(q.X-a.X) }
	for i, q := range in {
		r := in[(i+1)%len(in)]
		sq, sr := side(q), side(r)
		if sq >= 0 {
			out = append(out, q)
		}
		if (sq >= 0) != (sr >= 0) {
			t := sq / (sq - sr)
			out = append(out, Point{q.X + t*(r.X-q.X), q.Y + t*(r.Y-q.Y)})
		}
	}
	return out
}

// planeOf is the Z of the plane through tri at a point of the X/Y plane.
func planeOf(tri [3]Spot) func(Point) float64 {
	a, b, c := tri[0], tri[1], tri[2]
	det := (b.X-a.X)*(c.Y-a.Y) - (c.X-a.X)*(b.Y-a.Y)
	return func(q Point) float64 {
		u := ((q.X-a.X)*(c.Y-a.Y) - (c.X-a.X)*(q.Y-a.Y)) / det
		w := ((b.X-a.X)*(q.Y-a.Y) - (q.X-a.X)*(b.Y-a.Y)) / det
		return a.Z + u*(b.Z-a.Z) + w*(c.Z-a.Z)
	}
}

// contains reports whether q lies inside polygon o (even-odd rule) and
// whether it lies on its boundary.
func contains(o Outline, q Point) (in, on bool) {
	for i, a := range o {
		b := o[(i+1)%len(o)]
		cross := (b.X-a.X)*(q.Y-a.Y) - (b.Y-a.Y)*(q.X-a.X)
		if math.Abs(cross) <= onSlack*(math.Abs(b.X-a.X)+math.Abs(b.Y-a.Y)) &&
			q.X >= min(a.X, b.X) && q.X <= max(a.X, b.X) && q.Y >= min(a.Y, b.Y) && q.Y <= max(a.Y, b.Y) {
			on = true
		}
		if (a.Y > q.Y) != (b.Y > q.Y) && q.X < a.X+(q.Y-a.Y)*(b.X-a.X)/(b.Y-a.Y) {
			in = !in
		}
	}
	return in, on
}

// onSlack is how far, in units, a point may lie from an edge and still be
// on it: well above float64 rounding at server coordinates, well below
// anything a zone or a terrain cell resolves.
const onSlack = 1e-6

// inTriangle reports q inside counter-clockwise tri or on its edges.
func inTriangle(tri [3]Spot, q Point) bool {
	for k := range 3 {
		a, b := tri[k], tri[(k+1)%3]
		if (b.X-a.X)*(q.Y-a.Y)-(b.Y-a.Y)*(q.X-a.X) < -onSlack*(math.Abs(b.X-a.X)+math.Abs(b.Y-a.Y)) {
			return false
		}
	}
	return true
}

// crossing is where segments pq and rs cross at a point interior to both;
// false when they don't, touch at an end or run parallel.
func crossing(p, q, r, s Point) (Point, bool) {
	dx1, dy1 := q.X-p.X, q.Y-p.Y
	dx2, dy2 := s.X-r.X, s.Y-r.Y
	den := dx1*dy2 - dy1*dx2
	if den == 0 {
		return Point{}, false
	}
	t := ((r.X-p.X)*dy2 - (r.Y-p.Y)*dx2) / den
	u := ((r.X-p.X)*dy1 - (r.Y-p.Y)*dx1) / den
	const eps = 1e-12
	if t <= eps || t >= 1-eps || u <= eps || u >= 1-eps {
		return Point{}, false
	}
	return Point{p.X + t*dx1, p.Y + t*dy1}, true
}

// counterClockwise is o wound counter-clockwise (a copy when reversed).
func counterClockwise(o Outline) Outline {
	if signedArea2(o) >= 0 {
		return o
	}
	r := make(Outline, len(o))
	for i, q := range o {
		r[len(o)-1-i] = q
	}
	return r
}

// signedArea2 is twice the signed area of polygon o, positive when it
// winds counter-clockwise.
func signedArea2(o []Point) float64 {
	var a2 float64
	for i, q := range o {
		r := o[(i+1)%len(o)]
		a2 += q.X*r.Y - r.X*q.Y
	}
	return a2
}

func spot(v geom.Vec3) Spot { return Spot{float64(v.X), float64(v.Y), float64(v.Z)} }
func point(s Spot) Point    { return Point{s.X, s.Y} }
