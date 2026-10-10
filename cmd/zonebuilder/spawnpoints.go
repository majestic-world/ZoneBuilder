package main

import (
	"image"
	"log"
	"math"
	"math/rand/v2"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/model"
	"zonebuilder/internal/placement"
	"zonebuilder/internal/render"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/spawn"
	"zonebuilder/internal/zone"
)

// A spawn point's pin in the overlay: a stem as tall as the preview
// monster with a handle on top, over a circle of the area's radius on the
// floor. Both lie pinLift over the point so they win the depth test
// against the floor they stand on; the circle has pinSides sides.
const (
	pinLift  = 2
	pinSides = 24
)

// pinHeight is the stem's height: the preview monster's.
func pinHeight() int { return int(math.Round(float64(model.MonsterHeight))) }

// staleColor is the stem and circle colour of stale points (spec story
// 48); their handles are filled with the overlay's problem colour.
var staleColor = [3]float32{0.42, 0.42, 0.42}

// generation is one Gerar or Regerar: what it read when it started, and
// the distribution it brings back from its goroutine.
type generation struct {
	// doc is the document the area was read from: a project opened
	// meanwhile drops the result.
	doc   *spawn.Document
	area  spawn.AreaID
	name  string
	seed  uint64
	fp    spawn.Fingerprint
	count int
	// regenerate is set for Regerar; replaced when the area had points,
	// which the result throws away with any hand edits.
	regenerate, replaced bool
	res                  placement.Result
	took                 time.Duration
}

// pointDrag is a press on a point's pin being dragged: index is the point
// of the current area, from where it was, to where the drag drops it once
// moved past clickSlop over a surface.
type pointDrag struct {
	active, moved bool
	index         int
	press         f32.Point
	from, to      spawn.Point
}

// pointsKey is what the panel's points section was last built for: the
// area, the document's edits (not the overlay's version, which a drag
// bumps every frame) and the language.
type pointsKey struct {
	area  spawn.AreaID
	edits int
	lang  locale.Language
}

// pointsInfo is the panel's points section of the current area.
type pointsInfo struct {
	note            string
	stats, warnings []string
}

// placementRequest is the distribution of area a's points with seed: its
// outline, range and parameters, and the preview monster's height as the
// slice obstacles are cut to.
func placementRequest(a spawn.Area, seed uint64) placement.Request {
	o := make([]placement.Vertex, len(a.Outline))
	for i, v := range a.Outline {
		o[i] = placement.Vertex{X: float64(v.X), Y: float64(v.Y)}
	}
	return placement.Request{
		Outline: o, ZMin: float64(a.ZMin), ZMax: float64(a.ZMax),
		Count: a.Params.Count, Radius: float64(a.Params.Radius), Clearance: float64(a.Params.Clearance),
		Height: float64(model.MonsterHeight), Seed: seed,
	}
}

// otherSeed is a seed for Regerar, never old.
func otherSeed(old uint64) uint64 {
	for {
		if s := uint64(rand.Uint32()); s != old {
			return s
		}
	}
}

// generate distributes the current area's points over w in the background
// (spec D3): with the area's seed, or another one when regenerate. The
// result comes back as one SetPoints in receive. It returns the status
// message.
//
// The snapshot always counts the static meshes, even while the Meshes
// switch hides them from the viewport (and from World.Geometry): hiding
// them is a way to look at the floor, not a statement that monsters may
// stand inside a rock or a fence.
func (e *spawnEditor) generate(w *scene.World, regenerate bool) locale.Message {
	a, ok := e.current()
	switch {
	case !ok:
		return locale.Message{Key: "spawn.generate.select"}
	case w == nil:
		return locale.Message{Key: "spawn.generate.no_map"}
	case e.generating != 0:
		return locale.Message{Key: "spawn.generate.busy"}
	}
	for _, p := range e.doc.AreaProblems(a.ID) {
		if outlineRule(p.Rule) || p.Rule == spawn.InvalidCount {
			return locale.Message{Key: "spawn.generate.blocked", Args: map[string]string{"name": a.Name}, Parts: map[string]locale.Message{"problem": p.Message()}}
		}
	}
	seed := a.Seed
	if regenerate {
		seed = otherSeed(a.Seed)
	}
	snap := scene.NewWorld(w.Origin)
	for _, s := range w.Scenes() {
		snap.Add(s)
	}
	snap.HideMeshes = false
	req := placementRequest(a, seed)
	g := generation{
		doc: e.doc, area: a.ID, name: a.Name, seed: seed, fp: a.Fingerprint(seed), count: a.Params.Count,
		regenerate: regenerate, replaced: len(a.Points) > 0,
	}
	e.generating = a.ID
	e.version++
	log.Printf("spawn: %s: gerando %s com a semente %d", a.Name, inflect.Count(a.Params.Count, "ponto", "pontos"), seed)
	results, win := e.results, e.win
	go func() {
		began := time.Now()
		g.res = placement.Distribute(snap, req)
		g.took = time.Since(began)
		results <- g
		win.Invalidate()
	}()
	return locale.Message{Key: "spawn.generate.running", Args: map[string]string{"name": a.Name}}
}

