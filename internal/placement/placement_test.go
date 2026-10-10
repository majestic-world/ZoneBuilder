package placement_test

import (
	"math"
	"slices"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/placement"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/scene/scenetest"
)

// world is a synthetic Geometry: its triangles and water volumes, in
// server coordinates, handed out whatever the box. Reversed hands them
// out in the opposite order, as a world whose tiles loaded the other way
// round would.
type world struct {
	tris     []scene.Triangle
	water    []scene.WaterVolume
	reversed bool
}

func (w *world) Geometry(box geom.Box, tri func(scene.Triangle), water func(scene.WaterVolume)) {
	tris, vols := w.tris, w.water
	if w.reversed {
		tris, vols = slices.Clone(tris), slices.Clone(vols)
		slices.Reverse(tris)
		slices.Reverse(vols)
	}
	if tri != nil {
		for _, t := range tris {
			tri(t)
		}
	}
	if water != nil {
		for _, v := range vols {
			water(v)
		}
	}
}

func v(x, y, z float32) geom.Vec3 { return geom.Vec3{X: x, Y: y, Z: z} }

// tri is a triangle of surface s facing its right-hand normal.
func tri(s scene.Surface, a, b, c geom.Vec3) scene.Triangle {
	return scene.Triangle{A: a, B: b, C: c, Surface: s, Normal: b.Sub(a).Cross(c.Sub(a)).Normalize()}
}

// ground is flat terrain at height z over [x0, x1] × [y0, y1].
func ground(x0, y0, x1, y1, z float32) []scene.Triangle {
	a, b, c, d := v(x0, y0, z), v(x1, y0, z), v(x1, y1, z), v(x0, y1, z)
	return []scene.Triangle{tri(scene.SurfaceTerrain, a, b, c), tri(scene.SurfaceTerrain, a, c, d)}
}

// solid is the 12 triangles of the box lo..hi, of surface s, facing out.
func solid(s scene.Surface, lo, hi geom.Vec3) []scene.Triangle {
	c := [8]geom.Vec3{
		v(lo.X, lo.Y, lo.Z), v(hi.X, lo.Y, lo.Z), v(hi.X, hi.Y, lo.Z), v(lo.X, hi.Y, lo.Z),
		v(lo.X, lo.Y, hi.Z), v(hi.X, lo.Y, hi.Z), v(hi.X, hi.Y, hi.Z), v(lo.X, hi.Y, hi.Z),
	}
	var out []scene.Triangle
	// Each quad counter-clockwise seen from outside.
	for _, q := range [][4]int{{3, 2, 1, 0}, {4, 5, 6, 7}, {0, 1, 5, 4}, {1, 2, 6, 5}, {2, 3, 7, 6}, {3, 0, 4, 7}} {
		out = append(out, tri(s, c[q[0]], c[q[1]], c[q[2]]), tri(s, c[q[0]], c[q[2]], c[q[3]]))
	}
	return out
}

// square is the outline [x0, x1] × [y0, y1].
func square(x0, y0, x1, y1 float64) []placement.Vertex {
	return []placement.Vertex{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
}

// request is a request over outline with a monster of radius 16 and
// height 80, a clearance of 32 and seed 1, the range [-100, 100].
func request(outline []placement.Vertex, n int) placement.Request {
	return placement.Request{Outline: outline, ZMin: -100, ZMax: 100, Count: n, Radius: 16, Clearance: 32, Height: 80, Seed: 1}
}

// inside reports whether (x, y) lies inside outline (crossing count).
func inside(outline []placement.Vertex, x, y float64) bool {
	in := false
	for i, a := range outline {
		b := outline[(i+1)%len(outline)]
		if (a.Y > y) != (b.Y > y) && x < a.X+(y-a.Y)*(b.X-a.X)/(b.Y-a.Y) {
			in = !in
		}
	}
	return in
}

// segment is the distance from (x, y) to the segment a-b.
func segment(x, y, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = math.Max(0, math.Min(1, ((x-ax)*dx+(y-ay)*dy)/l))
	}
	return math.Hypot(x-(ax+t*dx), y-(ay+t*dy))
}

