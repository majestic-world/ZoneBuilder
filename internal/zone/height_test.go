package zone_test

import (
	"reflect"
	"testing"

	"zonebuilder/internal/zone"
)

// heightZone builds a zone with an included square on [-3700, -3100] and
// an exclusion inside it on [-3650, -3300], plus a restart point.
func heightZone(t *testing.T, d *zone.Document) zone.ZoneID {
	t.Helper()
	id := d.NewZoneID()
	polygonZone(t, d, id, "[zb_height]", zone.PeaceZone, -3700, -3100,
		zone.Point{X: 83000, Y: 147000, Z: -3404},
		zone.Point{X: 84000, Y: 147000, Z: -3404},
		zone.Point{X: 84000, Y: 148000, Z: -3404},
		zone.Point{X: 83000, Y: 148000, Z: -3404},
	)
	apply(t, d,
		zone.AddShape{Zone: id, Kind: zone.Rectangle, Banned: true, ZMin: -3650, ZMax: -3300, Points: []zone.Point{
			{X: 83400, Y: 147400, Z: -3404}, {X: 83600, Y: 147600, Z: -3404},
		}},
		zone.AddRestartPoint{Zone: id, Point: zone.Point{X: 83200, Y: 147200, Z: -3404}},
	)
	return id
}

// Moving a zone moves every shape by the same amount on every axis, the
// exclusion included, and leaves the restart points on the ground they
// were clicked on. Catches an exclusion left behind, which would stop
// cutting its hole out of the moved zone.
func TestMovingAZoneMovesEveryShapeTogether(t *testing.T) {
	d := zone.NewDocument()
	id := heightZone(t, d)
	apply(t, d, zone.MoveZone{Zone: id, DX: 100, DY: -200, DZ: 500})
	want := []string{
		"83100 146800 -3200 -2600", "84100 146800 -3200 -2600", "84100 147800 -3200 -2600", "83100 147800 -3200 -2600",
		"83500 147200 -3150 -2800", "83700 147200 -3150 -2800", "83700 147400 -3150 -2800", "83500 147400 -3150 -2800",
		"83200 147200 -3404",
	}
	if got := coords(t, d); !reflect.DeepEqual(got, want) {
		t.Errorf("coords after moving the zone by 100 -200 500:\n%q\nwant\n%q", got, want)
	}
}

// Setting a zone's height keeps each shape's floor (zmin) where it is and
// moves only its top, so a zone laid on the ground stays on it. Catches a
// height applied around the middle of the range, which would lift the
// floor off the ground and let players walk under the zone.
func TestSettingAZoneHeightKeepsItsFloor(t *testing.T) {
	d := zone.NewDocument()
	id := heightZone(t, d)
	apply(t, d, zone.SetZoneHeight{Zone: id, Height: 400})
	want := []string{
		"83000 147000 -3700 -3300", "84000 147000 -3700 -3300", "84000 148000 -3700 -3300", "83000 148000 -3700 -3300",
		"83400 147400 -3650 -3250", "83600 147400 -3650 -3250", "83600 147600 -3650 -3250", "83400 147600 -3650 -3250",
		"83200 147200 -3404",
	}
	if got := coords(t, d); !reflect.DeepEqual(got, want) {
		t.Errorf("coords after setting the height to 400:\n%q\nwant\n%q", got, want)
	}
	if err := d.Apply(zone.SetZoneHeight{Zone: id, Height: -1}); err == nil {
		t.Error("a negative height was accepted")
	}
}
