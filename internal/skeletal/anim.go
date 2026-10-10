package skeletal

import (
	"fmt"
	"strings"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/l2pkg"
)

// Animation is a MeshAnimation: its animated bones and its sequences. The
// keys of a sequence are read on demand by Tracks, as UE2-Studio's
// AnimSetIndex reads them from the byte range each one occupies.
type Animation struct {
	// Bones names the animated bones; track k of every sequence drives the
	// bone named Bones[k]. Bind matches them to a mesh's skeleton.
	Bones     []string
	Sequences []Sequence

	pkg    *l2pkg.Package
	name   string
	chunks []chunkRange
}

// Sequence is one FMeshAnimSeq.
type Sequence struct {
	Name   string
	Frames int
	// Rate is in frames per second.
	Rate float32
}

// Track is one bone's keys. Each channel has its own count: a bone with N
// rotation keys often has a single position key, and a channel with one key
// is constant.
type Track struct {
	// Quats are rotation keys x, y, z, w.
	Quats     [][4]float32
	Positions []geom.Vec3
	// Times are key times in frames, from 0 to the sequence's Frames.
	Times []float32
}

type chunkRange struct{ start, end int }

// ReadAnimation decodes export i (0-based) of p as a MeshAnimation. Port of
// meshanim.rs AnimSetIndex::read for the rough-array layout (file version
// >= 123, licensee >= 0x19).
func ReadAnimation(p *l2pkg.Package, i int) (*Animation, error) {
	if err := checkVersion(p); err != nil {
		return nil, err
	}
	r, err := p.ExportReader(i)
	if err != nil {
		return nil, err
	}
	a := &Animation{pkg: p, name: p.Exports[i].ObjectName}
	fail := func(err error) (*Animation, error) {
		return nil, fmt.Errorf("MeshAnimation %s: %w", a.name, err)
	}
	if _, err := l2pkg.ReadProperties(p, r, p.Exports[i].Flags); err != nil {
		return fail(err)
	}
	r.I32() // Version
	a.Bones = make([]string, count(r, "ossos"))
	for k := range a.Bones {
		a.Bones[k] = name(p, r)
		r.U32() // Flags
		r.I32() // ParentIndex
	}

	// Moves: the rough array. Each chunk is preceded by its absolute end
	// offset, so it is skipped here and parsed by Tracks.
	arrayEnd := int(r.I32())
	a.chunks = make([]chunkRange, count(r, "movimentos"))
	for k := range a.chunks {
		end := int(r.I32())
		start := r.Pos
		if r.Err() == nil && end < start {
			r.Fail(fmt.Errorf("as chaves da sequência %d terminam em %d, antes do início em %d", k, end, start))
		}
		r.Skip(end - start)
		a.chunks[k] = chunkRange{start, end}
	}
	if r.Err() == nil && r.Pos != arrayEnd {
		r.Fail(fmt.Errorf("os movimentos terminam em %d, mas declaram o fim em %d", r.Pos, arrayEnd))
	}

	a.Sequences = make([]Sequence, count(r, "sequências"))
	for k := range a.Sequences {
		if r.Err() != nil {
			break
		}
		a.Sequences[k] = readSequence(p, r)
	}
	checkEnd(p, r, i)
	if err := r.Err(); err != nil {
		return fail(err)
	}
	return a, nil
}

