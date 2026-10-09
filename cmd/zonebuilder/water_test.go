package main

import (
	"encoding/json"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/unreal"
	"zonebuilder/internal/zone"
)

// waterBox is the axis-aligned water volume over x0..x1, y0..y1 (relative
// to tile 22_24's origin), from bottom to top, built by
// scene.NewWaterVolume from a brush Model with the box's 6 faces.
func waterBox(t *testing.T, export int, name string, x0, y0, x1, y1, bottom, top float32) scene.WaterVolume {
	t.Helper()
	tile := scene.Tile{X: 22, Y: 24}
	ox, oy := tile.Origin()
	m := &unreal.Model{Vectors: [][3]float32{{0, 0, 1}}, Surfs: []unreal.BSPSurf{{}}}
	for _, z := range []float32{bottom, top} {
		for _, p := range [][2]float32{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}} {
			m.Points = append(m.Points, [3]float32{p[0] + ox, p[1] + oy, z})
		}
	}
	for _, f := range [][4]int32{{3, 2, 1, 0}, {4, 5, 6, 7}, {0, 1, 5, 4}, {1, 2, 6, 5}, {2, 3, 7, 6}, {3, 0, 4, 7}} {
		m.Nodes = append(m.Nodes, unreal.BSPNode{VertPool: int32(len(m.Verts)), NumVertices: 4})
		for _, k := range f {
			m.Verts = append(m.Verts, unreal.BSPVert{Point: k})
		}
	}
	one := unreal.Scale{Scale: geom.Vec3{X: 1, Y: 1, Z: 1}}
	v, err := scene.NewWaterVolume(tile, export, name, &unreal.Brush{MainScale: one, PostScale: one}, m)
	if err != nil {
		t.Fatal(err)
	}
	return v
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
		t.Fatalf("compiled %d files, want 1", len(files))
	}
	if n := len(e.doc.Zones()); n != 2 {
		t.Fatalf("%d zones after compiling, want 2", n)
	}

	if !e.doc.Undo() {
		t.Fatal("nothing to undo")
	}
	if after := zonesJSON(t, e.doc); after != before {
		t.Errorf("after 1 undo the zones are\n%s\nwant\n%s", after, before)
	}
}
