package zone

import (
	"regexp"
	"strconv"

	"zonebuilder/internal/inflect"
	"zonebuilder/internal/locale"
)

// Rule is a validation rule a zone can break.
type Rule int

const (
	// TooFewVertices: a polygon needs 3 vertices or more (the server logs
	// "invalid territory data" below that).
	TooFewVertices Rule = iota
	// SelfIntersection: 2 non-adjacent edges of a polygon touch or cross.
	// The server's own check skips edge 0→1; this one tests every pair.
	SelfIntersection
	// RepeatedVertex: 2 consecutive vertices, the last and the first
	// included, sit at the same x y.
	RepeatedVertex
	// CornerCount: a rectangle is exactly 2 corners.
	CornerCount
	// InvertedZRange: a shape's zmin is above its zmax.
	InvertedZRange
	// NoIncludedShape: a zone needs an included shape (the server logs
	// "Empty territory" and fails later at runtime without one).
	NoIncludedShape
	// UnknownType: the type is not a ZoneType value.
	UnknownType
	// DuplicateName: another zone of the document has the same name (the
	// server keeps only the last one it reads).
	DuplicateName
	// MissingParam: a parameter the server reads with no default for the
	// zone's type is missing or not an integer, or a RESIDENCE zone is not
	// named residence_<id>.
	MissingParam
	// OutOfBounds: a vertex or restart point lies outside the server world.
	OutOfBounds
)

// The server world's x y bounds, inclusive (geodata.properties: GeoFirstX
// 15, GeoFirstY 10, GeoLastX 26, GeoLastY 26; World.java).
const (
	WorldMinX = -163840
	WorldMaxX = 229375
	WorldMinY = -262144
	WorldMaxY = 294911
)

// Problem is one rule a zone breaks, its target, and language-independent
// values used to present the rule. The document owns the problem slice.
type Problem struct {
	Rule Rule
	Zone ZoneID
	// Shape is -1 for a whole-zone problem.
	Shape int
	// Vertex is -1 for a whole-shape problem.
	Vertex int
	// Restart is -1 when this is not a restart point; PK distinguishes kinds.
	Restart int
	PK      bool
	Variant string
	Name, Value string
	Count int
	Numbers [4]int
}

// Text presents this problem without changing the validation result or target.
func (p Problem) Text(lang locale.Language) string {
	n := func(value int) string { return locale.Number(lang, float64(value), 0) }
	var text string
	switch p.Rule {
	case UnknownType:
		text = locale.Format(lang, "zone.problem.unknown_type", map[string]string{"type": p.Value})
	case DuplicateName:
		text = locale.Plural(lang, "zone.problem.duplicate_name", p.Count, map[string]string{"name": p.Name})
	case NoIncludedShape:
		text = locale.Text(lang, "zone.problem.no_included_shape")
	case MissingParam:
		switch p.Variant {
		case "absent":
			text = locale.Format(lang, "zone.problem.missing_param", map[string]string{"name": p.Name, "type": p.Value})
		case "not_integer":
			text = locale.Format(lang, "zone.problem.invalid_param", map[string]string{"name": p.Name})
		case "residence_name":
			text = locale.Text(lang, "zone.problem.residence_name")
		}
	case CornerCount:
		text = locale.Plural(lang, "zone.problem.corner_count", p.Numbers[0], nil)
	case TooFewVertices:
		text = locale.Plural(lang, "zone.problem.vertex_count", p.Numbers[0], nil)
	case RepeatedVertex:
		text = locale.Format(lang, "zone.problem.repeated_vertex", map[string]string{"first": n(p.Numbers[0]), "second": n(p.Numbers[1])})
	case SelfIntersection:
		text = locale.Format(lang, "zone.problem.self_intersection", map[string]string{
			"first": n(p.Numbers[0]), "next": n(p.Numbers[1]), "other": n(p.Numbers[2]), "last": n(p.Numbers[3]),
		})
	case InvertedZRange:
		text = locale.Format(lang, "zone.problem.inverted_z", map[string]string{"min": strconv.Itoa(p.Numbers[0]), "max": strconv.Itoa(p.Numbers[1])})
	case OutOfBounds:
		// Coordinates are data, not locale-dependent measurements.
		args := map[string]string{"index": n(p.Numbers[0]), "x": strconv.Itoa(p.Numbers[1]), "y": strconv.Itoa(p.Numbers[2])}
		switch {
		case p.Restart >= 0 && p.PK:
			text = locale.Format(lang, "zone.problem.pk_restart_outside", args)
		case p.Restart >= 0:
			text = locale.Format(lang, "zone.problem.restart_outside", args)
		default:
			text = locale.Format(lang, "zone.problem.vertex_outside", args)
		}
	}
	if p.Shape >= 0 {
		return locale.Format(lang, "zone.problem.shape", map[string]string{"index": n(p.Shape + 1), "problem": text})
	}
	return text
}

