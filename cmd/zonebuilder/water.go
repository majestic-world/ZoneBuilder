package main

import (
	"errors"
	"log"
	"slices"
	"strconv"
	"strings"

	"gioui.org/f32"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/render"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/water"
	"zonebuilder/internal/zone"
	"zonebuilder/internal/zonexml"
)

// waterSelection is the water volumes the user selected by clicking (spec
// D3), kept by identity so they survive the tiles' scenes being re-added
// and drop out when their tile goes.
type waterSelection struct {
	// ids are the selected volumes, in the order they were selected.
	ids []scene.WaterID
	// world is the world ids were picked in: opening a map replaces it.
	world *scene.World
	// version changes with every change of ids; it drives the overlay.
	version int
	// pressTaken is set when the zone editor took the last pointer press
	// (a vertex or midpoint handle, the height arrow): its release is a
	// click on the handle, not on the water.
	pressTaken bool
}

// volumes are the selected volumes of world w, in selection order.
func (ws *waterSelection) volumes(w *scene.World) []*scene.WaterVolume {
	if w == nil {
		return nil
	}
	vs := make([]*scene.WaterVolume, 0, len(ws.ids))
	for _, id := range ws.ids {
		if v, ok := w.WaterVolume(id); ok {
			vs = append(vs, v)
		}
	}
	return vs
}

// selectOne replaces the selection with volume v alone.
func (ws *waterSelection) selectOne(w *scene.World, v *scene.WaterVolume) {
	ws.ids = append(ws.ids[:0], v.ID())
	ws.world = w
	ws.version++
}

// toggle adds volume v to the selection or takes it out.
func (ws *waterSelection) toggle(w *scene.World, v *scene.WaterVolume) {
	id := v.ID()
	if i := slices.Index(ws.ids, id); i >= 0 {
		ws.ids = slices.Delete(ws.ids, i, i+1)
	} else {
		ws.ids = append(ws.ids, id)
	}
	ws.world = w
	ws.version++
}

// clear empties the selection and reports whether it held anything.
func (ws *waterSelection) clear() bool {
	if len(ws.ids) == 0 {
		return false
	}
	ws.ids = ws.ids[:0]
	ws.version++
	return true
}

// prune drops the volumes w no longer has: their tile was unloaded, or a
// map was opened in a new world.
func (ws *waterSelection) prune(w *scene.World) {
	if len(ws.ids) == 0 {
		return
	}
	if w == nil || w != ws.world {
		ws.clear()
		return
	}
	n := len(ws.ids)
	ws.ids = slices.DeleteFunc(ws.ids, func(id scene.WaterID) bool {
		_, ok := w.WaterVolume(id)
		return !ok
	})
	if len(ws.ids) != n {
		ws.version++
	}
}

// pickWaterAt is the water volume under viewport pixel p, the selectable
// volume the ray through p enters first before the surface h it hit (ok:
// it hit one), or before the far plane. Without one, unsupported is the
// Unsupported volume there, if any, so the click can say why it selects
// nothing (spec D1).
func pickWaterAt(w *scene.World, cam *camera.Camera, p f32.Point, viewport image.Point, h scene.Hit, ok bool) (v, unsupported *scene.WaterVolume) {
	maxDist := cam.Far
	if ok {
		maxDist = h.Distance
	}
	r := rayAt(w, cam, p, viewport)
	if v, _, found := w.PickWater(r, maxDist); found {
		return v, nil
	}
	if u, found := w.PickUnsupportedWater(r, maxDist); found {
		return nil, u
	}
	return nil, nil
}

// unsupportedStatus retains the volume identity and original diagnostic.
func unsupportedStatus(v *scene.WaterVolume) actionStatus {
	msg := actionArgs("actions.water.unsupported", map[string]string{
		"tile": v.Tile.Name(), "name": v.Name, "detail": v.Unsupported,
	})
	log.Printf("água: %s", msg.render(locale.PtBR))
	return msg
}

// click handles a left click at viewport pixel p, with no zone tool armed,
// that picked h (ok: something was hit): the volume under it becomes the
// selection, Ctrl adds it to the selection or takes it out, and a click
// elsewhere clears it. A click on an Unsupported volume says why it is not
// selected.
// It returns the status line, "" to keep the current one.
func (ws *waterSelection) click(w *scene.World, cam *camera.Camera, p f32.Point, viewport image.Point, h scene.Hit, ok, ctrl bool) actionStatus {
	v, unsupported := pickWaterAt(w, cam, p, viewport, h, ok)
	switch {
	case v != nil && ctrl:
		ws.toggle(w, v)
	case v != nil:
		ws.selectOne(w, v)
	case unsupported != nil:
		if !ctrl {
			ws.clear()
		}
		return unsupportedStatus(unsupported)
	case ctrl:
		return actionStatus{}
	case ok && h.Water:
		ws.clear()
		return action("actions.water.no_volume")
	case ws.clear():
		return action("actions.water.cleared")
	default:
		return actionStatus{}
	}
	msg := ws.status(w)
	log.Printf("água: %s", msg.render(locale.PtBR))
	return msg
}

