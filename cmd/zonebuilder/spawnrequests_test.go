package main

import (
	"testing"

	"gioui.org/app"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/spawn"
	"zonebuilder/internal/ui"
)

// Repeated generation clicks must not replace approved points using the old
// count when the focused field contains an invalid integer.
func TestInvalidPendingFieldPreventsGeneration(t *testing.T) {
	for _, field := range []ui.AreaField{ui.AreaCount, ui.AreaRadius, ui.AreaClearance} {
		for _, regenerate := range []bool{false, true} {
			e := newSpawnEditor(&app.Window{}, 9)
			id := e.doc.NewAreaID()
			params := spawn.DefaultParams(9)
			params.Count = 50
			if err := e.apply(spawn.CreateArea{ID: id, Name: "clearing", Outline: []spawn.Vertex{{X: 0, Y: 0}, {X: 500, Y: 0}, {X: 500, Y: 500}, {X: 0, Y: 500}}, ZMin: -100, ZMax: 100, Params: params}); err != nil {
				t.Fatal(err)
			}
			e.area = id
			area, _ := e.doc.Area(id)
			approved := []spawn.Point{{X: 200, Y: 200, Z: 0, Heading: 1234}}
			if err := e.apply(spawn.SetPoints{Area: id, Points: approved, Fingerprint: area.Fingerprint(area.Seed)}); err != nil {
				t.Fatal(err)
			}
			world := scene.NewWorld(geom.Vec3{})
			var cam camera.Camera
			for attempt := range 2 {
				message := e.listRequests([]any{
					ui.SetAreaField{Area: id, Field: field, Text: "80x"},
					ui.GenerateArea{Area: id, Regenerate: regenerate},
				}, world, &cam)
				after, _ := e.doc.Area(id)
				if message.Key != "spawn.field.integer" || e.generating != 0 || after.Params != params || len(after.Points) != 1 || after.Points[0] != approved[0] || after.Seed != area.Seed {
					t.Fatalf("field %v regenerate %v attempt %d: message=%v generating=%v area=%+v", field, regenerate, attempt, message, e.generating, after)
				}
			}
		}
	}
}
