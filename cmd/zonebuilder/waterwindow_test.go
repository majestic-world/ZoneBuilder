package main

import (
	"slices"
	"strings"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
)

const shearReason = "o MainScale do brush tem cisalhamento (SheerRate 0.5), que o app não aplica"

// waterWorld has tile 22_24 added before 22_22, its volumes out of name
// order, and 22_24's WaterVolume1 unsupported.
func waterWorld(t *testing.T) *scene.World {
	t.Helper()
	unsupported := waterBoxIn(t, scene.Tile{X: 22, Y: 24}, 3, "WaterVolume1", 400, 0, 500, 100, -4000, -3780)
	unsupported.Unsupported = shearReason
	w := scene.NewWorld(geom.Vec3{})
	w.Add(&scene.Scene{WaterVolumes: []scene.WaterVolume{
		waterBoxIn(t, scene.Tile{X: 22, Y: 24}, 1, "WaterVolume10", 0, 0, 100, 100, -4000, -3780),
		waterBoxIn(t, scene.Tile{X: 22, Y: 24}, 2, "WaterVolume2", 200, 0, 300, 100, -4000, -3746),
		unsupported,
	}})
	w.Add(&scene.Scene{WaterVolumes: []scene.WaterVolume{
		waterBoxIn(t, scene.Tile{X: 22, Y: 22}, 0, "WaterVolume0", 0, 0, 100, 100, -4000, -3780),
	}})
	return w
}

// selectedNames are the tile and name of the selected volumes, sorted.
func selectedNames(sel *waterSelection, w *scene.World) []string {
	var names []string
	for _, v := range sel.volumes(w) {
		names = append(names, v.Tile.Name()+" "+v.Name)
	}
	slices.Sort(names)
	return names
}

// The water window lists the world's volumes grouped by tile and, within
// a tile, by name in natural order, whatever order the tiles loaded in;
// the unsupported one is disabled and says why. Catches the list in load
// order, WaterVolume10 before WaterVolume2, or an unsupported volume
// offered for selection.
func TestWaterListGroupsByTileInNaturalOrderAndDisablesUnsupported(t *testing.T) {
	w := waterWorld(t)
	var sel waterSelection
	var l waterList
	if !l.refresh(w, &sel, locale.PtBR) {
		t.Fatal("first refresh built no rows")
	}
	var labels []string
	for _, r := range l.rows {
		labels = append(labels, r.Label)
	}
	want := []string{"22_22 · WaterVolume0", "22_24 · WaterVolume1", "22_24 · WaterVolume2", "22_24 · WaterVolume10"}
	if !slices.Equal(labels, want) {
		t.Fatalf("rows %q, want %q", labels, want)
	}
	for i, r := range l.rows {
		if tile, _, _ := strings.Cut(want[i], " "); r.Tile != tile {
			t.Errorf("row %d grouped under tile %q, want %q", i, r.Tile, tile)
		}
		if unsupported := i == 1; r.Disabled != unsupported {
			t.Errorf("row %d (%s) disabled %v, want %v", i, r.Label, r.Disabled, unsupported)
		}
	}
	if !strings.Contains(l.rows[1].Detail, shearReason) {
		t.Errorf("unsupported row detail %q, want it to give the reason %q", l.rows[1].Detail, shearReason)
	}
}

// The rows are rebuilt when the selection changes or a tile loads, and
// only then. Catches a list that keeps a stale check mark or misses a
// loaded tile's volumes, or one rebuilt every frame.
func TestWaterListRebuildsOnlyWhenScenesOrSelectionChange(t *testing.T) {
	w := waterWorld(t)
	var sel waterSelection
	var l waterList
	l.refresh(w, &sel, locale.PtBR)
	if l.refresh(w, &sel, locale.PtBR) {
		t.Error("refresh with nothing changed rebuilt the rows")
	}
	l.apply(ui.ToggleWater{Row: 0}, w, &sel)
	if !l.refresh(w, &sel, locale.PtBR) || !l.rows[0].Checked {
		t.Error("toggled row not checked after refresh")
	}
	w.Add(&scene.Scene{WaterVolumes: []scene.WaterVolume{
		waterBoxIn(t, scene.Tile{X: 23, Y: 22}, 0, "WaterVolume0", 0, 0, 100, 100, -4000, -3780),
	}})
	if !l.refresh(w, &sel, locale.PtBR) || len(l.rows) != 5 {
		t.Errorf("after a tile loaded the list has %s, want 5", inflect.Count(len(l.rows), "row", "rows"))
	}
}

// A toggle from row i selects that row's volume, and toggling it again
// takes it out. Catches a row index mapped to another volume than the
// one the row shows.
func TestWaterListToggleSelectsTheRowsVolume(t *testing.T) {
	w := waterWorld(t)
	var sel waterSelection
	var l waterList
	l.refresh(w, &sel, locale.PtBR)
	l.apply(ui.ToggleWater{Row: 3}, w, &sel)
	if got := selectedNames(&sel, w); !slices.Equal(got, []string{"22_24 WaterVolume10"}) {
		t.Fatalf("after toggling row 3 the selection is %q, want 22_24 WaterVolume10", got)
	}
	l.apply(ui.ToggleWater{Row: 3}, w, &sel)
	if got := selectedNames(&sel, w); len(got) != 0 {
		t.Fatalf("after toggling row 3 twice the selection is %q, want it empty", got)
	}
}

// Selecionar todos selects every supported volume of the world in one
// change, and Limpar empties the selection. Catches the unsupported
// volume selected (and then compiled), or the overlay and the list
// rebuilt once per volume.
func TestWaterListSelectAllSkipsUnsupportedAndClearEmpties(t *testing.T) {
	w := waterWorld(t)
	var sel waterSelection
	var l waterList
	l.refresh(w, &sel, locale.PtBR)
	l.apply(ui.ToggleWater{Row: 0}, w, &sel)
	before := sel.version
	l.apply(ui.SelectAllWater{}, w, &sel)
	want := []string{"22_22 WaterVolume0", "22_24 WaterVolume10", "22_24 WaterVolume2"}
	if got := selectedNames(&sel, w); !slices.Equal(got, want) {
		t.Fatalf("Selecionar todos selected %q, want %q", got, want)
	}
	if steps := sel.version - before; steps != 1 {
		t.Errorf("Selecionar todos changed the selection version %s, want once", inflect.Count(steps, "time", "times"))
	}
	l.apply(ui.ClearWater{}, w, &sel)
	if got := selectedNames(&sel, w); len(got) != 0 {
		t.Fatalf("Limpar left %q selected", got)
	}
}
