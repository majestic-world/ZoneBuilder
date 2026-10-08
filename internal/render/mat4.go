package render

import "math"

// mat4 is a column-major 4x4 matrix, the layout glUniformMatrix4fv expects
// with transpose = false.
type mat4 [16]float32

func identity() mat4 {
	return mat4{0: 1, 5: 1, 10: 1, 15: 1}
}

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

func translate(x, y, z float32) mat4 {
	m := identity()
	m[12], m[13], m[14] = x, y, z
	return m
}

func rotateX(a float32) mat4 {
	s, c := float32(math.Sin(float64(a))), float32(math.Cos(float64(a)))
	m := identity()
	m[5], m[6], m[9], m[10] = c, s, -s, c
	return m
}

func rotateY(a float32) mat4 {
	s, c := float32(math.Sin(float64(a))), float32(math.Cos(float64(a)))
	m := identity()
	m[0], m[2], m[8], m[10] = c, -s, s, c
	return m
}
