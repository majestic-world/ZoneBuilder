package render

import (
	"zonebuilder/internal/render/gles"
	"zonebuilder/internal/texture"
)

// maxAnisotropy is UE2-Studio's sampler anisotropy_clamp.
const maxAnisotropy = 8

// textureCache uploads each scene texture once per role. Material textures
// are sRGB with mipmaps, repeat and anisotropic filtering; masks are linear
// (their R is a coverage, not a colour), one level, clamped, as UE2-Studio's
// upload_material_mask.
//
// DXT goes to the GPU as stored, with the package's own mips (ADR 0002);
// ANGLE refuses DXT whose level 0 is not a multiple of 4 on both sides, so
// those, RGBA8 and P8 are decoded on the CPU, where a material gets its
// full chain from texture.Image.Mipmaps. A texture drawn Masked is always
// decoded on the CPU, for UE2-Studio's colour bleed under the cutout
// (texture.Image.MaskedMipmaps), and uploaded apart from its other uses.
type textureCache struct {
	anisotropic bool
	materials   map[materialKey]uint32
	masks       map[*texture.Texture]uint32
	// grey stands in for a batch without texture (UE2-Studio's untextured
	// [170,170,170,255]); white for a layer without alpha map.
	grey, white uint32
}

// materialKey is one upload of a material texture: masked ones are
// prepared for the cutout.
type materialKey struct {
	t      *texture.Texture
	masked bool
}

func newTextureCache(anisotropic bool) *textureCache {
	tc := &textureCache{
		anisotropic: anisotropic,
		materials:   make(map[materialKey]uint32),
		masks:       make(map[*texture.Texture]uint32),
	}
	tc.grey = solid(170, true)
	tc.white = solid(255, false)
	return tc
}

func solid(v byte, srgb bool) uint32 {
	id := gles.GenTexture()
	gles.BindTexture(gles.TEXTURE_2D, id)
	format := uint32(gles.RGBA8)
	if srgb {
		format = gles.SRGB8_ALPHA8
	}
	gles.TexImage2D(gles.TEXTURE_2D, 0, format, 1, 1, gles.RGBA, gles.UNSIGNED_BYTE, []byte{v, v, v, 255})
	sampling(0, gles.LINEAR, gles.REPEAT, 1)
	gles.BindTexture(gles.TEXTURE_2D, 0)
	return id
}

// material is the GL texture drawing t, prepared for an alpha cutout when
// masked; nil is the untextured grey.
func (tc *textureCache) material(t *texture.Texture, masked bool) uint32 {
	if t == nil {
		return tc.grey
	}
	key := materialKey{t, masked}
	if id, ok := tc.materials[key]; ok {
		return id
	}
	id := gles.GenTexture()
	gles.BindTexture(gles.TEXTURE_2D, id)
	levels := 0
	if f, ok := dxtFormat(t.Format, true); ok && !masked && nativeDXT(t) {
		for n, m := range t.Chain() {
			gles.CompressedTexImage2D(gles.TEXTURE_2D, n, f, m.Width, m.Height, m.Data[:texture.MipBytes(t.Format, m.Width, m.Height)])
			levels++
		}
	} else if img, err := t.RGBA(); err == nil {
		chain := img.Mipmaps
		if masked {
			chain = img.MaskedMipmaps
		}
		for n, l := range chain() {
			gles.TexImage2D(gles.TEXTURE_2D, n, gles.SRGB8_ALPHA8, l.Width, l.Height, gles.RGBA, gles.UNSIGNED_BYTE, l.Pix)
			levels++
		}
	}
	if levels == 0 {
		// The scene only hands over drawable textures; this is a guard,
		// not a path: draw the grey instead of an incomplete texture.
		gles.BindTexture(gles.TEXTURE_2D, 0)
		gles.DeleteTexture(id)
		return tc.grey
	}
	aniso := int32(1)
	if tc.anisotropic {
		aniso = maxAnisotropy
	}
	sampling(levels-1, gles.LINEAR_MIPMAP_LINEAR, gles.REPEAT, aniso)
	gles.BindTexture(gles.TEXTURE_2D, 0)
	tc.materials[key] = id
	return id
}

