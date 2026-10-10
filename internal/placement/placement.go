// Package placement spreads the spawn points of an area over the free
// floor under it (spec D3): a grid of 16-unit cells inside the outline,
// each standing on terrain or BSP floor within the area's Z range, out of
// water and clear of the static meshes and BSP walls around it, sampled by
// Mitchell's best candidate with a seeded generator. Everything is in
// server coordinates. No GL, no Gio.
//
// The result depends only on the geometry's triangles and water volumes,
// not on the order Geometry hands them out (which follows the order the
// tiles loaded in): the floor is the highest one, a cell is blocked by any
// obstacle, and the cells are walked in grid order.
package placement

import (
	"math"
	"math/rand/v2"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
)

const (
	// CellSize is the side of a candidate cell, 1 geodata cell, in world
	// units. Cells are aligned on multiples of it.
	CellSize = 16
	// FloorNormalZ is the least Z of the unit normal of a terrain or BSP
	// face a monster may stand on: the Play Map's limit.
	FloorNormalZ = 0.65
	// SliceBelow is how far below a cell's floor its obstacle slice starts.
	SliceBelow = 16
	// Candidates is how many cells Mitchell's best candidate draws for
	// each point.
	Candidates = 20
)

// Geometry is where the scene comes from: scene.World, or a synthetic
// scene in tests. Geometry calls tri with every triangle whose X/Y extent
// may meet the X/Y of box, and water with every water volume that may,
// all in server coordinates; extra ones outside box are allowed. The order
// does not matter.
type Geometry interface {
	Geometry(box geom.Box, tri func(scene.Triangle), water func(scene.WaterVolume))
}

// Vertex is a point of an outline on the X/Y plane, in server coordinates.
type Vertex struct{ X, Y float64 }

// Request is an area to fill: its outline (closed from the last vertex
// back to the first, either winding, maybe concave) and Z range, how many
// points, the monster's collision radius and height, the clearance kept
// from obstacles on top of the radius, and the generator's seed.
type Request struct {
	Outline    []Vertex
	ZMin, ZMax float64
	Count      int
	Radius     float64
	Clearance  float64
	Height     float64
	Seed       uint64
}

// Point is a spawn point in server coordinates, with its heading in
// 1..65535 (65536 is a full turn).
type Point struct{ X, Y, Z, Heading int }

// Result is a distribution: the points, in the order they were drawn,
// the area of free floor they were drawn from (units²), and the mean and
// least distance from each point to its nearest neighbour (0 with fewer
// than 2 points).
type Result struct {
	Points      []Point
	FreeArea    float64
	MeanSpacing float64
	MinSpacing  float64
}

// Fit is how many points fit, the K of "K of N fit": fewer than the
// request's Count when the free floor ran out first.
func (r Result) Fit() int { return len(r.Points) }

// jitterReach is the farthest a point may move from its cell's centre
// (half the cell's diagonal), plus 1 for rounding to whole units: the
// clearance a cell needs on top of the rules for its point to move
// freely.
const jitterReach = CellSize/2*math.Sqrt2 + 1

// Distribute spreads r.Count points over the free floor of g under r's
// outline, or as many as fit (spec D3):
//
//   - a cell is a candidate when its centre lies inside the outline, at
//     least Radius from its edges;
//   - its floor is the highest terrain or BSP face (not a water sheet)
//     with a normal Z of FloorNormalZ or more, between ZMin and ZMax, at
//     its centre: never a static mesh;
//   - a cell whose floor is under the water of a volume (the vertical line
//     through its centre crosses the volume above the floor, lower than
//     Height above it) is left out;
//   - a static mesh triangle, or a BSP face that is not floor, blocks the
//     cell when its part between SliceBelow under the floor and Height
//     above it comes within Radius + Clearance of the centre on the X/Y
//     plane: a free cell's centre lies farther;
//   - each point is the farthest from the points so far of Candidates
//     free cells drawn at random, never nearer than 2 Radius to any of
//     them, moved at random within its cell (no farther than keeps it
//     clear of the edges and obstacles, and never into water) and rounded
//     to whole units, its Z on the cell's floor plane, and its heading
//     drawn too, all from one PCG generator seeded with r.Seed;
//   - when no free cell is left 2 Radius from the points, the points so
//     far are the result: Fit < Count.
//
// The same geometry and request give the same points.
func Distribute(g Geometry, r Request) Result {
	if len(r.Outline) < 3 || r.Count < 1 {
		return Result{}
	}
	gr := newGrid(r)
	gr.mark(r)
	gr.scan(g, r)
	return gr.sample(r)
}

