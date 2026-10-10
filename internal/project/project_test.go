package project_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"zonebuilder/internal/project"
	"zonebuilder/internal/spawn"
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

// Saving a project and opening the file again gives back the same zones,
// the same open tiles and the same output folder, including a zone whose
// polygon was left at 2 vertices (no Z range yet) and one with no shape at
// all. Catches model state the file drops (an incomplete shape skipped, an
// exclusion, restart point, parameter, hidden flag or custom colour not
// written, parameters reordered) and ID bookkeeping lost on reopen: a zone
// created after reopening must not get the ID of a saved one.
func TestSavedProjectReopensWithTheSameDocument(t *testing.T) {
	d := zone.NewDocument()
	square := d.NewZoneID()
	apply(t, d,
		zone.CreateZone{ID: square, Name: "[zb_giran_square]", Type: zone.PeaceZone},
		zone.AddShape{Zone: square},
		zone.AddVertex{Zone: square, Shape: 0, Point: zone.Point{X: 83000, Y: 147600, Z: -3404}},
		zone.AddVertex{Zone: square, Shape: 0, Point: zone.Point{X: 83800, Y: 147600, Z: -3398}},
		zone.AddVertex{Zone: square, Shape: 0, Point: zone.Point{X: 83800, Y: 148300, Z: -3405}},
		zone.SetZRange{Zone: square, Shape: 0, ZMin: -3661, ZMax: -3142},
		zone.AddShape{Zone: square, Kind: zone.Rectangle, Banned: true, ZMin: -3600, ZMax: -3200,
			Points: []zone.Point{{X: 83200, Y: 147800, Z: -3400}, {X: 83400, Y: 148000, Z: -3401}}},
		zone.AddRestartPoint{Zone: square, Point: zone.Point{X: 82900, Y: 147500, Z: -3410}},
		zone.AddRestartPoint{Zone: square, PK: true, Point: zone.Point{X: 82000, Y: 147000, Z: -3420}},
		zone.SetParam{Zone: square, Name: "enabled", Value: "false"},
		zone.SetParam{Zone: square, Name: "playerMinLevel", Value: "40"},
		zone.SetColor{Zone: square, Color: zone.Color{0x12, 0xab, 0xff}},
	)
	incomplete := d.NewZoneID()
	apply(t, d,
		zone.CreateZone{ID: incomplete, Name: "[zb_half_drawn]", Type: zone.Siege},
		zone.AddShape{Zone: incomplete},
		zone.AddVertex{Zone: incomplete, Shape: 0, Point: zone.Point{X: 81000, Y: 149000, Z: -3500}},
		zone.AddVertex{Zone: incomplete, Shape: 0, Point: zone.Point{X: 81500, Y: 149200, Z: -3480}},
		zone.SetHidden{Zones: []zone.ZoneID{incomplete}, Hidden: true},
	)
	empty := d.NewZoneID()
	apply(t, d, zone.CreateZone{ID: empty, Name: "[zb_no_shape]", Type: zone.Water})
	unused := d.NewZoneID() // reserved, never created

	path := filepath.Join(t.TempDir(), "giran"+project.Ext)
	saved := project.Project{
		Client:   `C:\L2\Fafurion`,
		Tiles:    []string{"22_22", "21_22", "22_23_Classic"},
		Document: d,
	}
	if err := project.Save(path, saved); err != nil {
		t.Fatal(err)
	}
	got, err := project.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got.Client != saved.Client {
		t.Errorf("client %q, want %q", got.Client, saved.Client)
	}
	if !slices.Equal(got.Tiles, saved.Tiles) {
		t.Errorf("tiles %q, want %q", got.Tiles, saved.Tiles)
	}
	if !reflect.DeepEqual(got.Document.Zones(), d.Zones()) {
		t.Errorf("zones after reopening:\n%#v\nwant:\n%#v", got.Document.Zones(), d.Zones())
	}
	if next, want := got.Document.NewZoneID(), d.NewZoneID(); next != want || next <= unused {
		t.Errorf("next zone ID after reopening is %d, want %d", next, want)
	}
}

// generatedArea creates an area named name in d with the points of a
// distribution with seed and returns its ID.
func generatedArea(t *testing.T, d *spawn.Document, name string, outline []spawn.Vertex, seed uint64, points []spawn.Point, warnings ...spawn.Warning) spawn.AreaID {
	t.Helper()
	id := d.NewAreaID()
	params := spawn.DefaultParams(24)
	params.NPCID, params.Count = 20001, len(points)+len(warnings)
	if err := d.Apply(spawn.CreateArea{ID: id, Name: name, Outline: outline, ZMin: -3600, ZMax: -3200, Params: params}); err != nil {
		t.Fatal(err)
	}
	a, _ := d.Area(id)
	if err := d.Apply(spawn.SetPoints{Area: id, Seed: seed, Fingerprint: a.Fingerprint(seed), Points: points, Warnings: warnings}); err != nil {
		t.Fatal(err)
	}
	return id
}

