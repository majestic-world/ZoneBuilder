// Package play is the game mode's physics and third-person camera: a port
// of UE2-Studio's Play Map (src/play_map.rs and src/app/play_map.rs) with
// the same constants. Everything is in the client world frame of
// internal/scene: Unreal axes, Z up, unrebased. The Rust original works in
// the renderer's Y-up basis; this port swaps Y and Z throughout.
package play

import (
	"math"

	"zonebuilder/internal/geom"
)

// Triangle is a collision triangle in client world coordinates.
type Triangle [3]geom.Vec3

// leafTriangles is the most triangles a BVH leaf holds.
const leafTriangles = 12

// cameraRadius is the radius of the sphere swept along the camera arm.
const cameraRadius = 6

// World is the collision geometry: a bounding volume hierarchy of
// triangles split at the median centroid of their longest axis.
type World struct {
	triangles []Triangle
	nodes     []node
}

// node is a BVH node over triangles[start:end]; a leaf has left == -1.
type node struct {
	bounds      geom.Box
	start, end  int
	left, right int
}

// NewWorld builds the BVH. It owns tris from then on: it drops the
// triangles with a non-finite corner or a degenerate area and reorders the
// rest. Building a whole map takes a while; call it off the event loop.
func NewWorld(tris []Triangle) *World {
	kept := tris[:0]
	for _, t := range tris {
		if finite(t) && t[1].Sub(t[0]).Cross(t[2].Sub(t[0])).Length() > 1e-6 {
			kept = append(kept, t)
		}
	}
	w := &World{triangles: kept}
	if len(kept) > 0 {
		w.build(0, len(kept))
	}
	return w
}

func finite(t Triangle) bool {
	for _, v := range t {
		for i := range 3 {
			f := float64(v.Axis(i))
			if math.IsNaN(f) || math.IsInf(f, 0) {
				return false
			}
		}
	}
	return true
}

func (w *World) build(start, end int) int {
	bounds := geom.EmptyBox()
	for _, t := range w.triangles[start:end] {
		for _, p := range t {
			bounds.Include(p)
		}
	}
	index := len(w.nodes)
	w.nodes = append(w.nodes, node{bounds: bounds, start: start, end: end, left: -1, right: -1})
	if end-start > leafTriangles {
		e := bounds.Size()
		axis := 2
		if e.X >= e.Y && e.X >= e.Z {
			axis = 0
		} else if e.Y >= e.Z {
			axis = 1
		}
		middle := (start + end) / 2
		selectNth(w.triangles[start:end], middle-start, axis)
		left := w.build(start, middle)
		right := w.build(middle, end)
		w.nodes[index].left, w.nodes[index].right = left, right
	}
	return index
}

// centroidKey is 3 times the triangle's centroid along axis.
func centroidKey(t *Triangle, axis int) float32 {
	return t[0].Axis(axis) + t[1].Axis(axis) + t[2].Axis(axis)
}

// selectNth reorders ts so that ts[n] is the triangle a sort by centroid
// along axis would put there, with no greater key before it and no smaller
// one after it (Hoare quickselect).
func selectNth(ts []Triangle, n, axis int) {
	lo, hi := 0, len(ts)-1
	for lo < hi {
		mid := lo + (hi-lo)/2
		a, b, c := centroidKey(&ts[lo], axis), centroidKey(&ts[mid], axis), centroidKey(&ts[hi], axis)
		pivot := max(min(a, b), min(max(a, b), c))
		i, j := lo, hi
		for i <= j {
			for centroidKey(&ts[i], axis) < pivot {
				i++
			}
			for centroidKey(&ts[j], axis) > pivot {
				j--
			}
			if i <= j {
				ts[i], ts[j] = ts[j], ts[i]
				i++
				j--
			}
		}
		switch {
		case n <= j:
			hi = j
		case n >= i:
			lo = i
		default:
			return
		}
	}
}

// visit calls fn for every triangle in a leaf whose bounds touch box.
func (w *World) visit(index int, box geom.Box, fn func(Triangle)) {
	if index >= len(w.nodes) {
		return
	}
	n := &w.nodes[index]
	if !overlaps(n.bounds, box) {
		return
	}
	if n.left >= 0 {
		w.visit(n.left, box, fn)
		w.visit(n.right, box, fn)
		return
	}
	for _, t := range w.triangles[n.start:n.end] {
		fn(t)
	}
}