// cell is one candidate cell: its state, its floor (Z at the centre and
// the floor plane's slope along X and Y) and its slack, how far its
// centre may move and still keep the border and obstacle rules.
type cell struct {
	state          state
	z, gx, gy      float32
	slack          float32
	px, py, pz     int32 // the point drawn in the cell, once drawn
	drawn, claimed bool
}

type state uint8

const (
	out     state = iota // outside the outline or too near its edges
	noFloor              // inside, with no floor in range
	floored              // inside, on floor
	blocked              // on floor, under water or by an obstacle
)

// grid is the cells over the outline's box: nx × ny of them, row by row,
// cell (i, j)'s centre at (x0 + 16i + 8, y0 + 16j + 8).
type grid struct {
	x0, y0 float64
	nx, ny int
	cells  []cell
	// water is the water volumes around the outline.
	water []scene.WaterVolume
	// height is the monster's height.
	height float64
}

func newGrid(r Request) *grid {
	lo, hi := outlineBox(r.Outline)
	x0 := math.Floor(lo.X/CellSize) * CellSize
	y0 := math.Floor(lo.Y/CellSize) * CellSize
	nx := int(math.Ceil((hi.X - x0) / CellSize))
	ny := int(math.Ceil((hi.Y - y0) / CellSize))
	return &grid{x0: x0, y0: y0, nx: nx, ny: ny, cells: make([]cell, nx*ny), height: r.Height}
}

func outlineBox(o []Vertex) (lo, hi Vertex) {
	lo, hi = o[0], o[0]
	for _, p := range o[1:] {
		lo = Vertex{math.Min(lo.X, p.X), math.Min(lo.Y, p.Y)}
		hi = Vertex{math.Max(hi.X, p.X), math.Max(hi.Y, p.Y)}
	}
	return lo, hi
}

// centre is the centre of cell (i, j).
func (gr *grid) centre(i, j int) (float64, float64) {
	return gr.x0 + float64(i)*CellSize + CellSize/2, gr.y0 + float64(j)*CellSize + CellSize/2
}

// span is the range of cell indices along an axis starting at origin, of
// n cells, whose centres lie in [lo, hi]; empty (first > last) when none
// does.
func span(lo, hi, origin float64, n int) (first, last int) {
	first = max(int(math.Ceil((lo-origin-CellSize/2)/CellSize)), 0)
	last = min(int(math.Floor((hi-origin-CellSize/2)/CellSize)), n-1)
	return first, last
}

// mark sets the cells whose centre lies inside the outline at least
// r.Radius from its edges to noFloor, with that much slack.
func (gr *grid) mark(r Request) {
	o := r.Outline
	for j := range gr.ny {
		for i := range gr.nx {
			x, y := gr.centre(i, j)
			in := false
			d := math.Inf(1)
			for k, a := range o {
				b := o[(k+1)%len(o)]
				if (a.Y > y) != (b.Y > y) && x < a.X+(y-a.Y)*(b.X-a.X)/(b.Y-a.Y) {
					in = !in
				}
				d = math.Min(d, segmentDistance(x, y, a.X, a.Y, b.X, b.Y))
			}
			if in && d >= r.Radius {
				c := &gr.cells[j*gr.nx+i]
				c.state, c.slack = noFloor, float32(d-r.Radius)
			}
		}
	}
}

// scan reads g around the outline: the floor of every cell, then the
// water and the obstacles that block cells.
func (gr *grid) scan(g Geometry, r Request) {
	reach := r.Radius + r.Clearance + jitterReach
	box := geom.Box{
		Min: geom.Vec3{X: float32(gr.x0 - reach), Y: float32(gr.y0 - reach), Z: float32(r.ZMin - SliceBelow)},
		Max: geom.Vec3{
			X: float32(gr.x0 + float64(gr.nx)*CellSize + reach),
			Y: float32(gr.y0 + float64(gr.ny)*CellSize + reach),
			Z: float32(r.ZMax + r.Height),
		},
	}
	var obstacles []scene.Triangle
	g.Geometry(box, func(t scene.Triangle) {
		switch {
		case isFloor(t):
			gr.floor(t, r)
		case t.Surface == scene.SurfaceMesh || t.Surface == scene.SurfaceBSP && float64(t.Normal.Z) < FloorNormalZ:
			obstacles = append(obstacles, t)
		}
	}, func(v scene.WaterVolume) { gr.water = append(gr.water, v) })
	for k := range gr.water {
		gr.flood(&gr.water[k])
	}
	for _, t := range obstacles {
		gr.obstacle(t, r)
	}
}

