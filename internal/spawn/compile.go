package spawn

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

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
// Positive, unique npcIDs are required and assigned cyclically, restarting
// in each area. Empty, nonpositive or duplicate IDs return no file.
// If any area has a blocking problem nothing is compiled: the error is a
// *BlockedError. Distribution warnings do not block.
func (d *Document) Compile(fileName string, npcIDs []int) (spawnxml.File, error) {
	if err := validateNPCIDs(npcIDs); err != nil {
		return spawnxml.File{}, err
	}
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
		x := spawnxml.Area{Name: a.Name, NPCIDs: npcIDs, Points: make([]spawnxml.Point, len(a.Points))}
		for i, p := range a.Points {
			x.Points[i] = spawnxml.Point(p)
		}
		out = append(out, x)
	}
	return spawnxml.Compile(fileName, out), nil
}

// ParseNPCIDs reads whitespace-separated positive NPC IDs, preserving the
// first occurrence of each ID. Invalid input returns no IDs.
func ParseNPCIDs(text string) ([]int, error) {
	fields := strings.Fields(text)
	ids := make([]int, 0, len(fields))
	seen := make(map[int]struct{}, len(fields))
	for _, field := range fields {
		id, err := strconv.Atoi(field)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("spawn: ID de NPC inválido: %q", field)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("spawn: informe ao menos 1 ID de NPC")
	}
	return ids, nil
}

func validateNPCIDs(ids []int) error {
	if len(ids) == 0 {
		return fmt.Errorf("spawn: informe ao menos 1 ID de NPC")
	}
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("spawn: ID de NPC inválido: %d", id)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("spawn: ID de NPC repetido: %d", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}
