package main

import (
	"log"
	"math"
	"time"

	"zonebuilder/internal/coverage"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/zone"
)

// newShape is the shape index under which floorCoverage keeps the profile
// of the selected zone's shape being placed, not in the document yet.
const newShape = -1

// zSource is where a new shape's Z range comes from.
type zSource int

const (
	// zByFloor: the floor under the shape's whole area (spec D5).
	zByFloor zSource = iota
	// zMeasuring: the vertices, while the floor under the outline is
	// measured in the background (the preview only).
	zMeasuring
	// zOffTiles: the vertices, as part of the area lies off the loaded
	// tiles, where no floor is measured.
	zOffTiles
	// zNoFloor: the vertices, as no floor under the area counts.
	zNoFloor
)

// zSuggestion is the Z range a new shape gets and where it comes from.
type zSuggestion struct {
	zmin, zmax int
	from       zSource
	// vmin and vmax are the range the shape's vertices suggest
	// (zone.SuggestZRange): what it gets when the floor gives nothing.
	vmin, vmax int
}

// note says, for the status line, where z's range came from.
func (z zSuggestion) note() string {
	switch z.from {
	case zByFloor:
		return "pelo chão da área"
	case zMeasuring:
		return "pelos vértices, medindo o chão…"
	case zOffTiles:
		return "pelos vértices: parte da área fora dos tiles carregados"
	}
	return "pelos vértices: nenhum chão medido na área"
}

// suggest is the Z range for a shape of e's selected zone not yet in the
// document, of outline pts over w (spec D5): margin below the lowest and
// above the highest floor under its whole area that the layer rule counts
// (coverage.Profile.Ground). The rule measures BSP and mesh floors from
// the range the shape would get without them: vmin…vmax, from its
// vertices, or, with fromTerrain, the terrain under the outline plus the
// margin (the whole tile, whose vertices lie on nothing). vmin…vmax is the range
// itself when part of the outline lies off w's tiles, when no floor
// counts, and, unless now, while the outline's profile is measured in the
// background; now measures it on the spot.
func (c *floorCoverage) suggest(e *zoneEditor, w *scene.World, pts []zone.Point, vmin, vmax int, fromTerrain, now bool) zSuggestion {
	z := zSuggestion{zmin: vmin, zmax: vmax, vmin: vmin, vmax: vmax}
	if w == nil || offTiles(w, coverageOutline(pts)) {
		z.from = zOffTiles
		return z
	}
	p := c.newProfile(e, w, pts, now)
	if p == nil {
		z.from = zMeasuring
		return z
	}
	lo, hi := float64(vmin), float64(vmax)
	if fromTerrain {
		var ok bool
		if lo, hi, ok = terrainSpan(p); !ok {
			z.from = zNoFloor
			return z
		}
		lo, hi = lo-float64(e.margin), hi+float64(e.margin)
	}
	g := p.Ground(lo, hi)
	if !g.Measured {
		z.from = zNoFloor
		return z
	}
	z.zmin, z.zmax = g.Range(e.margin)
	z.from = zByFloor
	return z
}

// newProfile is the floor profile of the new shape of e's selected zone,
// of outline pts over w: the one kept when it is for pts, else measured
// on the spot when now, else nil and measured in the background once
// nothing else is.
func (c *floorCoverage) newProfile(e *zoneEditor, w *scene.World, pts []zone.Point, now bool) *coverage.Profile {
	c.receive(e)
	key := newCoverageKey(shapeRef{e.zone, newShape}, pts, w)
	if s := c.shapes[key.shape]; s != nil && s.done == key {
		return s.profile
	}
	if !now {
		if !c.running {
			c.start(key, pts, w)
		}
		return nil
	}
	began := time.Now()
	p := coverage.Measure(w, coverageOutline(pts))
	log.Printf("cobertura: perfil do shape novo medido na hora em %v", time.Since(began).Round(time.Millisecond))
	c.shapes[key.shape] = &shapeCoverage{done: key, profile: p, hist: p.Histogram()}
	return p
}

// adopt hands the new shape's profile, when it is for outline pts over w,
// to shape i of e's selected zone, just created with that outline, so it
// is not measured again.
func (c *floorCoverage) adopt(e *zoneEditor, w *scene.World, i int, pts []zone.Point) {
	from := shapeRef{e.zone, newShape}
	s := c.shapes[from]
	if s == nil || w == nil || s.done != newCoverageKey(from, pts, w) {
		return
	}
	delete(c.shapes, from)
	to := shapeRef{e.zone, i}
	s.done.shape = to
	c.shapes[to] = &shapeCoverage{done: s.done, profile: s.profile, hist: s.hist}
}

// offSlack is how much of an outline's area, in server units², may lie
// off the loaded tiles before it counts as off them: float error only.
const offSlack = 0.5

// offTiles reports whether part of outline o lies off the squares of w's
// tiles, where no floor is measured.
func offTiles(w *scene.World, o coverage.Outline) bool {
	in := 0.0
	for _, s := range w.Scenes() {
		for _, t := range s.Tiles {
			x, y := t.Origin()
			lo := coverage.Point{X: float64(x), Y: float64(y)}
			in += o.AreaIn(lo, coverage.Point{X: lo.X + scene.TileSpan, Y: lo.Y + scene.TileSpan})
		}
	}
	return in < o.Area()-offSlack
}

// terrainSpan is the lowest and highest terrain in p, false when p has
// none.
func terrainSpan(p *coverage.Profile) (lo, hi float64, ok bool) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for pc := range p.Pieces() {
		if pc.Surface == scene.SurfaceTerrain {
			lo, hi = min(lo, pc.Low.Z), max(hi, pc.High.Z)
		}
	}
	return lo, hi, lo <= hi
}
