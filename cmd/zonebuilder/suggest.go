package main

import (
	"math"

	"zonebuilder/internal/coverage"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/zone"
)

// zSource is where a new shape's Z range comes from.
type zSource int

const (
	// zByFloor: the floor under the shape's whole area (spec D5).
	zByFloor zSource = iota
	// zByVertices: the clicked points alone, as the panel asks (spec
	// D2); nothing is measured.
	zByVertices
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
func (z zSuggestion) note() string { return z.noteFor(locale.PtBR) }

func (z zSuggestion) sourceMessage() locale.Message {
	switch z.from {
	case zByFloor:
		return locale.Message{Key: "editor.source.ground"}
	case zByVertices:
		return locale.Message{Key: "editor.source.vertices"}
	case zMeasuring:
		return locale.Message{Key: "editor.source.measuring"}
	case zOffTiles:
		return locale.Message{Key: "editor.source.off_tiles"}
	}
	return locale.Message{Key: "editor.source.no_ground"}
}

func (z zSuggestion) noteFor(lang locale.Language) string {
	return z.sourceMessage().Render(lang)
}

// suggest is the Z range for src's shape ref, being added with outline
// pts over w (spec D5): margin below the lowest and above the highest
// floor under its whole area that the layer rule counts
// (coverage.Profile.Fit). The rule judges every floor, the terrain
// included (ADR 0006), from the range the shape would get without floor:
// vmin…vmax, from its vertices, or, with fromTerrain, the terrain under
// the outline plus the margin (the whole tile, whose vertices lie on
// nothing). vmin…vmax is the range itself when the panel asks for the
// clicked points (spec "Zona oca e faixa pelo clique", D2; not with
// fromTerrain: the whole tile always takes the floor), when part of the
// outline lies off w's tiles, when no floor counts, and, unless now,
// while the outline's profile is measured in the background; now
// measures it on the spot. The profile is kept as ref's, the shape's once
// added.
func (c *floorCoverage) suggest(src coverageSource, w *scene.World, ref shapeRef, pts []zone.Point, vmin, vmax int, fromTerrain, now bool) zSuggestion {
	z := zSuggestion{zmin: vmin, zmax: vmax, vmin: vmin, vmax: vmax}
	if src.zFromVertices() && !fromTerrain {
		z.from = zByVertices
		return z
	}
	if w == nil || offTiles(w, coverageOutline(pts)) {
		z.from = zOffTiles
		return z
	}
	p := c.profile(src, w, ref, pts, now)
	if p == nil {
		z.from = zMeasuring
		return z
	}
	lo, hi := vmin, vmax
	if fromTerrain {
		tlo, thi, ok := p.TerrainSpan()
		if !ok {
			z.from = zNoFloor
			return z
		}
		margin := src.zMargin()
		lo, hi = int(math.Floor(tlo))-margin, int(math.Ceil(thi))+margin
	}
	zmin, zmax, g := p.Fit(lo, hi, src.zMargin(), coverage.BothSides)
	if !g.Measured {
		z.from = zNoFloor
		return z
	}
	z.zmin, z.zmax, z.from = zmin, zmax, zByFloor
	return z
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
