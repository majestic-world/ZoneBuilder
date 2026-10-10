package spawn

import (
	"strings"

	"zonebuilder/internal/zone"
)

// Rule is a validation rule an area can break, or a warning its last
// distribution reported.
type Rule int

const (
	// TooFewVertices: an outline needs 3 vertices or more.
	TooFewVertices Rule = iota
	// SelfIntersection: 2 non-adjacent outline edges touch or cross (the
	// rule of zone.Crossings).
	SelfIntersection
	// RepeatedVertex: 2 consecutive outline vertices, the last and the
	// first included, sit at the same x y.
	RepeatedVertex
	// InvertedZRange: the area's zmin is above its zmax.
	InvertedZRange
	// InvalidNPC: the npc id is not positive.
	InvalidNPC
	// InvalidCount: the count is below 1.
	InvalidCount
	// NegativeRespawn: the respawn is below 0.
	NegativeRespawn
	// RespawnRandAboveRespawn: respawn_rand exceeds respawn, which the
	// server rejects together with the whole file.
	RespawnRandAboveRespawn
	// EmptyName: the name is empty or only spaces.
	EmptyName
	// DuplicateName: another area of the document has the same name, so
	// their <spawn> names collide.
	DuplicateName
	// NoPoints: the area has no point to compile.
	NoPoints
	// StalePoints: the outline, Z range, count, radius or clearance
	// changed since the last distribution (Area.Stale).
	StalePoints
	// OutOfBounds: a point lies outside the server world.
	OutOfBounds

	// FitsOnly warns that the distribution placed K of the N points asked
	// for (Numbers: K, N). It does not block.
	FitsOnly
	// NoFreeCell warns that the area has no free cell at all. It does not
	// block.
	NoFreeCell
)

// Blocks reports whether r stops the document from compiling; only the
// distribution warnings (FitsOnly, NoFreeCell) do not.
func (r Rule) Blocks() bool { return !r.warning() }

func (r Rule) warning() bool { return r == FitsOnly || r == NoFreeCell }

// Warning is what a distribution reported, kept with the area until the
// next one. Rule is FitsOnly or NoFreeCell.
type Warning struct {
	Rule Rule
	// Placed and Requested are the K and N of FitsOnly.
	Placed, Requested int
}

// Problem is one rule an area breaks or one warning of its distribution,
// its target, and language-independent values used to present it. The
// document owns the problem slice.
type Problem struct {
	Rule Rule
	Area AreaID
	// Vertex is the outline vertex at fault, -1 for none.
	Vertex int
	// Point is the point at fault, -1 for none.
	Point int
	// Name is the repeated name of DuplicateName.
	Name string
	// Count is how many areas share the name of DuplicateName.
	Count int
	// Numbers are the rule's values: the vertex count of TooFewVertices
	// (1-based vertex numbers elsewhere), zmin zmax, respawn_rand respawn,
	// a point's number x y, K N of FitsOnly.
	Numbers [4]int
}

// Blocks reports whether p stops the document from compiling.
func (p Problem) Blocks() bool { return p.Rule.Blocks() }

// Problems is every rule the areas break and every warning of their last
// distribution, area by area in creation order: the area's own problems,
// then its outline's, then its points', then its warnings. A stale area
// shows no warnings: they describe a distribution of other inputs. The
// slice belongs to d: read it, never modify it. It is computed once per
// change of the areas.
func (d *Document) Problems() []Problem {
	if !d.checked {
		d.problems = d.validate()
		d.checked = true
	}
	return d.problems
}

// AreaProblems is the problems of area id, in Problems order.
func (d *Document) AreaProblems(id AreaID) []Problem {
	var out []Problem
	for _, p := range d.Problems() {
		if p.Area == id {
			out = append(out, p)
		}
	}
	return out
}

// changed drops the cached Problems after the areas changed.
func (d *Document) changed() {
	d.problems, d.checked = nil, false
}

func (d *Document) validate() []Problem {
	names := make(map[string]int, len(d.areas))
	for _, a := range d.areas {
		names[a.Name]++
	}
	var out []Problem
	for _, a := range d.areas {
		add := func(r Rule, vertex, point int, numbers ...int) {
			p := Problem{Rule: r, Area: a.ID, Vertex: vertex, Point: point}
			copy(p.Numbers[:], numbers)
			out = append(out, p)
		}
		if strings.TrimSpace(a.Name) == "" {
			add(EmptyName, -1, -1)
		} else if n := names[a.Name]; n > 1 {
			out = append(out, Problem{Rule: DuplicateName, Area: a.ID, Vertex: -1, Point: -1, Name: a.Name, Count: n})
		}
		p := a.Params
		if p.NPCID <= 0 {
			add(InvalidNPC, -1, -1, p.NPCID)
		}
		if p.Count < 1 {
			add(InvalidCount, -1, -1, p.Count)
		}
		if p.Respawn < 0 {
			add(NegativeRespawn, -1, -1, p.Respawn)
		}
		if p.RespawnRand > p.Respawn {
			add(RespawnRandAboveRespawn, -1, -1, p.RespawnRand, p.Respawn)
		}
		if a.ZMin > a.ZMax {
			add(InvertedZRange, -1, -1, a.ZMin, a.ZMax)
		}
		stale := a.Stale()
		if stale {
			add(StalePoints, -1, -1)
		}
		if len(a.Points) == 0 {
			add(NoPoints, -1, -1)
		}
		pts := a.Outline
		if len(pts) < 3 {
			add(TooFewVertices, -1, -1, len(pts))
		} else {
			for k := range pts {
				if next := (k + 1) % len(pts); pts[k] == pts[next] {
					add(RepeatedVertex, next, -1, k+1, next+1)
				}
			}
			outline := make([]zone.Point, len(pts))
			for k, v := range pts {
				outline[k] = zone.Point{X: v.X, Y: v.Y}
			}
			for _, x := range zone.Crossings(outline) {
				add(SelfIntersection, x[0], -1, x[0]+1, (x[0]+1)%len(pts)+1, x[1]+1, (x[1]+1)%len(pts)+1)
			}
		}
		for i, pt := range a.Points {
			if !inWorld(pt) {
				add(OutOfBounds, -1, i, i+1, pt.X, pt.Y)
			}
		}
		if !stale {
			for _, w := range a.Warnings {
				add(w.Rule, -1, -1, w.Placed, w.Requested)
			}
		}
	}
	return out
}

func inWorld(p Point) bool {
	return zone.WorldMinX <= p.X && p.X <= zone.WorldMaxX && zone.WorldMinY <= p.Y && p.Y <= zone.WorldMaxY
}
