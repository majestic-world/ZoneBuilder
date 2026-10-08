package zone_test

import (
	"regexp"
	"strings"
	"testing"

	"zonebuilder/internal/zone"
)

// apply applies cmds in order and fails the test on the first error.
func apply(t *testing.T, d *zone.Document, cmds ...zone.Command) {
	t.Helper()
	for _, c := range cmds {
		if err := d.Apply(c); err != nil {
			t.Fatalf("Apply(%#v): %v", c, err)
		}
	}
}

// polygonZone applies the commands that create zone id with one polygon
// through pts and the Z range [zmin, zmax].
func polygonZone(t *testing.T, d *zone.Document, id zone.ZoneID, name string, typ zone.Type, zmin, zmax int, pts ...zone.Point) {
	t.Helper()
	apply(t, d, zone.CreateZone{ID: id, Name: name, Type: typ}, zone.AddShape{Zone: id})
	for _, p := range pts {
		apply(t, d, zone.AddVertex{Zone: id, Shape: 0, Point: p})
	}
	apply(t, d, zone.SetZRange{Zone: id, Shape: 0, ZMin: zmin, ZMax: zmax})
}

var coordsLoc = regexp.MustCompile(`<coords loc="([^"]*)"`)

// Every coords of a compiled polygon carries the shape's own zmin zmax as
// its 3rd and 4th numbers, whatever Z each vertex was clicked at. Catches
// coords with 3 numbers (the ZoneParser then ignores Z and spans the whole
// world height) or with per-vertex Z (it keeps only the last coords' range).
func TestCompiledPolygonCoordsCarryTheShapeZRange(t *testing.T) {
	d := zone.NewDocument()
	polygonZone(t, d, d.NewZoneID(), "[giran_square]", zone.PeaceZone, -3660, -3148,
		zone.Point{X: 83000, Y: 147600, Z: -3404},
		zone.Point{X: 83800, Y: 147600, Z: -3398},
		zone.Point{X: 83800, Y: 148300, Z: -3405},
		zone.Point{X: 83000, Y: 148300, Z: -3402},
	)
	files, err := d.Compile(d.ZoneIDs())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	got := coordsLoc.FindAllStringSubmatch(string(files[0].Data), -1)
	want := []string{
		"83000 147600 -3660 -3148",
		"83800 147600 -3660 -3148",
		"83800 148300 -3660 -3148",
		"83000 148300 -3660 -3148",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d coords, want %d:\n%s", len(got), len(want), files[0].Data)
	}
	for i, m := range got {
		if m[1] != want[i] {
			t.Errorf("coords %d: loc %q, want %q", i, m[1], want[i])
		}
	}
	if strings.Contains(string(files[0].Data), "-3404") {
		t.Errorf("a vertex's own Z leaked into the output:\n%s", files[0].Data)
	}
}

// Compiling the same document twice gives the same files, byte for byte,
// so a project's XML diffs in git only where a zone changed. The zones span
// several types and are created out of name order, which is what a
// map-ordered grouping or an unstable sort would scramble.
func TestCompilingTwiceGivesIdenticalBytes(t *testing.T) {
	d := zone.NewDocument()
	square := []zone.Point{{X: 83000, Y: 147600, Z: -3404}, {X: 83800, Y: 147600, Z: -3398}, {X: 83800, Y: 148300, Z: -3405}}
	polygonZone(t, d, d.NewZoneID(), "[zb_c]", zone.PeaceZone, -3660, -3148, square...)
	polygonZone(t, d, d.NewZoneID(), "[zb_a]", zone.Water, -3700, -3100, square...)
	polygonZone(t, d, d.NewZoneID(), "[zb_b]", zone.PeaceZone, -3661, -3149, square...)
	polygonZone(t, d, d.NewZoneID(), "[zb_d]", zone.BattleZone, -3662, -3150, square...)
	polygonZone(t, d, d.NewZoneID(), "[zb_e]", zone.Water, -3663, -3151, square...)

	first, err := d.Compile(d.ZoneIDs())
	if err != nil {
		t.Fatal(err)
	}
	for range 20 {
		again, err := d.Compile(d.ZoneIDs())
		if err != nil {
			t.Fatal(err)
		}
		if len(again) != len(first) {
			t.Fatalf("got %d files, then %d", len(first), len(again))
		}
		for i := range first {
			if again[i].Name != first[i].Name || string(again[i].Data) != string(first[i].Data) {
				t.Fatalf("file %d differs between compilations:\n%s\n%s\n---\n%s\n%s",
					i, first[i].Name, first[i].Data, again[i].Name, again[i].Data)
			}
		}
	}
}
