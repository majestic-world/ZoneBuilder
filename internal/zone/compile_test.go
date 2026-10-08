package zone_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"zonebuilder/internal/zone"
	"zonebuilder/internal/zonexml"
)

// Only the selected zones are compiled: an unselected zone's type gets no
// file and its name appears in none, and the selected zones of a type come
// out in name order whatever order they were created in.
func TestCompileTakesOnlyTheSelectedZones(t *testing.T) {
	d := zone.NewDocument()
	b, a, w, left := d.NewZoneID(), d.NewZoneID(), d.NewZoneID(), d.NewZoneID()
	polygonZone(t, d, b, "[zb_b]", zone.PeaceZone, -3660, -3148, square...)
	polygonZone(t, d, a, "[zb_a]", zone.PeaceZone, -3660, -3148, square...)
	polygonZone(t, d, w, "[zb_w]", zone.Water, -3660, -3148, square...)
	polygonZone(t, d, left, "[zb_left_out]", zone.Dummy, -3660, -3148, square...)

	files, err := d.Compile([]zone.ZoneID{b, w, a})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, f.Name)
		if bytes.Contains(f.Data, []byte("[zb_left_out]")) {
			t.Errorf("%s has the unselected zone:\n%s", f.Name, f.Data)
		}
	}
	slices.Sort(names)
	if want := []string{"zonebuilder_peace_zone.xml", "zonebuilder_water.xml"}; !slices.Equal(names, want) {
		t.Fatalf("files = %q, want %q", names, want)
	}
	peace := string(files[slices.IndexFunc(files, func(f zonexml.File) bool { return f.Name == "zonebuilder_peace_zone.xml" })].Data)
	if ia, ib := strings.Index(peace, "[zb_a]"), strings.Index(peace, "[zb_b]"); ia < 0 || ib < 0 || ia > ib {
		t.Errorf("peace zones not in name order:\n%s", peace)
	}
}

// A project with 1 valid zone of each of the 23 types compiles to 23 files,
// one per type, each named with the Zone Builder prefix; writing them into
// a folder that already holds zone files leaves the files with other names
// byte for byte as they were.
func TestOneZoneOfEachTypeGivesOnePrefixedFilePerType(t *testing.T) {
	d := zone.NewDocument()
	for _, typ := range zone.Types {
		id := d.NewZoneID()
		name := "[zb_" + strings.ToLower(string(typ)) + "]"
		if typ == zone.Residence {
			name = "residence_99"
		}
		polygonZone(t, d, id, name, typ, -3660, -3148, square...)
		switch typ {
		case zone.Siege, zone.Headquarter:
			apply(t, d, zone.SetParam{Zone: id, Name: "residence", Value: "5"})
		case zone.Fishing:
			apply(t, d, zone.SetParam{Zone: id, Name: "distribution_id", Value: "1"},
				zone.SetParam{Zone: id, Name: "fishing_place_type", Value: "0"})
		}
	}
	if p := d.Problems(); len(p) > 0 {
		t.Fatalf("problems: %v", p)
	}
	files, err := d.Compile(d.ZoneIDs())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(zone.Types) {
		t.Fatalf("got %d files, want %d", len(files), len(zone.Types))
	}
	for _, typ := range zone.Types {
		if !slices.ContainsFunc(files, func(f zonexml.File) bool { return f.Name == zonexml.FilePrefix+string(typ)+".xml" }) {
			t.Errorf("no %s%s.xml", zonexml.FilePrefix, typ)
		}
	}

	dir := t.TempDir()
	others := map[string]string{
		"peace_zone.xml":   "<list>datapack's own peace zones</list>\n",
		"water.xml":        "<list>datapack's own water</list>\n",
		"zone.dtd":         "<!ELEMENT list (zone)*>\n",
		"notes/readme.txt": "untouched\n",
	}
	for name, data := range others {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := zonexml.Write(dir, files)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != len(files) {
		t.Fatalf("Write returned %d paths for %d files", len(paths), len(files))
	}
	for name, data := range others {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != data {
			t.Errorf("%s changed: %q, %v", name, got, err)
		}
	}
}