func overlaps(a, b geom.Box) bool {
	return a.Min.X <= b.Max.X && a.Max.X >= b.Min.X &&
		a.Min.Y <= b.Max.Y && a.Max.Y >= b.Min.Y &&
		a.Min.Z <= b.Max.Z && a.Max.Z >= b.Min.Z
}

// CameraPosition is how far from anchor towards desired a sphere of radius
// 6 travels before touching a triangle, edges included. Testing the whole
// segment keeps a fast orbit from tunnelling through a thin wall.
func (w *World) CameraPosition(anchor, desired geom.Vec3) geom.Vec3 {
	delta := desired.Sub(anchor)
	if delta.Length() < 0.001 {
		return anchor
	}
	margin := geom.Vec3{X: cameraRadius, Y: cameraRadius, Z: cameraRadius}
	box := geom.EmptyBox()
	for _, p := range [2]geom.Vec3{anchor, desired} {
		box.Include(p.Sub(margin))
		box.Include(p.Add(margin))
	}
	fraction := float32(1)
	w.visit(0, box, func(t Triangle) {
		intersects := func(f float32) bool {
			a, b := closestCapsule(anchor, anchor.Add(delta.Scale(f)), t)
			return a.Sub(b).Length() <= cameraRadius
		}
		if !intersects(fraction) {
			return
		}
		low, high := float32(0), fraction
		for range 12 {
			middle := (low + high) * 0.5
			if intersects(middle) {
				high = middle
			} else {
				low = middle
			}
		}
		fraction = low
	})
	return anchor.Add(delta.Scale(fraction))
}

// Floor is the height of the highest walkable triangle under p, within
// 250000 units.
func (w *World) Floor(p geom.Vec3) (float32, bool) {
	return w.floorBelow(p, 250_000)
}

func (w *World) floorBelow(eye geom.Vec3, reach float32) (float32, bool) {
	bottom := eye.Z - reach
	box := geom.Box{Min: geom.Vec3{X: eye.X, Y: eye.Y, Z: bottom}, Max: eye}
	var best float32
	found := false
	w.visit(0, box, func(t Triangle) {
		n := t[1].Sub(t[0]).Cross(t[2].Sub(t[0])).Normalize()
		if abs(n.Z) < floorNormalZ {
			return
		}
		distance := n.Dot(t[0].Sub(eye)) / -n.Z
		p := geom.Vec3{X: eye.X, Y: eye.Y, Z: eye.Z - distance}
		if distance >= 0 && p.Z >= bottom &&
			closestTriangle(p, t).Sub(p).Length() < 0.1 &&
			(!found || p.Z > best) {
			best, found = p.Z, true
		}
	})
	return best, found
}

// resolve pushes the capsule standing at feet out of the triangles, the
// deepest contact first, for at most 6 iterations, and removes the velocity
// going into each contact. It reports a contact walkable as ground.
//
// A contact on the face of a slope too steep to walk pushes out
// horizontally only. UE2-Studio pushes out along the contact normal there
// too, whose upward share lets a run climb a 60° slope against gravity; the
// horizontal push stops the climb and lets gravity slide the capsule down.
// A walkable contact, on a face or on an edge, pushes out vertically only,
// and stops the fall without a downhill share: pushed out along the contact
// normal, every frame of gravity would move the capsule downhill, and a
// capsule standing still would slide down a slope, or along a ridge, until
// it reached flat ground. Other edge contacts keep the normal push, so the
// rounded bottom still climbs a low step's edge.
func (w *World) resolve(feet, velocity *geom.Vec3) bool {
	grounded := false
	const reach = radius + skin
	pad := geom.Vec3{X: reach, Y: reach, Z: reach}
	for range resolveIterations {
		start := feet.Add(geom.Vec3{Z: radius})
		end := feet.Add(geom.Vec3{Z: Height - radius})
		var depth float32
		var normal geom.Vec3
		found, steep := false, false
		w.visit(0, geom.Box{Min: start.Sub(pad), Max: end.Add(pad)}, func(t Triangle) {
			onBody, onTriangle := closestCapsule(start, end, t)
			offset := onBody.Sub(onTriangle)
			d := offset.Length()
			if d >= reach {
				return
			}
			face := t[1].Sub(t[0]).Cross(t[2].Sub(t[0])).Normalize()
			var n geom.Vec3
			if d > 1e-5 {
				n = offset.Scale(1 / d)
			} else {
				n = face
				if n.Dot(*velocity) > 0 {
					n = n.Scale(-1)
				}
			}
			if !found || reach-d > depth {
				depth, normal, found = reach-d, n, true
				steep = n.Z > 0 && n.Z <= floorNormalZ && abs(n.Dot(face)) > 0.999
			}
		})
		if !found {
			break
		}
		switch {
		case steep:
			horizontal := geom.Vec3{X: normal.X, Y: normal.Y}
			l := horizontal.Length()
			normal, depth = horizontal.Scale(1/l), depth/l
		case normal.Z > floorNormalZ:
			normal, depth = geom.Vec3{Z: 1}, depth/normal.Z
		}
		*feet = feet.Add(normal.Scale(depth))
		if inward := velocity.Dot(normal); inward < 0 {
			*velocity = velocity.Sub(normal.Scale(inward))
		}
		grounded = grounded || normal.Z > floorNormalZ
	}
	return grounded
}

