package main

import (
	"image"
	"log"
	"math"
	"slices"

	"gioui.org/f32"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/coverage"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/render"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// Overlay colours (linear RGB): exclusions, restart points and player-killer
// restart points stand apart from the zones' included shapes.
var (
	bannedColor    = [3]float32{0.1, 0.45, 1}
	restartColor   = [3]float32{0.1, 0.9, 0.2}
	pkRestartColor = [3]float32{0.9, 0.05, 0.6}
)

const (
	// minCircleRadius is the smallest circle the tool takes, in server
	// units: below it the polygon's rounded vertices start to merge.
	minCircleRadius = 32
	// restartPinHeight is how far above its point a restart point's pin
	// reaches in the overlay.
	restartPinHeight = 160
	// groundAbove and groundReach bound the vertical pick that drops a
	// generated vertex (a circle vertex, a rectangle's other corners) onto
	// the surface under it: it starts groundAbove over the clicked height
	// and takes a hit no further than groundReach from it, so a roof or a
	// cave floor far from the drawing level is not picked (the reach of
	// the range rule, spec D5).
	groundAbove = zone.DefaultZMargin
	groundReach = coverage.GroundReach
)

// arm makes tool take the viewport clicks for the selected zone; a shape
// tool draws an exclusion when banned is set. It returns the status line.
func (e *zoneEditor) arm(tool ui.Tool, banned bool) string {
	if e.drawing {
		return e.present(locale.Message{Key: "editor.tool.finish_first"})
	}
	if _, ok := e.doc.Zone(e.zone); !ok {
		return e.present(locale.Message{Key: "editor.tool.create_first"})
	}
	e.tool, e.armed, e.banned = tool, true, banned && tool.IsShape()
	e.anchored, e.hovering = false, false
	e.version++
	return e.present(e.hintMessage())
}

// hint is the armed tool's instruction, or "" when none is armed.
func (e *zoneEditor) hint() string {
	if !e.armed {
		return ""
	}
	return e.hintMessage().Render(e.Language)
}

func (e *zoneEditor) hintMessage() locale.Message {
	switch e.tool {
	case ui.ToolRectangle:
		switch {
		case e.banned && e.anchored:
			return locale.Message{Key: "editor.hint.exclusion_rectangle_opposite"}
		case e.banned:
			return locale.Message{Key: "editor.hint.exclusion_rectangle_first"}
		case e.anchored:
			return locale.Message{Key: "editor.hint.rectangle_opposite"}
		}
		return locale.Message{Key: "editor.hint.rectangle_first"}
	case ui.ToolCircle:
		switch {
		case e.banned && e.anchored:
			return locale.Message{Key: "editor.hint.exclusion_circle_radius"}
		case e.banned:
			return locale.Message{Key: "editor.hint.exclusion_circle_center"}
		case e.anchored:
			return locale.Message{Key: "editor.hint.circle_radius"}
		}
		return locale.Message{Key: "editor.hint.circle_center"}
	case ui.ToolRestart:
		return locale.Message{Key: "editor.hint.restart"}
	case ui.ToolPKRestart:
		return locale.Message{Key: "editor.hint.pk_restart"}
	}
	if e.drawing {
		if e.banned {
			return locale.Message{Key: "editor.hint.exclusion_polygon_continue"}
		}
		return locale.Message{Key: "editor.hint.polygon_continue"}
	}
	if e.banned {
		return locale.Message{Key: "editor.hint.exclusion_polygon_first"}
	}
	return locale.Message{Key: "editor.hint.polygon_first"}
}

// escape puts the armed tool down, dropping a rectangle's first corner or
// a circle's center. An open polygon stays: it is in the document and
// closes with Enter. It returns the status line.
func (e *zoneEditor) escape() string {
	switch {
	case e.drawing:
		return e.present(locale.Message{Key: "editor.polygon.open"})
	case !e.armed:
		return ""
	}
	e.armed, e.anchored, e.hovering = false, false, false
	e.version++
	return e.present(locale.Message{Key: "editor.tool.put_away"})
}

// rectangleClick takes corner v: the first one anchors the rectangle, the
// second adds it to the selected zone with the Z range placed suggests.
func (e *zoneEditor) rectangleClick(s *scene.World, c *floorCoverage, v zone.Point) string {
	if !e.anchored {
		e.anchor, e.anchored = v, true
		e.version++
		return e.present(locale.Message{Key: "editor.rectangle.anchor", Args: pointArgs(v)})
	}
	a := e.anchor
	if a.X == v.X || a.Y == v.Y {
		return e.present(locale.Message{Key: "editor.rectangle.zero_size"})
	}
	_, fit := e.placed(s, c, v, true)
	z, _ := e.doc.Zone(e.zone)
	if e.apply(zone.AddShape{Zone: e.zone, Kind: zone.Rectangle, Banned: e.banned, Points: []zone.Point{a, v}, ZMin: fit.zmin, ZMax: fit.zmax}) != nil {
		return e.present(locale.Message{Key: "editor.rectangle.failed"})
	}
	e.armed, e.anchored, e.hovering = false, false, false
	e.shape = len(z.Shapes)
	what := "Retângulo"
	if e.banned {
		what = "Exclusão retangular"
	}
	log.Printf("zona: %s: %s %d %d … %d %d, z %d..%d %s", z.Name, what, a.X, a.Y, v.X, v.Y, fit.zmin, fit.zmax, fit.note())
	args := shapeArgs(z.Name, a, v, fit)
	if e.banned {
		return e.present(locale.Message{Key: "editor.rectangle.exclusion_done", Args: args, Parts: map[string]locale.Message{"source": fit.sourceMessage()}})
	}
	return e.present(locale.Message{Key: "editor.rectangle.done", Args: args, Parts: map[string]locale.Message{"source": fit.sourceMessage()}})
}

// wholeTile adds to the selected zone a polygon over tile t's whole square,
// [origin, origin+TileSpan-1] on X and Y (the last unit stays inside the
// tile, and inside the world bounds for the last tile), with the Z range
// suggested by the floor under it, measured over w from the terrain (spec
// D5); with no floor there, the range spans everything the tile's scene s
// holds, terrain, BSP and meshes, plus the margin. It is a polygon, not a
// rectangle, so its vertices can be moved and more inserted to trim it.
// banned adds it as an exclusion.
func (e *zoneEditor) wholeTile(c *floorCoverage, w *scene.World, t scene.Tile, s *scene.Scene, banned bool) string {
	z, ok := e.doc.Zone(e.zone)
	switch {
	case e.drawing:
		return e.present(locale.Message{Key: "editor.tile.finish_first"})
	case !ok:
		return e.present(locale.Message{Key: "editor.tile.create_first"})
	case s.Bounds.Empty():
		return e.present(locale.Message{Key: "editor.tile.no_geometry", Args: map[string]string{"tile": t.Name()}})
	}
	ox, oy := t.Origin()
	x0, y0 := int(ox), int(oy)
	x1, y1 := x0+scene.TileSpan-1, y0+scene.TileSpan-1
	vmin := int(math.Floor(float64(s.Bounds.Min.Z+scene.ServerZOffset))) - e.margin
	vmax := int(math.Ceil(float64(s.Bounds.Max.Z+scene.ServerZOffset))) + e.margin
	floor := vmin + e.margin
	pts := []zone.Point{{X: x0, Y: y0, Z: floor}, {X: x1, Y: y0, Z: floor}, {X: x1, Y: y1, Z: floor}, {X: x0, Y: y1, Z: floor}}
	fit := c.suggest(e, w, len(z.Shapes), pts, vmin, vmax, true, true)
	if e.apply(zone.AddShape{Zone: e.zone, Banned: banned, Points: pts, ZMin: fit.zmin, ZMax: fit.zmax}) != nil {
		return e.present(locale.Message{Key: "editor.tile.failed"})
	}
	e.armed, e.anchored, e.hovering = false, false, false
	e.shape = len(z.Shapes)
	what := "Tile inteiro"
	if banned {
		what = "Exclusão do tile inteiro"
	}
	log.Printf("zona: %s: %s %s: %d %d … %d %d, z %d..%d %s", z.Name, what, t.Name(), x0, y0, x1, y1, fit.zmin, fit.zmax, fit.note())
	args := shapeArgs(z.Name, pts[0], pts[2], fit)
	args["tile"] = t.Name()
	if banned {
		return e.present(locale.Message{Key: "editor.tile.exclusion_done", Args: args, Parts: map[string]locale.Message{"source": fit.sourceMessage()}})
	}
	return e.present(locale.Message{Key: "editor.tile.done", Args: args, Parts: map[string]locale.Message{"source": fit.sourceMessage()}})
}

// wholeTile covers, in the selected zone, the tile in view: the one under
// the viewport's centre, or under the camera when the centre meets nothing.
func wholeTile(e *zoneEditor, c *floorCoverage, ts *tiles, cam *camera.Camera, vp image.Point, banned bool) string {
	if ts.world == nil {
		return e.present(locale.Message{Key: "editor.tile.open_first"})
	}
	at := worldPosition(ts.world, cam.Position)
	if h, ok := pickAt(ts.world, cam, f32.Pt(float32(vp.X)/2, float32(vp.Y)/2), vp); ok {
		at = h.Pos
	}
	t, s, ok := ts.sceneAt(at.X, at.Y)
	if !ok {
		return e.present(locale.Message{Key: "editor.tile.not_loaded"})
	}
	return e.wholeTile(c, ts.world, t, s, banned)
}

// circleClick takes the center, then a point on the circle: the circle
// goes into the selected zone as a polygon of zone.CircleSides vertices,
// each dropped onto the surface under it, with the Z range placed
// suggests.
func (e *zoneEditor) circleClick(s *scene.World, c *floorCoverage, v zone.Point) string {
	if !e.anchored {
		e.anchor, e.anchored = v, true
		e.version++
		return e.present(locale.Message{Key: "editor.circle.anchor", Args: pointArgs(v)})
	}
	a := e.anchor
	r := radius(a, v)
	if r < minCircleRadius {
		return e.present(locale.Message{Key: "editor.circle.small_radius", Args: map[string]string{"radius": intArg(r), "min": intArg(minCircleRadius)}})
	}
	pts, fit := e.placed(s, c, v, true)
	z, _ := e.doc.Zone(e.zone)
	if e.apply(zone.AddShape{Zone: e.zone, Banned: e.banned, Points: pts, ZMin: fit.zmin, ZMax: fit.zmax}) != nil {
		return e.present(locale.Message{Key: "editor.circle.failed"})
	}
	e.armed, e.anchored, e.hovering = false, false, false
	e.shape = len(z.Shapes)
	what := "Círculo"
	if e.banned {
		what = "Exclusão circular"
	}
	log.Printf("zona: %s: %s centro %d %d raio %d → polígono de %s, z %d..%d %s", z.Name, what, a.X, a.Y, r, inflect.Count(len(pts), "vértice", "vértices"), fit.zmin, fit.zmax, fit.note())
	args := map[string]string{"name": z.Name, "radius": intArg(r), "vertices": intArg(len(pts)), "min": intArg(fit.zmin), "max": intArg(fit.zmax)}
	parts := map[string]locale.Message{"source": fit.sourceMessage()}
	if e.banned {
		return e.present(locale.Message{Key: "editor.circle.exclusion_done", Count: len(pts), Plural: true, Args: args, Parts: parts})
	}
	return e.present(locale.Message{Key: "editor.circle.done", Count: len(pts), Plural: true, Args: args, Parts: parts})
}

// restartClick adds v to the selected zone's restart points (player
// killers' with the PK tool).
func (e *zoneEditor) restartClick(v zone.Point) string {
	pk := e.tool == ui.ToolPKRestart
	if e.apply(zone.AddRestartPoint{Zone: e.zone, PK: pk, Point: v}) != nil {
		return e.present(locale.Message{Key: "editor.restart.failed"})
	}
	z, _ := e.doc.Zone(e.zone)
	name, n := "restart_point", len(z.RestartPoints)
	if pk {
		name, n = "PKrestart_point", len(z.PKRestartPoints)
	}
	log.Printf("zona: %s: %s %d em %d %d %d", z.Name, name, n, v.X, v.Y, v.Z)
	return e.present(locale.Message{Key: "editor.restart.done", Args: map[string]string{"kind": name, "index": intArg(n), "name": z.Name, "x": intArg(v.X), "y": intArg(v.Y), "z": intArg(v.Z)}})
}

// radius is the circle radius from center c to v, in server units.
func radius(c, v zone.Point) int {
	return int(math.Round(math.Hypot(float64(v.X-c.X), float64(v.Y-c.Y))))
}

// placed is the shape the armed rectangle or circle tool adds from the
// anchor to v: its outline (a rectangle's 4 corners, a circle's polygon),
// with the generated vertices dropped onto the ground of s, and its Z
// range, suggested by the floor under its area with the configured margin
// (floorCoverage.suggest, from the range its vertices suggest). Clicks
// and the preview both go through it, so the preview is what gets added:
// the preview asks for the outline's profile in the background and shows
// the vertex range until it is in; a click (now) measures it on the spot
// when it is not in yet.
func (e *zoneEditor) placed(s *scene.World, c *floorCoverage, v zone.Point, now bool) ([]zone.Point, zSuggestion) {
	a := e.anchor
	var pts, vertices []zone.Point
	if e.tool == ui.ToolCircle {
		pts = zone.CirclePoints(a, max(radius(a, v), 1), zone.CircleSides)
		for i := range pts {
			pts[i].Z = groundZ(s, pts[i].X, pts[i].Y, a.Z)
		}
		vertices = append(slices.Clone(pts), a)
	} else {
		r := zone.RectangleCorners(a, v)
		r[1].Z = groundZ(s, r[1].X, r[1].Y, r[1].Z)
		r[3].Z = groundZ(s, r[3].X, r[3].Y, r[3].Z)
		pts = r[:]
		vertices = pts
	}
	vmin, vmax := zone.SuggestZRange(vertices, e.margin)
	return pts, c.suggest(e, s, e.nextShape(), pts, vmin, vmax, false, now)
}

// nextShape is the index the next shape added to the selected zone gets.
func (e *zoneEditor) nextShape() int {
	z, _ := e.doc.Zone(e.zone)
	return len(z.Shapes)
}

// hoverAt tracks the surface point under the cursor (h, when ok) while a
// rectangle or circle is anchored, and places the preview shape on s;
// while the preview's floor is being measured, it takes the range again
// from c, so it changes when the profile comes in. It returns the status
// line when the preview's range changed, else "".
func (e *zoneEditor) hoverAt(s *scene.World, c *floorCoverage, h scene.Hit, ok bool) string {
	ok = ok && s != nil && e.armed && e.anchored
	p := zone.Point{}
	if ok {
		p = serverPoint(h)
	}
	moved := ok != e.hovering || p != e.hover
	if !moved && !(ok && e.ghostZ.from == zMeasuring) {
		return ""
	}
	e.hover, e.hovering = p, ok
	if !ok {
		e.version++
		return ""
	}
	fit := e.ghostZ
	if moved {
		e.ghost, fit = e.placed(s, c, p, false)
	} else {
		fit = c.suggest(e, s, e.nextShape(), e.ghost, fit.vmin, fit.vmax, false, false)
	}
	if !moved && fit == e.ghostZ {
		return ""
	}
	e.ghostZ = fit
	e.version++
	return e.present(locale.Message{Key: "editor.preview", Args: map[string]string{"min": intArg(fit.zmin), "max": intArg(fit.zmax)}, Parts: map[string]locale.Message{"hint": e.hintMessage(), "source": fit.sourceMessage()}})
}

// preview is the rectangle or circle being placed: from the anchor to the
// cursor, or the anchor alone while the cursor is off every surface.
func (e *zoneEditor) preview() (render.ZoneShape, bool) {
	if !e.armed || !e.anchored {
		return render.ZoneShape{}, false
	}
	z, _ := e.doc.Zone(e.zone)
	color := shapeColor(e.banned, linearColor(z.DisplayColor()))
	a := e.anchor
	if !e.hovering {
		return render.ZoneShape{Points: []geom.Vec3{serverVec(a)}, Color: color, Marked: 0}, true
	}
	rs := overlayShape(e.ghost, e.ghostZ.zmin, e.ghostZ.zmax, color)
	if e.tool == ui.ToolRectangle {
		rs.Marked = 0
	}
	return rs, true
}

// shapeColor is the overlay colour of a shape: included, the zone's own
// colour (linear); banned, the one colour of exclusions.
func shapeColor(banned bool, included [3]float32) [3]float32 {
	if banned {
		return bannedColor
	}
	return included
}

// restartPin is a restart point in the overlay: a vertical line from the
// point up, with handles at both ends.
func restartPin(p zone.Point, color [3]float32) render.ZoneShape {
	top := p
	top.Z += restartPinHeight
	return render.ZoneShape{Points: []geom.Vec3{serverVec(p), serverVec(top)}, Color: color, Marked: -1}
}

func serverVec(p zone.Point) geom.Vec3 {
	return geom.Vec3{X: float32(p.X), Y: float32(p.Y), Z: float32(p.Z)}
}

// groundZ is the server Z of the surface under x y near height ref, or ref
// when nothing is there: the first surface straight down from groundAbove
// over ref, counted only within groundReach of ref, so a roof or a cave
// floor far from the drawing level is not picked.
func groundZ(s *scene.World, x, y, ref int) int {
	o := scene.FromServer(serverVec(zone.Point{X: x, Y: y, Z: ref + groundAbove}))
	h, ok := s.Pick(scene.Ray{Origin: o, Dir: geom.Vec3{Z: -1}})
	if !ok {
		return ref
	}
	if z := round(h.Pos.Z); z >= ref-groundReach && z <= ref+groundReach {
		return z
	}
	return ref
}