// border is the distance from (x, y) to outline's edges.
func border(outline []placement.Vertex, x, y float64) float64 {
	d := math.Inf(1)
	for i, a := range outline {
		b := outline[(i+1)%len(outline)]
		d = math.Min(d, segment(x, y, a.X, a.Y, b.X, b.Y))
	}
	return d
}

// spaced fails t unless every pair of pts is at least 2r apart.
func spaced(t *testing.T, pts []placement.Point, r float64) {
	t.Helper()
	for i := range pts {
		for j := i + 1; j < len(pts); j++ {
			if d := math.Hypot(float64(pts[i].X-pts[j].X), float64(pts[i].Y-pts[j].Y)); d < 2*r {
				t.Fatalf("points %v and %v %.2f apart, want ≥ %v", pts[i], pts[j], d, 2*r)
			}
		}
	}
}

// lShape is the concave L of [0, 600]² without its corner [300, 600]².
var lShape = []placement.Vertex{{X: 0, Y: 0}, {X: 600, Y: 0}, {X: 600, Y: 300}, {X: 300, Y: 300}, {X: 300, Y: 600}, {X: 0, Y: 600}}

// On a flat field with no obstacle, a few points all fit: each inside the
// L, at least 1 radius from its edges (the notch's too), on the ground,
// with a heading the server takes, and every pair 2 radii apart. Catches
// an ignored border, a convex outline test, or overlapping points.
func TestFlatFieldPlacesEveryPointInsideAndApart(t *testing.T) {
	w := &world{tris: ground(-1000, -1000, 2000, 2000, 40)}
	r := request(lShape, 12)
	got := placement.Distribute(w, r)
	if len(got.Points) != 12 || got.Fit() != 12 {
		t.Fatalf("%d points (fit %d), want 12", len(got.Points), got.Fit())
	}
	for _, p := range got.Points {
		x, y := float64(p.X), float64(p.Y)
		if !inside(lShape, x, y) || border(lShape, x, y) < r.Radius {
			t.Errorf("point %v outside the L or nearer than %v to its edges", p, r.Radius)
		}
		if p.Z != 40 {
			t.Errorf("point %v off the ground at z 40", p)
		}
		if p.Heading < 1 || p.Heading > 65535 {
			t.Errorf("point %v heading outside 1..65535", p)
		}
	}
	spaced(t, got.Points, r.Radius)
}

// The same geometry and request give the same points even when the
// triangles come in the opposite order (tiles loaded the other way
// round), and another seed gives other points. The scene has 2 floors
// meeting at the same height over a ramp's foot and a BSP block, so the
// order would show. Catches a distribution that depends on the order of
// the geometry, on Go map order, or ignores the seed.
func TestSameSeedSamePointsWhateverTheOrder(t *testing.T) {
	tris := ground(-1000, -1000, 300, 2000, 0)
	// A ramp rising from z 0 at x 300, its foot shared with the ground.
	a, b, c, d := v(300, -1000, 0), v(2000, -1000, 400), v(2000, 2000, 400), v(300, 2000, 0)
	tris = append(tris, tri(scene.SurfaceTerrain, a, b, c), tri(scene.SurfaceTerrain, a, c, d))
	tris = append(tris, solid(scene.SurfaceBSP, v(100, 100, -20), v(200, 300, 30))...)
	r := request(square(0, 0, 600, 600), 30)
	first := placement.Distribute(&world{tris: tris}, r)
	again := placement.Distribute(&world{tris: tris, reversed: true}, r)
	if len(first.Points) == 0 || !slices.Equal(first.Points, again.Points) {
		t.Fatalf("seed %d gave\n%v\nthen, in the opposite order,\n%v", r.Seed, first.Points, again.Points)
	}
	r.Seed = 2
	other := placement.Distribute(&world{tris: tris}, r)
	if slices.Equal(first.Points, other.Points) {
		t.Errorf("seeds 1 and 2 gave the same points %v", first.Points)
	}
}