// Saving a project with 2 generated areas and opening it again gives back
// the same areas and points, warnings, hidden flag and fingerprints
// included, with the areas still up to date. Catches area state the file
// drops (a point, a heading, the seed, a warning), a fingerprint the reload
// no longer matches (every reopened area would be stale), and ID
// bookkeeping lost on reopen.
func TestSavedProjectReopensWithTheSameSpawnAreas(t *testing.T) {
	d := spawn.NewDocument()
	generatedArea(t, d, "giran_square",
		[]spawn.Vertex{{X: 83000, Y: 147600}, {X: 83400, Y: 147600}, {X: 83400, Y: 148000}, {X: 83000, Y: 148000}},
		1<<63+5, []spawn.Point{{X: 83100, Y: 147700, Z: -3404, Heading: 1200}, {X: 83300, Y: 147900, Z: -3398, Heading: 65535}})
	north := generatedArea(t, d, "giran_north",
		[]spawn.Vertex{{X: 83000, Y: 146000}, {X: 83600, Y: 146100}, {X: 83300, Y: 146700}},
		42, []spawn.Point{{X: 83300, Y: 146300, Z: -3500, Heading: 1}},
		spawn.Warning{Rule: spawn.FitsOnly, Placed: 1, Requested: 2})
	if err := d.Apply(spawn.SetHidden{Areas: []spawn.AreaID{north}, Hidden: true}); err != nil {
		t.Fatal(err)
	}
	unused := d.NewAreaID() // reserved, never created

	path := filepath.Join(t.TempDir(), "giran"+project.Ext)
	if err := project.Save(path, project.Project{Client: `C:\L2\Fafurion`, Spawns: d}); err != nil {
		t.Fatal(err)
	}
	got, err := project.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Spawns.Areas(), d.Areas()) {
		t.Errorf("areas after reopening:\n%#v\nwant:\n%#v", got.Spawns.Areas(), d.Areas())
	}
	for _, a := range got.Spawns.Areas() {
		if a.Stale() {
			t.Errorf("area %s is stale after reopening", a.Name)
		}
	}
	if next, want := got.Spawns.NewAreaID(), d.NewAreaID(); next != want || next <= unused {
		t.Errorf("next area ID after reopening is %d, want %d", next, want)
	}
}

// A file the version 1 app saved (zones only, no Spawns key) opens with its
// zones intact and no spawn area. Catches the format bump breaking the
// projects users already have.
func TestVersion1ProjectOpensWithoutSpawnAreas(t *testing.T) {
	const v1 = `{
	"Version": 1,
	"Client": "C:\\L2\\Fafurion",
	"Tiles": ["22_22"],
	"Document": {
		"LastID": 2,
		"Zones": [
			{
				"ID": 2,
				"Name": "[zb_giran_square]",
				"Type": "peace_zone",
				"Params": [{"Name": "enabled", "Value": "false"}],
				"Shapes": [{"Kind": 0, "Banned": false, "Points": [{"X": 83000, "Y": 147600, "Z": -3404}, {"X": 83800, "Y": 147600, "Z": -3398}, {"X": 83800, "Y": 148300, "Z": -3405}], "ZMin": -3661, "ZMax": -3142}],
				"RestartPoints": [{"X": 82900, "Y": 147500, "Z": -3410}],
				"PKRestartPoints": null,
				"Hidden": true,
				"Color": [18, 171, 255]
			}
		]
	}
}
`
	path := filepath.Join(t.TempDir(), "old"+project.Ext)
	if err := os.WriteFile(path, []byte(v1), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := project.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []zone.Zone{{
		ID: 2, Name: "[zb_giran_square]", Type: zone.PeaceZone,
		Params: []zone.Param{{Name: "enabled", Value: "false"}},
		Shapes: []zone.Shape{{Kind: zone.Polygon, ZMin: -3661, ZMax: -3142,
			Points: []zone.Point{{X: 83000, Y: 147600, Z: -3404}, {X: 83800, Y: 147600, Z: -3398}, {X: 83800, Y: 148300, Z: -3405}}}},
		RestartPoints: []zone.Point{{X: 82900, Y: 147500, Z: -3410}},
		Hidden:        true,
		Color:         zone.Color{18, 171, 255},
	}}
	if !reflect.DeepEqual(got.Document.Zones(), want) {
		t.Errorf("zones of the version 1 file:\n%#v\nwant:\n%#v", got.Document.Zones(), want)
	}
	if got.Spawns == nil || len(got.Spawns.Areas()) != 0 {
		t.Errorf("spawn document of the version 1 file is %v, want an empty one", got.Spawns)
	}
}

// A file of a format newer than the app's is refused instead of opened
// half-read. Catches an old binary loading a project whose spawn areas it
// cannot see, which saving over would then erase.
func TestNewerProjectVersionIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future"+project.Ext)
	data := fmt.Sprintf(`{"Version": %d, "Client": "C:\\L2", "Spawns": {"LastID": 0, "Areas": []}}`, project.Version+1)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := project.Load(path); err == nil {
		t.Errorf("Load accepted a version %d file", project.Version+1)
	}
}
