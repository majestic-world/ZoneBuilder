package water_test

import (
	"math"
	"os"
	"slices"
	"testing"

	"zonebuilder/internal/scene"
	"zonebuilder/internal/water"
	"zonebuilder/internal/zone"
)

// loadVolumes is the live water volumes of tile name of the real client,
// named by ZB_CLIENT. The test skips without it.
func loadVolumes(t *testing.T, name string) []scene.WaterVolume {
	t.Helper()
	root := os.Getenv("ZB_CLIENT")
	if root == "" {
		t.Skip("ZB_CLIENT not set: tests against the real client skipped")
	}
	tile, err := scene.ParseTile(name)
	if err != nil {
		t.Fatal(err)
	}
	s, err := scene.Load(root, []scene.Tile{tile})
	if err != nil {
		t.Fatalf("Load(%s): %v", name, err)
	}
	return s.WaterVolumes
}

// applied applies every plan's commands to doc in one Batch and returns the
// zone each plan names.
func applied(t *testing.T, doc *zone.Document, plans []water.Plan) []zone.Zone {
	t.Helper()
	var b zone.Batch
	for _, p := range plans {
		b = append(b, p.Commands...)
	}
	if err := doc.Apply(b); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	zones := make([]zone.Zone, len(plans))
	for i, p := range plans {
		z, ok := doc.Zone(p.Zone)
		if !ok {
			t.Fatalf("plan %s: no zone %d in the document", p.Name, p.Zone)
		}
		zones[i] = z
	}
	return zones
}

// sameRing reports whether polygons a and b list the same XY points in the
// same cyclic order, from any start and in either direction.
func sameRing(a []zone.Point, b [][2]int) bool {
	if len(a) != len(b) {
		return false
	}
	n := len(a)
	at := func(i int) [2]int { return [2]int{a[i%n].X, a[i%n].Y} }
	for start := range n {
		fwd, back := true, true
		for k := range n {
			if at(start+k) != b[k] {
				fwd = false
			}
			if at(start+n-k) != b[k] {
				back = false
			}
		}
		if fwd || back {
			return true
		}
	}
	return false
}

type datapackShape struct {
	name       string // the datapack zone
	points     [][2]int
	zmin, zmax int
}

