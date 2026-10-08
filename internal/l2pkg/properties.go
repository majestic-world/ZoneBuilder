package l2pkg

import (
	"fmt"
	"strings"
)

// RFHasStack is the export flag (RF_HasStack) of an object serialized with
// a script state frame in front of its property list.
const RFHasStack = 0x0200_0000

// Kind is the type of a tagged property's value as the reader decoded it.
type Kind uint8

// Value kinds. KindNone is a property whose payload was skipped because its
// layout is not self-describing (an unknown struct, a string).
const (
	KindNone Kind = iota
	KindByte
	KindInt
	KindBool
	KindFloat
	KindIndex // object reference or name index
	KindColor
	KindVector
	KindRotator
	KindBytes // a dynamic array kept as its raw element bytes
	KindMaps  // nested property lists: a struct, or an array of them
)

// Rotator is an Unreal FRotator; 65536 is one full turn.
type Rotator struct {
	Pitch, Yaw, Roll int32
}

// Value is one decoded property value; only the field its Kind names is set.
type Value struct {
	Kind    Kind
	Int     int32 // KindByte, KindInt, KindIndex; KindBool is 0 or 1
	Float   float32
	Color   [4]uint8 // R, G, B, A
	Vector  [3]float32
	Rotator Rotator
	Bytes   []byte // aliases the package data
	Maps    []Properties
}

// Properties is a decoded tagged property list. A name that occurs more
// than once (the slots of a fixed-size array) keeps every occurrence in
// file order; the typed getters read the last one, as UE2-Studio does.
type Properties struct {
	values map[string][]Value
}

func (ps Properties) last(name string, kind Kind) (Value, bool) {
	vs := ps.values[name]
	if len(vs) == 0 || vs[len(vs)-1].Kind != kind {
		return Value{}, false
	}
	return vs[len(vs)-1], true
}

// Byte is a byte property (an enum value, a texture Format).
func (ps Properties) Byte(name string) (uint8, bool) {
	v, ok := ps.last(name, KindByte)
	return uint8(v.Int), ok
}

// Int is an int property.
func (ps Properties) Int(name string) (int32, bool) {
	v, ok := ps.last(name, KindInt)
	return v.Int, ok
}

// Bool is a bool property; an absent one is false.
func (ps Properties) Bool(name string) bool {
	v, ok := ps.last(name, KindBool)
	return ok && v.Int != 0
}

// Float is a float property.
func (ps Properties) Float(name string) (float32, bool) {
	v, ok := ps.last(name, KindFloat)
	return v.Float, ok
}

// Index is an object reference (negative import, positive export) or a name
// index.
func (ps Properties) Index(name string) (int32, bool) {
	v, ok := ps.last(name, KindIndex)
	return v.Int, ok
}

// Color is a Color struct property as R, G, B, A.
func (ps Properties) Color(name string) ([4]uint8, bool) {
	v, ok := ps.last(name, KindColor)
	return v.Color, ok
}

// Vector is a Vector struct property, in Unreal's basis (Z up).
func (ps Properties) Vector(name string) ([3]float32, bool) {
	v, ok := ps.last(name, KindVector)
	return v.Vector, ok
}

// Rotator is a Rotator struct property.
func (ps Properties) Rotator(name string) (Rotator, bool) {
	v, ok := ps.last(name, KindRotator)
	return v.Rotator, ok
}

// Bytes is a dynamic array property kept as raw element bytes
// (QuadVisibilityBitmap, EdgeTurnBitmap, Skins).
func (ps Properties) Bytes(name string) ([]byte, bool) {
	v, ok := ps.last(name, KindBytes)
	return v.Bytes, ok
}

// Maps is the element list of an array of tagged structs (Materials), or
// the single list of a struct property.
func (ps Properties) Maps(name string) []Properties {
	v, _ := ps.last(name, KindMaps)
	return v.Maps
}

// StructSlots is every occurrence of a fixed-size array of structs
// (TerrainInfo.Layers), in file order: such an array is serialized as one
// repeated property per non-default slot.
func (ps Properties) StructSlots(name string) []Properties {
	var out []Properties
	for _, v := range ps.values[name] {
		if v.Kind == KindMaps && len(v.Maps) > 0 {
			out = append(out, v.Maps[0])
		}
	}
	return out
}

