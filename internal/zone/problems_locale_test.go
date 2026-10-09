package zone_test

import (
    "strings"
    "testing"

    "zonebuilder/internal/locale"
    "zonebuilder/internal/zone"
)

func TestProblemsRetainTargetsWhileLanguageChanges(t *testing.T) {
    d := zone.NewDocument()
    id := d.NewZoneID()
    apply(t, d,
        zone.CreateZone{ID: id, Name: "[zone_1]", Type: zone.PeaceZone},
        zone.AddShape{Zone: id, Kind: zone.Polygon, ZMin: 1200, ZMax: -1200,
            Points: []zone.Point{{X: 83000, Y: 147000}, {X: 83000, Y: 147000}, {X: 90000, Y: 147000}}},
    )
    before := d.Problems()
    if len(before) < 2 { t.Fatalf("expected several problems, got %v", before) }
    for _, p := range before {
        pt, en := p.Text(locale.PtBR), p.Text(locale.En)
        if pt == en || pt == "" || en == "" { t.Errorf("problem %v: pt=%q en=%q", p.Rule, pt, en) }
        if again := p.Text(locale.PtBR); again != pt { t.Errorf("problem changed after language switch: %q != %q", pt, again) }
    }
    after := d.Problems()
    if len(after) != len(before) || &after[0] != &before[0] { t.Fatal("language change invalidated problem cache") }
    for i := range before {
        if after[i].Rule != before[i].Rule || after[i].Zone != before[i].Zone || after[i].Shape != before[i].Shape || after[i].Vertex != before[i].Vertex || after[i].Restart != before[i].Restart {
            t.Errorf("problem target %d changed: before=%+v after=%+v", i, before[i], after[i])
        }
    }
}

func TestProblemCountsAndValuesAreLocalizedWithoutChangingCompilation(t *testing.T) {
    d := zone.NewDocument()
    id := d.NewZoneID()
    apply(t, d, zone.CreateZone{ID: id, Name: "[bad]", Type: zone.Fishing},
        zone.AddShape{Zone: id, Kind: zone.Rectangle, ZMin: 1200, ZMax: 1000,
            Points: []zone.Point{{X: 83000, Y: 147000}}},
    )
    problems := d.Problems()
    for _, p := range problems {
        if p.Rule == zone.InvertedZRange {
            if !strings.Contains(p.Text(locale.PtBR), "1200") || !strings.Contains(p.Text(locale.En), "1200") {
                t.Errorf("range data changed by presentation: %q / %q", p.Text(locale.PtBR), p.Text(locale.En))
            }
        }
    }
    ptFiles, ptErr := d.Compile([]zone.ZoneID{id})
    for _, p := range problems { _ = p.Text(locale.En) }
    enFiles, enErr := d.Compile([]zone.ZoneID{id})
    if len(ptFiles) != len(enFiles) || (ptErr == nil) != (enErr == nil) { t.Fatalf("compile changed by presentation: %v/%v vs %v/%v", ptFiles, ptErr, enFiles, enErr) }
}
