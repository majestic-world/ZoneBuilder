package render

import (
	"math"

	"zonebuilder/internal/geom"
)

// mat4 is a column-major 4x4 matrix, the layout glUniformMatrix4fv expects
// with transpose = false.
type mat4 [16]float32

// mul returns a*b (b applied first).
func mul(a, b mat4) mat4 {
	var m mat4
	for c := range 4 {
		for r := range 4 {
			var s float32
			for k := range 4 {
				s += a[k*4+r] * b[c*4+k]
			}
			m[c*4+r] = s
		}
	}
	return m
}

// reversedPerspective is a right-handed perspective projection (camera looks
// down -Z) that maps the near plane to depth 1 and the far plane to depth 0,
// for clip control ZERO_TO_ONE and a GREATER depth test (ADR 0001).
func reversedPerspective(fovY, aspect, near, far float32) mat4 {
	f := float32(1 / math.Tan(float64(fovY)/2))
	return mat4{
		0:  f / aspect,
		5:  f,
		10: near / (far - near),
		11: -1,
		14: near * far / (far - near),
	}
}

// lookAt is a right-handed view matrix for an eye looking along forward
// with the given right and up (an orthonormal basis).
func lookAt(eye, forward, right, up geom.Vec3) mat4 {
	return mat4{
		right.X, up.X, -forward.X, 0,
		right.Y, up.Y, -forward.Y, 0,
		right.Z, up.Z, -forward.Z, 0,
		-right.Dot(eye), -up.Dot(eye), forward.Dot(eye), 1,
	}
}

// unrealToRender swaps Y and Z: Unreal's Z-up basis to the Y-up render
// basis (scene.ToRender as a matrix).
var unrealToRender = mat4{
	0:  1,
	6:  1,
	9:  1,
	15: 1,
}
