package main

import (
    "strings"
    "testing"

    "zonebuilder/internal/coverage"
    "zonebuilder/internal/locale"
    "zonebuilder/internal/zone"
)

func TestFloorWarningsRemainSeparateAndDoNotBlockCompilation(t *testing.T) {
    e := newZoneEditor()
    id := e.doc.NewZoneID()
    if err := e.apply(zone.CreateZone{ID: id, Name: "[valid]", Type: zone.PeaceZone}); err != nil { t.Fatal(err) }
    if err := e.apply(zone.AddShape{Zone: id, Kind: zone.Rectangle, ZMin: -3600, ZMax: -3200,
        Points: []zone.Point{{X: 83000, Y: 147000}, {X: 83400, Y: 147400}}}); err != nil { t.Fatal(err) }
    warning := floorWarning{zone: id, shape: 0, Warning: coverage.Warning{Kind: coverage.NoGround, Area: 1234}}
    rows, ok := e.problemRows([]floorWarning{warning}, true, locale.PtBR)
    if !ok || len(rows) != 1 || !rows[0].Warning || !strings.Contains(rows[0].Message, "1.234") {
        t.Fatalf("floor warning row = %+v, rebuilt %v", rows, ok)
    }
    en, ok := e.problemRows([]floorWarning{warning}, false, locale.En)
    if !ok || len(en) != 1 || !en[0].Warning || !strings.Contains(en[0].Message, "1,234") {
        t.Fatalf("English floor warning row = %+v, rebuilt %v", en, ok)
    }
    if len(e.doc.Problems()) != 0 { t.Fatalf("floor warning became blocking: %+v", e.doc.Problems()) }
    files, err := e.doc.Compile([]zone.ZoneID{id})
    if err != nil || len(files) != 1 { t.Fatalf("floor warning blocked compile: %d files, err %v", len(files), err) }
}