// flood blocks the floored cells whose floor lies under v's water at
// their centre.
func (gr *grid) flood(v *scene.WaterVolume) {
	i0, i1 := span(float64(v.Bounds.Min.X), float64(v.Bounds.Max.X), gr.x0, gr.nx)
	j0, j1 := span(float64(v.Bounds.Min.Y), float64(v.Bounds.Max.Y), gr.y0, gr.ny)
	for j := j0; j <= j1; j++ {
		for i := i0; i <= i1; i++ {
			c := &gr.cells[j*gr.nx+i]
			if c.state != floored {
				continue
			}
			x, y := gr.centre(i, j)
			if gr.wet(v, x, y, float64(c.z)) {
				c.state = blocked
			}
		}
	}
}

// wet reports floor at (x, y, z) under v's water: the vertical line
// through (x, y) crosses v, above z and less than the monster's height
// above it (a monster standing there would be in the water).
func (gr *grid) wet(v *scene.WaterVolume, x, y, z float64) bool {
	lo, hi, ok := column(v.Planes, x, y)
	return ok && z < hi && lo < z+gr.height
}

// column is the Z range where the vertical line through (x, y) runs
// inside the convex volume of planes; false when it misses it.
func column(planes []scene.Plane, x, y float64) (lo, hi float64, ok bool) {
	lo, hi = math.Inf(-1), math.Inf(1)
	for _, p := range planes {
		nx, ny, nz := float64(p.Normal.X), float64(p.Normal.Y), float64(p.Normal.Z)
		// Inside: nz·z ≤ room.
		room := float64(p.D) - nx*x - ny*y
		switch {
		case math.Abs(nz) < 1e-9:
			if room < 0 {
				return 0, 0, false // outside a wall
			}
		case nz > 0:
			hi = math.Min(hi, room/nz)
		default:
			lo = math.Max(lo, room/nz)
		}
	}
	return lo, hi, lo <= hi
}

// obstacle blocks the floored cells t's part in their slice comes within
// r.Radius + r.Clearance of (free cells lie farther), and takes from the
// others' slack how far beyond that it lies. Cells farther than that and
// the jitter reach on either axis (t's box expanded by them) are left
// alone up front.
func (gr *grid) obstacle(t scene.Triangle, r Request) {
	keep := r.Radius + r.Clearance
	reach := keep + jitterReach
	tri := [3][3]float64{
		{float64(t.A.X), float64(t.A.Y), float64(t.A.Z)},
		{float64(t.B.X), float64(t.B.Y), float64(t.B.Z)},
		{float64(t.C.X), float64(t.C.Y), float64(t.C.Z)},
	}
	zlo := min(tri[0][2], tri[1][2], tri[2][2])
	zhi := max(tri[0][2], tri[1][2], tri[2][2])
	i0, i1 := span(min(tri[0][0], tri[1][0], tri[2][0])-reach, max(tri[0][0], tri[1][0], tri[2][0])+reach, gr.x0, gr.nx)
	j0, j1 := span(min(tri[0][1], tri[1][1], tri[2][1])-reach, max(tri[0][1], tri[1][1], tri[2][1])+reach, gr.y0, gr.ny)
	for j := j0; j <= j1; j++ {
		for i := i0; i <= i1; i++ {
			c := &gr.cells[j*gr.nx+i]
			if c.state != floored {
				continue
			}
			lo, hi := float64(c.z)-SliceBelow, float64(c.z)+r.Height
			if zhi < lo || zlo > hi {
				continue
			}
			x, y := gr.centre(i, j)
			d, ok := sliceDistance(tri, lo, hi, x, y)
			switch {
			case !ok:
			case d <= keep:
				c.state = blocked
			default:
				c.slack = min(c.slack, float32(d-keep))
			}
		}
	}
}

