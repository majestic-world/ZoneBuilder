package play

import (
	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
)

const (
	// cameraArm is the third-person camera's distance behind the eye.
	cameraArm = 240
	// bodyClearance is how close to the eye the camera may come before the
	// body is hidden: a wall forcing the camera into the head.
	bodyClearance = 28
	// The pitch range the third-person view starts in.
	minStartPitch = -0.45
	maxStartPitch = -0.12
)

// Input is one time step of commands.
type Input struct {
	// Seconds is the frame time; steps longer than 0.1 s are cut to it.
	Seconds float32
	// Forward (W−S), Right (D−A) and Up (E−Q, only in flight) are -1..1.
	Forward, Right, Up float32
	// Fast doubles the speed (Shift held).
	Fast bool
	// Jump and ToggleFlight are key presses (Space, F), not key state.
	Jump, ToggleFlight bool
	// LookX and LookY are a mouse drag in pixels, as camera.Camera.Look.
	LookX, LookY float32
}

// Session is a capsule walking a World, seen by a third-person camera.
type Session struct {
	world      *World
	player     player
	yaw, pitch float32
	camera     geom.Vec3
	body       bool
}

// NewSession stands the capsule on the floor under eye, or flying at eye
// when there is no floor, looking along yaw and pitch (radians, with
// camera.Camera's convention: in client axes the view direction is
// (cos yaw·cos pitch, sin yaw·cos pitch, sin pitch)). The pitch is brought
// into the third-person range.
func NewSession(w *World, eye geom.Vec3, yaw, pitch float32) *Session {
	s := &Session{
		world:  w,
		player: newPlayer(w, eye),
		yaw:    yaw,
		pitch:  min(max(pitch, minStartPitch), maxStartPitch),
	}
	s.positionCamera()
	return s
}

// ReplaceWorld installs rebuilt collision without resetting movement,
// position, flight, velocity or view.
func (s *Session) ReplaceWorld(w *World) {
	s.world = w
	s.positionCamera()
}

// Step runs one time step: flight toggle, jump, look, then the move along
// the view (horizontal on foot, the full view direction in flight).
func (s *Session) Step(in Input) {
	p := &s.player
	if in.ToggleFlight {
		p.toggleFlight()
	}
	if in.Jump {
		p.jump()
	}
	c := camera.Camera{Yaw: s.yaw, Pitch: s.pitch}
	c.Look(in.LookX, in.LookY)
	s.yaw, s.pitch = c.Yaw, c.Pitch
	view := s.forward()
	forward := geom.Vec3{X: view.X, Y: view.Y}.Normalize()
	right := geom.Vec3{X: -forward.Y, Y: forward.X}
	if p.flying {
		forward = view
	}
	wish := forward.Scale(in.Forward).Add(right.Scale(in.Right))
	if p.flying {
		wish.Z += in.Up
	}
	p.advance(s.world, in.Seconds, wish, in.Fast)
	s.positionCamera()
}

// forward is the unit view direction in client axes.
func (s *Session) forward() geom.Vec3 {
	f := (&camera.Camera{Yaw: s.yaw, Pitch: s.pitch}).Forward()
	return geom.Vec3{X: f.X, Y: f.Z, Z: f.Y}
}

// positionCamera puts the camera 240 units behind the eye along the view,
// pulled in where a sphere of radius 6 swept from the eye hits a triangle.
func (s *Session) positionCamera() {
	eye := s.player.eye()
	s.camera = s.world.CameraPosition(eye, eye.Sub(s.forward().Scale(cameraArm)))
	s.body = s.camera.Sub(eye).Length() > bodyClearance
}

// Feet is the bottom of the capsule.
func (s *Session) Feet() geom.Vec3 { return s.player.feet }

// Eye is the point the camera orbits, 64 units above the feet.
func (s *Session) Eye() geom.Vec3 { return s.player.eye() }

// Velocity is in units per second; in flight it is zero (flight moves the
// capsule straight from the input).
func (s *Session) Velocity() geom.Vec3 { return s.player.velocity }

// Motion is the capsule's state.
func (s *Session) Motion() Motion { return s.player.motion() }

// Camera is the third-person camera's position; it looks along View.
func (s *Session) Camera() geom.Vec3 { return s.camera }

// View is the camera's yaw and pitch, in camera.Camera's convention.
func (s *Session) View() (yaw, pitch float32) { return s.yaw, s.pitch }

// BodyVisible is false when a wall has pulled the camera into the head.
func (s *Session) BodyVisible() bool { return s.body }
