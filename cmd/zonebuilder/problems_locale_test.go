package main

import (
    "testing"

    "zonebuilder/internal/locale"
    "zonebuilder/internal/zone"
)

func TestProblemRowsSwitchLanguageWithoutRevalidationOrLosingClickTargets(t *testing.T) {
    e := newZoneEditor()
    id := e.doc.NewZoneID()
    if err := e.apply(zone.CreateZone{ID: id, Name: "[bad]", Type: zone.PeaceZone}); err != nil { t.Fatal(err) }
    if err := e.apply(zone.AddShape{Zone: id, Kind: zone.Rectangle, ZMin: 1200, ZMax: 1000,
        Points: []zone.Point{{X: 83000, Y: 147000}}}); err != nil { t.Fatal(err) }
    pt, ok := e.problemRows(locale.PtBR)
    if !ok || len(pt) == 0 { t.Fatalf("Portuguese rows = %+v, rebuilt %v", pt, ok) }
    problems := e.doc.Problems()
    if _, ok := e.problemRows(locale.PtBR); ok { t.Fatal("unchanged panel rebuilt") }
    en, ok := e.problemRows(locale.En)
    if !ok || len(en) != len(pt) || &e.doc.Problems()[0] != &problems[0] { t.Fatalf("language-only rebuild changed validation or count: %+v / %+v", pt, en) }
    for i := range pt {
        if pt[i].Message == en[i].Message || pt[i].Warning != en[i].Warning || pt[i].Zone != en[i].Zone {
            t.Errorf("row %d changed identity or did not translate: %+v / %+v", i, pt[i], en[i])
        }
    }
    if status := e.goToProblem(0, nil, nil, locale.En); status == "" || e.zone != id || e.shape != max(problems[0].Shape, 0) {
        t.Errorf("problem click changed target: status=%q zone=%d shape=%d", status, e.zone, e.shape)
    }
    if status := e.reformatProblemClick(locale.PtBR); status == "" || e.zone != id { t.Errorf("click status not rerendered: %q", status) }
}
