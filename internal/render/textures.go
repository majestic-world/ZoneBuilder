package render

import (
	"zonebuilder/internal/render/gles"
	"zonebuilder/internal/texture"
)

// maxAnisotropy is UE2-Studio's sampler anisotropy_clamp.
const maxAnisotropy = 8

// textureCache holds the GL textures of every scene on the GPU, one per
// textureKey, shared by the scenes that use it and deleted when the last
// of them lets go. Material textures are sRGB with mipmaps, repeat and
// anisotropic filtering; masks are linear (their R is a coverage, not a
// colour), one level, clamped, as UE2-Studio's upload_material_mask.
//
// DXT goes to the GPU as stored, with the package's own mips (ADR 0002);
// ANGLE refuses DXT whose level 0 is not a multiple of 4 on both sides, so
// those, RGBA8 and P8 are decoded on the CPU (Prepare), where a material
// gets its full chain from texture.Image.Mipmaps. A texture drawn Masked is
// always decoded on the CPU, for UE2-Studio's colour bleed under the cutout
// (texture.Image.MaskedMipmaps), and uploaded apart from its other uses.
type textureCache struct {
	anisotropic bool
	entries     map[textureKey]*cachedTexture
	// grey stands in for a batch without texture (UE2-Studio's untextured
	// [170,170,170,255]); white for a layer without alpha map.
	grey, white uint32
}

// cachedTexture is one GL texture (0 when it could not be uploaded) and
// how many scenes on the GPU use it.
type cachedTexture struct {
	id   uint32
	refs int
}

func newTextureCache(anisotropic bool) *textureCache {
	tc := &textureCache{anisotropic: anisotropic, entries: make(map[textureKey]*cachedTexture)}
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

// upload puts pt on the GPU unless it is there already. It takes no
// reference: acquire does.
func (tc *textureCache) upload(pt *preparedTexture) {
	if _, ok := tc.entries[pt.key]; ok {
		return
	}
	id := gles.GenTexture()
	gles.BindTexture(gles.TEXTURE_2D, id)
	mask := pt.key.role == roleMask
	levels := 0
	if f, ok := dxtFormat(pt.t.Format, !mask); ok && pt.compressed {
		for n, m := range pt.t.Chain() {
			gles.CompressedTexImage2D(gles.TEXTURE_2D, n, f, m.Width, m.Height, m.Data[:texture.MipBytes(pt.t.Format, m.Width, m.Height)])
			levels++
			if mask {
				break // a mask keeps level 0 only
			}
		}
	} else {
		format := uint32(gles.SRGB8_ALPHA8)
		if mask {
			format = gles.RGBA8
		}
		for n, l := range pt.levels {
			gles.TexImage2D(gles.TEXTURE_2D, n, format, l.Width, l.Height, gles.RGBA, gles.UNSIGNED_BYTE, l.Pix)
			levels++
		}
	}
	if levels == 0 {
		// Prepare could not decode it: the stand-in draws instead.
		gles.BindTexture(gles.TEXTURE_2D, 0)
		gles.DeleteTexture(id)
		tc.entries[pt.key] = &cachedTexture{}
		return
	}
	if mask {
		sampling(0, gles.LINEAR, gles.CLAMP_TO_EDGE, 1)
	} else {
		aniso := int32(1)
		if tc.anisotropic {
			aniso = maxAnisotropy
		}
		sampling(levels-1, gles.LINEAR_MIPMAP_LINEAR, gles.REPEAT, aniso)
	}
	gles.BindTexture(gles.TEXTURE_2D, 0)
	tc.entries[pt.key] = &cachedTexture{id: id}
}

// acquire takes a reference on k, which upload put on the GPU, and returns
// its GL texture. The zero key, and a texture that could not be uploaded,
// draw with the stand-in of their role: grey material, white mask.
func (tc *textureCache) acquire(k textureKey) uint32 {
	if e := tc.entries[k]; e != nil {
		e.refs++
		if e.id != 0 {
			return e.id
		}
	}
	if k.role == roleMask {
		return tc.white
	}
	return tc.grey
}

// drop lets go of one reference on k, deleting its GL texture with the
// last one.
func (tc *textureCache) drop(k textureKey) {
	e := tc.entries[k]
	if e == nil {
		return
	}
	if e.refs--; e.refs > 0 {
		return
	}
	if e.id != 0 {
		gles.DeleteTexture(e.id)
	}
	delete(tc.entries, k)
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
	for k, e := range tc.entries {
		if e.id != 0 {
			gles.DeleteTexture(e.id)
		}
		delete(tc.entries, k)
	}
	gles.DeleteTexture(tc.grey)
	gles.DeleteTexture(tc.white)
}
