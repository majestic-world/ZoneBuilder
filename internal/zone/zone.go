// Package zone is the zone document: the zones of a project and the
// commands that edit them. The UI never changes a zone directly; it builds
// a Command and hands it to Document.Apply, and Document.Compile turns the
// selected zones into the server's XML.
//
// Coordinates are the server's: a vertex is the x y z a viewport click
// picked (scene.Hit.Pos, rounded), the numbers that go into the XML.
package zone

import "slices"

// Type is a ZoneType value of the Java server (Zone.java, enum ZoneType),
// in its exact case: Enum.valueOf is case-sensitive and a wrong value makes
// the ZoneParser drop the rest of the file.
type Type string

// The 23 ZoneType values, in the enum's order.
const (
	Siege        Type = "SIEGE"
	Residence    Type = "RESIDENCE"
	Headquarter  Type = "HEADQUARTER"
	Fishing      Type = "FISHING"
	Water        Type = "water"
	BattleZone   Type = "battle_zone"
	Damage       Type = "damage"
	InstantSkill Type = "instant_skill"
	MotherTree   Type = "mother_tree"
	PeaceZone    Type = "peace_zone"
	Poison       Type = "poison"
	SSQZone      Type = "ssq_zone"
	Swamp        Type = "swamp"
	NoEscape     Type = "no_escape"
	NoLanding    Type = "no_landing"
	NoRestart    Type = "no_restart"
	NoSummon     Type = "no_summon"
	Dummy        Type = "dummy"
	Offshore     Type = "offshore"
	Epic         Type = "epic"
	Fun          Type = "fun"
	BuffStore    Type = "buff_store"
	Jumping      Type = "JUMPING"
)

// Types lists every Type in the server enum's order.
var Types = []Type{
	Siege, Residence, Headquarter, Fishing, Water, BattleZone, Damage, InstantSkill,
	MotherTree, PeaceZone, Poison, SSQZone, Swamp, NoEscape, NoLanding, NoRestart,
	NoSummon, Dummy, Offshore, Epic, Fun, BuffStore, Jumping,
}

// Valid reports whether t is one of the 23 values, in the exact case.
func (t Type) Valid() bool { return slices.Contains(Types, t) }

// ZoneID identifies a zone inside its Document for the zone's lifetime;
// names are not identities (two zones may share one while being edited).
type ZoneID int

// Point is a vertex in server coordinates.
type Point struct{ X, Y, Z int }

// Shape is a polygon or a rectangle and the one Z range its prism spans.
// A polygon's Points are its vertices in order (the closing edge from the
// last back to the first is implied); a rectangle's are 2 opposite corners.
// Each point keeps the Z it was picked at, which the Z range is suggested
// from; the XML carries only the range. A banned shape is an exclusion: it
// cuts its area out of the zone's included shapes.
type Shape struct {
	Kind       ShapeKind
	Banned     bool
	Points     []Point
	ZMin, ZMax int
}

// Zone is a named, typed set of shapes, included and banned, and the points
// the server sends players to on restart: RestartPoints for any player,
// PKRestartPoints for player killers.
type Zone struct {
	ID   ZoneID
	Name string
	Type Type
	// Params are the zone's <set> parameters in document order: the order
	// they were first set, which is the order they compile in.
	Params          []Param
	Shapes          []Shape
	RestartPoints   []Point
	PKRestartPoints []Point
	// Hidden keeps the zone out of the viewport; it still compiles.
	Hidden bool
	// Color is the zone's viewport colour; the zero Color means its
	// type's (DisplayColor).
	Color Color
}

// Param is one <set name val> of a zone: a known ZoneTemplate parameter
// (see KnownParams) or a free one that scripts read.
type Param struct {
	Name, Value string
}

// DefaultZMargin is how far the suggested Z range reaches below the lowest
// and above the highest vertex, the margin the server's own
// //zone_panel uses (AdminZoneBuilder).
const DefaultZMargin = 256

// SuggestZRange is the Z range for vertices pts: from margin below the
// lowest vertex to margin above the highest. With no vertices it is
// [-margin, margin].
func SuggestZRange(pts []Point, margin int) (zmin, zmax int) {
	if len(pts) == 0 {
		return -margin, margin
	}
	lo, hi := pts[0].Z, pts[0].Z
	for _, p := range pts[1:] {
		lo, hi = min(lo, p.Z), max(hi, p.Z)
	}
	return lo - margin, hi + margin
}
