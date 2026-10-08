package project_test

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"zonebuilder/internal/project"
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
// all. Catches model state the file drops (an incomplete shape skipped, a
// field not written) and ID bookkeeping lost on reopen: a zone created
// after reopening must not get the ID of a saved one.
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
	)
	incomplete := d.NewZoneID()
	apply(t, d,
		zone.CreateZone{ID: incomplete, Name: "[zb_half_drawn]", Type: zone.Siege},
		zone.AddShape{Zone: incomplete},
		zone.AddVertex{Zone: incomplete, Shape: 0, Point: zone.Point{X: 81000, Y: 149000, Z: -3500}},
		zone.AddVertex{Zone: incomplete, Shape: 0, Point: zone.Point{X: 81500, Y: 149200, Z: -3480}},
	)
	empty := d.NewZoneID()
	apply(t, d, zone.CreateZone{ID: empty, Name: "[zb_no_shape]", Type: zone.Water})
	unused := d.NewZoneID() // reserved, never created

	path := filepath.Join(t.TempDir(), "giran"+project.Ext)
	saved := project.Project{
		Client:   `C:\L2\Fafurion`,
		Output:   `C:\server\data\zone\custom`,
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

	if got.Client != saved.Client || got.Output != saved.Output {
		t.Errorf("client %q, output %q; want %q, %q", got.Client, got.Output, saved.Client, saved.Output)
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
