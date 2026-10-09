package coverage

import "math"

// Area is o's X/Y area, whichever way it winds.
func (o Outline) Area() float64 {
	if len(o) < 3 {
		return 0
	}
	return math.Abs(signedArea2(o)) / 2
}

// AreaIn is the X/Y area of o inside the box from lo to hi: o clipped by
// the box's 4 sides. A concave o clips into one polygon whose arms are
// bridged by edges of no area, so the area is exact.
func (o Outline) AreaIn(lo, hi Point) float64 {
	if len(o) < 3 || hi.X <= lo.X || hi.Y <= lo.Y {
		return 0
	}
	box := [4]Point{lo, {hi.X, lo.Y}, hi, {lo.X, hi.Y}}
	poly := []Point(counterClockwise(o))
	for i, a := range box {
		if poly = clipHalfPlane(poly, nil, a, box[(i+1)%4]); len(poly) < 3 {
			return 0
		}
	}
	return math.Abs(signedArea2(poly)) / 2
}
