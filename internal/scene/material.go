package scene

import (
	"fmt"
	"strings"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/texture"
	"zonebuilder/internal/unreal"
)

// loader is what one Load shares across its tiles: the client's packages,
// every Texture export, material and StaticMesh already read from them,
// and the batch each batchKey draws into.
type loader struct {
	c         *l2pkg.Client
	textures  map[objectKey]textureEntry
	materials map[materialKey]material
	meshes    map[objectKey]*unreal.StaticMesh
	batches   map[batchKey]int
}

// objectKey names one export: its owning package and 0-based index.
type objectKey struct {
	pkg    *l2pkg.Package
	export int
}

// materialKey is a material reference and the package it belongs to.
type materialKey struct {
	pkg *l2pkg.Package
	ref int32
}

// textureEntry is a read Texture export: the texture, or why it cannot be
// drawn.
type textureEntry struct {
	t   *texture.Texture
	err error
}

// batchKey is a batch's identity outside the terrain: the Texture pointer
// (shared by every user of one export), the render mode, and whether the
// texture's alpha is ignored (Batch.OpaqueTexture). Those batches have no
// Mask.
type batchKey struct {
	tex    *texture.Texture
	mode   RenderMode
	opaque bool
}

func newLoader(c *l2pkg.Client) *loader {
	return &loader{
		c:         c,
		textures:  make(map[objectKey]textureEntry),
		materials: make(map[materialKey]material),
		meshes:    make(map[objectKey]*unreal.StaticMesh),
		batches:   make(map[batchKey]int),
	}
}

// batch is the index of the batch k draws into, appended on first use.
func (s *Scene) batch(ld *loader, k batchKey) int {
	if i, ok := ld.batches[k]; ok {
		return i
	}
	i := len(s.Batches)
	s.Batches = append(s.Batches, Batch{Mode: k.mode, Texture: k.tex, OpaqueTexture: k.opaque, Bounds: geom.EmptyBox()})
	ld.batches[k] = i
	return i
}

// maxMaterialDepth is how many material nodes the walk follows before it
// gives up (UE2-Studio's MAX_MATERIAL_DEPTH).
const maxMaterialDepth = 8

// material is what a material reference resolves to for drawing.
type material struct {
	// texture is the first Texture down the graph, nil when the graph has
	// none, it cannot be decoded, or the walk gave up: drawn untextured.
	texture *texture.Texture
	mode    RenderMode
	// masked is set when a node or the Texture asked for a binary cutout,
	// whatever mode the blend flags chose.
	masked bool
	// vertexOpacity is set when the Opacity input of the last node with one
	// is a VertexColor: the texture's alpha is ignored and the vertex
	// colour's alpha is the coverage.
	vertexOpacity bool
}

// material is what reference ref of p draws with, walked once per Load.
func (ld *loader) material(p *l2pkg.Package, ref int32) (material, error) {
	k := materialKey{p, ref}
	if m, ok := ld.materials[k]; ok {
		return m, nil
	}
	m, err := ld.walkMaterial(p, ref)
	if err != nil {
		return material{}, err
	}
	ld.materials[k] = m
	return m, nil
}

// walkMaterial walks the material graph from reference ref of p down to
// its first Texture, at most maxMaterialDepth nodes, gathering the blend
// flags on the way. Port of UE2-Studio's visual_material_ref: a missing
// package or object, a node of another class, and a walk that runs out of
// depth all draw untextured and opaque; any node whose path contains
// "water" makes a translucent result Water.
func (ld *loader) walkMaterial(p *l2pkg.Package, ref int32) (material, error) {
	var masked, translucent, brighten, water, vertexOpacity bool
	result := func(t *texture.Texture) material {
		return material{texture: t, mode: materialMode(masked, translucent, brighten, water), masked: masked, vertexOpacity: vertexOpacity}
	}
	owner := p
	for range maxMaterialDepth {
		if ref == 0 {
			return result(nil), nil
		}
		if path, err := owner.ObjectPath(ref); err == nil && strings.Contains(strings.ToLower(path), "water") {
			water = true
		}
		pkg, i, err := ld.c.Resolve(owner, ref)
		if l2pkg.IsMissing(err) {
			return material{}, nil
		}
		if err != nil {
			return material{}, err
		}
		class := pkg.Exports[i].ClassName
		switch {
		case class == "Texture":
			t, err := ld.textureAt(pkg, i)
			if err != nil {
				return material{}, err
			}
			if !vertexOpacity {
				masked = masked || t.t.Masked
			}
			if t.err != nil {
				return result(nil), nil
			}
			return result(t.t), nil
		case unreal.IsMaterialNode(class):
			n, err := unreal.ReadMaterialNode(pkg, i)
			if err != nil {
				return material{}, err
			}
			if n.Opacity != 0 {
				vertexOpacity = className(pkg, n.Opacity) == "VertexColor"
			}
			switch n.Blend() {
			case unreal.BlendMasked:
				masked = true
			case unreal.BlendTranslucent:
				translucent = true
			case unreal.BlendBrighten:
				brighten = true
			}
			owner, ref = pkg, n.Inner
		default:
			return material{}, nil
		}
	}
	return material{}, nil
}