// receive stores the generations that came back, and returns the status
// message of the last one; empty when none did.
func (e *spawnEditor) receive() locale.Message {
	var msg locale.Message
	for {
		select {
		case g := <-e.results:
			if m := e.store(g); m.Key != "" {
				msg = m
			}
		default:
			return msg
		}
	}
}

// store applies generation g as one SetPoints: its points, seed,
// fingerprint and warnings ("cabem K de N" when fewer than the count fit,
// "nenhuma célula livre" when no floor was free).
func (e *spawnEditor) store(g generation) locale.Message {
	if g.doc != e.doc {
		return locale.Message{}
	}
	e.generating = 0
	e.version++
	pts := make([]spawn.Point, len(g.res.Points))
	for i, p := range g.res.Points {
		pts[i] = spawn.Point(p)
	}
	var warnings []spawn.Warning
	switch {
	case g.res.FreeArea == 0:
		warnings = []spawn.Warning{{Rule: spawn.NoFreeCell}}
	case g.res.Fit() < g.count:
		warnings = []spawn.Warning{{Rule: spawn.FitsOnly, Placed: g.res.Fit(), Requested: g.count}}
	}
	if e.apply(spawn.SetPoints{Area: g.area, Seed: g.seed, Fingerprint: g.fp, Points: pts, Warnings: warnings, Measurement: spawn.Measurement{Known: true, FreeArea: g.res.FreeArea}}) != nil {
		return locale.Message{Key: "spawn.generate.failed", Args: map[string]string{"name": g.name}}
	}
	if g.area == e.area {
		e.point, e.pointDrag = areaPoint{}, pointDrag{}
	}
	log.Printf("spawn: %s: %s de %d em %v, semente %d, chão livre %.0f u², espaçamento médio %.1f, menor %.1f",
		g.name, inflect.Count(g.res.Fit(), "ponto gerado", "pontos gerados"), g.count, g.took.Round(time.Millisecond),
		g.seed, g.res.FreeArea, g.res.MeanSpacing, g.res.MinSpacing)
	args := map[string]string{"name": g.name, "requested": intArg(g.count), "ms": intArg(int(g.took.Milliseconds()))}
	msg := locale.Message{Key: "spawn.generate.done", Count: g.res.Fit(), Plural: true, Args: args}
	switch {
	case g.res.FreeArea == 0:
		msg = locale.Message{Key: "spawn.generate.no_free_cell", Args: args}
	case g.res.Fit() < g.count:
		msg = locale.Message{Key: "spawn.generate.fits_only", Count: g.res.Fit(), Plural: true, Args: args}
	}
	if g.regenerate && g.replaced {
		msg = locale.Message{Key: "spawn.generate.regenerated", Parts: map[string]locale.Message{"result": msg}}
	}
	return msg
}

// selectedPoint is the selected point of the current area.
func (e *spawnEditor) selectedPoint() (int, bool) {
	a, ok := e.current()
	if !ok || e.point.area != a.ID || e.point.index >= len(a.Points) {
		return -1, false
	}
	return e.point.index, true
}

// shownSpawnPoints is a's points as the overlay draws them: the one being
// dragged where the drag drops it.
func (e *spawnEditor) shownSpawnPoints(a spawn.Area) []spawn.Point {
	d := e.pointDrag
	if !d.active || !d.moved || a.ID != e.area || d.index >= len(a.Points) {
		return a.Points
	}
	pts := append([]spawn.Point(nil), a.Points...)
	pts[d.index] = d.to
	return pts
}

