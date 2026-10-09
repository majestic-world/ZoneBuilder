package water_test

import (
	"slices"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/unreal"
	"zonebuilder/internal/water"
	"zonebuilder/internal/zone"
)

// tile is where the synthetic volumes lie: 22_24 starts at x 65536,
// y 196608.
var tile = scene.Tile{X: 22, Y: 24}

const ox, oy = 65536, 196608

// volume is water volume name of tile built by scene.NewWaterVolume from
// a brush Model with the 6 quad faces of the hexahedron whose bottom
// corners are c[0..3] and top corners c[4..7], counter-clockwise seen from
// above, each corner given relative to the tile's origin.
func volume(t *testing.T, export int, name string, c [8]geom.Vec3) scene.WaterVolume {
	t.Helper()
	m := &unreal.Model{Vectors: [][3]float32{{0, 0, 1}}, Surfs: []unreal.BSPSurf{{}}}
	for _, p := range c {
		m.Points = append(m.Points, [3]float32{p.X + ox, p.Y + oy, p.Z})
	}
	for _, f := range [][4]int32{
		{3, 2, 1, 0}, {4, 5, 6, 7}, // bottom, top
		{0, 1, 5, 4}, {1, 2, 6, 5}, {2, 3, 7, 6}, {3, 0, 4, 7},
	} {
		m.Nodes = append(m.Nodes, unreal.BSPNode{VertPool: int32(len(m.Verts)), NumVertices: 4})
		for _, k := range f {
			m.Verts = append(m.Verts, unreal.BSPVert{Point: k})
		}
	}
	one := unreal.Scale{Scale: geom.Vec3{X: 1, Y: 1, Z: 1}}
	v, err := scene.NewWaterVolume(tile, export, name, &unreal.Brush{MainScale: one, PostScale: one}, m)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// box is the axis-aligned volume over x0..x1, y0..y1 (tile-relative),
// from bottom to top.
func box(t *testing.T, export int, name string, x0, y0, x1, y1, bottom, top float32) scene.WaterVolume {
	return volume(t, export, name, [8]geom.Vec3{
		{X: x0, Y: y0, Z: bottom}, {X: x1, Y: y0, Z: bottom}, {X: x1, Y: y1, Z: bottom}, {X: x0, Y: y1, Z: bottom},
		{X: x0, Y: y0, Z: top}, {X: x1, Y: y0, Z: top}, {X: x1, Y: y1, Z: top}, {X: x0, Y: y1, Z: top},
	})
}

func kinds(ws []water.Warning) []water.WarningKind {
	var ks []water.WarningKind
	for _, w := range ws {
		ks = append(ks, w.Kind)
	}
	return ks
}

// Volumes with 2 tops make 2 zones, and every polygon keeps its own floor:
// the server tests Z per shape, so a lake whose basins have different
// depths keeps them. Catches one zone for every top (the low water would
// breathe at the high top) or one zmin per zone.
func TestTwoTopsMakeTwoZonesAndEachPolygonKeepsItsFloor(t *testing.T) {
	vols := []scene.WaterVolume{
		box(t, 1, "WaterVolume1", 0, 0, 100, 100, -100.4, 0),
		box(t, 2, "WaterVolume2", 100, 0, 200, 100, -300, 0.3),
		box(t, 3, "WaterVolume3", 200, 0, 300, 100, -200, -50),
	}
	doc := zone.NewDocument()
	plans := water.Compile(vols, vols, doc)
	zones := applied(t, doc, plans)
	type shape struct {
		zone       string
		zmin, zmax int
	}
	var got []shape
	for _, z := range zones {
		for _, s := range z.Shapes {
			got = append(got, shape{z.Name, s.ZMin, s.ZMax})
		}
	}
	want := []shape{
		{"[22_24_WaterVolume1]", -130, -30},
		{"[22_24_WaterVolume1]", -330, -30},
		{"[22_24_WaterVolume3]", -230, -80},
	}
	if !slices.Equal(got, want) {
		t.Errorf("shapes = %v, want %v", got, want)
	}
}

// A hexahedron with one slanted wall (its top reaches 50 further in x than
// its bottom) becomes the convex footprint of all its corners, without the
// bottom corners that fall on the footprint's edges, and is warned as
// approximate. Catches the bottom face used as footprint, collinear or
// repeated points (RepeatedVertex, or vertices the server walks for
// nothing), or a slanted volume passed as exact.
func TestSlantedWallBecomesConvexFootprintAndApproximate(t *testing.T) {
	v := volume(t, 7, "WaterVolume7", [8]geom.Vec3{
		{X: 0, Y: 0, Z: -100}, {X: 100, Y: 0, Z: -100}, {X: 100, Y: 100, Z: -100}, {X: 0, Y: 100, Z: -100},
		{X: 0, Y: 0, Z: 0}, {X: 150, Y: 0, Z: 0}, {X: 150, Y: 100, Z: 0}, {X: 0, Y: 100, Z: 0},
	})
	doc := zone.NewDocument()
	plans := water.Compile([]scene.WaterVolume{v}, nil, doc)
	z := applied(t, doc, plans)[0]
	want := [][2]int{{ox, oy}, {ox + 150, oy}, {ox + 150, oy + 100}, {ox, oy + 100}}
	if len(z.Shapes) != 1 || !sameRing(z.Shapes[0].Points, want) {
		t.Errorf("shapes = %v, want 1 polygon %v", z.Shapes, want)
	}
	if !slices.Equal(kinds(plans[0].Warnings), []water.WarningKind{water.Approximate}) {
		t.Errorf("warnings = %v, want only approximate", plans[0].Warnings)
	}
}

// A selected volume that crosses, in XY and Z, a live volume with another
// top left out of the selection is warned as overlapping water: where they
// cross the server takes the higher top. A volume that only touches it is
// not. Catches overlap tested on the bounding boxes' touch, on XY alone, or
// against the selection itself.
func TestOverlapWarnsOnlyWhenWaterCrosses(t *testing.T) {
	selected := box(t, 1, "WaterVolume1", 0, 0, 100, 100, -100, 0)
	crossing := box(t, 2, "WaterVolume2", 50, 50, 150, 150, -50, 200)
	touching := box(t, 3, "WaterVolume3", 100, 0, 200, 50, -50, 200)
	sameTop := box(t, 4, "WaterVolume4", 50, 0, 150, 50, -50, 0)

	for _, c := range []struct {
		name  string
		live  []scene.WaterVolume
		other string
	}{
		{"crossing", []scene.WaterVolume{selected, crossing}, "22_24 WaterVolume2"},
		{"touching", []scene.WaterVolume{selected, touching}, ""},
		{"same top", []scene.WaterVolume{selected, sameTop}, ""},
	} {
		plans := water.Compile([]scene.WaterVolume{selected}, c.live, zone.NewDocument())
		var others []string
		for _, w := range plans[0].Warnings {
			if w.Kind == water.Overlap {
				others = append(others, w.Other)
			}
		}
		want := []string{}
		if c.other != "" {
			want = append(want, c.other)
		}
		if !slices.Equal(others, want) {
			t.Errorf("%s: overlap warnings against %q, want %q", c.name, others, want)
		}
	}
	// The crossing volume selected too is not left out: no warning.
	plans := water.Compile([]scene.WaterVolume{selected, crossing}, []scene.WaterVolume{selected, crossing}, zone.NewDocument())
	for _, p := range plans {
		if slices.Contains(kinds(p.Warnings), water.Overlap) {
			t.Errorf("%s: %v, want no overlap with a selected volume", p.Name, p.Warnings)
		}
	}
}

// Compiling water whose zone name the project already has creates nothing
// and points at the project's zone, so the user's edits are never
// overwritten. Catches a second zone with the same name (DuplicateName) or
// an overwrite.
func TestExistingNameCreatesNothingAndReusesTheZone(t *testing.T) {
	doc := zone.NewDocument()
	id := doc.NewZoneID()
	if err := doc.Apply(zone.CreateZone{ID: id, Name: "[22_24_WaterVolume1]", Type: zone.Water}); err != nil {
		t.Fatal(err)
	}
	v := box(t, 1, "WaterVolume1", 0, 0, 100, 100, -100, 0)
	plans := water.Compile([]scene.WaterVolume{v}, nil, doc)
	if len(plans) != 1 || !plans[0].Existing || plans[0].Zone != id || len(plans[0].Commands) != 0 {
		t.Fatalf("plans = %+v, want 1 existing plan for zone %d without commands", plans, id)
	}
}

// The zones of a selection, applied to an empty project, have no problem
// and compile: rounded, deduplicated, convex polygons with zmin below zmax.
// Catches a footprint that self-intersects or repeats a vertex after
// rounding, an inverted Z range, or a type the server does not know.
func TestCompiledZonesHaveNoProblem(t *testing.T) {
	vols := []scene.WaterVolume{
		box(t, 1, "WaterVolume1", 0.4, 0.4, 100.3, 100.2, -100.6, -0.4),
		box(t, 2, "WaterVolume2", 100.3, 0.4, 200, 100.2, -300, 40),
		volume(t, 3, "WaterVolume3", [8]geom.Vec3{
			{X: 300, Y: 300, Z: -80}, {X: 400.2, Y: 300.4, Z: -80}, {X: 400, Y: 400, Z: -80}, {X: 300, Y: 400, Z: -80},
			{X: 280, Y: 290, Z: 10}, {X: 420, Y: 300.3, Z: 12}, {X: 400.4, Y: 430, Z: 11}, {X: 300.3, Y: 400.2, Z: 10},
		}),
	}
	doc := zone.NewDocument()
	plans := water.Compile(vols, vols, doc)
	applied(t, doc, plans)
	if ps := doc.Problems(); len(ps) != 0 {
		t.Fatalf("Problems() = %v, want none", ps)
	}
	var ids []zone.ZoneID
	for _, p := range plans {
		ids = append(ids, p.Zone)
	}
	files, err := doc.Compile(ids)
	if err != nil || len(files) != 1 {
		t.Errorf("Compile = %d files, %v; want 1 file (zonebuilder_water.xml), no error", len(files), err)
	}
}