// requiredParams are the parameters the server reads with getInteger and
// no default for a zone of each type: a zone without them breaks when the
// server reaches that code (Zone.java siege logout, FishingSkill).
var requiredParams = map[Type][]string{
	Siege:       {"residence"},
	Headquarter: {"residence"},
	Fishing:     {"distribution_id", "fishing_place_type"},
}

// residenceName is the name Residence.initZone looks a RESIDENCE zone up
// by: "residence_" + the residence id.
var residenceName = regexp.MustCompile(`^residence_[0-9]+$`)

// BlockedError is Compile's refusal: these problems are in the selected
// zones, and no XML was produced.
type BlockedError struct {
	Problems []Problem
}

func (e *BlockedError) Error() string {
	return "zona: compilação bloqueada: " + inflect.Count(len(e.Problems), "problema", "problemas") + " nas zonas selecionadas"
}

// Problems is every rule the zones break, zone by zone in creation order:
// the zone's own problems, then each shape's, then the restart points'.
// The slice belongs to d: read it, never modify it. It is computed once
// per change of the zones.
func (d *Document) Problems() []Problem {
	if !d.checked {
		d.problems = d.validate()
		d.checked = true
	}
	return d.problems
}

// ZoneProblems is the problems of zone id, in Problems order.
func (d *Document) ZoneProblems(id ZoneID) []Problem {
	var out []Problem
	for _, p := range d.Problems() {
		if p.Zone == id {
			out = append(out, p)
		}
	}
	return out
}

// changed drops the cached Problems after the zones changed.
func (d *Document) changed() {
	d.problems, d.checked = nil, false
}

func (d *Document) validate() []Problem {
	names := make(map[string]int, len(d.zones))
	for _, z := range d.zones {
		names[z.Name]++
	}
	var out []Problem
	for _, z := range d.zones {
		zoneProblem := func(r Rule, variant, name, value string, count int) {
			out = append(out, Problem{Rule: r, Zone: z.ID, Shape: -1, Vertex: -1, Restart: -1,
				Variant: variant, Name: name, Value: value, Count: count})
		}
		if !z.Type.Valid() {
			zoneProblem(UnknownType, "", "", string(z.Type), 0)
		}
		if n := names[z.Name]; n > 1 {
			zoneProblem(DuplicateName, "", z.Name, "", n)
		}
		included := false
		for _, s := range z.Shapes {
			included = included || !s.Banned
		}
		if !included {
			zoneProblem(NoIncludedShape, "", "", "", 0)
		}
		for _, name := range requiredParams[z.Type] {
			v, ok := z.param(name)
			switch {
			case !ok:
				zoneProblem(MissingParam, "absent", name, string(z.Type), 0)
			case !isInt(v):
				zoneProblem(MissingParam, "not_integer", name, "", 0)
			}
		}
		if z.Type == Residence && !residenceName.MatchString(z.Name) {
			zoneProblem(MissingParam, "residence_name", "", "", 0)
		}
		for i, s := range z.Shapes {
			out = append(out, shapeProblems(z.ID, i, s)...)
		}
		for _, pk := range []bool{false, true} {
			pts := z.RestartPoints
			if pk {
				pts = z.PKRestartPoints
			}
			for i, p := range pts {
				if !inWorld(p) {
					out = append(out, Problem{
						Rule: OutOfBounds, Zone: z.ID, Shape: -1, Vertex: -1, Restart: i, PK: pk,
						Numbers: [4]int{i + 1, p.X, p.Y},
					})
				}
			}
		}
	}
	return out
}

