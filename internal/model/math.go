package model

import (
	"math"

	"zonebuilder/internal/geom"
)

// minPositive is Rust's f32::MIN_POSITIVE, the smallest normal float32 (not
// Go's math.SmallestNonzeroFloat32, which is subnormal).
const minPositive float32 = 0x1p-126

// Quat is a rotation quaternion, stored and serialized in X Y Z W order.
type Quat struct{ X, Y, Z, W float32 }

// IdentityQuat is the rotation that does nothing.
var IdentityQuat = Quat{W: 1}

func (q Quat) conjugate() Quat { return Quat{-q.X, -q.Y, -q.Z, q.W} }

func (q Quat) dot(o Quat) float32 { return q.X*o.X + q.Y*o.Y + q.Z*o.Z + q.W*o.W }

// axis is the rotation's basis as the rows of a matrix (UEViewer CQuat::ToAxis;
// the products are pre-doubled).
func (q Quat) axis() [3]geom.Vec3 {
	x2, y2, z2 := q.X*2, q.Y*2, q.Z*2
	xx, xy, xz := q.X*x2, q.X*y2, q.X*z2
	yy, yz, zz := q.Y*y2, q.Y*z2, q.Z*z2
	wx, wy, wz := q.W*x2, q.W*y2, q.W*z2
	return [3]geom.Vec3{
		{X: 1 - (yy + zz), Y: xy - wz, Z: xz + wy},
		{X: xy + wz, Y: 1 - (xx + zz), Z: yz - wx},
		{X: xz - wy, Y: yz + wx, Z: 1 - (xx + yy)},
	}
}

// slerp interpolates along the shorter arc, as UEViewer's Slerp: a negative dot
// flips the sign of the second scale, near-parallel falls back to a plain lerp,
// and the result is not renormalized.
func (q Quat) slerp(o Quat, alpha float32) Quat {
	if alpha <= 0 {
		return q
	}
	if alpha >= 1 {
		return o
	}
	cosine := q.dot(o)
	var sign float32 = 1
	if cosine < 0 {
		cosine, sign = -cosine, -1
	}
	scaleA, scaleB := 1-alpha, alpha
	if 1-cosine > 1e-6 {
		squared := 1 - cosine*cosine
		inverseSine := 1 / float32(math.Sqrt(float64(max(squared, minPositive))))
		omega := math.Atan2(float64(squared*inverseSine), float64(cosine))
		scaleA = float32(math.Sin(float64(1-alpha)*omega)) * inverseSine
		scaleB = float32(math.Sin(float64(alpha)*omega)) * inverseSine
	}
	scaleB *= sign
	return Quat{
		scaleA*q.X + scaleB*o.X,
		scaleA*q.Y + scaleB*o.Y,
		scaleA*q.Z + scaleB*o.Z,
		scaleA*q.W + scaleB*o.W,
	}
}

// Affine is Unreal's FCoords: an origin and three basis ROWS. Point(p) is
// origin + Axis[0]*p.X + Axis[1]*p.Y + Axis[2]*p.Z.
type Affine struct {
	Origin geom.Vec3
	Axis   [3]geom.Vec3
}

// IdentityAffine is the transform that does nothing.
var IdentityAffine = Affine{Axis: [3]geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}}}

func newAffine(origin geom.Vec3, q Quat) Affine { return Affine{origin, q.axis()} }

// Point applies the transform to a position.
func (a Affine) Point(p geom.Vec3) geom.Vec3 { return a.Origin.Add(a.Vector(p)) }

// Vector applies the transform's basis to a direction (no translation).
func (a Affine) Vector(v geom.Vec3) geom.Vec3 {
	return a.Axis[0].Scale(v.X).Add(a.Axis[1].Scale(v.Y)).Add(a.Axis[2].Scale(v.Z))
}

// Compose returns a applied after inner (UEViewer UnTransformCoords): each row
// of inner's basis goes through a, which is not a naive matrix product.
func (a Affine) Compose(inner Affine) Affine {
	return Affine{
		Origin: a.Point(inner.Origin),
		Axis:   [3]geom.Vec3{a.Vector(inner.Axis[0]), a.Vector(inner.Axis[1]), a.Vector(inner.Axis[2])},
	}
}

// invert is the inverse of an orthonormal transform, a bone's (UEViewer
// InvertCoords): the basis transposed and the origin negated through the
// forward basis.
func (a Affine) invert() Affine {
	x, y, z := a.Axis[0], a.Axis[1], a.Axis[2]
	return Affine{
		Origin: geom.Vec3{X: -a.Origin.Dot(x), Y: -a.Origin.Dot(y), Z: -a.Origin.Dot(z)},
		Axis: [3]geom.Vec3{
			{X: x.X, Y: y.X, Z: z.X},
			{X: x.Y, Y: y.Y, Z: z.Y},
			{X: x.Z, Y: y.Z, Z: z.Z},
		},
	}
}
