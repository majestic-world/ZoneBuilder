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
	"cmp"
	"math"
	"slices"

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
	// area is the outline's area; floored the part of it with floor on at
	// least 1 layer.
	area, floored float64
	// layers is the most layers of floor in one column under the outline.
	layers int
	pieces []piece
	// byLow is the BSP and mesh pieces, lowest floor first: what the
	// layer rule reads its left-out layers from. reachHi[k] is the
	// highest floor of the pieces of byLow[:k+1] under no ban, and
	// bannedBuilt the BSP and mesh pieces under a ban: with them, whether
	// any BSP or mesh floor reaches a range is a binary search.
	byLow       []lowPiece
	reachHi     []float64
	bannedBuilt []int32
	// terrainLo and terrainHi are the lowest and highest terrain under
	// the outline (terrainLo > terrainHi with none), judged as one layer;
	// terrainFree is whether some of it lies under no ban.
	terrainLo, terrainHi float64
	terrainFree          bool
	// verts are the pieces' vertices, piece by piece.
	verts []Spot
	// clipped are the triangles that pieces cut by the outline come from,
	// with each piece's extremes.
	clipped []clipped
	// edges is the floor along each outline edge, in the outline's order.
	edges [][]Span
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

// Station is a point of floor on an outline edge, D units from the edge's
// first end.
type Station struct {
	D float64
	Spot
}

// Span is a straight run of floor along an outline edge: the edge across
// one floor triangle, from where it enters it to where it leaves. Its ends
// are the edge's crossings with the triangle's edges, so the line is
// exact.
type Span struct {
	From, To Station
	// piece is the piece cut from the same triangle, whose layer decides
	// whether the span is in the range's floor line.
	piece int32
}

// piece is a flat polygon of floor inside the outline: verts[first :
// first+n], its X/Y area, its Z extent and the surface it lies on. A
// piece is a whole floor triangle (its 3 verts, counter-clockwise) when
// clip is -1, else the part of clipped[clip].tri inside the outline.
type piece struct {
	first, n int32
	clip     int32
	surface  scene.Surface
	area     float64
	// zlo and zhi are its lowest and highest floor (Profile.span).
	zlo, zhi float64
	// cz is the Z at the piece's centroid; bans indexes banned[bans :
	// bans+nbans].
	cz          float64
	bans, nbans int32
}

// clipped is the floor triangle a piece was cut from, counter-clockwise,
// and the lowest and highest floor of that piece.
type clipped struct {
	tri    [3]Spot
	lo, hi Spot
}

// Report is a Profile sorted by a Z range [zmin, zmax]. Areas are X/Y
// areas in server units². The floor of the range is the floor neither
// excluded by a ban (spec D1) nor another layer by the layer rule (spec
// D5, reaches): only it is sorted into Inside, Above and Below and gives
// the extremes, the clearances and the warnings.
type Report struct {
	// Measured is false when the range has no floor under the outline:
	// none at all, or all of it excluded or in other layers. GroundMin,
	// GroundMax and the clearances are then meaningless.
	Measured bool
	// GroundMin and GroundMax are the lowest and highest floor of the
	// range under the outline.
	GroundMin, GroundMax Spot
	// FloorClearance is GroundMin.Z − zmin, negative when floor lies
	// below the shape's floor; TopClearance is zmax − GroundMax.Z,
	// negative when floor pierces its top.
	FloorClearance, TopClearance float64
	// Inside is the floor with zmin ≤ z ≤ zmax, Above the floor over zmax
	// and Below the floor under zmin, every layer of the range counted: a
	// bridge over the terrain adds its deck to the terrain under it.
	Inside, Above, Below float64
	// Excluded is the floor whose piece centroid lies under a ban and
	// within its Z range; it is in none of Inside, Above, Below.
	Excluded float64
	// Other is the floor the layer rule leaves out: a roof or a tree
	// canopy far over the range, a cave far under it, the terrain under a
	// tower's top. Others are its layers, lowest first. It is in none of
	// Inside, Above, Below, nor Excluded.
	Other  float64
	Others []Layer
	// Terrain is whether the terrain under the outline is floor of the
	// range by the layer rule (Profile.terrainCounts); false, it is in
	// Other. Sum leaves it out: it is per shape.
	Terrain bool
	// NoGround is the outline's area with no floor on any layer: an
	// invisible terrain quad no building floor covers, a tile not loaded,
	// off the map.
	NoGround float64
	// Layers is the most layers of floor (terrain, a building's floor, a
	// bridge) stacked in one column under the outline, 0 with no floor.
	Layers int
	// Edges is the floor along each edge of the outline, edge i running
	// from outline point i to the next: its spans by distance from point
	// i. Only floor the layer rule counts is in it (another layer, such as
	// a tower's lower storeys, is not). Where the edge has no such floor
	// there is a gap; where floors overlap, spans overlap.
	Edges [][]Span
	// Warnings are the ways the range fits its floor badly (spec D6).
	// Sum leaves them and Others out: they are per shape.
	Warnings []Warning
}