// rightClick handles a right click at viewport pixel p (spec D4): a water
// volume outside the selection becomes the selection alone;
// over any water volume, open reports that the context menu opens, with
// the selection's status. Over an Unsupported volume msg says why it is
// not selected; elsewhere nothing changes.
func (ws *waterSelection) rightClick(w *scene.World, cam *camera.Camera, p f32.Point, viewport image.Point) (msg actionStatus, open bool) {
	h, ok := pickAt(w, cam, p, viewport)
	v, unsupported := pickWaterAt(w, cam, p, viewport, h, ok)
	if unsupported != nil {
		return unsupportedStatus(unsupported), false
	}
	if v == nil {
		return actionStatus{}, false
	}
	if !slices.Contains(ws.ids, v.ID()) {
		ws.selectOne(w, v)
	}
	return ws.status(w), true
}

// selected are copies of the selected volumes of w and every live volume
// of its tiles, the two lists water.Compile takes.
func (ws *waterSelection) selected(w *scene.World) (selected, live []scene.WaterVolume) {
	for _, v := range ws.volumes(w) {
		selected = append(selected, *v)
	}
	if w != nil {
		for _, s := range w.Scenes() {
			live = append(live, s.WaterVolumes...)
		}
	}
	return selected, live
}

// waterBusy is why the water can't be compiled while a polygon is open.
const waterBusy = "actions.water.busy"

// compileWater turns the selected water volumes into water zones (spec
// D5): the zones they don't have yet are created in one undo step, then
// only those zones are compiled, leaving the list's compile selection
// alone. live are every volume of the loaded tiles, for the overlap
// warning. It returns the status line and the files for the XML window.
func (e *zoneEditor) compileWater(selected, live []scene.WaterVolume) (actionStatus, []zonexml.File) {
	if e.drawing {
		return action(waterBusy), nil
	}
	plans := water.Compile(selected, live, e.doc)
	if len(plans) == 0 {
		return action("actions.water.none"), nil
	}
	var (
		b zone.Batch
		ids []zone.ZoneID
		report waterSummary
	)
	report.kind = waterResult
	for _, p := range plans {
		b = append(b, p.Commands...)
		ids = append(ids, p.Zone)
		if p.Existing {
			report.existing = append(report.existing, p.Name)
		} else {
			report.created = append(report.created, waterCreated{name: p.Name, polygons: len(p.Volumes)})
		}
		report.warnings = append(report.warnings, p.Warnings...)
	}
	if len(b) > 0 {
		if err := e.apply(b); err != nil {
			return actionError("actions.error.edit_water", err, nil), nil
		}
	}
	for _, w := range report.warnings {
		log.Printf("água: aviso: %s", w)
	}
	files, err := e.doc.Compile(ids)
	if bl, ok := errors.AsType[*zone.BlockedError](err); ok {
		e.blockedStatus(bl, e.Language)
		return waterAction("zone.problem.blocked_status", &waterSummary{kind: waterBlocked, blocked: bl}), nil
	}
	if err != nil {
		log.Printf("zona: compilação: %v", err)
		return actionError("actions.error.compile_water", err, nil), nil
	}
	msg := waterAction("actions.water.compiled", &report)
	log.Printf("água: %s", msg.render(locale.PtBR))
	return msg, files
}

type waterSummaryKind uint8

const (
	waterSelectionStatus waterSummaryKind = iota
	waterResult
	waterBlocked
)

type waterCreated struct {
	name string
	polygons int
}

// waterSummary retains numeric and structural data for each presentation.
type waterSummary struct {
	kind waterSummaryKind
	volumes int
	tops []int
	exact bool
	created []waterCreated
	existing []string
	warnings []water.Warning
	blocked *zone.BlockedError
}

