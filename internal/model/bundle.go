package model

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"math"
	"unicode/utf8"

	"zonebuilder/internal/geom"
)

// Magic opens every UE2HUM01 bundle.
const Magic = "UE2HUM01"

// NoParent is the Bone.Parent of a root bone.
const NoParent uint16 = 0xFFFF

// MaxBones is UE2-Studio's skin::MAX_BONES, the largest skeleton a part may have.
const MaxBones = 1024

// maxList is the largest element count any list may declare.
const maxList = 1_000_000

// ErrInvalid wraps every Decode failure.
var ErrInvalid = errors.New("model: invalid UE2HUM01 bundle")

// Bundle is one UE2HUM01 file, field for field as UE2-Studio's bundle.rs lays
// it out. Encode writes exactly these fields, so a Bundle built from scratch
// encodes as well as a decoded one, and Encode(Decode(b)) == b.
type Bundle struct {
	// Placement takes posed model space to the actor frame: scale and the
	// offset that puts the feet at Z = 0 in the first clip's frame 0.
	Placement Affine
	// JumpClips are the standing and running jump clip indices.
	JumpClips [2]uint32
	// Names are the animation track bone names: track i of every clip drives
	// the bones named Names[i] (ASCII case-insensitive, first match wins).
	Names []string
	Clips []Clip
	Parts []Part
}

// Clip is one animation sequence. Its tracks are indexed like Bundle.Names.
type Clip struct {
	Frames  int32
	Rate    float32 // frames per second
	Looping bool
	Tracks  []Track
}

// Track holds one bone's keys. KeyTime is in frame units; empty channels keep
// the bind pose, and KeyTime empty means keys spread evenly over the clip.
type Track struct {
	KeyQuat []Quat
	KeyPos  []geom.Vec3
	KeyTime []float32
}

// Part is one skinned mesh. Every part has its own copy of the skeleton.
type Part struct {
	// Base is the mesh's actor-space base (facing, scale, origin) applied to
	// the skeleton root.
	Base        Affine
	Bones       []Bone
	InverseBind []Affine // one per bone; excludes Base
	Vertices    []Vertex
	Indices     []uint32 // triangle list
	Sections    []Section
}

// Bone is one skeleton bone; Parent is NoParent or a smaller index.
type Bone struct {
	Name        string
	Parent      uint16
	Position    geom.Vec3
	Orientation Quat
	Length      float32
	Size        geom.Vec3
}

// Vertex is one skinned vertex in skeleton model space.
type Vertex struct {
	Position geom.Vec3
	Normal   geom.Vec3
	UV       [2]float32
	Bones    [4]uint16
	Weights  [4]float32
}

// RenderMode is a section's material mode, by its index on disk.
type RenderMode uint8

const (
	Opaque RenderMode = iota
	Masked
	TerrainLayer
	Translucent
	Brighten
	Water
	Additive
	Modulated
	Overlay
)

// Section is a run of Indices drawn with one texture. IndexCount may be 0;
// such sections draw nothing.
type Section struct {
	FirstIndex    uint32
	IndexCount    uint32
	PolygonFlags  uint32
	RenderMode    RenderMode
	Masked        bool
	VertexOpacity bool
	Texture       string // package path of the skin, informative
	PNG           []byte // 8-bit RGBA PNG, written verbatim; see EncodePNG
}

// Pixels decodes the section's skin.
func (s *Section) Pixels() (*image.NRGBA, error) {
	if len(s.PNG) < 26 || string(s.PNG[:8]) != "\x89PNG\r\n\x1a\n" || string(s.PNG[12:16]) != "IHDR" {
		return nil, errors.New("not a PNG")
	}
	if depth, color := s.PNG[24], s.PNG[25]; depth != 8 || color != 6 {
		return nil, fmt.Errorf("PNG has bit depth %d, color type %d; want 8-bit RGBA", depth, color)
	}
	img, err := png.Decode(bytes.NewReader(s.PNG))
	if err != nil {
		return nil, err
	}
	rgba, ok := img.(*image.NRGBA)
	if !ok {
		return nil, fmt.Errorf("PNG decoded as %T", img)
	}
	return rgba, nil
}

