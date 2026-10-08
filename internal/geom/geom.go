// Package geom holds the small float32 vector and box types the scene, the
// camera and the renderer share.
package geom

import "math"

// Vec3 is a 3-component float32 vector.
type Vec3 struct{ X, Y, Z float32 }

func (a Vec3) Add(b Vec3) Vec3      { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3      { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Scale(s float32) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func (a Vec3) Mul(b Vec3) Vec3      { return Vec3{a.X * b.X, a.Y * b.Y, a.Z * b.Z} }
func (a Vec3) Dot(b Vec3) float32   { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Length() float32      { return float32(math.Sqrt(float64(a.Dot(a)))) }
func (a Vec3) Min(b Vec3) Vec3      { return Vec3{min(a.X, b.X), min(a.Y, b.Y), min(a.Z, b.Z)} }
func (a Vec3) Max(b Vec3) Vec3      { return Vec3{max(a.X, b.X), max(a.Y, b.Y), max(a.Z, b.Z)} }
func (a Vec3) Axis(i int) float32   { return [3]float32{a.X, a.Y, a.Z}[i] }
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}

// Normalize returns a unit vector along a, or zero for a zero vector.
func (a Vec3) Normalize() Vec3 {
	l := a.Length()
	if l == 0 {
		return Vec3{}
	}
	return a.Scale(1 / l)
}

// Box is an axis-aligned box. The zero Box is the point at the origin; use
// EmptyBox for a box that contains nothing yet.
type Box struct{ Min, Max Vec3 }

// EmptyBox returns a box that Include grows from nothing.
func EmptyBox() Box {
	inf := float32(math.Inf(1))
	return Box{Min: Vec3{inf, inf, inf}, Max: Vec3{-inf, -inf, -inf}}
}

// Empty reports a box that contains no point.
func (b Box) Empty() bool { return b.Min.X > b.Max.X || b.Min.Y > b.Max.Y || b.Min.Z > b.Max.Z }

// Include grows b to contain p.
func (b *Box) Include(p Vec3) { b.Min, b.Max = b.Min.Min(p), b.Max.Max(p) }

// Union grows b to contain o.
func (b *Box) Union(o Box) {
	if !o.Empty() {
		b.Min, b.Max = b.Min.Min(o.Min), b.Max.Max(o.Max)
	}
}

// Center is the box's centre.
func (b Box) Center() Vec3 { return b.Min.Add(b.Max).Scale(0.5) }

// Size is the box's extent on each axis.
func (b Box) Size() Vec3 { return b.Max.Sub(b.Min) }