// Total is the outline's area as the report splits it: the floor in each
// state, every layer counted, plus the area with no floor.
func (r Report) Total() float64 {
	return r.Inside + r.Above + r.Below + r.Excluded + r.Other + r.NoGround
}

// Coverage is the fraction of Total that is floor inside the range, 0 for
// an empty outline. Area with no floor counts against it, never as
// covered.
func (r Report) Coverage() float64 {
	if t := r.Total(); t > 0 {
		return r.Inside / t
	}
	return 0
}

// Sum is the report of a zone made of the shapes reported in rs, each
// classified by its own Z range (spec D3): their areas summed, so floor
// under 2 overlapping shapes counts twice ("soma dos shapes"), the lowest
// and highest floor under any of them, the worst clearance of each side
// and the most layers. Shapes whose range has no floor add their areas
// and nothing else.
func Sum(rs ...Report) Report {
	var z Report
	for _, r := range rs {
		z.Inside += r.Inside
		z.Above += r.Above
		z.Below += r.Below
		z.Excluded += r.Excluded
		z.Other += r.Other
		z.NoGround += r.NoGround
		z.Layers = max(z.Layers, r.Layers)
		if !r.Measured {
			continue
		}
		if !z.Measured {
			z.Measured = true
			z.GroundMin, z.GroundMax = r.GroundMin, r.GroundMax
			z.FloorClearance, z.TopClearance = r.FloorClearance, r.TopClearance
			continue
		}
		if r.GroundMin.Z < z.GroundMin.Z {
			z.GroundMin = r.GroundMin
		}
		if r.GroundMax.Z > z.GroundMax.Z {
			z.GroundMax = r.GroundMax
		}
		z.FloorClearance = min(z.FloorClearance, r.FloorClearance)
		z.TopClearance = min(z.TopClearance, r.TopClearance)
	}
	return z
}

// Measure clips the floor under outline o into its Profile. An outline of
// fewer than 3 points, or of no area, has an empty profile. Each piece is
// matched to the bans whose outline holds its centroid; their Z ranges
// are applied by Classify.
func Measure(f Floor, o Outline, bans []Ban) *Profile {
	p := &Profile{terrainLo: math.Inf(1), terrainHi: math.Inf(-1)}
	p.banZ = make([][2]float64, len(bans))
	for i, b := range bans {
		p.banZ[i] = [2]float64{b.ZMin, b.ZMax}
	}
	if len(o) < 3 {
		return p
	}
	edges := o
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
	m := measurer{p: p, outline: o, edges: edges, bans: bans, lo: Point{X: float64(box.Min.X), Y: float64(box.Min.Y)}, hi: Point{X: float64(box.Max.X), Y: float64(box.Max.Y)}}
	p.edges = make([][]Span, len(edges))
	f.Floor(box, m.triangle)
	p.stack(o)
	p.sortByLow()
	p.indexReach()
	for _, spans := range p.edges {
		slices.SortFunc(spans, func(a, b Span) int { return cmp.Compare(a.From.D, b.From.D) })
	}
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
	edges   Outline // the outline as given, whose edges Edges follows
	bans    []Ban
	lo, hi  Point // the outline's X/Y box
	// clip and spare are scratch polygons for Sutherland–Hodgman.
	clip, spare []Point
	// surface is the current triangle's; cut, when set, the clipped entry
	// of the piece being cut from it, whose extremes extreme keeps too.
	surface scene.Surface
	cut     *clipped
}

