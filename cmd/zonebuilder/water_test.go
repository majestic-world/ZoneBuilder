package main

import (
	"encoding/json"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/scene/scenetest"
	"zonebuilder/internal/zone"
)

// waterBox is the axis-aligned water volume over x0..x1, y0..y1 (relative
// to tile 22_24's origin), from bottom to top.
func waterBox(t *testing.T, export int, name string, x0, y0, x1, y1, bottom, top float32) scene.WaterVolume {
	tile := scene.Tile{X: 22, Y: 24}
	ox, oy := tile.Origin()
	c := scenetest.Box(geom.Vec3{X: x0 + ox, Y: y0 + oy, Z: bottom}, geom.Vec3{X: x1 + ox, Y: y1 + oy, Z: top})
	return scenetest.Volume(t, tile, export, name, scenetest.UnitBrush(), scenetest.Hexahedron(c))
}

// zonesJSON is the document's zones, deep, to compare two moments of it.
func zonesJSON(t *testing.T, d *zone.Document) string {
	t.Helper()
	b, err := json.Marshal(d.Zones())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Compiling the water of 3 volumes is one undo step: a single Undo leaves
// the project as it was. Catches the plans' commands applied one by one,
// which would take 4 Ctrl+Z to undo and leave a half-built zone after the
// first.
func TestCompilingWaterUndoesInOneStep(t *testing.T) {
	e := newZoneEditor()
	if err := e.apply(zone.CreateZone{ID: e.doc.NewZoneID(), Name: "[giran]", Type: zone.PeaceZone}); err != nil {
		t.Fatal(err)
	}
	before := zonesJSON(t, e.doc)

	vols := []scene.WaterVolume{
		waterBox(t, 1, "WaterVolume1", 0, 0, 100, 100, -200, 0),
		waterBox(t, 2, "WaterVolume2", 100, 0, 200, 100, -300, 0),
		waterBox(t, 3, "WaterVolume3", 200, 0, 300, 100, -400, 0),
	}
	if _, files := e.compileWater(vols, vols); len(files) != 1 {
		t.Fatalf("compiled %s, want 1", inflect.Count(len(files), "file", "files"))
	}
	if n := len(e.doc.Zones()); n != 2 {
		t.Fatalf("%s after compiling, want 2", inflect.Count(n, "zone", "zones"))
	}

	if !e.doc.Undo() {
		t.Fatal("nothing to undo")
	}
	if after := zonesJSON(t, e.doc); after != before {
		t.Errorf("after 1 undo the zones are\n%s\nwant\n%s", after, before)
	}
}
