package scene

import (
	"math"

	"zonebuilder/internal/geom"
)

// Ray is a half-line in client world coordinates (Unreal basis, the
// scene's geometry space): the points Origin + Dir*t for t >= 0.
type Ray struct {
	Origin, Dir geom.Vec3
}

// Surface is the kind of geometry a pick hit.
type Surface uint8

const (
	// SurfaceTerrain is a tile's height field.
	SurfaceTerrain Surface = iota + 1
)

// Hit is where a ray first meets the scene.
type Hit struct {
	// Pos is the hit in server coordinates (ToServer of the surface point):
	// what //pos prints standing there.
	Pos geom.Vec3
	// Distance is how far along the ray the surface lies, in world units.
	Distance float32
	Surface  Surface
}

// Pick returns the nearest point where r meets the scene's geometry, or
// false when it meets nothing (it misses every terrain, points away from
// them, or has no direction). Only drawn triangles are hit: an invisible
// terrain quad is a hole.
func (s *Scene) Pick(r Ray) (Hit, bool) {
	if r.Dir.Dot(r.Dir) == 0 {
		return Hit{}, false
	}
	r.Dir = r.Dir.Normalize()
	best := Hit{Distance: float32(math.Inf(1))}
	for i := range s.Terrains {
		if d, ok := s.Terrains[i].pick(r); ok && d < best.Distance {
			best = Hit{Pos: ToServer(r.Origin.Add(r.Dir.Scale(d))), Distance: d, Surface: SurfaceTerrain}
		}
	}
	return best, best.Surface != 0
}

// pick walks the cells under the ray's footprint on the grid in order
// (2D Amanatides–Woo DDA) and returns the distance to the first triangle
// hit. Port of UE2-Studio's pick_terrain_grid (src/scene.rs), done in the
// terrain's local frame (world minus Position) to keep float precision.
func (t *Terrain) pick(r Ray) (float32, bool) {
	if t.Scale.X == 0 || t.Scale.Y == 0 {
		return 0, false
	}
	o := r.Origin.Sub(t.Position)
	from := [2]float32{o.X / t.Scale.X, o.Y / t.Scale.Y}
	along := [2]float32{r.Dir.X / t.Scale.X, r.Dir.Y / t.Scale.Y}
	limits := [2]float32{float32(t.Width - 1), float32(t.Height - 1)}

	// Clip the ray to the grid's footprint, entry clamped to the origin.
	enter, exit := float32(0), float32(math.Inf(1))
	for a := range 2 {
		if along[a] == 0 {
			if from[a] < 0 || from[a] > limits[a] {
				return 0, false
			}
			continue
		}
		first := -from[a] / along[a]
		last := (limits[a] - from[a]) / along[a]
		enter = max(enter, min(first, last))
		exit = min(exit, max(first, last))
		if enter > exit {
			return 0, false
		}
	}
	sample := enter
	if exit > enter {
		sample += min(exit-enter, 1e-4)
	}
	cellsX, cellsY := t.Width-1, t.Height-1
	x := clampInt(int(math.Floor(float64(from[0]+along[0]*sample))), 0, cellsX-1)
	y := clampInt(int(math.Floor(float64(from[1]+along[1]*sample))), 0, cellsY-1)

	stepX, nextX, deltaX := ddaAxis(x, from[0], along[0])
	stepY, nextY, deltaY := ddaAxis(y, from[1], along[1])
	for x >= 0 && x < cellsX && y >= 0 && y < cellsY {
		if tris, ok := t.cell(x, y); ok {
			nearest, hit := float32(math.Inf(1)), false
			for k := 0; k < 6; k += 3 {
				d, ok := rayTriangle(o, r.Dir, t.local(tris[k]), t.local(tris[k+1]), t.local(tris[k+2]))
				if ok && d < nearest {
					nearest, hit = d, true
				}
			}
			if hit {
				return nearest, true
			}
		}
		crossing := min(nextX, nextY)
		if crossing > exit || math.IsInf(float64(crossing), 0) {
			break
		}
		switch {
		case nextX < nextY:
			x += stepX
			nextX += deltaX
		case nextY < nextX:
			y += stepY
			nextY += deltaY
		default: // through a corner: both axes at once
			x += stepX
			y += stepY
			nextX += deltaX
			nextY += deltaY
		}
	}
	return 0, false
}

// ddaAxis is one axis of the DDA: the cell step, the ray parameter of the
// next cell boundary, and the parameter span of one cell.
func ddaAxis(cell int, from, along float32) (step int, next, delta float32) {
	inf := float32(math.Inf(1))
	switch {
	case along > 0:
		return 1, (float32(cell+1) - from) / along, 1 / along
	case along < 0:
		return -1, (float32(cell) - from) / along, -1 / along
	}
	return 0, inf, inf
}

// local is the position of batch vertex k relative to Position.
func (t *Terrain) local(k uint32) geom.Vec3 {
	x, y := int(k)%t.Width, int(k)/t.Width
	return geom.Vec3{X: float32(x), Y: float32(y), Z: float32(t.Heights[k])}.Mul(t.Scale)
}

// rayTriangle is the two-sided Möller–Trumbore test (UE2-Studio's
// ray_triangle_distance): the distance along dir to triangle abc, false
// when the ray misses it, runs parallel to it or meets it behind origin.
func rayTriangle(origin, dir, a, b, c geom.Vec3) (float32, bool) {
	ab, ac := b.Sub(a), c.Sub(a)
	across := dir.Cross(ac)
	det := ab.Dot(across)
	if math.Abs(float64(det)) < epsilon {
		return 0, false
	}
	inv := 1 / det
	toOrigin := origin.Sub(a)
	u := toOrigin.Dot(across) * inv
	if u < 0 || u > 1 {
		return 0, false
	}
	sideways := toOrigin.Cross(ab)
	v := dir.Dot(sideways) * inv
	if v < 0 || u+v > 1 {
		return 0, false
	}
	d := ac.Dot(sideways) * inv
	return d, d >= 0
}

// epsilon is f32::EPSILON, the determinant below which a triangle is
// parallel to the ray or degenerate.
const epsilon = 1.1920929e-07

func clampInt(v, lo, hi int) int { return min(max(v, lo), hi) }
