package coverage

import (
	"cmp"
	"iter"
	"math"
	"runtime"
	"slices"
	"sync"

	"zonebuilder/internal/scene"
)

// LayerGap is how far apart, in Z, 2 floors of one column must be to be 2
// layers: closer, a character cannot stand between them (a mesh paving
// laid on the terrain, a BSP floor and a mesh floor on top of each other).
// [INFERENCE] set to the scale of a character, not measured.
const LayerGap = 32

// stackChunk is the fewest pieces worth a goroutine of their own when
// counting layers.
const stackChunk = 8192

// Piece is a flat piece of floor under the outline: the part of one floor
// triangle inside it.
type Piece struct {
	Surface scene.Surface
	// Area is its X/Y area in server units².
	Area float64
	// Low and High are its lowest and highest floor.
	Low, High Spot
}

// Pieces are the profile's pieces of floor, every layer, in the order the
// floor gave them: what a rule over the layers (which surfaces pull a
// shape's Z range) reads instead of measuring again.
func (p *Profile) Pieces() iter.Seq[Piece] {
	return func(yield func(Piece) bool) {
		for i := range p.pieces {
			pc := &p.pieces[i]
			lo, hi := p.span(pc)
			if !yield(Piece{Surface: pc.surface, Area: pc.area, Low: lo, High: hi}) {
				return
			}
		}
	}
}

// span is the lowest and highest floor of piece pc: of its vertices for a
// whole triangle, of the true corners of triangle ∩ outline for a cut
// piece (piece.zlo and zhi are their Z).
func (p *Profile) span(pc *piece) (lo, hi Spot) {
	if pc.clip >= 0 {
		c := &p.clipped[pc.clip]
		return c.lo, c.hi
	}
	vs := p.verts[pc.first : pc.first+pc.n]
	lo, hi = vs[0], vs[0]
	for _, q := range vs[1:] {
		if q.Z < lo.Z {
			lo = q
		}
		if q.Z > hi.Z {
			hi = q
		}
	}
	return lo, hi
}

