package unreal

import (
	"fmt"
	"strings"

	"zonebuilder/internal/l2pkg"
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
