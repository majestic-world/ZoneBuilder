package spawn

import (
	"strconv"

	"zonebuilder/internal/locale"
)

// Message presents p in the spawn catalog, retained so the problem panel
// and the status line render it again after a language change. Vertex and
// point numbers are 1-based, coordinates and Z values are the data as is.
func (p Problem) Message() locale.Message {
	n := strconv.Itoa
	switch p.Rule {
	case TooFewVertices:
		return locale.Message{Key: "spawn.problem.too_few_vertices", Count: p.Numbers[0], Plural: true}
	case SelfIntersection:
		return locale.Message{Key: "spawn.problem.self_intersection", Args: map[string]string{
			"first": n(p.Numbers[0]), "next": n(p.Numbers[1]), "other": n(p.Numbers[2]), "last": n(p.Numbers[3]),
		}}
	case RepeatedVertex:
		return locale.Message{Key: "spawn.problem.repeated_vertex", Args: map[string]string{"first": n(p.Numbers[0]), "second": n(p.Numbers[1])}}
	case InvertedZRange:
		return locale.Message{Key: "spawn.problem.inverted_z", Args: map[string]string{"min": n(p.Numbers[0]), "max": n(p.Numbers[1])}}
	case InvalidCount:
		return locale.Message{Key: "spawn.problem.invalid_count", Args: map[string]string{"value": n(p.Numbers[0])}}
	case EmptyName:
		return locale.Message{Key: "spawn.problem.empty_name"}
	case DuplicateName:
		return locale.Message{Key: "spawn.problem.duplicate_name", Count: p.Count, Plural: true, Args: map[string]string{"name": p.Name}}
	case NoPoints:
		return locale.Message{Key: "spawn.problem.no_points"}
	case StalePoints:
		return locale.Message{Key: "spawn.problem.stale_points"}
	case OutOfBounds:
		return locale.Message{Key: "spawn.problem.out_of_bounds", Args: map[string]string{"index": n(p.Numbers[0]), "x": n(p.Numbers[1]), "y": n(p.Numbers[2])}}
	case FitsOnly:
		return locale.Message{Key: "spawn.problem.fits_only", Count: p.Numbers[0], Plural: true, Args: map[string]string{"requested": n(p.Numbers[1])}}
	case NoFreeCell:
		return locale.Message{Key: "spawn.problem.no_free_cell"}
	}
	return locale.Message{Key: "spawn.problem.unknown", Args: map[string]string{"rule": n(int(p.Rule))}}
}

// Text presents p in lang.
func (p Problem) Text(lang locale.Language) string { return p.Message().Render(lang) }
