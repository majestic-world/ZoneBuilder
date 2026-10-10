package scene

import (
	"math"

	"zonebuilder/internal/geom"
)

// FloorTriangle is one triangle of floor, the surface a character can
// stand on, in server coordinates (ToServer of the geometry).
type FloorTriangle struct {
	A, B, C geom.Vec3
	Surface Surface
}

// Floor calls fn once with every floor triangle of the world whose cell
// or set box meets the X/Y of box (server coordinates; box's Z is not
// looked at), so a few triangles just outside box may come too. The floor
// is what is drawn and picked: every visible terrain quad, as its 2
// triangles split on the EdgeTurn diagonal, and nothing of an invisible
// quad; plus every BSP or static mesh triangle facing up (normal.z ≥
// floorNormalZ), the meshes left out while HideMeshes is set. Floor only
// reads the scenes: it may run off the event loop on a World that the
// loop does not Add to or Remove from meanwhile.
func (w *World) Floor(box geom.Box, fn func(FloorTriangle)) {
	w.Geometry(box, func(t Triangle) {
		if t.Surface == SurfaceTerrain || upward(t.Normal) {
			fn(FloorTriangle{A: t.A, B: t.B, C: t.C, Surface: t.Surface})
		}
	}, nil)
}

// floorNormalZ is the least Z of a unit normal that a BSP or mesh face can
// stand on (spec "Chão"): a slope up to 60°.
const floorNormalZ = 0.5

// upward reports a face of normal n (any length) as one to stand on.
func upward(n geom.Vec3) bool { return n.Z > 0 && n.Z >= floorNormalZ*n.Length() }

// at is the world position of batch vertex k.
func (t *Terrain) at(k uint32) geom.Vec3 {
	return t.Vertex(int(k)%t.Width, int(k)/t.Width)
}

// cellSpan is the range of cells, of n along an axis that starts at origin
// with cells of size step, under [lo, hi]; empty (first > last) when none
// is.
func cellSpan(lo, hi, origin, step float32, n int) (first, last int) {
	a, b := (lo-origin)/step, (hi-origin)/step
	if step < 0 {
		a, b = b, a
	}
	first = max(int(math.Floor(float64(a))), 0)
	last = min(int(math.Floor(float64(b))), n-1)
	return first, last
}