// EncodePNG writes img as an 8-bit RGBA PNG, the only kind Decode accepts.
// image/png writes an opaque image as 8-bit RGB, which Decode refuses.
func EncodePNG(img *image.NRGBA) []byte {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	var raw bytes.Buffer
	z, _ := zlib.NewWriterLevel(&raw, zlib.BestCompression)
	for y := range h {
		z.Write([]byte{0}) // filter: none
		z.Write(img.Pix[y*img.Stride : y*img.Stride+4*w])
	}
	z.Close()
	out := []byte("\x89PNG\r\n\x1a\n")
	chunk := func(kind string, data []byte) {
		out = binary.BigEndian.AppendUint32(out, uint32(len(data)))
		start := len(out)
		out = append(append(out, kind...), data...)
		out = binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(out[start:]))
	}
	ihdr := binary.BigEndian.AppendUint32(nil, uint32(w))
	ihdr = binary.BigEndian.AppendUint32(ihdr, uint32(h))
	chunk("IHDR", append(ihdr, 8, 6, 0, 0, 0))
	chunk("IDAT", raw.Bytes())
	chunk("IEND", nil)
	return out
}

type reader struct {
	b   []byte
	pos int
	err error
}

func (r *reader) fail(format string, args ...any) {
	if r.err == nil {
		r.err = fmt.Errorf("%w: "+format, append([]any{ErrInvalid}, args...)...)
	}
}

func (r *reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > len(r.b)-r.pos {
		r.fail("truncated at byte %d", r.pos)
		return nil
	}
	s := r.b[r.pos : r.pos+n]
	r.pos += n
	return s
}

func (r *reader) u8() uint8 {
	if s := r.take(1); s != nil {
		return s[0]
	}
	return 0
}

func (r *reader) u16() uint16 {
	if s := r.take(2); s != nil {
		return binary.LittleEndian.Uint16(s)
	}
	return 0
}

func (r *reader) u32() uint32 {
	if s := r.take(4); s != nil {
		return binary.LittleEndian.Uint32(s)
	}
	return 0
}

func (r *reader) f32() float32 { return math.Float32frombits(r.u32()) }

func (r *reader) vec3() geom.Vec3 { return geom.Vec3{X: r.f32(), Y: r.f32(), Z: r.f32()} }

func (r *reader) quat() Quat { return Quat{r.f32(), r.f32(), r.f32(), r.f32()} }

func (r *reader) affine() Affine {
	return Affine{Origin: r.vec3(), Axis: [3]geom.Vec3{r.vec3(), r.vec3(), r.vec3()}}
}

// count reads a list length; elemSize is the smallest encoding of one element,
// so a count the remaining bytes cannot hold fails before allocating.
func (r *reader) count(elemSize int) int {
	n := r.u32()
	if r.err != nil {
		return 0
	}
	if n > maxList {
		r.fail("list of %d at byte %d", n, r.pos-4)
		return 0
	}
	if int(n)*elemSize > len(r.b)-r.pos {
		r.fail("truncated at byte %d", r.pos)
		return 0
	}
	return int(n)
}

func (r *reader) string() string {
	s := r.take(r.count(1))
	if r.err == nil && !utf8.Valid(s) {
		r.fail("invalid UTF-8 string at byte %d", r.pos-len(s))
	}
	return string(s)
}

func list[T any](r *reader, elemSize int, read func(*reader) T) []T {
	n := r.count(elemSize)
	out := make([]T, n)
	for i := range out {
		if r.err != nil {
			return nil
		}
		out[i] = read(r)
	}
	return out
}

