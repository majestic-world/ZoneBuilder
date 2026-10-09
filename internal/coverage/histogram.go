package coverage

import (
	"math"

	"zonebuilder/internal/scene"
)

// HistogramStep is the Z height of a Histogram bin, in units: on a fixed
// grid of absolute Z, so the histograms of a zone's shapes share bins.
const HistogramStep = 16

// Histogram is a profile's floor area by Z (spec D7's ruler): Terrain[i]
// and Built[i] are the X/Y areas of the terrain and of the BSP and mesh
// floor with Z in [(First+i)·HistogramStep, (First+i+1)·HistogramStep).
// It depends only on the profile; Split sorts it by a Z range.
type Histogram struct {
	First          int
	Terrain, Built []float64
}

// Histogram spreads the profile's floor over Z, exactly: a piece spanning
// several bins is cut at their edges, where its Z is affine. An empty
// profile has no bins.
func (p *Profile) Histogram() Histogram {
	if len(p.pieces) == 0 {
		return Histogram{}
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, pc := range p.pieces {
		lo, hi = min(lo, pc.zlo), max(hi, pc.zhi)
	}
	h := Histogram{First: histogramBin(lo)}
	h.Terrain = make([]float64, histogramBin(hi)-h.First+1)
	h.Built = make([]float64, len(h.Terrain))
	var cut []Spot // scratch polygon
	for _, pc := range p.pieces {
		area := h.Built
		if pc.surface == scene.SurfaceTerrain {
			area = h.Terrain
		}
		b0, b1 := histogramBin(pc.zlo), histogramBin(pc.zhi)
		if b0 == b1 {
			area[b0-h.First] += pc.area
			continue
		}
		vs := p.verts[pc.first : pc.first+pc.n]
		var under float64 // the piece's area under bin b's floor
		for b := b0; b < b1; b++ {
			cut = clipZ(vs, cut[:0], float64((b+1)*HistogramStep), false)
			a := min(spotArea(cut), pc.area)
			area[b-h.First] += max(0, a-under)
			under = max(under, a)
		}
		area[b1-h.First] += max(0, pc.area-under)
	}
	return h
}

// Split is h's floor with Z in [lo, hi] sorted by the Z range [zmin,
// zmax]: inside it, above zmax and below zmin. Floor the layer rule
// leaves out is another layer (spec D5) and left out: BSP and mesh floor
// farther than GroundReach from the range, and the terrain when terrain
// is false (Report.Terrain of the range). A bin cut by lo, hi, zmin, zmax
// or the reach is apportioned as if its floor spread evenly over its Z,
// so the parts of a bin are exact only where its floor does, and a BSP or
// mesh piece across the reach loses its part beyond it, which Classify
// counts whole.
func (h Histogram) Split(lo, hi, zmin, zmax float64, terrain bool) (inside, above, below float64) {
	b0 := max(histogramBin(lo), h.First)
	b1 := min(histogramBin(hi), h.First+len(h.Terrain)-1)
	split := func(a, s, e float64) {
		if a == 0 || e <= s {
			return
		}
		per := a / HistogramStep
		inside += per * overlap(s, e, zmin, zmax)
		above += per * overlap(s, e, zmax, math.Inf(1))
		below += per * overlap(s, e, math.Inf(-1), zmin)
	}
	for b := b0; b <= b1; b++ {
		s := max(lo, float64(b*HistogramStep))
		e := min(hi, float64((b+1)*HistogramStep))
		if terrain {
			split(h.Terrain[b-h.First], s, e)
		}
		split(h.Built[b-h.First], max(s, zmin-GroundReach), min(e, zmax+GroundReach))
	}
	return inside, above, below
}

// overlap is the length of [a, b] ∩ [c, d], 0 when they don't meet.
func overlap(a, b, c, d float64) float64 { return max(0, min(b, d)-max(a, c)) }

// histogramBin is the bin holding Z z.
func histogramBin(z float64) int { return int(math.Floor(z / HistogramStep)) }