// ReadProperties reads the tagged property list an object body opens with.
// flags is the export's object flags: with RFHasStack set, the state frame
// in front of the list is skipped first. It is a port of UE2-Studio's
// props.rs read_properties.
func ReadProperties(p *Package, r *Reader, flags uint32) (Properties, error) {
	if flags&RFHasStack != 0 {
		node := r.Index()
		r.Index() // StateNode
		r.U64()   // ProbeMask
		r.I32()   // LatentAction
		if node != 0 {
			r.Index() // Offset
		}
	}
	ps := Properties{values: map[string][]Value{}}
	for {
		name, err := p.name(r.Index())
		if err := firstError(r.Err(), err); err != nil {
			return Properties{}, fmt.Errorf("propriedades: %w", err)
		}
		if name == "None" {
			break
		}
		info := r.U8()
		typ := info & 0x0f
		isArray := info&0x80 != 0
		var structName string
		if typ == 10 {
			structName, err = p.name(r.Index())
			if err := firstError(r.Err(), err); err != nil {
				return Properties{}, fmt.Errorf("propriedade %s: %w", name, err)
			}
		}
		size := propertySize(r, (info>>4)&0x07)
		v, err := readValue(p, r, name, typ, structName, size, isArray)
		if err := firstError(r.Err(), err); err != nil {
			return Properties{}, fmt.Errorf("propriedade %s: %w", name, err)
		}
		ps.values[name] = append(ps.values[name], v)
	}
	return ps, nil
}

func readValue(p *Package, r *Reader, name string, typ uint8, structName string, size int, isArray bool) (Value, error) {
	if typ == 3 {
		// A bool keeps its value in the array bit and has no payload.
		v := Value{Kind: KindBool}
		if isArray {
			v.Int = 1
		}
		return v, nil
	}
	if isArray {
		// The static array index: one byte below 128, else two.
		if b := r.U8(); b >= 128 {
			r.U8()
		}
	}
	switch typ {
	case 1:
		return Value{Kind: KindByte, Int: int32(r.U8())}, nil
	case 2:
		return Value{Kind: KindInt, Int: r.I32()}, nil
	case 4:
		return Value{Kind: KindFloat, Float: r.F32()}, nil
	case 5, 6:
		return Value{Kind: KindIndex, Int: r.Index()}, nil
	case 9:
		return readArray(p, r, name, size)
	case 10:
		switch structName {
		case "Vector":
			return Value{Kind: KindVector, Vector: readVector(r)}, nil
		case "Rotator":
			return Value{Kind: KindRotator, Rotator: readRotator(r)}, nil
		case "Color":
			b, g, rd, a := r.U8(), r.U8(), r.U8(), r.U8()
			return Value{Kind: KindColor, Color: [4]uint8{rd, g, b, a}}, nil
		case "TerrainLayer":
			m, err := ReadProperties(p, r, 0)
			return Value{Kind: KindMaps, Maps: []Properties{m}}, err
		case "PointRegion":
			end := r.Pos + size
			m, err := ReadProperties(p, r, 0)
			if err != nil {
				return Value{}, err
			}
			if r.Pos < end {
				r.Skip(end - r.Pos)
			}
			return Value{Kind: KindMaps, Maps: []Properties{m}}, nil
		}
	case 11:
		return Value{Kind: KindVector, Vector: readVector(r)}, nil
	case 12:
		return Value{Kind: KindRotator, Rotator: readRotator(r)}, nil
	}
	r.Skip(size)
	return Value{Kind: KindNone}, nil
}

// readArray reads a dynamic array property: a compact count, then size
// bytes in all. Arrays of tagged structs (Materials) are decoded and must
// end exactly at the declared end; any other array is kept as raw bytes.
func readArray(p *Package, r *Reader, name string, size int) (Value, error) {
	if name == "ZoneRenderState" {
		// Its own codec in UE2-Studio (zone_states.rs); no geometry needs it.
		r.Skip(size)
		return Value{Kind: KindNone}, nil
	}
	start := r.Pos
	count := r.Index()
	if count < 0 || count > maxTableCount {
		return Value{}, fmt.Errorf("array com quantidade inválida: %d", count)
	}
	remaining := size - (r.Pos - start)
	if remaining < 0 {
		return Value{}, fmt.Errorf("array declara %d bytes, menos que a própria quantidade", size)
	}
	if !strings.EqualFold(name, "Materials") {
		return Value{Kind: KindBytes, Bytes: r.Bytes(remaining)}, nil
	}
	end := r.Pos + remaining
	maps := make([]Properties, 0, count)
	for range count {
		m, err := ReadProperties(p, r, 0)
		if err != nil {
			return Value{}, err
		}
		maps = append(maps, m)
	}
	if r.Pos != end {
		return Value{}, fmt.Errorf("array %s termina em %d, não no fim declarado %d", name, r.Pos, end)
	}
	return Value{Kind: KindMaps, Maps: maps}, nil
}

// propertySize decodes the size field of a property's info byte.
func propertySize(r *Reader, sizeType uint8) int {
	switch sizeType {
	case 0:
		return 1
	case 1:
		return 2
	case 2:
		return 4
	case 3:
		return 12
	case 4:
		return 16
	case 5:
		return int(r.U8())
	case 6:
		return int(r.U16())
	default:
		return int(r.U32())
	}
}

func readVector(r *Reader) [3]float32 {
	return [3]float32{r.F32(), r.F32(), r.F32()}
}

func readRotator(r *Reader) Rotator {
	return Rotator{Pitch: r.I32(), Yaw: r.I32(), Roll: r.I32()}
}
