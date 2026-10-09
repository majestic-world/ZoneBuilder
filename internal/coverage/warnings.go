package coverage

// The thresholds of the floor warnings (spec D6).
const (
	// StrayShare is the share of a shape's floor that may lie out of its
	// range on one side without a warning, so a 1-cell sliver of a cliff
	// is not reported.
	StrayShare = 0.01
	// StrayDepth is how far, in units, floor may lie out of the range on
	// one side without a warning, for the same reason.
	StrayDepth = 16
	// MinClearance is the clearance under which a side of the range is
	// tight: the server's geodata lies on average 32 units above the
	// client's floor (ADR 0003), so floor within that margin may fall out
	// of the range in game though it is inside in the app.
	MinClearance = 32
)

// WarningKind is what a floor warning is about.
type WarningKind int

const (
	// AboveTop is floor over the shape's top: StrayShare of its floor or
	// more, or deeper than StrayDepth.
	AboveTop WarningKind = iota
	// BelowFloor is floor under the shape's floor, by the same rule.
	BelowFloor
	// TightTop and TightFloor are a clearance of that side from 0 up to,
	// not including, MinClearance.
	TightTop
	TightFloor
	// NoGround is area of the outline with no floor measured, such as a
	// tile not loaded.
	NoGround
)

// Warning is a way a shape's range fits the floor badly that does not
// block compiling (spec D6). Floor a ban excludes, and the other layers
// of the layer rule, are never warned of.
type Warning struct {
	Kind WarningKind
	// At is the worst floor: the highest for AboveTop and TightTop, the
	// lowest for BelowFloor and TightFloor; Clearance is that side's
	// clearance there. Both are zero for NoGround.
	At        Spot
	Clearance float64
	// Share is the fraction of the floor out of range on that side, for
	// AboveTop and BelowFloor.
	Share float64
	// Area is the area with no floor, for NoGround.
	Area float64
}

// warnings is the warnings of report r, judged by the floor of its range
// (Report).
func warnings(r Report) []Warning {
	var ws []Warning
	if r.Measured {
		var above, below float64
		if floor := r.Inside + r.Above + r.Below + r.Excluded; floor > 0 {
			above, below = r.Above/floor, r.Below/floor
		}
		ws = side(ws, AboveTop, TightTop, r.GroundMax, r.TopClearance, above)
		ws = side(ws, BelowFloor, TightFloor, r.GroundMin, r.FloorClearance, below)
	}
	if r.NoGround > 0 {
		ws = append(ws, Warning{Kind: NoGround, Area: r.NoGround})
	}
	return ws
}

// side appends the warning of one side of the range, if any: out when
// floor lies out of it beyond the thresholds, else tight when its
// clearance c at the worst floor at is under MinClearance.
func side(ws []Warning, out, tight WarningKind, at Spot, c, share float64) []Warning {
	switch {
	case (share > 0 && share >= StrayShare) || c < -StrayDepth:
		return append(ws, Warning{Kind: out, At: at, Clearance: c, Share: share})
	case c >= 0 && c < MinClearance:
		return append(ws, Warning{Kind: tight, At: at, Clearance: c})
	}
	return ws
}
