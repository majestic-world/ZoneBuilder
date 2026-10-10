package scene

import (
	"fmt"

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
	// defaults are the script classes' collision defaults.
	defaults *unreal.ClassDefaults
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
// (shared by every user of one export), the render mode, whether the
// texture's alpha is ignored (Batch.OpaqueTexture), and whether it draws
// static mesh actors (Batch.Mesh), which never share a batch with the BSP.
// Those batches have no Mask.
type batchKey struct {
	tex    *texture.Texture
	mode   RenderMode
	opaque bool
	mesh   bool
}

func newLoader(c *l2pkg.Client) *loader {
	return &loader{
		c:         c,
		textures:  make(map[objectKey]textureEntry),
		materials: make(map[materialKey]material),
		meshes:    make(map[objectKey]*unreal.StaticMesh),
		batches:   make(map[batchKey]int),
		defaults:  unreal.NewClassDefaults(c),
	}
}

// batch is the index of the batch k draws into, appended on first use.
func (s *Scene) batch(ld *loader, k batchKey) int {
	if i, ok := ld.batches[k]; ok {
		return i
	}
	i := len(s.Batches)
	s.Batches = append(s.Batches, Batch{Mode: k.mode, Texture: k.tex, OpaqueTexture: k.opaque, Mesh: k.mesh, Bounds: geom.EmptyBox()})
	ld.batches[k] = i
	return i
}

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
// its first Texture (unreal.WalkMaterial), reading the Texture once per
// Load.
func (ld *loader) walkMaterial(p *l2pkg.Package, ref int32) (material, error) {
	m, err := unreal.WalkMaterial(ld.c, p, ref, func(pkg *l2pkg.Package, i int) (*texture.Texture, bool, error) {
		e, err := ld.textureAt(pkg, i)
		return e.t, e.err == nil, err
	})
	if err != nil {
		return material{}, err
	}
	return material{
		texture:       m.Texture,
		mode:          materialMode(m.Masked, m.Translucent, m.Brighten, m.Water),
		masked:        m.Masked,
		vertexOpacity: m.VertexOpacity,
	}, nil
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
