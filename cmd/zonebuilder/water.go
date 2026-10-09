package main

import (
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
	"zonebuilder/internal/zone"
)

// waterServerZOffset is what a water zone's Z adds to the client Z of its
// volume (spec D6): the status shows the server top with it.
const waterServerZOffset = -30

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
// it hit one), or before the far plane.
func pickWaterAt(w *scene.World, cam *camera.Camera, p f32.Point, viewport image.Point, h scene.Hit, ok bool) (*scene.WaterVolume, bool) {
	maxDist := cam.Far
	if ok {
		maxDist = h.Distance
	}
	v, _, found := w.PickWater(rayAt(w, cam, p, viewport), maxDist)
	return v, found
}

// click handles a left click at viewport pixel p, with no zone tool armed,
// that picked h (ok: something was hit): the water body under it becomes
// the selection, Ctrl toggles the one volume, and a click elsewhere
// clears it. It returns the status line, "" to keep the current one.
func (ws *waterSelection) click(w *scene.World, cam *camera.Camera, p f32.Point, viewport image.Point, h scene.Hit, ok, ctrl bool) string {
	v, found := pickWaterAt(w, cam, p, viewport, h, ok)
	switch {
	case found && ctrl:
		ws.toggle(w, v)
	case found:
		ws.selectBody(w, v)
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
		client[i], server[i] = strconv.Itoa(t), strconv.Itoa(t+waterServerZOffset)
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
		inflect.Count(len(vs), "volume", "volumes"), topWord, listPT(client), listPT(server), kind)
}

// listPT joins items as a pt-BR list: "a", "a e b", "a, b e c".
func listPT(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " e " + items[len(items)-1]
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
