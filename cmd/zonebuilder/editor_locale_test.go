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
	created := zonesJSON(t, e.doc)
	e.Language = locale.En
	if got := e.Status(e.Language); got != "Zone [giran] created. Polygon: click a surface for the first vertex; Esc cancels" {
		t.Errorf("retained creation and instruction: %q", got)
	}
	if zonesJSON(t, e.doc) != created {
		t.Fatal("rerender repeated creation")
	}
	e.Language = locale.PtBR
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

// Switching after the first rectangle corner updates its instruction but
// preserves the anchor and does not place a shape.
func TestRectangleInstructionSwitchKeepsAnchor(t *testing.T) {
	e := newZoneEditor()
	e.Language = locale.PtBR
	e.create("area", zone.PeaceZone, ui.ToolRectangle)
	anchor := zone.Point{X: 123, Y: 456, Z: 789}
	e.rectangleClick(nil, nil, anchor)
	before := zonesJSON(t, e.doc)
	e.Language = locale.En
	if got := e.hint(); got != "Rectangle: click the opposite corner; Esc cancels" {
		t.Errorf("English instruction: %q", got)
	}
	if got := e.Status(e.Language); got != "First corner at 123 456 789; click the opposite corner" {
		t.Errorf("retained anchor status: %q", got)
	}
	if !e.anchored || e.anchor != anchor || zonesJSON(t, e.doc) != before {
		t.Fatal("changing language moved the anchor or modified the document")
	}
}
