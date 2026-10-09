package zone_test

import (
    "strings"
    "testing"

    "zonebuilder/internal/locale"
    "zonebuilder/internal/zone"
)

func TestInvalidRectangleCornerCountInBothLanguages(t *testing.T) {
    for _, tc := range []struct {
        count int
        pt, en string
    }{
        {0, "0 cantos", "0 corners"},
        {1, "1 canto", "1 corner"},
        {2, "", ""},
    } {
        d := zone.NewDocument()
        id := d.NewZoneID()
        pts := []zone.Point{{X: 83000, Y: 147000}, {X: 83400, Y: 147400}}
        apply(t, d, zone.CreateZone{ID: id, Name: "[count]", Type: zone.PeaceZone},
            zone.AddShape{Zone: id, Kind: zone.Rectangle, ZMin: -3600, ZMax: -3200, Points: pts[:tc.count]})
        var corner *zone.Problem
        for i := range d.Problems() {
            if d.Problems()[i].Rule == zone.CornerCount { corner = &d.Problems()[i] }
        }
        if tc.count == 2 {
            if corner != nil { t.Errorf("valid rectangle reported corner count: %+v", corner) }
            continue
        }
        if corner == nil || !strings.Contains(corner.Text(locale.PtBR), tc.pt) || !strings.Contains(corner.Text(locale.En), tc.en) {
            t.Errorf("%d corners: got %+v, expected %q and %q", tc.count, corner, tc.pt, tc.en)
        }
    }
}
