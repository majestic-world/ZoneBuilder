package texture

import (
	"fmt"
	"math"
	"sync"
)

// Drawable reports why the texture cannot be drawn, or nil: its format must
// be Decodable and mip 0 must hold the bytes its size needs. A texture that
// fails is drawn untextured, as UE2-Studio does when as_visual is None.
func (t *Texture) Drawable() error {
	if !Decodable(t.Format) {
		return fmt.Errorf("formato %d não decodificado", t.Format)
	}
	if len(t.Mips) == 0 {
		return fmt.Errorf("textura sem mip")
	}
	m := t.Mips[0]
	if m.Width <= 0 || m.Height <= 0 {
		return fmt.Errorf("mip 0 de %d×%d", m.Width, m.Height)
	}
	if need := MipBytes(t.Format, m.Width, m.Height); len(m.Data) < need {
		return fmt.Errorf("mip 0 truncado: %d bytes de %d", len(m.Data), need)
	}
	return nil
}

// Chain is the run of package mips, from mip 0, that forms a valid GPU mip
// chain: each level half the previous one (rounded down, at least 1) with
// the bytes its size needs. A package that stops before 1×1, or carries a
// level out of step, keeps only the levels before the break.
func (t *Texture) Chain() []Mip {
	if len(t.Mips) == 0 {
		return nil
	}
	w, h := t.Mips[0].Width, t.Mips[0].Height
	for n, m := range t.Mips {
		if m.Width != w || m.Height != h || len(m.Data) < MipBytes(t.Format, w, h) {
			return t.Mips[:n]
		}
		w, h = max(1, w/2), max(1, h/2)
	}
	return t.Mips
}

// Mipmaps is the full chain of img down to 1×1, img first. Each level is a
// 2×2 box filter of the previous one, colour averaged in linear light and
// alpha averaged as stored, clamping at odd edges. Port of UE2-Studio's
// material.rs downsample_rgba_into.
func (img *Image) Mipmaps() []*Image {
	levels := []*Image{img}
	for prev := img; prev.Width > 1 || prev.Height > 1; {
		next := &Image{Width: max(1, prev.Width/2), Height: max(1, prev.Height/2)}
		next.Pix = make([]byte, 4*next.Width*next.Height)
		downsample(prev, next)
		levels = append(levels, next)
		prev = next
	}
	return levels
}

func downsample(src, dst *Image) {
	lin := srgbToLinear()
	at := func(x, y int) int {
		return 4 * (min(y, src.Height-1)*src.Width + min(x, src.Width-1))
	}
	for y := range dst.Height {
		for x := range dst.Width {
			c := [4]int{at(2*x, 2*y), at(2*x+1, 2*y), at(2*x, 2*y+1), at(2*x+1, 2*y+1)}
			o := 4 * (y*dst.Width + x)
			for ch := range 3 {
				var sum float32
				for _, k := range c {
					sum += lin[src.Pix[k+ch]]
				}
				dst.Pix[o+ch] = linearToSRGB(sum * 0.25)
			}
			var a int
			for _, k := range c {
				a += int(src.Pix[k+3])
			}
			dst.Pix[o+3] = uint8(a / 4)
		}
	}
}

var srgbToLinear = sync.OnceValue(func() *[256]float32 {
	var t [256]float32
	for v := range t {
		c := float64(v) / 255
		if c <= 0.04045 {
			t[v] = float32(c / 12.92)
		} else {
			t[v] = float32(math.Pow((c+0.055)/1.055, 2.4))
		}
	}
	return &t
})

func linearToSRGB(c float32) uint8 {
	var e float64
	if c <= 0.0031308 {
		e = float64(c) * 12.92
	} else {
		e = 1.055*math.Pow(float64(c), 1/2.4) - 0.055
	}
	return uint8(min(max(e*255+0.5, 0), 255))
}
