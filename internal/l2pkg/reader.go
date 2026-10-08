package l2pkg

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"zonebuilder/internal/inflect"
)

// Reader is a little-endian cursor over package plaintext.
//
// Pos is an absolute offset into the package: a reader over one export's
// body starts at the export's serial offset, so the absolute end offsets a
// TLazyArray carries compare directly against it.
//
// Errors are sticky: after the first failed read every later read returns
// zero and Err reports the first failure. Callers check Err once a logical
// unit (a table row, an object) has been read.
type Reader struct {
	data []byte
	Pos  int
	err  error
}

// NewReader returns a reader over data starting at absolute offset pos.
func NewReader(data []byte, pos int) *Reader {
	return &Reader{data: data, Pos: pos}
}

// Err is the first read failure, or nil.
func (r *Reader) Err() error { return r.err }

// Fail records err unless an earlier failure is already recorded.
func (r *Reader) Fail(err error) {
	if r.err == nil {
		r.err = err
	}
}

func (r *Reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || r.Pos < 0 || r.Pos > len(r.data) || n > len(r.data)-r.Pos {
		r.err = fmt.Errorf("fim inesperado do pacote no offset %d (leitura de %s)", r.Pos, inflect.Count(n, "byte", "bytes"))
		return nil
	}
	b := r.data[r.Pos : r.Pos+n]
	r.Pos += n
	return b
}

// Skip advances past n bytes.
func (r *Reader) Skip(n int) { r.take(n) }

// Bytes returns the next n bytes without copying them.
func (r *Reader) Bytes(n int) []byte { return r.take(n) }

func (r *Reader) U8() uint8 {
	if b := r.take(1); b != nil {
		return b[0]
	}
	return 0
}

func (r *Reader) U16() uint16 {
	if b := r.take(2); b != nil {
		return binary.LittleEndian.Uint16(b)
	}
	return 0
}

func (r *Reader) U32() uint32 {
	if b := r.take(4); b != nil {
		return binary.LittleEndian.Uint32(b)
	}
	return 0
}

func (r *Reader) I32() int32 { return int32(r.U32()) }

func (r *Reader) U64() uint64 {
	if b := r.take(8); b != nil {
		return binary.LittleEndian.Uint64(b)
	}
	return 0
}

func (r *Reader) F32() float32 { return math.Float32frombits(r.U32()) }

// Index reads Unreal's compact index: a sign bit and six payload bits in the
// first byte, then seven payload bits per continuation byte, at most five
// bytes in all.
func (r *Reader) Index() int32 {
	b := r.U8()
	negative := b&0x80 != 0
	value := int32(b & 0x3f)
	if b&0x40 != 0 {
		for shift := 6; shift < 32; shift += 7 {
			b = r.U8()
			value |= int32(b&0x7f) << shift
			if b&0x80 == 0 {
				break
			}
		}
	}
	if negative {
		return -value
	}
	return value
}

// String reads an FString: a compact-index length, positive for a
// NUL-terminated byte string and negative for a NUL-terminated UTF-16LE one
// counted in code units.
//
// A byte string that is not UTF-8 is a legacy code page (CP949 in a Lineage
// II client) and comes back escaped by escapeANSI, which keeps distinct
// byte strings distinct.
func (r *Reader) String() string {
	length := r.Index()
	if r.err != nil {
		return ""
	}
	if length < 0 {
		units := -int64(length)
		if units > int64(len(r.data)) {
			r.Fail(fmt.Errorf("comprimento de string UTF-16 inválido: %d", length))
			return ""
		}
		b := r.take(int(units) * 2)
		if b == nil {
			return ""
		}
		codes := make([]uint16, units)
		for i := range codes {
			codes[i] = binary.LittleEndian.Uint16(b[i*2:])
		}
		if len(codes) > 0 && codes[len(codes)-1] == 0 {
			codes = codes[:len(codes)-1]
		}
		return string(utf16.Decode(codes))
	}
	b := r.take(int(length))
	if b == nil {
		return ""
	}
	if n := len(b); n > 0 && b[n-1] == 0 {
		b = b[:n-1]
	}
	if utf8.Valid(b) {
		return string(b)
	}
	return escapeANSI(b)
}

// SkipString advances past an FString without decoding it.
func (r *Reader) SkipString() {
	length := int64(r.Index())
	if length < 0 {
		length *= -2
	}
	if r.err == nil && length > int64(len(r.data)) {
		r.Fail(fmt.Errorf("comprimento de string inválido: %d", length))
		return
	}
	r.take(int(length))
}

// escapeANSI spells a byte string with its printable ASCII bytes as
// themselves and every other byte as \xNN, the same spelling UE2-Studio
// gives a legacy-code-page name.
func escapeANSI(b []byte) string {
	var s strings.Builder
	s.Grow(len(b))
	for _, c := range b {
		if c >= 0x20 && c < 0x7f {
			s.WriteByte(c)
		} else {
			fmt.Fprintf(&s, `\x%02x`, c)
		}
	}
	return s.String()
}