// Decode parses and validates a UE2HUM01 bundle with bundle.rs's rules: at
// least 3 clips, jump clips in range, positive frames and rate, parent before
// child, every index in range, every part bound to at least one track, 8-bit
// RGBA skins and no trailing bytes.
func Decode(data []byte) (*Bundle, error) {
	r := &reader{b: data}
	if string(r.take(len(Magic))) != Magic {
		r.fail("bad magic")
		return nil, r.err
	}
	b := &Bundle{Placement: r.affine(), JumpClips: [2]uint32{r.u32(), r.u32()}}
	b.Names = list(r, 4, (*reader).string)
	b.Clips = list(r, 13, decodeClip)
	if r.err == nil && (len(b.Clips) < 3 || int(b.JumpClips[0]) >= len(b.Clips) || int(b.JumpClips[1]) >= len(b.Clips)) {
		r.fail("%d clips with jump clips %v", len(b.Clips), b.JumpClips)
	}
	b.Parts = list(r, 48+5*4, func(r *reader) Part { return decodePart(r, b.Names) })
	if r.err == nil && len(b.Parts) == 0 {
		r.fail("no parts")
	}
	if r.err == nil && r.pos != len(data) {
		r.fail("%d trailing bytes", len(data)-r.pos)
	}
	if r.err != nil {
		return nil, r.err
	}
	return b, nil
}

func decodeClip(r *reader) Clip {
	c := Clip{Frames: int32(r.u32()), Rate: r.f32(), Looping: r.u8() != 0}
	if r.err == nil && (c.Frames <= 0 || math.IsInf(float64(c.Rate), 0) || !(c.Rate > 0)) {
		r.fail("clip timing %d frames at %v", c.Frames, c.Rate)
		return c
	}
	c.Tracks = list(r, 12, func(r *reader) Track {
		return Track{
			KeyQuat: list(r, 16, (*reader).quat),
			KeyPos:  list(r, 12, (*reader).vec3),
			KeyTime: list(r, 4, (*reader).f32),
		}
	})
	return c
}

func decodePart(r *reader, names []string) Part {
	p := Part{Base: r.affine()}
	p.Bones = list(r, 4+2+12+16+4+12, func(r *reader) Bone {
		return Bone{Name: r.string(), Parent: r.u16(), Position: r.vec3(), Orientation: r.quat(), Length: r.f32(), Size: r.vec3()}
	})
	p.InverseBind = list(r, 48, (*reader).affine)
	if r.err != nil {
		return p
	}
	if len(p.Bones) == 0 || len(p.Bones) > MaxBones || len(p.InverseBind) != len(p.Bones) {
		r.fail("%d bones with %d inverse binds", len(p.Bones), len(p.InverseBind))
		return p
	}
	for i, bone := range p.Bones {
		if bone.Parent != NoParent && int(bone.Parent) >= i {
			r.fail("bone %d has parent %d", i, bone.Parent)
			return p
		}
	}
	p.Vertices = list(r, 56, func(r *reader) Vertex {
		v := Vertex{Position: r.vec3(), Normal: r.vec3(), UV: [2]float32{r.f32(), r.f32()}}
		for k := range v.Bones {
			if v.Bones[k] = r.u16(); r.err == nil && int(v.Bones[k]) >= len(p.Bones) {
				r.fail("vertex bone %d of %d", v.Bones[k], len(p.Bones))
			}
		}
		v.Weights = [4]float32{r.f32(), r.f32(), r.f32(), r.f32()}
		return v
	})
	p.Indices = list(r, 4, (*reader).u32)
	for _, i := range p.Indices {
		if int(i) >= len(p.Vertices) {
			r.fail("index %d of %d vertices", i, len(p.Vertices))
			return p
		}
	}
	p.Sections = list(r, 4*3+3+4+4, func(r *reader) Section {
		s := Section{FirstIndex: r.u32(), IndexCount: r.u32(), PolygonFlags: r.u32()}
		if r.err == nil && uint64(s.FirstIndex)+uint64(s.IndexCount) > uint64(len(p.Indices)) {
			r.fail("section %d+%d of %d indices", s.FirstIndex, s.IndexCount, len(p.Indices))
			return s
		}
		if s.RenderMode = RenderMode(r.u8()); r.err == nil && s.RenderMode > Overlay {
			r.fail("render mode %d", s.RenderMode)
			return s
		}
		s.Masked = r.u8() != 0
		s.VertexOpacity = r.u8() != 0
		s.Texture = r.string()
		s.PNG = bytes.Clone(r.take(r.count(1)))
		if r.err == nil {
			if _, err := s.Pixels(); err != nil {
				r.fail("skin %q: %v", s.Texture, err)
			}
		}
		return s
	})
	if r.err == nil && (bound(bindTracks(p.Bones, names)) == 0 || len(p.Vertices) == 0 || len(p.Sections) == 0) {
		r.fail("part with %d vertices, %d sections, no bone bound to a track", len(p.Vertices), len(p.Sections))
	}
	return p
}

