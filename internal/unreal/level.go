package unreal

import (
	"fmt"

	"zonebuilder/internal/l2pkg"
)

// Level is a map's Level object: the actors Unreal instantiates and the
// BSP model. Port of UE2-Studio's Level::read (src/unreal/objects.rs).
type Level struct {
	// Actors are object references (positive = local export ref-1).
	Actors []int32
	// Model is the object reference of the BSP Model.
	Model int32
}

// FindLevel is the 0-based index of the first Level export of p, or -1.
func FindLevel(p *l2pkg.Package) int {
	for i := range p.Exports {
		if p.Exports[i].ClassName == "Level" {
			return i
		}
	}
	return -1
}

// ReadLevel decodes export i (0-based) of p as a Level. Licensees above 20
// serialize a second actor array, which replaces the first.
func ReadLevel(p *l2pkg.Package, i int) (*Level, error) {
	r, err := p.ExportReader(i)
	if err != nil {
		return nil, err
	}
	if _, err := l2pkg.ReadProperties(p, r, p.Exports[i].Flags); err != nil {
		return nil, fmt.Errorf("Level: %w", err)
	}
	l := &Level{Actors: readLevelObjects(r)}
	if p.Header.LicenseeVersion > 20 {
		l.Actors = readLevelObjects(r)
	}
	for range 4 { // URL Protocol, Host, Map, Portal
		r.SkipString()
	}
	opts := readCount(r, "opções da URL")
	for range opts {
		r.SkipString()
	}
	r.I32() // URL Port
	r.U8()  // URL Valid
	r.Skip(2)
	reach := readCount(r, "reach specs")
	for range reach {
		r.I32()
		r.Index()
		r.Index()
		r.I32()
		r.I32()
		r.I32()
		r.U8()
	}
	l.Model = r.Index()
	if err := r.Err(); err != nil {
		return nil, fmt.Errorf("Level: %w", err)
	}
	return l, nil
}

func readLevelObjects(r *l2pkg.Reader) []int32 {
	n, dup := r.I32(), r.I32()
	if r.Err() != nil {
		return nil
	}
	if n < 0 || n > maxCount || dup != n {
		r.Fail(fmt.Errorf("array de objetos do Level com contagens %d e %d", n, dup))
		return nil
	}
	out := make([]int32, 0, n)
	for range n {
		out = append(out, r.Index())
	}
	return out
}

// maxCount is UE2-Studio's check_count limit on serialized array counts.
const maxCount = 10_000_000

// readCount reads a compact array count, failing r when it is out of range.
func readCount(r *l2pkg.Reader, what string) int {
	n := r.Index()
	if r.Err() == nil && (n < 0 || n > maxCount) {
		r.Fail(fmt.Errorf("contagem inválida de %s: %d", what, n))
	}
	if r.Err() != nil {
		return 0
	}
	return int(n)
}