func (s *waterSummary) render(lang locale.Language) string {
	if s.kind == waterBlocked {
		return blockedStatusText(s.blocked, lang)
	}
	if s.kind == waterSelectionStatus {
		if s.volumes == 0 {
			return locale.Text(lang, "actions.water.none")
		}
		client, server := make([]string, len(s.tops)), make([]string, len(s.tops))
		for i, top := range s.tops {
			client[i] = strconv.Itoa(top)
			server[i] = strconv.Itoa(top+water.ServerZOffset)
		}
		quality := locale.Text(lang, "actions.water.exact")
		if !s.exact {
			quality = locale.Text(lang, "actions.water.approximate")
		}
		return locale.Format(lang, "actions.water.selection", map[string]string{
			"volumes": locale.Plural(lang, "actions.water.volumes", s.volumes, nil),
			"tops": locale.Plural(lang, "actions.water.top", len(s.tops), map[string]string{"values": localizedList(client, lang)}),
			"server": localizedList(server, lang), "kind": quality,
		})
	}
	var parts []string
	if len(s.created) > 0 {
		names := make([]string, len(s.created))
		for i, created := range s.created {
			names[i] = locale.Format(lang, "actions.water.created_item", map[string]string{
				"name": created.name, "polygons": locale.Plural(lang, "actions.water.polygons", created.polygons, nil),
			})
		}
		parts = append(parts, locale.Plural(lang, "actions.water.created", len(names), map[string]string{"zones": localizedList(names, lang)}))
	}
	if len(s.existing) > 0 {
		parts = append(parts, locale.Plural(lang, "actions.water.existing", len(s.existing), map[string]string{"zones": localizedList(s.existing, lang)}))
	}
	offset := scene.ServerZOffset - water.ServerZOffset
	return locale.Format(lang, "actions.water.compiled", map[string]string{
		"result": strings.Join(parts, ". "), "offset": strconv.Itoa(offset),
		"warnings": waterWarnings(s.warnings, lang),
	})
}

func localizedList(items []string, lang locale.Language) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	conjunction := locale.Text(lang, "actions.list.and")
	return strings.Join(items[:len(items)-1], ", ") + conjunction + items[len(items)-1]
}

// waterWarnings renders the warning data; the source Warning.String stays for logs.
func waterWarnings(ws []water.Warning, lang locale.Language) string {
	var kinds []water.WarningKind
	byKind := make(map[water.WarningKind][]water.Warning)
	for _, w := range ws {
		if _, ok := byKind[w.Kind]; !ok {
			kinds = append(kinds, w.Kind)
		}
		byKind[w.Kind] = append(byKind[w.Kind], w)
	}
	slices.Sort(kinds)
	var b strings.Builder
	for _, kind := range kinds {
		var names []string
		for _, w := range byKind[kind] {
			if !slices.Contains(names, w.Volume) {
				names = append(names, w.Volume)
			}
		}
		var sentence string
		switch kind {
		case water.Overlap:
			crossings := make([]string, len(names))
			for i, name := range names {
				tile, _, _ := strings.Cut(name, " ")
				var others []string
				for _, w := range byKind[kind] {
					if w.Volume == name {
						others = append(others, strings.TrimPrefix(w.Other, tile+" "))
					}
				}
				crossings[i] = locale.Format(lang, "actions.water.warning_crossing", map[string]string{
					"volume": name, "others": localizedList(others, lang),
				})
			}
			sentence = locale.Format(lang, "actions.water.warning_overlap", map[string]string{"crossings": strings.Join(crossings, "; ")})
		case water.Approximate:
			sentence = locale.Format(lang, "actions.water.warning_approximate", map[string]string{"volumes": localizedList(names, lang)})
		case water.OutsideTile:
			sentence = locale.Format(lang, "actions.water.warning_outside", map[string]string{"volumes": localizedList(names, lang)})
		}
		if sentence != "" {
			b.WriteString(". ")
			b.WriteString(sentence)
		}
	}
	return b.String()
}

// status describes the selected water volumes without storing a translated sentence.
func (ws *waterSelection) status(w *scene.World) actionStatus {
	vs := ws.volumes(w)
	s := &waterSummary{kind: waterSelectionStatus, volumes: len(vs), exact: true}
	for _, v := range vs {
		if top := round(v.Top()); !slices.Contains(s.tops, top) {
			s.tops = append(s.tops, top)
		}
		s.exact = s.exact && v.Exact
	}
	return waterAction("actions.water.selection", s)
}

// overlay is the selected volumes as overlay prisms in the water zone's
// colour: each volume's footprint from its bottom to its top, in server
// coordinates so that the overlay's FromServer lands them on the volume.
// An approximate volume, whose prism covers it with room to spare, is
// marked as a problem.
func (ws *waterSelection) overlay(w *scene.World) []render.ZoneShape {
	color := linearColor(zone.Water.Color())
	var shapes []render.ZoneShape
	for _, v := range ws.volumes(w) {
		fp := v.Footprint()
		pts := make([]geom.Vec3, len(fp))
		for k, p := range fp {
			pts[k] = scene.ToServer(p)
		}
		shapes = append(shapes, render.ZoneShape{
			Points:  pts,
			ZMin:    scene.ToServer(geom.Vec3{Z: v.Bottom()}).Z,
			ZMax:    scene.ToServer(geom.Vec3{Z: v.Top()}).Z,
			Closed:  true,
			Color:   color,
			Marked:  -1,
			Problem: !v.Exact,
		})
	}
	return shapes
}