// mask is the GL texture of an alpha map t; nil covers everything.
func (tc *textureCache) mask(t *texture.Texture) uint32 {
	if t == nil {
		return tc.white
	}
	if id, ok := tc.masks[t]; ok {
		return id
	}
	id := gles.GenTexture()
	gles.BindTexture(gles.TEXTURE_2D, id)
	ok := true
	if f, isDXT := dxtFormat(t.Format, false); isDXT && nativeDXT(t) {
		m := t.Mips[0]
		gles.CompressedTexImage2D(gles.TEXTURE_2D, 0, f, m.Width, m.Height, m.Data[:texture.MipBytes(t.Format, m.Width, m.Height)])
	} else if img, err := t.RGBA(); err == nil {
		gles.TexImage2D(gles.TEXTURE_2D, 0, gles.RGBA8, img.Width, img.Height, gles.RGBA, gles.UNSIGNED_BYTE, img.Pix)
	} else {
		ok = false
	}
	if !ok {
		gles.BindTexture(gles.TEXTURE_2D, 0)
		gles.DeleteTexture(id)
		return tc.white
	}
	sampling(0, gles.LINEAR, gles.CLAMP_TO_EDGE, 1)
	gles.BindTexture(gles.TEXTURE_2D, 0)
	tc.masks[t] = id
	return id
}

// sampling sets the bound texture's filtering: maxLevel is its last mip,
// minFilter LINEAR or LINEAR_MIPMAP_LINEAR, aniso 1 for none.
func sampling(maxLevel int, minFilter, wrap uint32, aniso int32) {
	gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_MAX_LEVEL, int32(maxLevel))
	gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_MIN_FILTER, int32(minFilter))
	gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_MAG_FILTER, gles.LINEAR)
	gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_WRAP_S, int32(wrap))
	gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_WRAP_T, int32(wrap))
	if aniso > 1 {
		gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_MAX_ANISOTROPY_EXT, aniso)
	}
}

// dxtFormat is the compressed GL format of a DXT texture format, sRGB for
// colour, linear for masks. DXT1 is always the RGBA variant, which keeps
// its 1-bit alpha (ADR 0002).
func dxtFormat(format uint8, srgb bool) (uint32, bool) {
	switch format {
	case texture.FormatDXT1:
		if srgb {
			return gles.COMPRESSED_SRGB_ALPHA_S3TC_DXT1_EXT, true
		}
		return gles.COMPRESSED_RGBA_S3TC_DXT1_EXT, true
	case texture.FormatDXT3:
		if srgb {
			return gles.COMPRESSED_SRGB_ALPHA_S3TC_DXT3_EXT, true
		}
		return gles.COMPRESSED_RGBA_S3TC_DXT3_EXT, true
	case texture.FormatDXT5:
		if srgb {
			return gles.COMPRESSED_SRGB_ALPHA_S3TC_DXT5_EXT, true
		}
		return gles.COMPRESSED_RGBA_S3TC_DXT5_EXT, true
	}
	return 0, false
}

// nativeDXT reports whether ANGLE takes t's level 0 as compressed data:
// both sides a multiple of 4 (ADR 0002).
func nativeDXT(t *texture.Texture) bool {
	c := t.Chain()
	return len(c) > 0 && c[0].Width%4 == 0 && c[0].Height%4 == 0
}

func (tc *textureCache) release() {
	tc.clear()
	gles.DeleteTexture(tc.grey)
	gles.DeleteTexture(tc.white)
}

// clear forgets the scene's textures, keeping the solid ones.
func (tc *textureCache) clear() {
	for t, id := range tc.materials {
		gles.DeleteTexture(id)
		delete(tc.materials, t)
	}
	for t, id := range tc.masks {
		gles.DeleteTexture(id)
		delete(tc.masks, t)
	}
}
