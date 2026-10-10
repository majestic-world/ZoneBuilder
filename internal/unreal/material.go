package unreal

import (
	"fmt"
	"strings"

	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/texture"
)

// materialClasses are the material classes serialized as properties alone,
// each wrapping another material (UE2-Studio texture-engine object_json.rs
// MATERIAL_CLASSES).
var materialClasses = [...]string{
	"Shader", "Combiner", "FinalBlend", "ColorModifier", "TexEnvMap", "TexPanner",
	"TexRotator", "TexScaler", "TexOscillator", "OpacityModifier", "TexCoordSource", "Modifier",
}

// IsMaterialNode reports whether class is one of the wrapping material
// classes ReadMaterialNode reads (case-insensitive).
func IsMaterialNode(class string) bool {
	for _, c := range materialClasses {
		if strings.EqualFold(c, class) {
			return true
		}
	}
	return false
}

// MaterialBlend is how a material node asks to be composited.
type MaterialBlend uint8

const (
	BlendOpaque MaterialBlend = iota
	BlendMasked
	BlendTranslucent
	BlendBrighten
)

// MaterialNode is a material that wraps another one: a Shader, FinalBlend,
// Combiner or modifier. Port of UE2-Studio's MaterialWrapper
// (src/unreal/objects.rs).
type MaterialNode struct {
	// Inner is the wrapped material, an object reference in the node's
	// package: the first of Diffuse (a Shader's visible bitmap), Material
	// (modifiers, FinalBlend), Material1 (a Combiner's base) and Material2
	// that is set; 0 for none.
	Inner int32
	// Opacity is the Opacity material reference, 0 for none.
	Opacity        int32
	AlphaTest      bool
	OutputBlending uint8
}

// ReadMaterialNode decodes the properties of export i (0-based) of p as a
// MaterialNode.
func ReadMaterialNode(p *l2pkg.Package, i int) (*MaterialNode, error) {
	r, err := p.ExportReader(i)
	if err != nil {
		return nil, err
	}
	props, err := l2pkg.ReadProperties(p, r, p.Exports[i].Flags)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", p.Exports[i].ClassName, p.Exports[i].ObjectName, err)
	}
	n := &MaterialNode{}
	for _, name := range [...]string{"Diffuse", "Material", "Material1", "Material2"} {
		if ref, ok := props.Index(name); ok {
			n.Inner = ref
			break
		}
	}
	n.Opacity, _ = props.Index("Opacity")
	n.AlphaTest = props.Bool("AlphaTest")
	n.OutputBlending, _ = props.Byte("OutputBlending")
	return n, nil
}

// Blend is the compositing the node selects: OutputBlending first
// (1 masked, 5 brighten, 2, 3 and 6 translucent), then AlphaTest (masked),
// then an Opacity input (translucent).
func (n *MaterialNode) Blend() MaterialBlend {
	switch n.OutputBlending {
	case 1:
		return BlendMasked
	case 5:
		return BlendBrighten
	case 2, 3, 6:
		return BlendTranslucent
	}
	switch {
	case n.AlphaTest:
		return BlendMasked
	case n.Opacity != 0:
		return BlendTranslucent
	}
	return BlendOpaque
}

// maxMaterialDepth is how many material nodes WalkMaterial follows before it
// gives up (UE2-Studio's MAX_MATERIAL_DEPTH).
const maxMaterialDepth = 8

// Material is where a material graph leads: its first Texture and the blend
// flags gathered on the way down.
type Material struct {
	// Texture is the first Texture down the graph, nil when the graph has
	// none, it cannot be drawn, or the walk gave up: drawn untextured.
	Texture *texture.Texture
	// Masked is set when a node or the Texture asked for a binary cutout.
	Masked      bool
	Translucent bool
	Brighten    bool
	// Water is set when a node's path contains "water".
	Water bool
	// VertexOpacity is set when the Opacity input of the last node with one
	// is a VertexColor: the texture's alpha is ignored and the vertex
	// colour's alpha is the coverage.
	VertexOpacity bool
	// Shader is the first Shader node the walk went through, nil for none.
	Shader *MaterialNode
}

// TextureFunc reads Texture export i of p for WalkMaterial. drawable is
// false for a texture that reads but cannot be drawn (its Masked flag still
// counts); err is for a Texture that does not read.
type TextureFunc func(p *l2pkg.Package, i int) (t *texture.Texture, drawable bool, err error)

// WalkMaterial walks the material graph from object reference ref of p down
// to its first Texture, at most maxMaterialDepth nodes, gathering the blend
// flags on the way; readTexture reads that Texture. Port of UE2-Studio's
// visual_material_ref: a null reference keeps the flags so far, untextured;
// a missing package or object, a node of another class, and a walk that runs
// out of depth give the zero Material (untextured and opaque).
func WalkMaterial(c *l2pkg.Client, p *l2pkg.Package, ref int32, readTexture TextureFunc) (Material, error) {
	var m Material
	owner := p
	for range maxMaterialDepth {
		if ref == 0 {
			return m, nil
		}
		if path, err := owner.ObjectPath(ref); err == nil && strings.Contains(strings.ToLower(path), "water") {
			m.Water = true
		}
		pkg, i, err := c.Resolve(owner, ref)
		if l2pkg.IsMissing(err) {
			return Material{}, nil
		}
		if err != nil {
			return Material{}, err
		}
		class := pkg.Exports[i].ClassName
		switch {
		case class == "Texture":
			t, drawable, err := readTexture(pkg, i)
			if err != nil {
				return Material{}, err
			}
			if !m.VertexOpacity {
				m.Masked = m.Masked || t.Masked
			}
			if drawable {
				m.Texture = t
			}
			return m, nil
		case IsMaterialNode(class):
			n, err := ReadMaterialNode(pkg, i)
			if err != nil {
				return Material{}, err
			}
			if class == "Shader" && m.Shader == nil {
				m.Shader = n
			}
			if n.Opacity != 0 {
				m.VertexOpacity = className(pkg, n.Opacity) == "VertexColor"
			}
			switch n.Blend() {
			case BlendMasked:
				m.Masked = true
			case BlendTranslucent:
				m.Translucent = true
			case BlendBrighten:
				m.Brighten = true
			}
			owner, ref = pkg, n.Inner
		default:
			return Material{}, nil
		}
	}
	return Material{}, nil
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
