package main

import (
	"errors"
	"fmt"
	"image"
	"log"
	"slices"
	"strconv"
	"strings"

	"gioui.org/f32"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
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

// selectBody replaces the selection with the body of water v belongs to.
func (ws *waterSelection) selectBody(w *scene.World, v *scene.WaterVolume) {
	ws.ids = ws.ids[:0]
	for _, o := range w.WaterBody(v) {
		ws.ids = append(ws.ids, o.ID())
	}
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

// unsupportedStatus says why water volume v can't be selected.
func unsupportedStatus(v *scene.WaterVolume) string {
	msg := "Volume " + v.Tile.Name() + " " + v.Name + " não suportado: " + v.Unsupported
	log.Printf("água: %s", msg)
	return msg
}

// click handles a left click at viewport pixel p, with no zone tool armed,
// that picked h (ok: something was hit): the water body under it becomes
// the selection, Ctrl toggles the one volume, and a click elsewhere
// clears it. A click on an Unsupported volume says why it is not selected.
// It returns the status line, "" to keep the current one.
func (ws *waterSelection) click(w *scene.World, cam *camera.Camera, p f32.Point, viewport image.Point, h scene.Hit, ok, ctrl bool) string {
	v, unsupported := pickWaterAt(w, cam, p, viewport, h, ok)
	switch {
	case v != nil && ctrl:
		ws.toggle(w, v)
	case v != nil:
		ws.selectBody(w, v)
	case unsupported != nil:
		if !ctrl {
			ws.clear()
		}
		return unsupportedStatus(unsupported)
	case ctrl:
		return ""
	case ok && h.Water:
		ws.clear()
		return "Esta água não tem WaterVolume no mapa"
	case ws.clear():
		return "Seleção de água limpa"
	default:
		return ""
	}
	msg := ws.status(w)
	log.Printf("água: %s", msg)
	return msg
}

// rightClick handles a right click at viewport pixel p (spec D4): over a
// water volume outside the selection, its body becomes the selection;
// over any water volume, open reports that the context menu opens, with
// the selection's status. Over an Unsupported volume msg says why it is
// not selected; elsewhere nothing changes.
func (ws *waterSelection) rightClick(w *scene.World, cam *camera.Camera, p f32.Point, viewport image.Point) (msg string, open bool) {
	h, ok := pickAt(w, cam, p, viewport)
	v, unsupported := pickWaterAt(w, cam, p, viewport, h, ok)
	if unsupported != nil {
		return unsupportedStatus(unsupported), false
	}
	if v == nil {
		return "", false
	}
	if !slices.Contains(ws.ids, v.ID()) {
		ws.selectBody(w, v)
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

// waterBusy is why the water can't be compiled while a polygon is open:
// the context menu greys its item out and shows it.
const waterBusy = "Feche o polígono aberto antes de compilar a zona de água"

// compileWater turns the selected water volumes into water zones (spec
// D5): the zones they don't have yet are created in one undo step, then
// only those zones are compiled, leaving the list's compile selection
// alone. live are every volume of the loaded tiles, for the overlap
// warning. It returns the status line and the files for the XML window.
func (e *zoneEditor) compileWater(selected, live []scene.WaterVolume) (string, []zonexml.File) {
	if e.drawing {
		return waterBusy, nil
	}
	plans := water.Compile(selected, live, e.doc)
	if len(plans) == 0 {
		return "Nenhuma água selecionada", nil
	}
	var (
		b                 zone.Batch
		ids               []zone.ZoneID
		created, existing []string
		warnings          []water.Warning
	)
	for _, p := range plans {
		b = append(b, p.Commands...)
		ids = append(ids, p.Zone)
		if p.Existing {
			existing = append(existing, p.Name)
		} else {
			created = append(created, fmt.Sprintf("%s (%s)", p.Name, inflect.Count(len(p.Volumes), "polígono", "polígonos")))
		}
		warnings = append(warnings, p.Warnings...)
	}
	if len(b) > 0 {
		if err := e.apply(b); err != nil {
			return err.Error(), nil
		}
	}
	for _, w := range warnings {
		log.Printf("água: aviso: %s", w)
	}
	files, err := e.doc.Compile(ids)
	if bl, ok := errors.AsType[*zone.BlockedError](err); ok {
		return e.blockedStatus(bl), nil
	}
	if err != nil {
		log.Printf("zona: compilação: %v", err)
		return err.Error(), nil
	}
	msg := waterCompiled(created, existing) + warningsStatus(warnings)
	log.Printf("água: %s", msg)
	return msg, files
}

// waterCompiled says which zones compileWater created, with their
// polygons, and which it found in the project and compiled as they are.
func waterCompiled(created, existing []string) string {
	var parts []string
	switch len(created) {
	case 0:
	case 1:
		parts = append(parts, "Criada a zona de água "+created[0])
	default:
		parts = append(parts, fmt.Sprintf("Criadas %d zonas de água: %s", len(created), inflect.List(created)))
	}
	switch len(existing) {
	case 0:
	case 1:
		parts = append(parts, "Zona "+existing[0]+" já existe no projeto: compilada a versão do projeto")
	default:
		parts = append(parts, "Zonas "+inflect.List(existing)+" já existem no projeto: compiladas as versões do projeto")
	}
	below := scene.ServerZOffset - water.ServerZOffset
	return strings.Join(parts, ". ") + fmt.Sprintf(". No viewport o topo fica %d abaixo da água (ADR 0005)", below)
}

// warningsStatus is the D7 warnings for the status line, a sentence per
// kind ("" without any); the full text of each goes to the log.
func warningsStatus(ws []water.Warning) string {
	var kinds []water.WarningKind
	byKind := map[water.WarningKind][]water.Warning{}
	for _, w := range ws {
		if _, ok := byKind[w.Kind]; !ok {
			kinds = append(kinds, w.Kind)
		}
		byKind[w.Kind] = append(byKind[w.Kind], w)
	}
	slices.Sort(kinds)
	var b strings.Builder
	for _, k := range kinds {
		b.WriteString(". " + k.Status(byKind[k]))
	}
	return b.String()
}

// status describes the selection: "Água: 8 volumes · topo -3780 (servidor
// -3810) · exata".
func (ws *waterSelection) status(w *scene.World) string {
	vs := ws.volumes(w)
	if len(vs) == 0 {
		return "Nenhuma água selecionada"
	}
	var tops []int
	exact := true
	for _, v := range vs {
		if t := round(v.Top()); !slices.Contains(tops, t) {
			tops = append(tops, t)
		}
		exact = exact && v.Exact
	}
	client, server := make([]string, len(tops)), make([]string, len(tops))
	for i, t := range tops {
		client[i], server[i] = strconv.Itoa(t), strconv.Itoa(t+water.ServerZOffset)
	}
	topWord := "topo"
	if len(tops) > 1 {
		topWord = "topos"
	}
	kind := "exata"
	if !exact {
		kind = "aproximada"
	}
	return fmt.Sprintf("Água: %s · %s %s (servidor %s) · %s",
		inflect.Count(len(vs), "volume", "volumes"), topWord, inflect.List(client), inflect.List(server), kind)
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