// rectDistance is the distance from (x, y) to the rectangle [x0, x1] ×
// [y0, y1] on the X/Y plane, 0 inside.
func rectDistance(x, y, x0, y0, x1, y1 float64) float64 {
	return math.Hypot(math.Max(0, math.Max(x0-x, x-x1)), math.Max(0, math.Max(y0-y, y-y1)))
}

// A mesh block in the middle of a field keeps every point more than
// radius + clearance (48) from its sides, yet points crowd up to that
// line; a mesh wall just outside the outline's west edge keeps the points
// 48 from it, inside. Catches the clearance counted from the mesh's
// centre (points come nearer its sides) or obstacles looked for only
// inside the outline (points at the radius from the west edge).
func TestMeshesBlockRadiusPlusClearanceAroundThem(t *testing.T) {
	tris := ground(-1000, -1000, 2000, 2000, 0)
	tris = append(tris, solid(scene.SurfaceMesh, v(400, 400, -10), v(600, 600, 150))...)
	tris = append(tris, solid(scene.SurfaceMesh, v(-30, -200, -10), v(-10, 1200, 150))...)
	r := request(square(0, 0, 1024, 1024), 600)
	got := placement.Distribute(&world{tris: tris}, r)
	reach := r.Radius + r.Clearance
	nearBlock, nearWall := false, false
	for _, p := range got.Points {
		x, y := float64(p.X), float64(p.Y)
		d := rectDistance(x, y, 400, 400, 600, 600)
		if d <= reach {
			t.Errorf("point %v %.2f from the block, want > %v", p, d, reach)
		}
		nearBlock = nearBlock || d < reach+CellSize
		if w := x - -10; w <= reach {
			t.Errorf("point %v %.2f from the wall outside, want > %v", p, w, reach)
		} else if w < reach+CellSize {
			nearWall = true
		}
	}
	if !nearBlock || !nearWall {
		t.Errorf("no point within a cell of the cleared line (block %v, wall %v): too much blocked", nearBlock, nearWall)
	}
	spaced(t, got.Points, r.Radius)
}

// A tree: a canopy of mesh from 100 above the ground (over the monster's
// 80) up to 300, wide over the field's west half, and a trunk of mesh
// cutting the slice in the east half. Points come under the canopy, none
// near the trunk. Catches the slice ignored (canopy blocks) or cut too
// thin (trunk lets through).
func TestCanopyAboveTheMonsterDoesNotBlockButTrunkDoes(t *testing.T) {
	tris := ground(-1000, -1000, 2000, 2000, 0)
	tris = append(tris, solid(scene.SurfaceMesh, v(-100, -100, 100), v(500, 1100, 300))...)
	tris = append(tris, solid(scene.SurfaceMesh, v(740, 490, -50), v(760, 510, 400))...)
	r := request(square(0, 0, 1000, 1000), 300)
	got := placement.Distribute(&world{tris: tris}, r)
	under := 0
	for _, p := range got.Points {
		if p.X < 500 {
			under++
		}
		if d := rectDistance(float64(p.X), float64(p.Y), 740, 490, 760, 510); d <= r.Radius+r.Clearance {
			t.Errorf("point %v %.2f from the trunk, want > %v", p, d, r.Radius+r.Clearance)
		}
	}
	if under < 50 {
		t.Errorf("%d points under the canopy, want the west half's share of %d", under, len(got.Points))
	}
}

// A ramp whose normal Z is cos θ.
func ramp(x0, x1 float32, nz float64) []scene.Triangle {
	slope := float32(math.Tan(math.Acos(nz)))
	a, b, c, d := v(x0, -1000, 0), v(x1, -1000, (x1-x0)*slope), v(x1, 2000, (x1-x0)*slope), v(x0, 2000, 0)
	return []scene.Triangle{tri(scene.SurfaceTerrain, a, b, c), tri(scene.SurfaceTerrain, a, c, d)}
}

