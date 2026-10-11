package main

import (
	"log"
	"slices"
	"strconv"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/water"
)

// waterList is the water window's rows, one per water volume of the
// world's scenes, and, in the same order, the identity of the volume each
// row shows: the window reports rows by index and never sees a WaterID.
type waterList struct {
	rows []ui.WaterRow
	ids  []scene.WaterID
	// built is set once rows are; scenes, version and lang are the
	// world's scenes, the selection version and the language they were
	// built for.
	built   bool
	scenes  []*scene.Scene
	version int
	lang    locale.Language
}

// refresh rebuilds the rows from the volumes of w (worldWaterVolumes),
// checking the ones sel holds, when the scenes, the selection or the
// language changed since the last build, and reports whether it did.
func (l *waterList) refresh(w *scene.World, sel *waterSelection, lang locale.Language) bool {
	var scenes []*scene.Scene
	if w != nil {
		scenes = w.Scenes()
	}
	if l.built && l.version == sel.version && l.lang == lang && slices.Equal(l.scenes, scenes) {
		return false
	}
	l.built, l.version, l.lang = true, sel.version, lang
	l.scenes = append(l.scenes[:0], scenes...)
	vols := worldWaterVolumes(w)
	l.rows, l.ids = l.rows[:0], l.ids[:0]
	for _, v := range vols {
		id, tile := v.ID(), v.Tile.Name()
		l.ids = append(l.ids, id)
		l.rows = append(l.rows, ui.WaterRow{
			Tile:     tile,
			Label:    tile + " · " + v.Name,
			Detail:   waterDetail(v, lang),
			Checked:  slices.Contains(sel.ids, id),
			Disabled: v.Unsupported != "",
		})
	}
	return true
}

// waterDetail is a row's detail: the volume's top in the client and on
// the server and whether it is exact, or why it is unsupported.
func waterDetail(v *scene.WaterVolume, lang locale.Language) string {
	if v.Unsupported != "" {
		return locale.Format(lang, "actions.water.row_unsupported", map[string]string{"detail": v.Unsupported})
	}
	kind := locale.Text(lang, "actions.water.exact")
	if !v.Exact {
		kind = locale.Text(lang, "actions.water.approximate")
	}
	top := round(v.Top())
	return locale.Format(lang, "actions.water.row", map[string]string{
		"top": strconv.Itoa(top), "server": strconv.Itoa(top + water.ServerZOffset), "kind": kind,
	})
}

// apply carries out the water window's request req on sel, for the
// volumes of w, through the rows req was made on: a toggle is the
// Ctrl+click's, Selecionar todos takes every supported volume, Limpar
// empties the selection. It returns the selection's status after a
// change, nothing otherwise.
func (l *waterList) apply(req any, w *scene.World, sel *waterSelection) actionStatus {
	if w == nil {
		return actionStatus{}
	}
	switch r := req.(type) {
	case ui.ToggleWater:
		if r.Row < 0 || r.Row >= len(l.ids) {
			return actionStatus{}
		}
		v, ok := w.WaterVolume(l.ids[r.Row])
		if !ok || v.Unsupported != "" {
			return actionStatus{}
		}
		sel.toggle(w, v)
	case ui.SelectAllWater:
		sel.selectAll(w)
	case ui.ClearWater:
		if !sel.clear() {
			return actionStatus{}
		}
	default:
		return actionStatus{}
	}
	msg := sel.status(w)
	log.Printf("água: %s", msg.render(locale.PtBR))
	return msg
}
