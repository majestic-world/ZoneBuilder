// Package spawn is the spawn document: the spawn areas of a project and the
// commands that edit them. The UI never changes an area directly; it builds
// a Command and hands it to Document.Apply, as with zone.Document.
//
// Coordinates are the server's: an outline vertex is the x y a viewport
// click picked, and a point is the x y z heading a <spawn> pos carries.
package spawn

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"slices"
)

// AreaID identifies an area inside its Document for the area's lifetime;
// names are not identities (two areas may share one while being edited).
type AreaID int

// Vertex is an outline vertex in server x y. The outline is only tested
// for containment in x y: its height is the area's Z range.
type Vertex struct{ X, Y int }

// Point is one monster of an area: a <spawn> pos in server coordinates.
// Heading is the server's 0..65535 turn.
type Point struct{ X, Y, Z, Heading int }

// Params are what the user types for an area. NPCID, Respawn and
// RespawnRand only go into the XML; Count, Radius and Clearance drive the
// distribution and are part of its Fingerprint.
type Params struct {
	// NPCID is the npc id of the datapack; 0 until the user types one.
	NPCID int
	// Count is how many points the distribution asks for.
	Count int
	// Respawn and RespawnRand are in seconds.
	Respawn, RespawnRand int
	// Radius is the monster's collision radius: points keep it from the
	// outline and 2 × Radius from each other.
	Radius int
	// Clearance is the distance points keep from static meshes and walls
	// beyond Radius.
	Clearance int
}

// The defaults of a new area's Params.
const (
	DefaultRespawn     = 60
	DefaultRespawnRand = 0
	DefaultClearance   = 32
)

// DefaultParams is a new area's Params for a monster of radius: respawn
// 60, respawn_rand 0, clearance 32. NPCID and Count start at 0, which are
// problems until the user types them.
func DefaultParams(radius int) Params {
	return Params{Respawn: DefaultRespawn, RespawnRand: DefaultRespawnRand, Radius: radius, Clearance: DefaultClearance}
}

// Fingerprint identifies the inputs of a distribution: outline, Z range,
// Count, Radius, Clearance and seed. Equal inputs give equal fingerprints;
// the empty Fingerprint is "never generated".
type Fingerprint string

// Area is a spawn area: an outline (always a polygon, the closing edge
// from the last vertex back to the first implied), the Z range it spans,
// its Params, and the points of its last distribution.
type Area struct {
	ID         AreaID
	Name       string
	Outline    []Vertex
	ZMin, ZMax int
	Params     Params
	// Seed is the seed of the last distribution.
	Seed uint64
	// Points are the monsters, in XML order. Manual adjustments change
	// them and leave Generated alone.
	Points []Point
	// Generated is the Fingerprint of the inputs of the last distribution
	// (SetPoints); empty when the area was never generated.
	Generated Fingerprint
	// Warnings are what the last distribution reported. They never block.
	Warnings []Warning
	// Hidden keeps the area out of the viewport; it still compiles.
	Hidden bool
}

// Fingerprint is the fingerprint of a distribution of a's current inputs
// with seed. A generation takes it with the inputs it read and hands it to
// SetPoints, so an area edited while the distribution ran ends up stale.
func (a Area) Fingerprint(seed uint64) Fingerprint {
	buf := make([]byte, 0, 8*(2*len(a.Outline)+6))
	buf = binary.AppendVarint(buf, int64(len(a.Outline)))
	for _, v := range a.Outline {
		buf = binary.AppendVarint(buf, int64(v.X))
		buf = binary.AppendVarint(buf, int64(v.Y))
	}
	for _, n := range []int{a.ZMin, a.ZMax, a.Params.Count, a.Params.Radius, a.Params.Clearance} {
		buf = binary.AppendVarint(buf, int64(n))
	}
	buf = binary.AppendUvarint(buf, seed)
	sum := sha256.Sum256(buf)
	return Fingerprint(hex.EncodeToString(sum[:16]))
}

// Stale reports whether a's inputs changed since its last distribution:
// its points are no longer what that distribution gave for them.
func (a Area) Stale() bool {
	return a.Generated != "" && a.Generated != a.Fingerprint(a.Seed)
}

// clone is a deep copy of a: none of its slices is shared with a. Nil
// slices stay nil, so a clone is reflect.DeepEqual to its source. A slice
// field added to Area must be copied here too.
func (a Area) clone() Area {
	a.Outline = slices.Clone(a.Outline)
	a.Points = slices.Clone(a.Points)
	a.Warnings = slices.Clone(a.Warnings)
	return a
}

func cloneAreas(as []Area) []Area {
	if as == nil {
		return nil
	}
	out := make([]Area, len(as))
	for i, a := range as {
		out[i] = a.clone()
	}
	return out
}