// stack counts the layers of the measured floor and the outline's area
// with floor on any of them (floored). Terrain never overlaps itself, so
// both only need looking at the columns of the other surfaces: one column
// per BSP or mesh piece, at a point inside it, where every piece over the
// column is a floor at its Z. The piece counts in floored when no terrain
// lies under that point and it is the lowest floor there; its area is
// then taken whole, so the error stays within 1 triangle along a terrain
// hole's border, as for exclusions (spec D1).
func (p *Profile) stack(o Outline) {
	var terrain float64
	type column struct {
		q     Point
		piece int32
	}
	var cols []column
	for i, pc := range p.pieces {
		if pc.surface == scene.SurfaceTerrain {
			terrain += pc.area
			continue
		}
		if q, ok := p.inner(i, o); ok {
			cols = append(cols, column{q, int32(i)})
		}
	}
	p.floored = terrain
	if len(p.pieces) > 0 {
		p.layers = 1
	}
	if len(cols) == 0 {
		return
	}

	// Bin the columns on a grid over their box, about 1 per cell.
	lo, hi := cols[0].q, cols[0].q
	for _, c := range cols[1:] {
		lo.X, lo.Y = min(lo.X, c.q.X), min(lo.Y, c.q.Y)
		hi.X, hi.Y = max(hi.X, c.q.X), max(hi.Y, c.q.Y)
	}
	side := max(1, int(math.Sqrt(float64(len(cols)))))
	cw, ch := max((hi.X-lo.X)/float64(side), 1e-9), max((hi.Y-lo.Y)/float64(side), 1e-9)
	cell := func(v, origin, size float64) int { return min(max(int((v-origin)/size), 0), side-1) }
	start := make([]int32, side*side+1)
	for _, c := range cols {
		start[cell(c.q.Y, lo.Y, ch)*side+cell(c.q.X, lo.X, cw)+1]++
	}
	for k := 1; k < len(start); k++ {
		start[k] += start[k-1]
	}
	binned := make([]int32, len(cols))
	fill := slices.Clone(start[:side*side])
	for i, c := range cols {
		k := cell(c.q.Y, lo.Y, ch)*side + cell(c.q.X, lo.X, cw)
		binned[fill[k]] = int32(i)
		fill[k]++
	}

	// Every piece over a column is a floor there. The pieces are split
	// among the CPUs: a town under a whole-tile outline has some 150 000
	// mesh pieces, each a column.
	type hit struct {
		col   int32
		piece int32
		z     float64
	}
	collect := func(from, to int) (hits []hit) {
		for i := from; i < to; i++ {
			tri := p.triangle(i)
			tlo := Point{min(tri[0].X, tri[1].X, tri[2].X), min(tri[0].Y, tri[1].Y, tri[2].Y)}
			thi := Point{max(tri[0].X, tri[1].X, tri[2].X), max(tri[0].Y, tri[1].Y, tri[2].Y)}
			if thi.X < lo.X || tlo.X > hi.X || thi.Y < lo.Y || tlo.Y > hi.Y {
				continue
			}
			var plane func(Point) float64
			for y := cell(tlo.Y, lo.Y, ch); y <= cell(thi.Y, lo.Y, ch); y++ {
				for x := cell(tlo.X, lo.X, cw); x <= cell(thi.X, lo.X, cw); x++ {
					for _, c := range binned[start[y*side+x]:start[y*side+x+1]] {
						q := cols[c].q
						if q.X < tlo.X || q.X > thi.X || q.Y < tlo.Y || q.Y > thi.Y || !inTriangle(tri, q) {
							continue
						}
						if plane == nil {
							plane = planeOf(tri)
						}
						hits = append(hits, hit{c, int32(i), plane(q)})
					}
				}
			}
		}
		return hits
	}
	parts := make([][]hit, min(runtime.GOMAXPROCS(0), (len(p.pieces)+stackChunk-1)/stackChunk))
	var wg sync.WaitGroup
	for k := range parts {
		wg.Go(func() { parts[k] = collect(k*len(p.pieces)/len(parts), (k+1)*len(p.pieces)/len(parts)) })
	}
	wg.Wait()
	// Group the hits by column (counting sort), each column bottom up.
	perCol := make([]int32, len(cols)+1)
	n := 0
	for _, part := range parts {
		for _, h := range part {
			perCol[h.col+1]++
		}
		n += len(part)
	}
	for k := 1; k < len(perCol); k++ {
		perCol[k] += perCol[k-1]
	}
	grouped := make([]hit, n)
	next := slices.Clone(perCol[:len(cols)])
	for _, part := range parts {
		for _, h := range part {
			grouped[next[h.col]] = h
			next[h.col]++
		}
	}
	for c := range cols {
		over := grouped[perCol[c]:perCol[c+1]]
		if len(over) == 0 {
			continue
		}
		slices.SortFunc(over, func(a, b hit) int { return cmp.Or(cmp.Compare(a.z, b.z), cmp.Compare(a.piece, b.piece)) })
		layers, onTerrain := 1, false
		for k, h := range over {
			if k > 0 && h.z-over[k-1].z > LayerGap {
				layers++
			}
			onTerrain = onTerrain || p.pieces[h.piece].surface == scene.SurfaceTerrain
		}
		p.layers = max(p.layers, layers)
		if !onTerrain && over[0].piece == cols[c].piece {
			p.floored += p.pieces[cols[c].piece].area
		}
	}
}

// triangle is the floor triangle piece i lies in, counter-clockwise.
func (p *Profile) triangle(i int) [3]Spot {
	pc := p.pieces[i]
	if pc.clip >= 0 {
		return p.clipped[pc.clip].tri
	}
	vs := p.verts[pc.first:]
	return [3]Spot{vs[0], vs[1], vs[2]}
}

// inner is a point strictly inside outline o and inside piece i: the
// centroid of a whole triangle; for a cut piece, its centroid or else the
// centroid of one of its fan triangles. False when none of them is.
func (p *Profile) inner(i int, o Outline) (Point, bool) {
	pc := p.pieces[i]
	tri := p.triangle(i)
	if pc.clip < 0 {
		return Point{(tri[0].X + tri[1].X + tri[2].X) / 3, (tri[0].Y + tri[1].Y + tri[2].Y) / 3}, true
	}
	vs := p.verts[pc.first : pc.first+pc.n]
	good := func(q Point) bool {
		in, on := contains(o, q)
		return in && !on && inTriangle(tri, q)
	}
	if q := point(centroid(vs)); good(q) {
		return q, true
	}
	for k := 1; k+1 < len(vs); k++ {
		a, b, c := vs[0], vs[k], vs[k+1]
		if math.Abs((b.X-a.X)*(c.Y-a.Y)-(c.X-a.X)*(b.Y-a.Y)) <= 1e-9 {
			continue
		}
		if q := (Point{(a.X + b.X + c.X) / 3, (a.Y + b.Y + c.Y) / 3}); good(q) {
			return q, true
		}
	}
	return Point{}, false
}
