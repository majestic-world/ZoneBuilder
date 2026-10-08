package zone_test

import (
	"strings"
	"testing"

	"zonebuilder/internal/zone"
)

// A duplicate is a zone of its own: editing the original afterwards leaves
// the copy's XML as it was, and each compiles alone under its own name.
// Catches a copy that shares the original's shape slices, where one more
// vertex or a new Z range on the original silently reshapes the copy too.
func TestDuplicateCompilesIndependentlyOfTheOriginal(t *testing.T) {
	d := zone.NewDocument()
	orig := d.NewZoneID()
	polygonZone(t, d, orig, "[zb_square]", zone.PeaceZone, -3660, -3148,
		zone.Point{X: 83000, Y: 147600, Z: -3404},
		zone.Point{X: 83800, Y: 147600, Z: -3398},
		zone.Point{X: 83800, Y: 148300, Z: -3405},
	)
	dup := d.NewZoneID()
	apply(t, d, zone.DuplicateZone{Zone: orig, ID: dup})
	before, err := d.Compile([]zone.ZoneID{dup})
	if err != nil {
		t.Fatal(err)
	}

	apply(t, d,
		zone.AddVertex{Zone: orig, Shape: 0, Point: zone.Point{X: 83000, Y: 148300, Z: -3402}},
		zone.SetZRange{Zone: orig, Shape: 0, ZMin: -4000, ZMax: -3000},
	)

	after, err := d.Compile([]zone.ZoneID{dup})
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || string(after[0].Data) != string(before[0].Data) {
		t.Fatalf("editing the original changed the copy:\nbefore\n%s\nafter\n%s", before[0].Data, after[0].Data)
	}
	if got := coordsLoc.FindAllStringSubmatch(string(after[0].Data), -1); len(got) != 3 || got[0][1] != "83000 147600 -3660 -3148" {
		t.Errorf("copy's coords %q, want the original's 3 vertices at z -3660 -3148", got)
	}
	origFiles, err := d.Compile([]zone.ZoneID{orig})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(coordsLoc.FindAllString(string(origFiles[0].Data), -1)); n != 4 {
		t.Errorf("original compiles with %d coords, want 4:\n%s", n, origFiles[0].Data)
	}
	if strings.Contains(string(after[0].Data), `name="[zb_square]"`) {
		t.Errorf("copy compiles under the original's name:\n%s", after[0].Data)
	}
}

// Every duplicate gets a name no zone of the project has, even when the
// obvious "_2" is taken or the original is itself a copy. Catches a copy
// named after an existing zone, which the server's ZoneHolder would let
// overwrite the other one in silence.
func TestDuplicateGetsAnUnusedName(t *testing.T) {
	d := zone.NewDocument()
	tri := []zone.Point{{X: 83000, Y: 147600, Z: -3404}, {X: 83800, Y: 147600, Z: -3398}, {X: 83800, Y: 148300, Z: -3405}}
	a := d.NewZoneID()
	polygonZone(t, d, a, "[zb_a]", zone.PeaceZone, -3660, -3148, tri...)
	a2 := d.NewZoneID()
	polygonZone(t, d, a2, "[zb_a_2]", zone.Water, -3660, -3148, tri...)
	plain := d.NewZoneID()
	polygonZone(t, d, plain, "zb_plain", zone.Swamp, -3660, -3148, tri...)

	for _, src := range []zone.ZoneID{a, a, a2, plain} {
		apply(t, d, zone.DuplicateZone{Zone: src, ID: d.NewZoneID()})
	}

	var got []string
	for _, z := range d.Zones() {
		got = append(got, z.Name)
	}
	want := []string{"[zb_a]", "[zb_a_2]", "zb_plain", "[zb_a_3]", "[zb_a_4]", "[zb_a_5]", "zb_plain_2"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("names %q, want %q", got, want)
	}
}