// sliceDistance is the distance on the X/Y plane from (x, y) to the part
// of triangle tri between heights lo and hi; false when no part of it is.
func sliceDistance(tri [3][3]float64, lo, hi, x, y float64) (float64, bool) {
	var above, between [8][3]float64
	poly := clipZ(above[:0], tri[:], lo, false)
	poly = clipZ(between[:0], poly, hi, true)
	if len(poly) == 0 {
		return 0, false
	}
	d := math.Inf(1)
	pos, neg := false, false
	for k, a := range poly {
		b := poly[(k+1)%len(poly)]
		d = math.Min(d, segmentDistance(x, y, a[0], a[1], b[0], b[1]))
		switch side := (b[0]-a[0])*(y-a[1]) - (b[1]-a[1])*(x-a[0]); {
		case side > 0:
			pos = true
		case side < 0:
			neg = true
		}
	}
	if len(poly) >= 3 && pos != neg {
		return 0, true // inside its shadow on the X/Y plane
	}
	return d, true
}

// clipZ appends to out the convex polygon poly cut to its part with
// Z ≥ z, or ≤ z when below is set (Sutherland-Hodgman against one
// plane): 1 vertex more than poly at most.
func clipZ(out, poly [][3]float64, z float64, below bool) [][3]float64 {
	in := func(p [3]float64) bool { return (p[2] >= z) != below || p[2] == z }
	for k, a := range poly {
		b := poly[(k+1)%len(poly)]
		if in(a) {
			out = append(out, a)
		}
		if in(a) != in(b) {
			s := (z - a[2]) / (b[2] - a[2])
			out = append(out, [3]float64{a[0] + s*(b[0]-a[0]), a[1] + s*(b[1]-a[1]), z})
		}
	}
	return out
}

// isFloor reports a triangle monsters may stand on: terrain or BSP, not a
// water sheet, facing up enough.
func isFloor(t scene.Triangle) bool {
	return (t.Surface == scene.SurfaceTerrain || t.Surface == scene.SurfaceBSP) && !t.Water && float64(t.Normal.Z) >= FloorNormalZ
}

// floor raises the floor of the cells whose centre t covers on the X/Y
// plane to t's Z there, when it lies in r's range and above the floor so
// far. Equal heights keep the plane that sorts first, so the order the
// triangles come in does not matter.
func (gr *grid) floor(t scene.Triangle, r Request) {
	ax, ay, az := float64(t.A.X), float64(t.A.Y), float64(t.A.Z)
	bx, by := float64(t.B.X), float64(t.B.Y)
	cx, cy := float64(t.C.X), float64(t.C.Y)
	i0, i1 := span(min(ax, bx, cx), max(ax, bx, cx), gr.x0, gr.nx)
	j0, j1 := span(min(ay, by, cy), max(ay, by, cy), gr.y0, gr.ny)
	nz := float64(t.Normal.Z)
	gx, gy := float32(-float64(t.Normal.X)/nz), float32(-float64(t.Normal.Y)/nz)
	for j := j0; j <= j1; j++ {
		for i := i0; i <= i1; i++ {
			c := &gr.cells[j*gr.nx+i]
			if c.state == out {
				continue
			}
			x, y := gr.centre(i, j)
			if !inTriangle(x, y, ax, ay, bx, by, cx, cy) {
				continue
			}
			z := az + float64(gx)*(x-ax) + float64(gy)*(y-ay)
			if z < r.ZMin || z > r.ZMax {
				continue
			}
			z32 := float32(z)
			if c.state == floored && (z32 < c.z || z32 == c.z && (gx > c.gx || gx == c.gx && gy >= c.gy)) {
				continue
			}
			c.state, c.z, c.gx, c.gy = floored, z32, gx, gy
		}
	}
}

// inTriangle reports whether (x, y) lies in the triangle a, b, c on the
// X/Y plane, edges included, whichever way it winds; never for a triangle
// seen edge-on.
func inTriangle(x, y, ax, ay, bx, by, cx, cy float64) bool {
	d1 := (bx-ax)*(y-ay) - (by-ay)*(x-ax)
	d2 := (cx-bx)*(y-by) - (cy-by)*(x-bx)
	d3 := (ax-cx)*(y-cy) - (ay-cy)*(x-cx)
	if d1 == 0 && d2 == 0 && d3 == 0 {
		return false
	}
	return (d1 >= 0 && d2 >= 0 && d3 >= 0) || (d1 <= 0 && d2 <= 0 && d3 <= 0)
}