// Encode writes b in the UE2HUM01 layout. It does not validate: a Bundle that
// Decode would refuse encodes to bytes Decode refuses.
func Encode(b *Bundle) []byte {
	w := &writer{}
	w.b = append(w.b, Magic...)
	w.affine(b.Placement)
	w.u32(b.JumpClips[0])
	w.u32(b.JumpClips[1])
	w.u32(uint32(len(b.Names)))
	for _, name := range b.Names {
		w.string(name)
	}
	w.u32(uint32(len(b.Clips)))
	for _, c := range b.Clips {
		w.u32(uint32(c.Frames))
		w.f32(c.Rate)
		w.bool(c.Looping)
		w.u32(uint32(len(c.Tracks)))
		for _, t := range c.Tracks {
			w.u32(uint32(len(t.KeyQuat)))
			for _, q := range t.KeyQuat {
				w.quat(q)
			}
			w.u32(uint32(len(t.KeyPos)))
			for _, p := range t.KeyPos {
				w.vec3(p)
			}
			w.u32(uint32(len(t.KeyTime)))
			for _, k := range t.KeyTime {
				w.f32(k)
			}
		}
	}
	w.u32(uint32(len(b.Parts)))
	for i := range b.Parts {
		p := &b.Parts[i]
		w.affine(p.Base)
		w.u32(uint32(len(p.Bones)))
		for _, bone := range p.Bones {
			w.string(bone.Name)
			w.u16(bone.Parent)
			w.vec3(bone.Position)
			w.quat(bone.Orientation)
			w.f32(bone.Length)
			w.vec3(bone.Size)
		}
		w.u32(uint32(len(p.InverseBind)))
		for _, a := range p.InverseBind {
			w.affine(a)
		}
		w.u32(uint32(len(p.Vertices)))
		for _, v := range p.Vertices {
			w.vec3(v.Position)
			w.vec3(v.Normal)
			w.f32(v.UV[0])
			w.f32(v.UV[1])
			for _, bone := range v.Bones {
				w.u16(bone)
			}
			for _, weight := range v.Weights {
				w.f32(weight)
			}
		}
		w.u32(uint32(len(p.Indices)))
		for _, i := range p.Indices {
			w.u32(i)
		}
		w.u32(uint32(len(p.Sections)))
		for _, s := range p.Sections {
			w.u32(s.FirstIndex)
			w.u32(s.IndexCount)
			w.u32(s.PolygonFlags)
			w.b = append(w.b, byte(s.RenderMode))
			w.bool(s.Masked)
			w.bool(s.VertexOpacity)
			w.string(s.Texture)
			w.u32(uint32(len(s.PNG)))
			w.b = append(w.b, s.PNG...)
		}
	}
	return w.b
}

type writer struct{ b []byte }

func (w *writer) u16(n uint16)  { w.b = binary.LittleEndian.AppendUint16(w.b, n) }
func (w *writer) u32(n uint32)  { w.b = binary.LittleEndian.AppendUint32(w.b, n) }
func (w *writer) f32(f float32) { w.u32(math.Float32bits(f)) }
func (w *writer) vec3(v geom.Vec3) {
	w.f32(v.X)
	w.f32(v.Y)
	w.f32(v.Z)
}
func (w *writer) quat(q Quat) {
	w.f32(q.X)
	w.f32(q.Y)
	w.f32(q.Z)
	w.f32(q.W)
}
func (w *writer) affine(a Affine) {
	w.vec3(a.Origin)
	for _, v := range a.Axis {
		w.vec3(v)
	}
}
func (w *writer) string(s string) {
	w.u32(uint32(len(s)))
	w.b = append(w.b, s...)
}
func (w *writer) bool(v bool) {
	if v {
		w.b = append(w.b, 1)
	} else {
		w.b = append(w.b, 0)
	}
}