// shapeProblems is the problems of shape i of zone id.
func shapeProblems(id ZoneID, i int, s Shape) []Problem {
	var out []Problem
	add := func(r Rule, vertex int, numbers ...int) {
		p := Problem{Rule: r, Zone: id, Shape: i, Vertex: vertex, Restart: -1}
		copy(p.Numbers[:], numbers)
		out = append(out, p)
	}
	pts := s.Points
	switch s.Kind {
	case Rectangle:
		if len(pts) != 2 {
			add(CornerCount, -1, len(pts))
		}
	case Polygon:
		if len(pts) < 3 {
			add(TooFewVertices, -1, len(pts))
			break
		}
		for k := range pts {
			if next := (k + 1) % len(pts); sameXY(pts[k], pts[next]) {
				add(RepeatedVertex, next, k+1, next+1)
			}
		}
		for _, x := range Crossings(pts) {
			add(SelfIntersection, x[0], x[0]+1, (x[0]+1)%len(pts)+1, x[1]+1, (x[1]+1)%len(pts)+1)
		}
	}
	if s.ZMin > s.ZMax {
		add(InvertedZRange, -1, s.ZMin, s.ZMax)
	}
	for k, p := range pts {
		if !inWorld(p) {
			add(OutOfBounds, k, k+1, p.X, p.Y)
		}
	}
	return out
}

// Crossings is every pair of non-adjacent edges of polygon pts that touch
// or cross, as the indices of the edges' first vertices; the
// SelfIntersection rule, which the spawn areas share. Edge k runs from
// vertex k to k+1, the last back to the first. Zero-length edges (repeated
// vertices, reported on their own) are left out, and edges are adjacent
// when only zero-length edges separate them, so a repeated vertex is not
// also reported as a crossing. Z plays no part.
func Crossings(pts []Point) [][2]int {
	n := len(pts)
	var edges []int
	for k := range n {
		if !sameXY(pts[k], pts[(k+1)%n]) {
			edges = append(edges, k)
		}
	}
	var out [][2]int
	m := len(edges)
	for a := range m {
		for b := a + 2; b < m; b++ {
			if a == 0 && b == m-1 {
				continue // the closing edge is adjacent to the first
			}
			i, j := edges[a], edges[b]
			if segmentsTouch(pts[i], pts[(i+1)%n], pts[j], pts[(j+1)%n]) {
				out = append(out, [2]int{i, j})
			}
		}
	}
	return out
}

// segmentsTouch reports whether the closed x y segments ab and cd share a
// point.
func segmentsTouch(a, b, c, d Point) bool {
	d1, d2 := orient(c, d, a), orient(c, d, b)
	d3, d4 := orient(a, b, c), orient(a, b, d)
	if ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) && ((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0)) {
		return true
	}
	return (d1 == 0 && onSegment(c, d, a)) || (d2 == 0 && onSegment(c, d, b)) ||
		(d3 == 0 && onSegment(a, b, c)) || (d4 == 0 && onSegment(a, b, d))
}

// orient is the sign of the turn a→b→c: >0 counterclockwise, <0
// clockwise, 0 collinear.
func orient(a, b, c Point) int64 {
	return int64(b.X-a.X)*int64(c.Y-a.Y) - int64(b.Y-a.Y)*int64(c.X-a.X)
}

// onSegment reports whether p, collinear with ab, lies within ab's box.
func onSegment(a, b, p Point) bool {
	return min(a.X, b.X) <= p.X && p.X <= max(a.X, b.X) && min(a.Y, b.Y) <= p.Y && p.Y <= max(a.Y, b.Y)
}

func sameXY(a, b Point) bool { return a.X == b.X && a.Y == b.Y }

func inWorld(p Point) bool {
	return WorldMinX <= p.X && p.X <= WorldMaxX && WorldMinY <= p.Y && p.Y <= WorldMaxY
}

// isInt reports whether v is what Integer.parseInt reads.
func isInt(v string) bool {
	_, err := strconv.ParseInt(v, 10, 32)
	return err == nil
}

// param is the value of the zone's parameter name.
func (z *Zone) param(name string) (string, bool) {
	for _, p := range z.Params {
		if p.Name == name {
			return p.Value, true
		}
	}
	return "", false
}
