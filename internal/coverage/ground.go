package coverage

import (
	"cmp"
	"math"
	"slices"

	"zonebuilder/internal/scene"
)

// GroundReach is how far, in Z, a BSP or mesh floor may lie from a shape's
// Z range and still pull it (spec D5): nearer, it is the floor of a
// building, a bridge or a ramp the zone is about; farther, it is a tower's
// roof, a tree canopy or a cave floor the zone is not meant to reach.
const GroundReach = 1024

// Ground is the floor a shape's Z range should span by the layer rule of
// spec D5: every terrain piece, and the BSP and mesh pieces that cross the
// range or lie within GroundReach of it.
type Ground struct {
	// Measured is false when no floor counts; Min and Max are then
	// meaningless.
	Measured bool
	// Min and Max are the lowest and highest floor that counts.
	Min, Max Spot
	// Others are the BSP and mesh floors left out, as layers: pieces
	// closer than LayerGap in Z are one layer. Lowest first.
	Others []Layer
}

// Layer is a span of floor in Z: from its lowest to its highest point.
type Layer struct{ Low, High float64 }

// Ground is the floor under the profile's outline that a Z range now at
// [zmin, zmax] should span (spec D5). The range is the reference that
// decides which BSP and mesh floors count: a shape's current range when
// fitting it to the ground, the range of its vertices when it is created.
func (p *Profile) Ground(zmin, zmax float64) Ground {
	var g Ground
	for pc := range p.Pieces() {
		if pc.Surface != scene.SurfaceTerrain && (pc.High.Z < zmin-GroundReach || pc.Low.Z > zmax+GroundReach) {
			g.Others = append(g.Others, Layer{pc.Low.Z, pc.High.Z})
			continue
		}
		if !g.Measured {
			g.Min, g.Max, g.Measured = pc.Low, pc.High, true
			continue
		}
		if pc.Low.Z < g.Min.Z {
			g.Min = pc.Low
		}
		if pc.High.Z > g.Max.Z {
			g.Max = pc.High
		}
	}
	g.Others = mergeLayers(g.Others)
	return g
}

// mergeLayers merges spans closer than LayerGap into layers, lowest first.
func mergeLayers(spans []Layer) []Layer {
	if len(spans) == 0 {
		return nil
	}
	slices.SortFunc(spans, func(a, b Layer) int { return cmp.Compare(a.Low, b.Low) })
	out := spans[:1]
	for _, s := range spans[1:] {
		last := &out[len(out)-1]
		if s.Low-last.High < LayerGap {
			last.High = max(last.High, s.High)
			continue
		}
		out = append(out, s)
	}
	return out
}

// Range is the Z range that spans g with margin units to spare on each
// side: margin below the lowest floor and above the highest, rounded
// outwards to whole units so the clearance is never under margin.
func (g Ground) Range(margin int) (zmin, zmax int) {
	return g.Floor(margin), g.Top(margin)
}

// Floor is the zmin margin units below g's lowest floor, rounded down.
func (g Ground) Floor(margin int) int { return int(math.Floor(g.Min.Z)) - margin }

// Top is the zmax margin units above g's highest floor, rounded up.
func (g Ground) Top(margin int) int { return int(math.Ceil(g.Max.Z)) + margin }
