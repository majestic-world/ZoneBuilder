package render

import "zonebuilder/internal/geom"

// frustum is the view volume of a reversed-Z, zero-to-one clip space as six
// planes n·p + d >= 0 (Gribb–Hartmann extraction): left, right, bottom,
// top, far (depth >= 0) and near (depth <= w).
type frustum [6]struct {
	n geom.Vec3
	d float32
}

// newFrustum extracts the planes of m, a column-major matrix to clip
// space; they live in the space m takes points from.
func newFrustum(m mat4) frustum {
	row := func(r int) [4]float32 { return [4]float32{m[r], m[4+r], m[8+r], m[12+r]} }
	x, y, z, w := row(0), row(1), row(2), row(3)
	var f frustum
	for i, p := range [6][4]float32{
		planeSum(w, x, 1), planeSum(w, x, -1),
		planeSum(w, y, 1), planeSum(w, y, -1),
		z, planeSum(w, z, -1),
	} {
		f[i].n, f[i].d = geom.Vec3{X: p[0], Y: p[1], Z: p[2]}, p[3]
	}
	return f
}

func planeSum(a, b [4]float32, s float32) [4]float32 {
	return [4]float32{a[0] + s*b[0], a[1] + s*b[1], a[2] + s*b[2], a[3] + s*b[3]}
}

// sees reports whether box b may be in view: false only when b lies wholly
// outside one plane.
func (f *frustum) sees(b geom.Box) bool {
	for i := range f {
		n := f[i].n
		p := b.Min
		if n.X >= 0 {
			p.X = b.Max.X
		}
		if n.Y >= 0 {
			p.Y = b.Max.Y
		}
		if n.Z >= 0 {
			p.Z = b.Max.Z
		}
		if n.Dot(p)+f[i].d < 0 {
			return false
		}
	}
	return true
}