// closestTriangle is the point of triangle t closest to p (Ericson's
// Voronoi region test).
func closestTriangle(p geom.Vec3, t Triangle) geom.Vec3 {
	a, b, c := t[0], t[1], t[2]
	ab, ac, ap := b.Sub(a), c.Sub(a), p.Sub(a)
	d1, d2 := ab.Dot(ap), ac.Dot(ap)
	if d1 <= 0 && d2 <= 0 {
		return a
	}
	bp := p.Sub(b)
	d3, d4 := ab.Dot(bp), ac.Dot(bp)
	if d3 >= 0 && d4 <= d3 {
		return b
	}
	vc := d1*d4 - d3*d2
	if vc <= 0 && d1 >= 0 && d3 <= 0 {
		return a.Add(ab.Scale(d1 / (d1 - d3)))
	}
	cp := p.Sub(c)
	d5, d6 := ab.Dot(cp), ac.Dot(cp)
	if d6 >= 0 && d5 <= d6 {
		return c
	}
	vb := d5*d2 - d1*d6
	if vb <= 0 && d2 >= 0 && d6 <= 0 {
		return a.Add(ac.Scale(d2 / (d2 - d6)))
	}
	va := d3*d6 - d5*d4
	if va <= 0 && d4 >= d3 && d5 >= d6 {
		return b.Add(c.Sub(b).Scale((d4 - d3) / ((d4 - d3) + (d5 - d6))))
	}
	inverse := 1 / (va + vb + vc)
	return a.Add(ab.Scale(vb * inverse)).Add(ac.Scale(vc * inverse))
}

// closestSegments is the closest pair of points of segments ab and cd.
func closestSegments(a, b, c, d geom.Vec3) (geom.Vec3, geom.Vec3) {
	u, v, w := b.Sub(a), d.Sub(c), a.Sub(c)
	aa, bb, cc, dd, ee := u.Dot(u), u.Dot(v), v.Dot(v), u.Dot(w), v.Dot(w)
	denominator := aa*cc - bb*bb
	var s float32
	if denominator > 1e-8 {
		s = clamp01((bb*ee - cc*dd) / denominator)
	}
	t := clamp01((bb*s + ee) / cc)
	s = clamp01((bb*t - dd) / aa)
	return a.Add(u.Scale(s)), c.Add(v.Scale(t))
}

// closestCapsule is the closest pair of points of the segment start-end
// (the capsule's axis) and triangle t: the axis point first.
func closestCapsule(start, end geom.Vec3, t Triangle) (geom.Vec3, geom.Vec3) {
	normal := t[1].Sub(t[0]).Cross(t[2].Sub(t[0]))
	axis := end.Sub(start)
	if denominator := normal.Dot(axis); abs(denominator) > 1e-8 {
		if f := normal.Dot(t[0].Sub(start)) / denominator; f >= 0 && f <= 1 {
			p := start.Add(axis.Scale(f))
			if closestTriangle(p, t).Sub(p).Length() < 1e-4 {
				return p, p
			}
		}
	}
	bestA, bestB := start, closestTriangle(start, t)
	consider := func(a, b geom.Vec3) {
		if d := a.Sub(b); d.Dot(d) < bestA.Sub(bestB).Dot(bestA.Sub(bestB)) {
			bestA, bestB = a, b
		}
	}
	consider(end, closestTriangle(end, t))
	for i := range 3 {
		consider(closestSegments(start, end, t[i], t[(i+1)%3]))
	}
	return bestA, bestB
}

func abs(f float32) float32 { return float32(math.Abs(float64(f))) }

// clamp01 clamps f to [0, 1]; NaN, as Rust's clamp, stays NaN.
func clamp01(f float32) float32 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}
