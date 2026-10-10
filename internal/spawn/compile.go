package spawn

import (
	"cmp"
	"slices"

	"zonebuilder/internal/inflect"
	"zonebuilder/internal/spawnxml"
)

// BlockedError is Compile's refusal: the areas have these blocking
// problems, and no XML was produced.
type BlockedError struct {
	Problems []Problem
}

func (e *BlockedError) Error() string {
	return "spawn: compilação bloqueada: " + inflect.Count(len(e.Problems), "problema", "problemas") + " nas áreas"
}

// Compile writes every area, hidden ones included, as one spawn file named
// spawnxml.FileName(fileName): areas in ID order, points in document order.
// If any area has a blocking problem nothing is compiled: the error is a
// *BlockedError. Distribution warnings do not block.
func (d *Document) Compile(fileName string) (spawnxml.File, error) {
	var blocked []Problem
	for _, p := range d.Problems() {
		if p.Blocks() {
			blocked = append(blocked, p)
		}
	}
	if len(blocked) > 0 {
		return spawnxml.File{}, &BlockedError{Problems: blocked}
	}
	areas := slices.SortedFunc(slices.Values(d.areas), func(a, b Area) int { return cmp.Compare(a.ID, b.ID) })
	out := make([]spawnxml.Area, 0, len(areas))
	for _, a := range areas {
		x := spawnxml.Area{Name: a.Name, NPCID: a.Params.NPCID, Respawn: a.Params.Respawn, RespawnRand: a.Params.RespawnRand}
		for _, p := range a.Points {
			x.Points = append(x.Points, spawnxml.Point(p))
		}
		out = append(out, x)
	}
	return spawnxml.Compile(fileName, out), nil
}