// triangle adds the part of t inside the outline to the profile.
func (m *measurer) triangle(t scene.FloorTriangle) {
	a, b, c := spot(t.A), spot(t.B), spot(t.C)
	m.surface = t.Surface
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
	inf := math.Inf(1)
	m.p.clipped = append(m.p.clipped, clipped{tri: tri, lo: Spot{Z: inf}, hi: Spot{Z: -inf}})
	pc := &m.p.pieces[len(m.p.pieces)-1]
	pc.clip = int32(len(m.p.clipped) - 1)
	m.cut = &m.p.clipped[len(m.p.clipped)-1]
	m.corners(tri, plane)
	if c := m.cut; c.lo.Z <= c.hi.Z {
		pc.zlo, pc.zhi = c.lo.Z, c.hi.Z
	}
	m.cut = nil
	// Only a triangle the outline cuts has an outline edge across it.
	m.edgeSpans(tri, int32(len(m.p.pieces)-1))
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

// edgeSpans adds to the profile the part of each outline edge over
// counter-clockwise tri, whose piece inside the outline is piece: the
// edge clipped by the triangle's 3 sides, its Z on the triangle's plane.
func (m *measurer) edgeSpans(tri [3]Spot, piece int32) {
	var plane func(Point) float64
	for i, p := range m.edges {
		q := m.edges[(i+1)%len(m.edges)]
		if max(p.X, q.X) < min(tri[0].X, tri[1].X, tri[2].X) || min(p.X, q.X) > max(tri[0].X, tri[1].X, tri[2].X) ||
			max(p.Y, q.Y) < min(tri[0].Y, tri[1].Y, tri[2].Y) || min(p.Y, q.Y) > max(tri[0].Y, tri[1].Y, tri[2].Y) {
			continue
		}
		t0, t1 := 0.0, 1.0
		for k := range 3 {
			a, b := tri[k], tri[(k+1)%3]
			// side(t) = fp + t·(fq − fp) ≥ 0 inside the triangle.
			fp := (b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X)
			fq := (b.X-a.X)*(q.Y-a.Y) - (b.Y-a.Y)*(q.X-a.X)
			switch {
			case fp < 0 && fq < 0:
				t0, t1 = 1, 0
			case fp < 0:
				t0 = max(t0, fp/(fp-fq))
			case fq < 0:
				t1 = min(t1, fp/(fp-fq))
			}
			if t1 <= t0 {
				break
			}
		}
		length := math.Hypot(q.X-p.X, q.Y-p.Y)
		if (t1-t0)*length <= onSlack {
			continue
		}
		if plane == nil {
			plane = planeOf(tri)
		}
		at := func(t float64) Station {
			x := Point{p.X + t*(q.X-p.X), p.Y + t*(q.Y-p.Y)}
			return Station{t * length, Spot{x.X, x.Y, plane(x)}}
		}
		m.p.edges[i] = append(m.p.edges[i], Span{From: at(t0), To: at(t1), piece: piece})
	}
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
}

// piece records verts[first:] as a piece of the given area.
func (m *measurer) piece(first int, area float64) {
	vs := m.p.verts[first:]
	pc := piece{first: int32(first), n: int32(len(vs)), clip: -1, surface: m.surface, area: area, zlo: math.Inf(1), zhi: math.Inf(-1)}
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
func (p *Profile) excluded(pc *piece) bool {
	for _, i := range p.banned[pc.bans : pc.bans+pc.nbans] {
		if z := p.banZ[i]; pc.cz >= z[0] && pc.cz <= z[1] {
			return true
		}
	}
	return false
}

// extreme counts q in the lowest and highest floor of the piece being
// cut.
func (m *measurer) extreme(q Spot) {
	c := m.cut
	if q.Z < c.lo.Z {
		c.lo = q
	}
	if q.Z > c.hi.Z {
		c.hi = q
	}
}

// Classify sorts the profile's floor by the Z range [zmin, zmax]: the
// excluded floor and the other layers apart, the floor of the range into
// Inside, Above and Below.
func (p *Profile) Classify(zmin, zmax float64) Report {
	terrain := p.terrainCounts(zmin, zmax)
	r := Report{Layers: p.layers, Edges: p.floorLine(zmin, zmax, terrain), Terrain: terrain}
	var cut, slab []Spot // scratch polygons
	for i := range p.pieces {
		pc := &p.pieces[i]
		switch {
		case pc.nbans > 0 && p.excluded(pc):
			r.Excluded += pc.area
			continue
		case !counts(pc, zmin, zmax, r.Terrain):
			r.Other += pc.area
			continue
		}
		if !r.Measured || pc.zlo < r.GroundMin.Z || pc.zhi > r.GroundMax.Z {
			lo, hi := p.span(pc)
			switch {
			case !r.Measured:
				r.GroundMin, r.GroundMax, r.Measured = lo, hi, true
			case lo.Z < r.GroundMin.Z:
				r.GroundMin = lo
			}
			if hi.Z > r.GroundMax.Z {
				r.GroundMax = hi
			}
		}
		switch {
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
	if r.Measured {
		r.FloorClearance, r.TopClearance = r.GroundMin.Z-zmin, zmax-r.GroundMax.Z
	}
	if r.Other > 0 {
		r.Others = p.others(zmin, zmax, r.Terrain)
	}
	r.NoGround = max(0, p.area-p.floored)
	if r.NoGround <= 1e-9*p.area {
		r.NoGround = 0
	}
	r.Warnings = warnings(r)
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
