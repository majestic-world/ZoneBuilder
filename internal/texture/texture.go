// Package texture reads Unreal Texture objects: their properties and mip
// chain. Port of UE2-Studio's texture-engine texture.rs.
package texture

import (
	"encoding/binary"
	"fmt"

	"zonebuilder/internal/inflect"
	"zonebuilder/internal/l2pkg"
)

// Texture formats (ETextureFormat) the readers here know.
const (
	FormatP8    = 0
	FormatDXT1  = 3
	FormatRGBA8 = 5
	FormatDXT3  = 7
	FormatDXT5  = 8
	FormatG16   = 10
)

// Mip is one level of the chain, largest first. Data aliases the package
// until Texture.Own copies it.
type Mip struct {
	Width, Height int
	Data          []byte
}

// Texture is a decoded Texture export.
type Texture struct {
	// Path is the export's dotted name, Package.Group.Name.
	Path         string
	Format       uint8
	USize, VSize int
	// Masked is bMasked: a P8 texture's palette index 0 is transparent.
	Masked bool
	// PaletteRef is the Palette property, an object reference in the
	// texture's package (0: none). Read resolves nothing: whoever can reach
	// the palette's package fills Palette (ReadPalette), which P8 decoding
	// needs.
	PaletteRef int32
	Palette    []Color
	Mips       []Mip
}

// Color is one palette entry: R, G, B, A.
type Color [4]uint8

// Read decodes export i (0-based) of p as a Texture: the tagged properties,
// the native material block, then the mip chain.
func Read(p *l2pkg.Package, i int) (*Texture, error) {
	r, err := p.ExportReader(i)
	if err != nil {
		return nil, err
	}
	props, err := l2pkg.ReadProperties(p, r, p.Exports[i].Flags)
	if err != nil {
		return nil, err
	}
	t := &Texture{}
	t.Path, _ = p.ObjectPath(int32(i + 1))
	t.Format, _ = props.Byte("Format")
	u, _ := props.Int("USize")
	v, _ := props.Int("VSize")
	t.USize, t.VSize = int(u), int(v)
	t.Masked = props.Bool("bMasked")
	t.PaletteRef, _ = props.Index("Palette")
	l2pkg.SkipMaterialData(r, p.Header.FileVersion, p.Header.LicenseeVersion)
	count := r.Index()
	if err := r.Err(); err != nil {
		return nil, fmt.Errorf("textura: %w", err)
	}
	if count < 0 || count > 64 {
		return nil, fmt.Errorf("textura: quantidade de mips inválida: %d", count)
	}
	t.Mips = make([]Mip, count)
	for m := range t.Mips {
		data := r.LazyBytes(p.Header.FileVersion)
		w, h := r.I32(), r.I32()
		r.Skip(2) // UBits, VBits
		t.Mips[m] = Mip{Width: int(w), Height: int(h), Data: data}
	}
	if err := r.Err(); err != nil {
		return nil, fmt.Errorf("textura, mips: %w", err)
	}
	return t, nil
}

// Own copies the mip data out of the package into one allocation of the
// texture's own. Data otherwise aliases the whole package file, which a
// texture kept after loading (a scene batch's) would hold in memory.
func (t *Texture) Own() {
	n := 0
	for _, m := range t.Mips {
		n += len(m.Data)
	}
	buf := make([]byte, 0, n)
	for i := range t.Mips {
		start := len(buf)
		buf = append(buf, t.Mips[i].Data...)
		t.Mips[i].Data = buf[start:len(buf):len(buf)]
	}
}

// Heights is the G16 height field of a TerrainInfo's TerrainMap: mip 0 as
// USize×VSize little-endian u16 samples, row-major (row = y).
func (t *Texture) Heights() ([]uint16, error) {
	if t.Format != FormatG16 {
		return nil, fmt.Errorf("TerrainMap no formato %d, não G16", t.Format)
	}
	if len(t.Mips) == 0 {
		return nil, fmt.Errorf("TerrainMap sem mip")
	}
	if t.USize <= 0 || t.VSize <= 0 {
		return nil, fmt.Errorf("TerrainMap com dimensões inválidas %d×%d", t.USize, t.VSize)
	}
	n := t.USize * t.VSize
	data := t.Mips[0].Data
	if len(data) < 2*n {
		return nil, fmt.Errorf("mip do TerrainMap truncado: %s para %d×%d amostras", inflect.Count(len(data), "byte", "bytes"), t.USize, t.VSize)
	}
	out := make([]uint16, n)
	for i := range out {
		out[i] = binary.LittleEndian.Uint16(data[2*i:])
	}
	return out, nil
}

// ReadPalette decodes export i (0-based) of p as a Palette: the tagged
// properties, then TArray<FColor> Colors, each stored B, G, R, A. The
// stored alpha is kept as read; P8 decoding ignores it.
func ReadPalette(p *l2pkg.Package, i int) ([]Color, error) {
	r, err := p.ExportReader(i)
	if err != nil {
		return nil, err
	}
	if _, err := l2pkg.ReadProperties(p, r, p.Exports[i].Flags); err != nil {
		return nil, err
	}
	n := r.Index()
	if err := r.Err(); err != nil {
		return nil, fmt.Errorf("Palette: %w", err)
	}
	if n < 0 || n > 256 {
		return nil, fmt.Errorf("Palette com %s", inflect.Count(int(n), "cor", "cores"))
	}
	colors := make([]Color, n)
	for k := range colors {
		b := r.Bytes(4)
		if b == nil {
			break
		}
		colors[k] = Color{b[2], b[1], b[0], b[3]}
	}
	if err := r.Err(); err != nil {
		return nil, fmt.Errorf("Palette: %w", err)
	}
	return colors, nil
}