// segmentDistance is the distance from (x, y) to the segment a-b.
func segmentDistance(x, y, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = math.Max(0, math.Min(1, ((x-ax)*dx+(y-ay)*dy)/l))
	}
	return math.Hypot(x-(ax+t*dx), y-(ay+t*dy))
}

// sample draws the points over the floored cells (Mitchell's best
// candidate) and measures them.
func (gr *grid) sample(r Request) Result {
	var free []int32
	for k := range gr.cells {
		if gr.cells[k].state == floored {
			free = append(free, int32(k))
		}
	}
	res := Result{FreeArea: float64(len(free)) * CellSize * CellSize}
	rng := rand.New(rand.NewPCG(r.Seed, 0))
	minD2 := 4 * r.Radius * r.Radius
	avail := free
	for len(res.Points) < r.Count && len(avail) > 0 {
		best, bestD2 := int32(-1), -1.0
		for got := 0; got < Candidates && len(avail) > 0; {
			k := rng.IntN(len(avail))
			ci := avail[k]
			c := &gr.cells[ci]
			if !c.drawn {
				gr.draw(ci, rng)
			}
			d2 := nearest(res.Points, c.px, c.py)
			if c.claimed || d2 < minD2 {
				// Points only add up: this cell is out for good.
				last := len(avail) - 1
				avail[k] = avail[last]
				avail = avail[:last]
				continue
			}
			got++
			if d2 > bestD2 {
				best, bestD2 = ci, d2
			}
		}
		if best < 0 {
			break
		}
		c := &gr.cells[best]
		c.claimed = true
		res.Points = append(res.Points, Point{X: int(c.px), Y: int(c.py), Z: int(c.pz), Heading: 1 + rng.IntN(65535)})
	}
	res.MeanSpacing, res.MinSpacing = Spacing(res.Points)
	return res
}

// draw moves cell ci's point at random within the cell, no farther from
// its centre than its slack less 1 allows, onto the cell's floor plane,
// and rounds it to whole units; back to the centre when that lands it in
// water the centre is clear of.
func (gr *grid) draw(ci int32, rng *rand.Rand) {
	c := &gr.cells[ci]
	i, j := int(ci)%gr.nx, int(ci)/gr.nx
	x, y := gr.centre(i, j)
	dx, dy := (rng.Float64()-0.5)*CellSize, (rng.Float64()-0.5)*CellSize
	if l, room := math.Hypot(dx, dy), math.Max(float64(c.slack)-1, 0); l > room {
		dx, dy = dx*room/l, dy*room/l
	}
	px, py := math.Round(x+dx), math.Round(y+dy)
	pz := float64(c.z) + float64(c.gx)*(px-x) + float64(c.gy)*(py-y)
	for k := range gr.water {
		if v := &gr.water[k]; gr.wet(v, px, py, pz) {
			px, py, pz = x, y, float64(c.z)
			break
		}
	}
	c.px, c.py, c.pz = int32(px), int32(py), int32(math.Round(pz))
	c.drawn = true
}

// nearest is the squared distance from (x, y) to the nearest of pts, +Inf
// with none.
func nearest(pts []Point, x, y int32) float64 {
	d := math.Inf(1)
	for _, p := range pts {
		dx, dy := float64(p.X)-float64(x), float64(p.Y)-float64(y)
		d = math.Min(d, dx*dx+dy*dy)
	}
	return d
}

// Spacing is the mean and least X/Y distance from each of pts to its
// nearest neighbour, 0 and 0 with fewer than 2: Result's MeanSpacing and
// MinSpacing, and the same numbers for points adjusted by hand.
func Spacing(pts []Point) (mean, least float64) {
	if len(pts) < 2 {
		return 0, 0
	}
	least = math.Inf(1)
	for k, p := range pts {
		d2 := math.Inf(1)
		for j, q := range pts {
			if j != k {
				dx, dy := float64(q.X)-float64(p.X), float64(q.Y)-float64(p.Y)
				d2 = math.Min(d2, dx*dx+dy*dy)
			}
		}
		d := math.Sqrt(d2)
		mean += d
		least = math.Min(least, d)
	}
	return mean / float64(len(pts)), least
}
