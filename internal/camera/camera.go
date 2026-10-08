// Package camera is UE2-Studio's fly camera (src/camera.rs), in the
// renderer's Y-up basis and in rebased coordinates: the scene's rebase
// origin is the render-space origin.
package camera

import (
	"math"

	"zonebuilder/internal/geom"
)

// Constants from UE2-Studio's camera.rs.
const (
	// LookSensitivity is radians of turn per pixel of mouse drag.
	LookSensitivity = 0.002
	// liftSpeedScale is the share of the flight speed one vertical drag
	// pixel lifts the camera by.
	liftSpeedScale = 0.01
	// pitchLimit stops short of straight up/down, where the view basis
	// degenerates.
	pitchLimit = 1.52
	// FastMultiplier is the speed gear while Shift is held.
	FastMultiplier = 8
	// WheelStep is the share of the flight speed one wheel notch flies
	// forward.
	WheelStep = 0.35
	// FovY is the vertical field of view.
	FovY = 55 * math.Pi / 180
	// Near is the near plane; reversed Z keeps it at one unit.
	Near           = 1.0
	minFar         = 250_000
	sceneFraming   = 0.8
	crossSeconds   = 5
	maxNormalSpeed = 1200
	minExtent      = 2000
)

// framingDirection is where the opening camera is parked from the scene's
// centre: up and to one side, looking back down.
var framingDirection = geom.Vec3{X: 0.72, Y: 0.65, Z: 0.72}

// Camera is a fly camera: a position, a yaw around Y and a pitch above the
// horizon.
type Camera struct {
	Position   geom.Vec3
	Yaw, Pitch float32
	// Speed is the flight speed in world units per second.
	Speed float32
	// Far is the far plane, derived from the framed scene.
	Far float32
}

// ForBounds frames a render-space box: parked one scene extent out along
// the (+X, +Y, +Z) diagonal from its centre and aimed back at it, with a
// speed that crosses the scene in five seconds (at most 1200 units/s).
func ForBounds(b geom.Box) Camera {
	var centre geom.Vec3
	if !b.Empty() {
		centre = b.Center()
	}
	extent := float32(minExtent)
	if !b.Empty() {
		s := b.Size()
		extent = max(s.X, s.Z, minExtent)
	}
	off := framingDirection.Scale(extent * sceneFraming)
	horizontal := float32(math.Hypot(float64(off.X), float64(off.Z)))
	return Camera{
		Position: centre.Add(off),
		Yaw:      atan2(-off.Z, -off.X),
		Pitch:    atan2(-off.Y, horizontal),
		Speed:    min(extent/crossSeconds, maxNormalSpeed),
		Far:      max((off.Length()+extent)*1.5, minFar),
	}
}

// Look turns the camera by a mouse delta in pixels.
func (c *Camera) Look(dx, dy float32) {
	c.Yaw -= dx * LookSensitivity
	c.Pitch = min(max(c.Pitch-dy*LookSensitivity, -pitchLimit), pitchLimit)
}

// Lift moves the camera vertically by a mouse delta in pixels: dragging up
// raises it.
func (c *Camera) Lift(dy float32) {
	c.Position.Y -= dy * c.Speed * liftSpeedScale
}

// Fly moves the camera. Each axis is a signed duration in seconds (key
// state times frame time): forward follows the full view direction, right
// stays horizontal, up is world up. fast selects the Shift gear.
func (c *Camera) Fly(forward, right, up float32, fast bool) {
	d := c.Speed
	if fast {
		d *= FastMultiplier
	}
	f := c.Forward()
	h := geom.Vec3{X: f.X, Z: f.Z}.Normalize()
	side := geom.Vec3{X: -h.Z, Z: h.X}
	c.Position = c.Position.Add(f.Scale(forward * d)).Add(side.Scale(right * d))
	c.Position.Y += up * d
}

// Forward is the unit view direction.
func (c *Camera) Forward() geom.Vec3 {
	cy, sy := cos(c.Yaw), sin(c.Yaw)
	cp, sp := cos(c.Pitch), sin(c.Pitch)
	return geom.Vec3{X: cy * cp, Y: sp, Z: sy * cp}
}

// Basis is the camera's right and true up, the pair a right-handed look-at
// derives from Forward and world up.
func (c *Camera) Basis() (right, up geom.Vec3) {
	f := c.Forward()
	right = f.Cross(geom.Vec3{Y: 1}).Normalize()
	return right, right.Cross(f)
}

// Ray is the view ray through pixel (x, y) of a width×height viewport
// (origin top-left, y down): it starts at the camera and its direction is
// a unit vector, both in the camera's rebased render space. It is the exact
// inverse of the renderer's projection (same FovY, aspect width/height and
// basis), as in UE2-Studio's camera.rs ray.
func (c *Camera) Ray(x, y float32, width, height int) (origin, dir geom.Vec3) {
	w, h := float32(max(width, 1)), float32(max(height, 1))
	ndcX := 2*x/w - 1
	ndcY := 1 - 2*y/h
	tan := float32(math.Tan(FovY / 2))
	right, up := c.Basis()
	dir = right.Scale(ndcX * tan * w / h).Add(up.Scale(ndcY * tan)).Add(c.Forward()).Normalize()
	return c.Position, dir
}

// Project is the pixel (origin top-left, y down) of a width×height viewport
// that render-space point p lands on, the inverse of Ray; ok is false when
// p is not in front of the camera.
func (c *Camera) Project(p geom.Vec3, width, height int) (x, y float32, ok bool) {
	w, h := float32(max(width, 1)), float32(max(height, 1))
	d := p.Sub(c.Position)
	depth := d.Dot(c.Forward())
	if depth < Near {
		return 0, 0, false
	}
	tan := float32(math.Tan(FovY / 2))
	right, up := c.Basis()
	ndcX := d.Dot(right) / (depth * tan * w / h)
	ndcY := d.Dot(up) / (depth * tan)
	return (ndcX + 1) / 2 * w, (1 - ndcY) / 2 * h, true
}

func atan2(y, x float32) float32 { return float32(math.Atan2(float64(y), float64(x))) }
func cos(a float32) float32      { return float32(math.Cos(float64(a))) }
func sin(a float32) float32      { return float32(math.Sin(float64(a))) }