// pins is every point of every shown area as a pin for the renderer's
// overlay: the circle of the area's radius on the floor, the stem and the
// handle on top, yellow for the selected point. Stale points are grey with
// a red handle. The population mode appends them to overlay's shapes.
func (e *spawnEditor) pins() []render.ZoneShape {
	var shapes []render.ZoneShape
	h := float32(pinHeight())
	sel, selOK := e.selectedPoint()
	for _, a := range e.doc.Areas() {
		if a.Hidden || (e.drawing && a.ID == e.area) || len(a.Points) == 0 {
			continue
		}
		c := linearColor(areaColor(a.ID))
		stale := a.Stale()
		if stale {
			c = staleColor
		}
		r := float64(a.Params.Radius)
		for i, p := range e.shownSpawnPoints(a) {
			foot := geom.Vec3{X: float32(p.X), Y: float32(p.Y), Z: float32(p.Z) + pinLift}
			top := geom.Vec3{X: foot.X, Y: foot.Y, Z: float32(p.Z) + h}
			circle := make([]geom.Vec3, pinSides+1)
			for k := range circle {
				t := 2 * math.Pi * float64(k) / pinSides
				circle[k] = geom.Vec3{X: float32(float64(p.X) + r*math.Cos(t)), Y: float32(float64(p.Y) + r*math.Sin(t)), Z: foot.Z}
			}
			head := render.ZoneShape{Points: []geom.Vec3{top}, Color: c, Marked: -1}
			if selOK && a.ID == e.area && i == sel {
				head.Marked = 0
			}
			if stale {
				head.BadVertices = []int{0}
			}
			shapes = append(shapes,
				render.ZoneShape{Points: circle, Color: c, Marked: -1, NoHandles: true},
				render.ZoneShape{Points: []geom.Vec3{foot, top}, Color: c, Marked: -1, NoHandles: true},
				head)
		}
	}
	return shapes
}

// pressPoint grabs the pin under ev (its top handle or its foot) of any
// shown area, selecting the point and its area.
func (e *spawnEditor) pressPoint(w *scene.World, cam *camera.Camera, ev pointer.Event, vp image.Point) (locale.Message, bool) {
	best := float32(grabSlop)
	var hit spawn.AreaID
	index := -1
	h := pinHeight()
	for _, a := range e.doc.Areas() {
		if a.Hidden {
			continue
		}
		for i, p := range a.Points {
			for _, z := range [2]int{p.Z + h, p.Z} {
				if d, ok := screenDist(w, cam, zone.Point{X: p.X, Y: p.Y, Z: z}, ev.Position, vp); ok && d <= best {
					best, hit, index = d, a.ID, i
				}
			}
		}
	}
	if hit == 0 {
		return locale.Message{}, false
	}
	e.area, e.sel, e.point = hit, -1, areaPoint{hit, index}
	e.version++
	a, _ := e.doc.Area(hit)
	p := a.Points[index]
	e.pointDrag = pointDrag{active: true, index: index, press: ev.Position, from: p}
	return locale.Message{Key: "spawn.point.grabbed", Args: map[string]string{"index": intArg(index + 1), "name": a.Name, "x": intArg(p.X), "y": intArg(p.Y), "z": intArg(p.Z)}}, true
}

// pointDragEvent follows a point drag: the point drops on the surface
// under the cursor, keeping its heading, and the release applies the move
// as one MovePoint.
func (e *spawnEditor) pointDragEvent(w *scene.World, cam *camera.Camera, ev pointer.Event, vp image.Point) locale.Message {
	switch ev.Kind {
	case pointer.Drag:
		d := &e.pointDrag
		if !d.moved && dist(ev.Position, d.press) <= clickSlop {
			return locale.Message{}
		}
		if h, ok := pickAt(w, cam, ev.Position, vp); ok {
			v := serverPoint(h)
			d.to, d.moved = spawn.Point{X: v.X, Y: v.Y, Z: v.Z, Heading: d.from.Heading}, true
			e.version++
			return locale.Message{Key: "spawn.drag.drop", Args: pointArgs(v)}
		}
		return locale.Message{Key: "editor.click.no_surface"}
	case pointer.Release:
		d := e.pointDrag
		e.pointDrag = pointDrag{}
		e.version++
		if !d.moved || d.to == d.from {
			return locale.Message{}
		}
		return e.movePoint(d.index, d.to)
	case pointer.Cancel:
		e.pointDrag = pointDrag{}
		e.version++
	}
	return locale.Message{}
}

func (e *spawnEditor) movePoint(i int, p spawn.Point) locale.Message {
	a, ok := e.current()
	if !ok || e.apply(spawn.MovePoint{Area: a.ID, Index: i, Point: p}) != nil {
		return locale.Message{Key: "spawn.point.move_failed"}
	}
	log.Printf("spawn: %s: ponto %d movido para %d %d %d", a.Name, i+1, p.X, p.Y, p.Z)
	return locale.Message{Key: "spawn.point.moved", Args: map[string]string{"index": intArg(i + 1), "name": a.Name, "x": intArg(p.X), "y": intArg(p.Y), "z": intArg(p.Z)}}
}