// readSequence reads one FMeshAnimSeq (meshanim.rs read_sequence): f28 comes
// before the name and the notifies before the rate.
func readSequence(p *l2pkg.Package, r *l2pkg.Reader) Sequence {
	r.F32() // f28 (ArVer >= 115)
	s := Sequence{Name: name(p, r)}
	for range count(r, "grupos") {
		r.Index()
	}
	r.I32() // StartFrame
	s.Frames = int(r.I32())
	for range count(r, "notificações") {
		r.F32()   // Time
		r.Index() // Function
		r.Index() // NotifyObj (ArVer >= 112)
		units := r.I32()
		if r.Err() == nil && (units < 0 || units > maxCount) {
			r.Fail(fmt.Errorf("texto de notificação com %s", inflect.Count(int(units), "unidade", "unidades")))
		}
		r.Skip(int(units) * 2) // UTF-16 label (ArVer >= 131)
	}
	s.Rate = r.F32()
	// Lineage tail (meshanim.rs skip_sequence_tail): f2C, f30, f34
	// (lic >= 2), f38, f3C (lic >= 0x14), f40 (lic >= 0x19), then Unk4
	// (lic >= 0x1b): u8, pairs, TArray<{i32, pairs}>, i32, i32, pairs.
	r.I32()
	r.I32()
	r.I32()
	r.Index()
	r.I32()
	r.I32()
	pairs := func() { skipArray(r, "pares", 8) }
	r.U8()
	pairs()
	for range count(r, "Unk4") {
		r.I32()
		pairs()
	}
	r.I32()
	r.I32()
	pairs()
	return s
}

// SequenceNamed is the index of the sequence called name, compared without
// case, or -1.
func (a *Animation) SequenceNamed(name string) int {
	for k, s := range a.Sequences {
		if strings.EqualFold(s.Name, name) {
			return k
		}
	}
	return -1
}

// Tracks reads the keys of sequence seq: one track per entry of Bones. Key
// times come back in frames, rescaled once by Frames / TrackTime as
// pose.rs rescale_chunk does (a chunk with no TrackTime is left as stored).
func (a *Animation) Tracks(seq int) ([]Track, error) {
	if seq < 0 || seq >= len(a.Sequences) || seq >= len(a.chunks) {
		return nil, fmt.Errorf("MeshAnimation %s: a sequência %d não tem chaves", a.name, seq)
	}
	c := a.chunks[seq]
	r := l2pkg.NewReader(a.pkg.Data[:c.end], c.start)
	r.Skip(12) // RootSpeed3D
	trackTime := r.F32()
	r.I32() // StartBone
	r.U32() // Flags
	skipArray(r, "BoneIndices", 4)
	tracks := make([]Track, count(r, "trilhas"))
	for k := range tracks {
		if r.Err() != nil {
			break
		}
		tracks[k] = readTrack(r)
	}
	readTrack(r) // RootTrack
	if r.Err() == nil && r.Pos != c.end {
		r.Fail(fmt.Errorf("as chaves terminam em %d, mas declaram o fim em %d", r.Pos, c.end))
	}
	if err := r.Err(); err != nil {
		return nil, fmt.Errorf("MeshAnimation %s, sequência %s: %w", a.name, a.Sequences[seq].Name, err)
	}
	if trackTime > 0 {
		scale := float32(a.Sequences[seq].Frames) / trackTime
		for k := range tracks {
			for t := range tracks[k].Times {
				tracks[k].Times[t] *= scale
			}
		}
	}
	return tracks, nil
}

// readTrack reads one uncompressed AnalogTrack.
func readTrack(r *l2pkg.Reader) Track {
	var t Track
	r.U32() // Flags
	t.Quats = make([][4]float32, count(r, "rotações"))
	for k := range t.Quats {
		t.Quats[k] = [4]float32{r.F32(), r.F32(), r.F32(), r.F32()}
	}
	t.Positions = make([]geom.Vec3, count(r, "posições"))
	for k := range t.Positions {
		t.Positions[k] = vector(r)
	}
	t.Times = make([]float32, count(r, "tempos"))
	for k := range t.Times {
		t.Times[k] = r.F32()
	}
	return t
}

// Bind gives, for each mesh bone, the index of the track that drives it: the
// first animated bone with the same name, compared without case
// (pose.rs BoundAnim::match_names), or -1 for a bone that keeps its bind
// pose.
func (a *Animation) Bind(bones []Bone) []int {
	out := make([]int, len(bones))
	for k, b := range bones {
		out[k] = -1
		for t, n := range a.Bones {
			if strings.EqualFold(n, b.Name) {
				out[k] = t
				break
			}
		}
	}
	return out
}
