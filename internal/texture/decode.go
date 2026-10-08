package texture

import (
	"encoding/binary"
	"fmt"

	"zonebuilder/internal/inflect"
)

// Image is tightly packed 8-bit RGBA pixels, row-major from the top-left
// texel. Colour channels are sRGB-encoded, alpha is linear.
type Image struct {
	Width, Height int
	Pix           []byte
}

// Decodable reports whether RGBA can decode format: DXT1, DXT3, DXT5 and
// RGBA8, the formats UE2-Studio decodes (texture.rs is_rgba_decodable),
// plus P8, which also needs the texture's Palette.
func Decodable(format uint8) bool {
	switch format {
	case FormatP8, FormatDXT1, FormatDXT3, FormatDXT5, FormatRGBA8:
		return true
	}
	return false
}

// RGBA decodes mip 0 to RGBA8. Port of UE2-Studio's decode_texture_rgba,
// sized by the mip's own width and height.
func (t *Texture) RGBA() (*Image, error) {
	if len(t.Mips) == 0 {
		return nil, fmt.Errorf("textura sem mip")
	}
	return t.DecodeMip(t.Mips[0])
}

// DecodeMip decodes one mip level of t to RGBA8.
func (t *Texture) DecodeMip(m Mip) (*Image, error) {
	if m.Width <= 0 || m.Height <= 0 {
		return nil, fmt.Errorf("mip de %d×%d", m.Width, m.Height)
	}
	img := &Image{Width: m.Width, Height: m.Height, Pix: make([]byte, 4*m.Width*m.Height)}
	var err error
	switch t.Format {
	case FormatP8:
		err = decodeP8(img, m.Data, t.Palette, t.Masked)
	case FormatDXT1:
		err = decodeDXT(img, m.Data, 1)
	case FormatDXT3:
		err = decodeDXT(img, m.Data, 3)
	case FormatDXT5:
		err = decodeDXT(img, m.Data, 5)
	case FormatRGBA8:
		err = decodeBGRA(img, m.Data)
	default:
		err = fmt.Errorf("formato de textura %d não decodificado", t.Format)
	}
	if err != nil {
		return nil, err
	}
	return img, nil
}

// MipBytes is the size of a w×h level in format, or 0 for a format whose
// size this package does not know.
func MipBytes(format uint8, w, h int) int {
	switch format {
	case FormatP8:
		return w * h
	case FormatDXT1:
		return ((w + 3) / 4) * ((h + 3) / 4) * 8
	case FormatDXT3, FormatDXT5:
		return ((w + 3) / 4) * ((h + 3) / 4) * 16
	case FormatRGBA8:
		return w * h * 4
	case FormatG16:
		return w * h * 2
	}
	return 0
}

// decodeP8 looks every index byte up in palette. The palette's stored
// alpha is not coverage (UE2 often stores 0): every texel is opaque, except
// index 0 of a masked texture, which keeps its palette colour with alpha 0.
// An index past the palette's end reads as opaque black.
func decodeP8(img *Image, src []byte, palette []Color, masked bool) error {
	n := img.Width * img.Height
	if len(palette) == 0 {
		return fmt.Errorf("P8 sem Palette")
	}
	if len(src) < n {
		return fmt.Errorf("P8 truncado: %s para %d×%d", inflect.Count(len(src), "byte", "bytes"), img.Width, img.Height)
	}
	for i, idx := range src[:n] {
		var c Color
		if int(idx) < len(palette) {
			c = palette[idx]
		}
		c[3] = 255
		if masked && idx == 0 {
			c[3] = 0
		}
		copy(img.Pix[4*i:], c[:])
	}
	return nil
}

// decodeBGRA converts Unreal's RGBA8, stored B, G, R, A.
func decodeBGRA(img *Image, src []byte) error {
	n := len(img.Pix)
	if len(src) < n {
		return fmt.Errorf("RGBA8 truncado: %s para %d×%d", inflect.Count(len(src), "byte", "bytes"), img.Width, img.Height)
	}
	for i := 0; i < n; i += 4 {
		img.Pix[i+0] = src[i+2]
		img.Pix[i+1] = src[i+1]
		img.Pix[i+2] = src[i+0]
		img.Pix[i+3] = src[i+3]
	}
	return nil
}