// removePoint deletes the selected point.
func (e *spawnEditor) removePoint() locale.Message {
	i, ok := e.selectedPoint()
	if !ok {
		return locale.Message{}
	}
	a, _ := e.current()
	if e.apply(spawn.RemovePoint{Area: a.ID, Index: i}) != nil {
		return locale.Message{Key: "spawn.point.remove_failed"}
	}
	e.point = areaPoint{}
	log.Printf("spawn: %s: ponto %d apagado", a.Name, i+1)
	return locale.Message{Key: "spawn.point.removed", Count: len(a.Points) - 1, Plural: true, Args: map[string]string{"index": intArg(i + 1), "name": a.Name}}
}

// setAdding starts (on) or stops adding points with viewport clicks to
// the current area; starting puts the armed tool down.
func (e *spawnEditor) setAdding(on bool) locale.Message {
	if !on {
		if !e.adding {
			return locale.Message{}
		}
		e.adding = false
		e.version++
		return locale.Message{Key: "spawn.point.adding_off"}
	}
	a, ok := e.current()
	if !ok {
		return locale.Message{Key: "spawn.point.select_area"}
	}
	e.adding, e.armed, e.anchored, e.hovering = true, false, false, false
	e.version++
	return locale.Message{Key: "spawn.hint.add_points", Args: map[string]string{"name": a.Name}}
}

// addPoint adds a point to the current area where the click picked h (ok:
// something was hit), facing a random heading, and selects it.
func (e *spawnEditor) addPoint(h scene.Hit, ok bool) locale.Message {
	a, found := e.current()
	switch {
	case !found:
		e.adding = false
		e.version++
		return locale.Message{Key: "spawn.point.select_area"}
	case !ok:
		return locale.Message{Key: "editor.click.missed"}
	}
	v := serverPoint(h)
	p := spawn.Point{X: v.X, Y: v.Y, Z: v.Z, Heading: 1 + rand.IntN(65535)}
	if e.apply(spawn.AddPoint{Area: a.ID, Point: p}) != nil {
		return locale.Message{Key: "spawn.point.add_failed"}
	}
	e.sel, e.point = -1, areaPoint{a.ID, len(a.Points)}
	log.Printf("spawn: %s: ponto %d adicionado em %d %d %d", a.Name, len(a.Points)+1, p.X, p.Y, p.Z)
	return locale.Message{Key: "spawn.point.added", Count: len(a.Points) + 1, Plural: true, Args: map[string]string{"name": a.Name, "x": intArg(p.X), "y": intArg(p.Y), "z": intArg(p.Z)}}
}

// pointsPanel is the panel's points section of the current area: the count,
// the free floor its generation measured and the spacing of its points as
// they are now, then the stale state and the distribution's warnings.
func (e *spawnEditor) pointsPanel(lang locale.Language) pointsInfo {
	a, ok := e.current()
	key := pointsKey{edits: e.edits, lang: lang}
	if ok {
		key.area = a.ID
	}
	if key == e.pointsKey {
		return e.pointsInfo
	}
	e.pointsKey = key
	if !ok {
		e.pointsInfo = pointsInfo{}
		return e.pointsInfo
	}
	info := pointsInfo{note: locale.Text(lang, "spawn.points.none")}
	if len(a.Points) > 0 {
		info.note = locale.Plural(lang, "spawn.points.count", len(a.Points), nil)
	}
	free := locale.Text(lang, "spawn.points.free_unknown")
	if a.Measurement.Known && a.Generated != "" && !a.Stale() {
		free = locale.Format(lang, "spawn.points.free", map[string]string{"area": locale.Number(lang, a.Measurement.FreeArea, 0)})
	}
	pts := make([]placement.Point, len(a.Points))
	for i, p := range a.Points {
		pts[i] = placement.Point(p)
	}
	mean, least := locale.Text(lang, "spawn.points.no_spacing"), locale.Text(lang, "spawn.points.no_spacing")
	if len(pts) >= 2 {
		m, l := placement.Spacing(pts)
		mean, least = locale.Number(lang, m, 1), locale.Number(lang, l, 1)
	}
	info.stats = []string{
		free,
		locale.Format(lang, "spawn.points.mean_spacing", map[string]string{"value": mean}),
		locale.Format(lang, "spawn.points.min_spacing", map[string]string{"value": least}),
	}
	for _, p := range e.doc.AreaProblems(a.ID) {
		if p.Rule == spawn.StalePoints || !p.Blocks() {
			info.warnings = append(info.warnings, p.Text(lang))
		}
	}
	e.pointsInfo = info
	return info
}