// className is the class name of object reference ref as p records it,
// "" when it is out of range (UE2-Studio reference_class_name).
func className(p *l2pkg.Package, ref int32) string {
	if ref < 0 {
		if int(-ref) <= len(p.Imports) {
			return p.Imports[-ref-1].ClassName
		}
		return ""
	}
	if int(ref) <= len(p.Exports) {
		return p.Exports[ref-1].ClassName
	}
	return ""
}

// materialMode is UE2-Studio's RenderMode::from_material_semantics.
func materialMode(masked, translucent, brighten, water bool) RenderMode {
	switch {
	case brighten:
		return Brighten
	case translucent && water:
		return Water
	case translucent:
		return Translucent
	case masked:
		return Masked
	}
	return Opaque
}

// texture is the drawable Texture that object reference ref of p points
// at, read once per Load: every batch drawn with one Texture export shares
// the pointer, which is the batch key's texture identity. It fails when the
// reference is not a Texture or the texture cannot be drawn
// (texture.Drawable).
func (ld *loader) texture(p *l2pkg.Package, ref int32) (*texture.Texture, error) {
	owner, i, err := ld.c.Resolve(p, ref)
	if err != nil {
		return nil, err
	}
	if cls := owner.Exports[i].ClassName; cls != "Texture" {
		return nil, fmt.Errorf("%s.%s é %s, não Texture", owner.Name, owner.Exports[i].ObjectName, cls)
	}
	t, err := ld.textureAt(owner, i)
	if err != nil {
		return nil, err
	}
	return t.t, t.err
}

// textureAt reads Texture export i of p once per Load, with the Palette a
// P8 texture decodes through. A texture that reads but cannot be drawn
// (texture.Drawable, a palette that is missing or unreadable) comes back
// with entry.err set; the error return is for a Texture that does not read.
func (ld *loader) textureAt(p *l2pkg.Package, i int) (textureEntry, error) {
	key := objectKey{p, i}
	if e, ok := ld.textures[key]; ok {
		return e, nil
	}
	t, err := texture.Read(p, i)
	if err != nil {
		return textureEntry{}, fmt.Errorf("%s.%s: %w", p.Name, p.Exports[i].ObjectName, err)
	}
	e := textureEntry{t: t}
	if t.Format == texture.FormatP8 && t.PaletteRef != 0 {
		t.Palette, e.err = ld.palette(p, t.PaletteRef)
	}
	if e.err == nil {
		e.err = t.Drawable()
	}
	if e.err != nil {
		e.err = fmt.Errorf("%s: %w", t.Path, e.err)
	} else {
		// A drawable texture outlives the Load in the scene's batches:
		// keep its mips, not the whole package they alias.
		t.Own()
	}
	ld.textures[key] = e
	return e, nil
}

// palette reads the Palette that reference ref of p points at.
func (ld *loader) palette(p *l2pkg.Package, ref int32) ([]texture.Color, error) {
	owner, i, err := ld.c.Resolve(p, ref)
	if err != nil {
		return nil, fmt.Errorf("Palette: %w", err)
	}
	if cls := owner.Exports[i].ClassName; cls != "Palette" {
		return nil, fmt.Errorf("Palette aponta para %s", cls)
	}
	return texture.ReadPalette(owner, i)
}
