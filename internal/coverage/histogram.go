package coverage

import "math"

// HistogramStep is the Z height of a Histogram bin, in units: on a fixed
// grid of absolute Z, so the histograms of a zone's shapes share bins.
const HistogramStep = 16

// Histogram is a profile's floor area by Z (spec D7's ruler): Area[i] is
// the X/Y area of the floor with Z in [(First+i)·HistogramStep,
// (First+i+1)·HistogramStep). It depends only on the profile; Split sorts
// it by a Z range.
type Histogram struct {
	First int
	Area  []float64
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
	h.Area = make([]float64, histogramBin(hi)-h.First+1)
	var cut []Spot // scratch polygon
	for _, pc := range p.pieces {
		b0, b1 := histogramBin(pc.zlo), histogramBin(pc.zhi)
		if b0 == b1 {
			h.Area[b0-h.First] += pc.area
			continue
		}
		vs := p.verts[pc.first : pc.first+pc.n]
		var under float64 // the piece's area under bin b's floor
		for b := b0; b < b1; b++ {
			cut = clipZ(vs, cut[:0], float64((b+1)*HistogramStep), false)
			a := min(spotArea(cut), pc.area)
			h.Area[b-h.First] += max(0, a-under)
			under = max(under, a)
		}
		h.Area[b1-h.First] += max(0, pc.area-under)
	}
	return h
}

// Split is h's floor with Z in [lo, hi] sorted by the Z range [zmin,
// zmax]: inside it, above zmax and below zmin. A bin cut by lo, hi, zmin
// or zmax is apportioned as if its floor spread evenly over its Z, so the
// parts of a bin are exact only where its floor does.
func (h Histogram) Split(lo, hi, zmin, zmax float64) (inside, above, below float64) {
	b0 := max(histogramBin(lo), h.First)
	b1 := min(histogramBin(hi), h.First+len(h.Area)-1)
	for b := b0; b <= b1; b++ {
		a := h.Area[b-h.First]
		if a == 0 {
			continue
		}
		s := max(lo, float64(b*HistogramStep))
		e := min(hi, float64((b+1)*HistogramStep))
		per := a / HistogramStep
		inside += per * overlap(s, e, zmin, zmax)
		above += per * overlap(s, e, zmax, math.Inf(1))
		below += per * overlap(s, e, math.Inf(-1), zmin)
	}
	return inside, above, below
}

// overlap is the length of [a, b] ∩ [c, d], 0 when they don't meet.
func overlap(a, b, c, d float64) float64 { return max(0, min(b, d)-max(a, c)) }

// histogramBin is the bin holding Z z.
func histogramBin(z float64) int { return int(math.Floor(z / HistogramStep)) }