// The 11 volumes of Floran's lake (22_24) with top -3780 compile to 1 water
// zone of 11 polygons, each the shape the Majestic datapack generated from
// that volume (water.xml, matched by the spec probe in datapack_matches.tsv).
// Catches a wrong offset, rounding, swapped bottom and top, Model.Points in
// place of the faces, or a top split into several zones.
func TestFloranLakeReproducesTheDatapack(t *testing.T) {
	want := map[string]datapackShape{
		"WaterVolume1":  {"[22_24_water1]", [][2]int{{93696, 224128}, {98304, 224128}, {98304, 229376}, {93696, 229376}}, -4810, -3810},
		"WaterVolume9":  {"[22_24_water2]", [][2]int{{98304, 201216}, {98304, 196608}, {93056, 196608}, {93056, 201216}}, -4810, -3810},
		"WaterVolume3":  {"[22_24_water3]", [][2]int{{76032, 196608}, {65536, 196608}, {65536, 200320}, {76032, 200320}}, -4810, -3810},
		"WaterVolume11": {"[22_24_water4]", [][2]int{{70024, 222217}, {77862, 222217}, {77862, 229376}, {70024, 229376}}, -4810, -3810},
		"WaterVolume0":  {"[22_24_water5]", [][2]int{{65536, 200320}, {70024, 200320}, {70024, 229376}, {65536, 229376}}, -4810, -3810},
		"WaterVolume12": {"[22_24_water6]", [][2]int{{70024, 222217}, {79174, 222217}, {79174, 200320}, {70024, 200320}}, -4298, -3810},
		"WaterVolume10": {"[22_24_water7]", [][2]int{{93696, 224128}, {93696, 229376}, {77862, 229376}, {77862, 224128}}, -4298, -3810},
		"WaterVolume8":  {"[22_24_water8]", [][2]int{{98304, 224128}, {98304, 201216}, {80192, 201216}, {80192, 224128}}, -4298, -3810},
		"WaterVolume6":  {"[22_24_water9]", [][2]int{{81856, 201216}, {81856, 196608}, {93056, 196608}, {93056, 201216}}, -4298, -3810},
		"WaterVolume13": {"[22_24_water12]", [][2]int{{80192, 209727}, {80192, 224128}, {79174, 224128}, {79174, 209727}}, -4298, -3810},
		"WaterVolume7":  {"[22_24_water13]", [][2]int{{80192, 201216}, {79174, 201216}, {79174, 208704}, {80192, 208704}}, -4298, -3810},
	}
	live := loadVolumes(t, "22_24")
	var lake []scene.WaterVolume
	for _, v := range live {
		if math.Abs(float64(v.Top())+3780) <= 1 {
			lake = append(lake, v)
		}
	}
	if len(lake) != len(want) {
		t.Fatalf("22_24 has %d volumes with top -3780, want %d", len(lake), len(want))
	}

	doc := zone.NewDocument()
	plans := water.Compile(lake, live, doc)
	if len(plans) != 1 {
		t.Fatalf("Compile gave %d plans, want 1", len(plans))
	}
	p := plans[0]
	if p.Name != "[22_24_WaterVolume0]" {
		t.Errorf("Name = %q, want [22_24_WaterVolume0]", p.Name)
	}
	z := applied(t, doc, plans)[0]
	if z.Name != p.Name || z.Type != zone.Water || len(z.Params) != 0 {
		t.Errorf("zone = %q type %q params %v, want %q type water without params", z.Name, z.Type, z.Params, p.Name)
	}
	if len(z.Shapes) != len(want) || len(p.Volumes) != len(want) {
		t.Fatalf("%d shapes from %d volumes, want %d", len(z.Shapes), len(p.Volumes), len(want))
	}
	for i, s := range z.Shapes {
		v := p.Volumes[i]
		w, ok := want[v.Name]
		if !ok {
			t.Errorf("shape %d comes from %s, not a lake volume", i, v.Name)
			continue
		}
		if s.Kind != zone.Polygon || s.Banned || !sameRing(s.Points, w.points) || s.ZMin != w.zmin || s.ZMax != w.zmax {
			t.Errorf("%s: %v polygon %v z %d..%d, want %s: %v z %d..%d", v.Name, s.Kind, s.Points, s.ZMin, s.ZMax, w.name, w.points, w.zmin, w.zmax)
		}
	}
}

// Floran's fountain is 2 volumes that touch with different tops
// (WaterVolume26, top -3746, and WaterVolume27, top -5529): they compile to
// 2 zones, each with its own surface. Catches grouping by touch instead of
// by top.
func TestFloranFountainGivesOneZonePerTop(t *testing.T) {
	live := loadVolumes(t, "22_24")
	var fountain []scene.WaterVolume
	for _, v := range live {
		if v.Name == "WaterVolume26" || v.Name == "WaterVolume27" {
			fountain = append(fountain, v)
		}
	}
	if len(fountain) != 2 {
		t.Fatalf("22_24 has %d of WaterVolume26/27, want 2", len(fountain))
	}
	doc := zone.NewDocument()
	plans := water.Compile(fountain, live, doc)
	var tops []int
	for _, z := range applied(t, doc, plans) {
		for _, s := range z.Shapes {
			tops = append(tops, s.ZMax)
		}
	}
	slices.Sort(tops)
	if len(plans) != 2 || !slices.Equal(tops, []int{-5559, -3776}) {
		t.Errorf("Compile gave %d plans with tops %v, want 2 with -5559 and -3776", len(plans), tops)
	}
}
