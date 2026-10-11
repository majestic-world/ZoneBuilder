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

// The rows follow the world: a tile loaded adds its volumes, a tile
// unloaded takes them and their check marks away, and a Ctrl+click in the
// viewport checks the volume's row. Catches a list that keeps a stale
// check mark or an unloaded tile's volumes, or misses a loaded tile's.
func TestWaterListFollowsTilesAndViewportToggles(t *testing.T) {
	w := waterWorld(t)
	var sel waterSelection
	var l waterList
	l.refresh(w, &sel, locale.PtBR)
	added := &scene.Scene{WaterVolumes: []scene.WaterVolume{
		waterBoxIn(t, scene.Tile{X: 23, Y: 22}, 0, "WaterVolume0", 0, 0, 100, 100, -4000, -3780),
	}}
	w.Add(added)
	l.refresh(w, &sel, locale.PtBR)
	if len(l.rows) != 5 || l.rows[4].Label != "23_22 · WaterVolume0" {
		t.Fatalf("after tile 23_22 loaded the rows are %v, want its volume last", l.rows)
	}
	sel.toggle(w, &added.WaterVolumes[0])
	l.refresh(w, &sel, locale.PtBR)
	if !l.rows[4].Checked {
		t.Error("the row of the Ctrl+clicked volume is not checked")
	}
	w.Remove(added)
	sel.prune(w)
	l.refresh(w, &sel, locale.PtBR)
	if len(l.rows) != 4 {
		t.Fatalf("after tile 23_22 unloaded the list has %s, want 4", inflect.Count(len(l.rows), "row", "rows"))
	}
	for _, r := range l.rows {
		if r.Checked {
			t.Errorf("row %s checked after the selected volume's tile unloaded", r.Label)
		}
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
// change, in the rows' order, and Limpar empties the selection. Catches
// the unsupported volume selected (and then compiled), the overlay and the
// list rebuilt once per volume, or a selection status in load order while
// the rows show tile and name order.
func TestWaterListSelectAllSkipsUnsupportedAndClearEmpties(t *testing.T) {
	w := waterWorld(t)
	var sel waterSelection
	var l waterList
	l.refresh(w, &sel, locale.PtBR)
	l.apply(ui.ToggleWater{Row: 0}, w, &sel)
	before := sel.version
	l.apply(ui.SelectAllWater{}, w, &sel)
	var got []string
	for _, v := range sel.volumes(w) {
		got = append(got, v.Tile.Name()+" "+v.Name)
	}
	if want := []string{"22_22 WaterVolume0", "22_24 WaterVolume2", "22_24 WaterVolume10"}; !slices.Equal(got, want) {
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
