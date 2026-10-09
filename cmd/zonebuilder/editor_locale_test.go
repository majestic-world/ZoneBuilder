package main

import (
	"strings"
	"testing"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// An open polygon and its current instruction survive a language change.
func TestEditorLanguageSwitchKeepsPolygonAndHistory(t *testing.T) {
	e := newZoneEditor()
	e.Language = locale.PtBR
	if msg := e.create("[giran]", zone.PeaceZone, ui.ToolPolygon); !strings.Contains(msg, "criada") {
		t.Fatalf("Portuguese creation: %q", msg)
	}
	if msg := e.polygonClick(zone.Point{X: 100, Y: 200, Z: 300}); !strings.Contains(msg, "vértice") {
		t.Fatalf("Portuguese vertex: %q", msg)
	}
	before := zonesJSON(t, e.doc)
	previous := e.LastMessage
	e.Language = locale.En
	if got := e.Status(e.Language); got != "1 vertex: 100 200 300" {
		t.Errorf("retained status after switching: %q", got)
	}
	if got := e.hint(); !strings.HasPrefix(got, "Polygon: click to add") {
		t.Errorf("open polygon instruction after switching: %q", got)
	}
	if e.LastMessage.Key != previous.Key || zonesJSON(t, e.doc) != before || !e.drawing {
		t.Fatal("language switch changed the action or drawing state")
	}
	if msg := e.polygonClick(zone.Point{X: 200, Y: 200, Z: 300}); !strings.Contains(msg, "vertices") {
		t.Fatalf("English second vertex: %q", msg)
	}
	e.Language = locale.PtBR
	if got := e.Status(e.Language); got != "2 vértices: 200 200 300" {
		t.Errorf("retained status after switching back: %q", got)
	}
	if !e.drawing || len(e.points()) != 2 {
		t.Fatal("polygon progression was reset by language changes")
	}
}

func TestEditorUndoStatusRerendersWithoutRepeatingUndo(t *testing.T) {
	e := newZoneEditor()
	e.Language = locale.PtBR
	e.create("zone", zone.PeaceZone, ui.ToolPolygon)
	before := zonesJSON(t, e.doc)
	if got := e.undo(); got != "Desfeito" {
		t.Fatalf("undo status: %q", got)
	}
	undone := zonesJSON(t, e.doc)
	e.Language = locale.En
	if got := e.Status(e.Language); got != "Undone" {
		t.Errorf("English undo status: %q", got)
	}
	if zonesJSON(t, e.doc) != undone || undone == before {
		t.Fatal("rerender repeated or reversed the undo")
	}
	if got := e.redo(); got != "Redone" {
		t.Errorf("English redo status: %q", got)
	}
	if zonesJSON(t, e.doc) != before {
		t.Fatal("redo changed zone data")
	}
}
