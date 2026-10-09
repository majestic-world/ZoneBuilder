package coverage

import (
	"cmp"
	"math"
	"runtime"
	"slices"
	"sync"

	"zonebuilder/internal/scene"
)

// GroundReach is how far, in Z, a BSP or mesh floor may lie from a shape's
// Z range and still pull it (spec D5): nearer, it is the floor of a
// building, a bridge or a ramp the zone is about; farther, it is a tower's
// roof, a tree canopy or a cave floor the zone is not meant to reach.
const GroundReach = 1024

// reaches is the layer rule of spec D5: whether piece pc is floor of the Z
// range [zmin, zmax]. Terrain always is; a BSP or mesh piece is when it
// crosses the range or lies within GroundReach of it, else it is another
// layer. Fit fits a range by it and Classify reports by it.
func reaches(pc *piece, zmin, zmax float64) bool {
	return pc.surface == scene.SurfaceTerrain || (pc.zhi >= zmin-GroundReach && pc.zlo <= zmax+GroundReach)
}

// Ground is the floor a Z range spans by the layer rule of spec D5.
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

// Side is a set of sides of a Z range for Fit to move.
type Side uint8

const (
	FloorSide Side = 1 << iota
	TopSide
	BothSides = FloorSide | TopSide
)

// Fit fits the given sides of the Z range [zmin, zmax] to the floor under
// the profile's outline (spec D5): the floor margin units below the
// lowest floor that counts, the top margin units above the highest,
// rounded outwards to whole units so the clearance is never under margin.
// Which BSP and mesh floor counts is judged once, from [zmin, zmax], so a
// tower's floors within GroundReach pull the range but not the roof
// above them. Classify judges from the fitted range, so a BSP or mesh
// layer beyond reach of [zmin, zmax] but within reach of the fitted range
// is reported above or below it, with its warning, right after the fit;
// fitting again takes it in. That is spec D5's rule: the user decides.
// Floor a ban excludes is not counted, as Classify does not. It gives the
// fitted range and the floor it spans; the range is [zmin, zmax]
// unchanged when no floor counts (g.Measured false).
func (p *Profile) Fit(zmin, zmax, margin int, sides Side) (lo, hi int, g Ground) {
	lo, hi = zmin, zmax
	g = p.ground(float64(zmin), float64(zmax))
	if !g.Measured {
		return lo, hi, g
	}
	if sides&FloorSide != 0 {
		lo = int(math.Floor(g.Min.Z)) - margin
	}
	if sides&TopSide != 0 {
		hi = int(math.Ceil(g.Max.Z)) + margin
	}
	return lo, hi, g
}

// ground is the floor of the Z range [zmin, zmax] by the layer rule, the
// excluded floor left out.
func (p *Profile) ground(zmin, zmax float64) Ground {
	var g Ground
	for i := range p.pieces {
		pc := &p.pieces[i]
		if (pc.nbans > 0 && p.excluded(pc)) || !reaches(pc, zmin, zmax) {
			continue
		}
		if g.Measured && pc.zlo >= g.Min.Z && pc.zhi <= g.Max.Z {
			continue
		}
		lo, hi := p.span(pc)
		if !g.Measured {
			g.Min, g.Max, g.Measured = lo, hi, true
			continue
		}
		if lo.Z < g.Min.Z {
			g.Min = lo
		}
		if hi.Z > g.Max.Z {
			g.Max = hi
		}
	}
	g.Others = p.others(zmin, zmax)
	return g
}

// others are the BSP and mesh floors the layer rule leaves out of the Z
// range [zmin, zmax], as layers, lowest first; excluded floor is not
// among them. They are read off byLow, the pieces sorted by their lowest
// floor: the pieces under the range's reach are a prefix of it and those
// over it a suffix, so a drag of the range skips the floor between.
func (p *Profile) others(zmin, zmax float64) []Layer {
	under, _ := slices.BinarySearchFunc(p.byLow, zmin-GroundReach, func(l lowPiece, z float64) int {
		return cmp.Compare(l.zlo, z)
	})
	over, _ := slices.BinarySearchFunc(p.byLow, zmax+GroundReach, func(l lowPiece, z float64) int {
		// The first piece whose lowest floor lies over z.
		if l.zlo <= z {
			return -1
		}
		return 1
	})
	var out []Layer
	add := func(l lowPiece) {
		pc := &p.pieces[l.piece]
		if reaches(pc, zmin, zmax) || (pc.nbans > 0 && p.excluded(pc)) {
			return
		}
		if n := len(out); n > 0 && pc.zlo-out[n-1].High < LayerGap {
			out[n-1].High = max(out[n-1].High, pc.zhi)
			return
		}
		out = append(out, Layer{pc.zlo, pc.zhi})
	}
	for _, l := range p.byLow[:under] {
		add(l)
	}
	for _, l := range p.byLow[max(over, under):] {
		add(l)
	}
	return out
}

// lowPiece is a piece of Profile.byLow and its lowest floor.
type lowPiece struct {
	zlo   float64
	piece int32
}

// sortByLow fills byLow with the BSP and mesh pieces, lowest floor first.
// A whole tile has some 145 000 mesh pieces, which take 11 ms to sort on
// 1 CPU: they are sorted in runs on every CPU, then merged pairwise.
func (p *Profile) sortByLow() {
	for i, pc := range p.pieces {
		if pc.surface != scene.SurfaceTerrain {
			p.byLow = append(p.byLow, lowPiece{pc.zlo, int32(i)})
		}
	}
	byZ := func(a, b lowPiece) int { return cmp.Compare(a.zlo, b.zlo) }
	n := len(p.byLow)
	runs := min(runtime.GOMAXPROCS(0), (n+stackChunk-1)/stackChunk)
	if runs <= 1 {
		slices.SortFunc(p.byLow, byZ)
		return
	}
	// bounds[k] is where run k starts; the last is n.
	bounds := make([]int, runs+1)
	for k := range bounds {
		bounds[k] = k * n / runs
	}
	var wg sync.WaitGroup
	for k := range runs {
		wg.Go(func() { slices.SortFunc(p.byLow[bounds[k]:bounds[k+1]], byZ) })
	}
	wg.Wait()
	src, dst := p.byLow, make([]lowPiece, n)
	for len(bounds) > 2 {
		next := []int{}
		for k := 0; k+1 < len(bounds); k += 2 {
			lo, mid := bounds[k], bounds[k+1]
			next = append(next, lo)
			if k+2 == len(bounds) {
				copy(dst[lo:mid], src[lo:mid]) // an odd run out
				continue
			}
			hi := bounds[k+2]
			wg.Go(func() { mergeLow(dst[lo:hi], src[lo:mid], src[mid:hi]) })
		}
		wg.Wait()
		src, dst, bounds = dst, src, append(next, n)
	}
	p.byLow = src
}

// mergeLow merges sorted a and b into out, of their length together.
func mergeLow(out, a, b []lowPiece) {
	i, j := 0, 0
	for k := range out {
		if j == len(b) || (i < len(a) && a[i].zlo <= b[j].zlo) {
			out[k] = a[i]
			i++
		} else {
			out[k] = b[j]
			j++
		}
	}
}