// decodeDXT decodes DXT1 (kind 1), DXT3 (3) or DXT5 (5) blocks, row-major
// over ceil(w/4)×ceil(h/4) blocks; texels past the image edge are dropped.
func decodeDXT(img *Image, src []byte, kind int) error {
	w, h := img.Width, img.Height
	blockBytes := 16
	if kind == 1 {
		blockBytes = 8
	}
	bw, bh := (w+3)/4, (h+3)/4
	if len(src) < bw*bh*blockBytes {
		return fmt.Errorf("DXT%d truncado: %s para %d×%d", kind, inflect.Count(len(src), "byte", "bytes"), w, h)
	}
	var alpha [16]uint8
	off := 0
	for by := range bh {
		for bx := range bw {
			block := src[off : off+blockBytes]
			off += blockBytes
			switch kind {
			case 1:
				alpha = [16]uint8{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}
			case 3:
				for i := range 16 {
					nib := block[i/2] & 0x0f
					if i%2 == 1 {
						nib = block[i/2] >> 4
					}
					alpha[i] = nib * 17
				}
				block = block[8:]
			default:
				var pal [8]uint16
				pal[0], pal[1] = uint16(block[0]), uint16(block[1])
				if pal[0] > pal[1] {
					for i := uint16(2); i < 8; i++ {
						pal[i] = ((8-i)*pal[0] + (i-1)*pal[1]) / 7
					}
				} else {
					for i := uint16(2); i < 6; i++ {
						pal[i] = ((6-i)*pal[0] + (i-1)*pal[1]) / 5
					}
					pal[6], pal[7] = 0, 255
				}
				var bits uint64
				for i, b := range block[2:8] {
					bits |= uint64(b) << (8 * i)
				}
				for i := range 16 {
					alpha[i] = uint8(pal[(bits>>(3*i))&7])
				}
				block = block[8:]
			}
			c0 := binary.LittleEndian.Uint16(block[0:])
			c1 := binary.LittleEndian.Uint16(block[2:])
			colors := dxtPalette(c0, c1)
			bits := binary.LittleEndian.Uint32(block[4:])
			for py := range 4 {
				y := by*4 + py
				if y >= h {
					break
				}
				for px := range 4 {
					x := bx*4 + px
					if x >= w {
						break
					}
					i := py*4 + px
					idx := (bits >> (2 * i)) & 3
					o := 4 * (y*w + x)
					if kind == 1 && c0 <= c1 && idx == 3 {
						// DXT1's 1-bit alpha: transparent black.
						img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = 0, 0, 0, 0
						continue
					}
					c := colors[idx]
					img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = c[0], c[1], c[2], alpha[i]
				}
			}
		}
	}
	return nil
}

// dxtPalette is the four RGB colours of a DXT colour block: the RGB565
// endpoints, then two interpolated colours (c0 > c1) or their midpoint and
// black, the slot DXT1 reads as transparent.
func dxtPalette(c0, c1 uint16) [4][3]uint8 {
	unpack := func(c uint16) [3]uint16 {
		return [3]uint16{(c >> 11 & 31) * 255 / 31, (c >> 5 & 63) * 255 / 63, (c & 31) * 255 / 31}
	}
	a, b := unpack(c0), unpack(c1)
	var p [4][3]uint8
	for ch := range 3 {
		p[0][ch], p[1][ch] = uint8(a[ch]), uint8(b[ch])
		if c0 > c1 {
			p[2][ch] = uint8((2*a[ch] + b[ch]) / 3)
			p[3][ch] = uint8((a[ch] + 2*b[ch]) / 3)
		} else {
			p[2][ch] = uint8((a[ch] + b[ch]) / 2)
		}
	}
	return p
}