// A flat mesh floor, alone in the range, gets no point; nor does terrain
// as steep as normal Z 0.6 (between the coverage floor's 0.5 and the
// Play Map's 0.65), while terrain at normal Z 0.7 does. Catches mesh
// floor taken as floor, or the slope limit swapped.
func TestNoPointOnMeshFloorNorOnSteepTerrain(t *testing.T) {
	r := request(square(0, 0, 400, 400), 20)
	r.ZMin, r.ZMax = -1000, 10000
	for _, c := range []struct {
		name string
		tris []scene.Triangle
		want bool
	}{
		{"mesh floor", []scene.Triangle{
			tri(scene.SurfaceMesh, v(-1000, -1000, 0), v(2000, -1000, 0), v(2000, 2000, 0)),
			tri(scene.SurfaceMesh, v(-1000, -1000, 0), v(2000, 2000, 0), v(-1000, 2000, 0)),
		}, false},
		{"terrain at normal z 0.6", ramp(-1000, 2000, 0.6), false},
		{"terrain at normal z 0.7", ramp(-1000, 2000, 0.7), true},
	} {
		got := placement.Distribute(&world{tris: c.tris}, r)
		if placed := len(got.Points) > 0; placed != c.want || (got.FreeArea > 0) != c.want {
			t.Errorf("%s: %d points over %v units² free, want points %v", c.name, len(got.Points), got.FreeArea, c.want)
		}
	}
}

// A water volume over the field's west half, its top above the ground,
// leaves that half without points. Catches water ignored.
func TestWaterLeavesItsHalfOfTheFieldEmpty(t *testing.T) {
	pool := scenetest.Volume(t, scene.Tile{X: 20, Y: 18}, 1, "pool", scenetest.UnitBrush(),
		scenetest.Hexahedron(scenetest.Box(v(-500, -500, -200), v(500, 1500, 30))))
	w := &world{tris: ground(-1000, -1000, 2000, 2000, 0), water: []scene.WaterVolume{pool}}
	r := request(square(0, 0, 1000, 1000), 200)
	got := placement.Distribute(w, r)
	dry := 0
	for _, p := range got.Points {
		if p.X < 500 {
			t.Errorf("point %v in the water (x < 500)", p)
		} else {
			dry++
		}
	}
	if dry < 50 {
		t.Errorf("%d points on the dry half, want it filled", dry)
	}
}

// A BSP bridge 20 thick, its deck at 300, spans the field's middle over
// ground at 0. With the range around the ground every point stands on the
// ground, under the bridge too (its underside is far above the
// monster); with the range around the deck every point stands on the
// deck, clear of its edges. Catches the wrong layer: the highest floor
// taken whatever the range, or the range ignored.
func TestBridgeRangePicksTheLayer(t *testing.T) {
	tris := ground(-1000, -1000, 2000, 2000, 0)
	tris = append(tris, solid(scene.SurfaceBSP, v(-200, 400, 280), v(1200, 600, 300))...)
	w := &world{tris: tris}
	r := request(square(0, 0, 1000, 1000), 100)
	r.ZMin, r.ZMax = -50, 100
	got := placement.Distribute(w, r)
	under := 0
	for _, p := range got.Points {
		if p.Z != 0 {
			t.Errorf("range on the ground: point %v off the ground", p)
		}
		if p.Y > 400 && p.Y < 600 {
			under++
		}
	}
	if len(got.Points) != 100 || under == 0 {
		t.Errorf("range on the ground: %d points, %d under the bridge, want 100 and some under it", len(got.Points), under)
	}
	r.ZMin, r.ZMax = 250, 350
	got = placement.Distribute(w, r)
	for _, p := range got.Points {
		if p.Z != 300 || float64(p.Y) <= 400+r.Radius+r.Clearance || float64(p.Y) >= 600-r.Radius-r.Clearance {
			t.Errorf("range on the deck: point %v off the deck or near its edges", p)
		}
	}
	if len(got.Points) < 10 {
		t.Errorf("range on the deck: %d points, want the deck filled", len(got.Points))
	}
}

