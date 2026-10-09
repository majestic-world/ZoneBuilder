package main

import (
    "strings"
    "testing"

    "zonebuilder/internal/coverage"
    "zonebuilder/internal/locale"
    "zonebuilder/internal/zone"
)

func TestProblemRowsSwitchLanguageWithoutRevalidationOrLosingClickTargets(t *testing.T) {
    e := newZoneEditor()
    id := e.doc.NewZoneID()
    if err := e.apply(zone.CreateZone{ID: id, Name: "[bad]", Type: zone.PeaceZone}); err != nil { t.Fatal(err) }
    if err := e.apply(zone.AddShape{Zone: id, Kind: zone.Rectangle, ZMin: 1200, ZMax: 1000,
        Points: []zone.Point{{X: 83000, Y: 147000}}}); err != nil { t.Fatal(err) }
    ws := []floorWarning{{zone: id, shape: 0, Warning: coverage.Warning{Kind: coverage.AboveTop, Clearance: -1200, Share: .125}}}
    pt, ok := e.problemRows(ws, true, locale.PtBR)
    if !ok || len(pt) < 2 { t.Fatalf("Portuguese rows = %+v, rebuilt %v", pt, ok) }
    problems := e.doc.Problems()
    if _, ok := e.problemRows(ws, false, locale.PtBR); ok { t.Fatal("unchanged panel rebuilt") }
    en, ok := e.problemRows(ws, false, locale.En)
    if !ok || len(en) != len(pt) || &e.doc.Problems()[0] != &problems[0] { t.Fatalf("language-only rebuild changed validation or count: %+v / %+v", pt, en) }
    for i := range pt {
        if pt[i].Message == en[i].Message || pt[i].Warning != en[i].Warning || pt[i].Zone != en[i].Zone {
            t.Errorf("row %d changed identity or did not translate: %+v / %+v", i, pt[i], en[i])
        }
    }
    if !strings.Contains(en[len(en)-1].Message, "1,200") || !strings.Contains(pt[len(pt)-1].Message, "1.200") {
        t.Errorf("warning measures not localized: %q / %q", pt[len(pt)-1].Message, en[len(en)-1].Message)
    }
    if status := e.goToProblem(0, nil, nil, locale.En); status == "" || e.zone != id || e.shape != max(problems[0].Shape, 0) {
        t.Errorf("problem click changed target: status=%q zone=%d shape=%d", status, e.zone, e.shape)
    }
    if status := e.reformatProblemClick(locale.PtBR); status == "" || e.zone != id { t.Errorf("click status not rerendered: %q", status) }
    if status := e.goToProblem(len(problems), nil, nil, locale.En); status == "" || e.zone != id || e.shape != 0 {
        t.Errorf("warning click changed target: status=%q zone=%d shape=%d", status, e.zone, e.shape)
    }
}
