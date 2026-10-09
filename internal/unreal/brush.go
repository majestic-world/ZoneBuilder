package unreal

import (
	"fmt"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
)

// Scale is an FScale struct: a per-axis scale and a shear of SheerRate
// along SheerAxis (an ESheerAxis value).
type Scale struct {
	Scale     geom.Vec3
	SheerRate float32
	SheerAxis uint8
}

// unitScale is FScale's default: no scaling and no shear.
var unitScale = Scale{Scale: geom.Vec3{X: 1, Y: 1, Z: 1}}

// Brush is the placement part of a placed brush actor (a Volume such as
// WaterVolume): its geometry is the Model Brush names, in brush space.
type Brush struct {
	Location, PrePivot   geom.Vec3
	Rotation             l2pkg.Rotator
	MainScale, PostScale Scale
	// Brush is the object reference of the brush's Model in the map
	// package's index space, 0 when the actor names none.
	Brush int32
	// DeleteMe is bDeleteMe: the actor is gone from the map.
	DeleteMe bool
}

// ReadBrush decodes the properties of export i (0-based) of p as a Brush.
// An absent MainScale or PostScale is the unit scale.
func ReadBrush(p *l2pkg.Package, i int) (*Brush, error) {
	r, err := p.ExportReader(i)
	if err != nil {
		return nil, err
	}
	props, err := l2pkg.ReadProperties(p, r, p.Exports[i].Flags)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", p.Exports[i].ClassName, p.Exports[i].ObjectName, err)
	}
	b := &Brush{}
	if v, ok := props.Vector("Location"); ok {
		b.Location = vec3(v)
	}
	if v, ok := props.Vector("PrePivot"); ok {
		b.PrePivot = vec3(v)
	}
	b.Rotation, _ = props.Rotator("Rotation")
	b.MainScale = readScale(props, "MainScale")
	b.PostScale = readScale(props, "PostScale")
	b.Brush, _ = props.Index("Brush")
	b.DeleteMe = props.Bool("bDeleteMe")
	return b, nil
}

// readScale is the Scale struct property name of props, with the fields it
// leaves out at their defaults.
func readScale(props l2pkg.Properties, name string) Scale {
	s := unitScale
	m := props.Maps(name)
	if len(m) == 0 {
		return s
	}
	if v, ok := m[0].Vector("Scale"); ok {
		s.Scale = vec3(v)
	}
	s.SheerRate, _ = m[0].Float("SheerRate")
	s.SheerAxis, _ = m[0].Byte("SheerAxis")
	return s
}

// Transform is the brush's brush-to-world transform, ABrush's rather than
// AActor's: no DrawScale, and the PrePivot taken off before the scales and
// the rotation. The shear is not applied.
func (b *Brush) Transform() BrushTransform {
	return BrushTransform{
		Location:  b.Location,
		PrePivot:  b.PrePivot,
		Rotation:  rotatorQuat(b.Rotation),
		MainScale: b.MainScale.Scale,
		PostScale: b.PostScale.Scale,
	}
}

// BrushTransform places brush-space points in the world, all in Unreal's
// basis: world = Location + PostScale·R(Rotation)·MainScale·(v − PrePivot).
type BrushTransform struct {
	Location, PrePivot   geom.Vec3
	Rotation             Quat
	MainScale, PostScale geom.Vec3
}

// Point is the world position of brush-space point v.
func (t BrushTransform) Point(v geom.Vec3) geom.Vec3 {
	return t.Location.Add(t.Rotation.Rotate(v.Sub(t.PrePivot).Mul(t.MainScale)).Mul(t.PostScale))
}