// An area room for a few points only gives those few, apart, and says so
// with its fit; an area too small for any cell gives none. Catches the
// spacing broken to place all N, or a loop that never gives up.
func TestTooSmallAreaGivesWhatFits(t *testing.T) {
	w := &world{tris: ground(-1000, -1000, 2000, 2000, 0)}
	r := request(square(0, 0, 96, 96), 10)
	got := placement.Distribute(w, r)
	if got.Fit() < 1 || got.Fit() >= r.Count {
		t.Fatalf("%d points fit in 96×96, want between 1 and %d", got.Fit(), r.Count-1)
	}
	spaced(t, got.Points, r.Radius)
	if got.MinSpacing < 2*r.Radius && got.Fit() > 1 {
		t.Errorf("least spacing %v, want ≥ %v", got.MinSpacing, 2*r.Radius)
	}
	r.Outline = square(0, 0, 20, 20)
	if got := placement.Distribute(w, r); got.Fit() != 0 || got.FreeArea != 0 {
		t.Errorf("20×20: %d points over %v units², want none", got.Fit(), got.FreeArea)
	}
}

// The inspector's spacing of an area's points after hand edits is the
// mean and least X/Y distance from each point to its nearest neighbour,
// worked out by hand for 3 points: nearest 30, 30 and 40. Catches the Z of
// a point dragged up a slope counted as distance, or a point taken as its
// own neighbour (least spacing 0).
func TestSpacingOfHandPlacedPointsIsNearestNeighbourOnThePlane(t *testing.T) {
	pts := []placement.Point{{X: 0, Y: 0, Z: 0}, {X: 30, Y: 0, Z: 500}, {X: 30, Y: 40, Z: 0}}
	mean, least := placement.Spacing(pts)
	if math.Abs(mean-100.0/3) > 1e-9 || least != 30 {
		t.Errorf("spacing mean %v least %v, want 33.33 and 30", mean, least)
	}
	if mean, least := placement.Spacing(pts[:1]); mean != 0 || least != 0 {
		t.Errorf("1 point: spacing %v %v, want 0 0", mean, least)
	}
}

// CellSize is placement.CellSize as a float.
const CellSize = float64(placement.CellSize)

func TestFinalPointsRemainSupportedOnNarrowBridgeAndInRampRange(t *testing.T) {
	for _, ramp := range []bool{false, true} {
		a, b, c, d := v(7, 0, 40), v(9, 0, 40), v(9, 256, 40), v(7, 256, 40)
		if ramp {
			a, b, c, d = v(0, 0, 0), v(256, 0, 256), v(256, 256, 256), v(0, 256, 0)
		}
		w := &world{tris: []scene.Triangle{tri(scene.SurfaceBSP, a, b, c), tri(scene.SurfaceBSP, a, c, d)}}
		r := request(square(0, 0, 256, 256), 8)
		r.Radius, r.Clearance = 1, 0
		if ramp {
			r.ZMin, r.ZMax = 7, 9
		}
		for seed := uint64(1); seed <= 64; seed++ {
			r.Seed = seed
			res := placement.Distribute(w, r)
			if res.Fit() == 0 {
				t.Fatalf("no supported points for ramp=%v seed=%d", ramp, seed)
			}
			for _, p := range res.Points {
				if p.X < 7 || p.X > 9 || (!ramp && p.Z != 40) || (ramp && p.Z != p.X) {
					t.Fatalf("unsupported or out-of-range point %v, ramp=%v seed=%d", p, ramp, seed)
				}
			}
		}
	}
}

func TestFinalJitterUsesHighestSupportedFloor(t *testing.T) {
	w := &world{tris: append(ground(0, 0, 16, 16, 10), ground(9, 0, 16, 16, 80)...)}
	r := request(square(0, 0, 16, 16), 1)
	r.Radius, r.Clearance = 1, 0
	reachedUpper := false
	for seed := uint64(1); seed <= 64; seed++ {
		r.Seed = seed
		res := placement.Distribute(w, r)
		if res.Fit() != 1 {
			t.Fatalf("seed %d: no point on supported cell", seed)
		}
		p := res.Points[0]
		if p.X >= 9 {
			reachedUpper = true
			if p.Z != 80 {
				t.Fatalf("point %v ignored the upper floor", p)
			}
		} else if p.Z != 10 {
			t.Fatalf("point %v is not on the lower floor", p)
		}
	}
	if !reachedUpper {
		t.Fatal("scenario never reached the upper floor")
	}
}
