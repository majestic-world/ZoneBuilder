// Package texture reads Unreal Texture objects: their properties and mip
// chain. Port of UE2-Studio's texture-engine texture.rs.
package texture

import (
	"encoding/binary"
	"fmt"

	"zonebuilder/internal/l2pkg"
)

// Texture formats (ETextureFormat) the readers here know.
const (
	FormatDXT1 = 3
	FormatG16  = 10
)

// Mip is one level of the chain, largest first. Data aliases the package.
type Mip struct {
	Width, Height int
	Data          []byte
}

// Texture is a decoded Texture export.
type Texture struct {
	Format       uint8
	USize, VSize int
	Mips         []Mip
}

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
	t.Format, _ = props.Byte("Format")
	u, _ := props.Int("USize")
	v, _ := props.Int("VSize")
	t.USize, t.VSize = int(u), int(v)
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
		return nil, fmt.Errorf("mip do TerrainMap truncado: %d bytes para %d×%d amostras", len(data), t.USize, t.VSize)
	}
	out := make([]uint16, n)
	for i := range out {
		out[i] = binary.LittleEndian.Uint16(data[2*i:])
	}
	return out, nil
}
