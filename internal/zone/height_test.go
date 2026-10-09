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

// Raising or lowering a zone moves every shape's Z range by the same
// amount, the exclusion included, and leaves the restart points on the
// ground they were clicked on. Catches an exclusion left at the old height,
// which would stop cutting its hole out of the moved zone.
func TestShiftingAZoneMovesEveryShapeTogether(t *testing.T) {
	d := zone.NewDocument()
	id := heightZone(t, d)
	apply(t, d, zone.ShiftZoneZ{Zone: id, DZ: 500})
	want := []string{
		"83000 147000 -3200 -2600", "84000 147000 -3200 -2600", "84000 148000 -3200 -2600", "83000 148000 -3200 -2600",
		"83400 147400 -3150 -2800", "83600 147400 -3150 -2800", "83600 147600 -3150 -2800", "83400 147600 -3150 -2800",
		"83200 147200 -3404",
	}
	if got := coords(t, d); !reflect.DeepEqual(got, want) {
		t.Errorf("coords after raising the zone by 500:\n%q\nwant\n%q", got, want)
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
