package play_test

import (
	"math"
	"testing"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/play"
)

// frame is the step of a 60 Hz frame.
const frame = float32(1) / 60

// quad is the 2 triangles of the quad a, b, c, d.
func quad(a, b, c, d geom.Vec3) []play.Triangle {
	return []play.Triangle{{a, b, c}, {a, c, d}}
}

// ground is a 4000-unit square at height z.
func ground(z float32) []play.Triangle {
	return quad(
		geom.Vec3{X: -2000, Y: -2000, Z: z}, geom.Vec3{X: 2000, Y: -2000, Z: z},
		geom.Vec3{X: 2000, Y: 2000, Z: z}, geom.Vec3{X: -2000, Y: 2000, Z: z},
	)
}

// run steps the session n times with the same input.
func run(s *play.Session, n int, in play.Input) {
	for range n {
		s.Step(in)
	}
}

// A capsule dropped from above a plane ends standing on it. Catches an
// unresolved penetration: gravity sinking the capsule into the floor.
func TestDroppedCapsuleStandsOnThePlane(t *testing.T) {
	w := play.NewWorld(ground(100))
	s := play.NewSession(w, geom.Vec3{X: 0, Y: 0, Z: 600}, 0, 0)
	// Floor under the start: the session starts on it, so lift it by
	// flying up and drop it from 500 units over the plane.
	run(s, 1, play.Input{ToggleFlight: true})
	run(s, 30, play.Input{Seconds: frame, Up: 1})
	if s.Feet().Z < 500 {
		t.Fatalf("flight lifted the feet only to %v", s.Feet().Z)
	}
	run(s, 1, play.Input{ToggleFlight: true})
	run(s, 240, play.Input{Seconds: frame})
	if z := s.Feet().Z; z < 100 || z > 100.5 {
		t.Fatalf("feet rest at %v, want on the plane at 100", z)
	}
	if s.Motion() != play.Grounded {
		t.Fatalf("motion %v, want Grounded", s.Motion())
	}
	if v := s.Velocity(); math.Abs(float64(v.Z)) > 1 {
		t.Fatalf("vertical velocity %v at rest", v.Z)
	}
}

// wall is a vertical quad across the X axis at x, from y -500 to 500 and z
// 0 to 1000.
func wall(x float32) []play.Triangle {
	return quad(
		geom.Vec3{X: x, Y: -500, Z: 0}, geom.Vec3{X: x, Y: 500, Z: 0},
		geom.Vec3{X: x, Y: 500, Z: 1000}, geom.Vec3{X: x, Y: -500, Z: 1000},
	)
}

// A capsule running into a wall stops against it. Catches an unresolved
// penetration: the capsule walking into or through the wall.
func TestCapsuleWalkingIntoAWallStopsAtIt(t *testing.T) {
	w := play.NewWorld(append(ground(0), wall(300)...))
	s := play.NewSession(w, geom.Vec3{Z: play.EyeHeight}, 0, 0)
	run(s, 180, play.Input{Seconds: frame, Forward: 1, Fast: true})
	// The capsule's radius is 16: its axis stops 16 units from the wall.
	if x := s.Feet().X; x < 283 || x > 284 {
		t.Fatalf("feet stop at x %v, want against the wall at 284", x)
	}
	if z := s.Feet().Z; z < 0 || z > 0.5 {
		t.Fatalf("feet left the ground: z %v", z)
	}
}

// ramp rises along +X from x 100 at degrees above the horizon, 1000 units
// up, 1000 units wide.
func ramp(degrees float64) []play.Triangle {
	rad := degrees * math.Pi / 180
	run := float32(1000 / math.Tan(rad))
	return quad(
		geom.Vec3{X: 100, Y: -500, Z: 0}, geom.Vec3{X: 100, Y: 500, Z: 0},
		geom.Vec3{X: 100 + run, Y: 500, Z: 1000}, geom.Vec3{X: 100 + run, Y: -500, Z: 1000},
	)
}

// walkUp runs the capsule into the ramp for 4 seconds and gives the height
// its feet reach.
func walkUp(degrees float64) float32 {
	w := play.NewWorld(append(ground(0), ramp(degrees)...))
	s := play.NewSession(w, geom.Vec3{Z: play.EyeHeight}, 0, 0)
	run(s, 240, play.Input{Seconds: frame, Forward: 1})
	return s.Feet().Z
}

// A 30° ramp is walked up and a 60° one is not: the capsule slides back
// down. Catches a wrong ground limit (n.z > 0.65): every slope walkable,
// or none.
func TestThirtyDegreeRampIsClimbedAndSixtyIsNot(t *testing.T) {
	if z := walkUp(30); z < 300 {
		t.Fatalf("30° ramp: feet reach only %v", z)
	}
	if z := walkUp(60); z > 20 {
		t.Fatalf("60° ramp: feet climb to %v", z)
	}
}

// A jump rises v²/2g = 360²/1960 ≈ 66 units and lands back on the ground,
// and running over a low step never launches the capsule: the step is
// climbed and the capsule stays on the ground. Catches a missing clamp of
// the upward velocity a rounded contact against the step's edge produces.
func TestJumpRisesAndLandsAndAStepDoesNotLaunch(t *testing.T) {
	w := play.NewWorld(ground(0))
	s := play.NewSession(w, geom.Vec3{Z: play.EyeHeight}, 0, 0)
	run(s, 1, play.Input{Seconds: frame, Jump: true})
	if s.Motion() != play.Jumping {
		t.Fatalf("motion %v after the jump, want Jumping", s.Motion())
	}
	top := s.Feet().Z
	for range 120 {
		s.Step(play.Input{Seconds: frame})
		top = max(top, s.Feet().Z)
	}
	if top < 62 || top > 70 {
		t.Fatalf("jump peaked at %v, want about 66", top)
	}
	if z := s.Feet().Z; z < 0 || z > 0.5 || s.Motion() != play.Grounded {
		t.Fatalf("after the jump: feet z %v, motion %v; want back on the ground", z, s.Motion())
	}

	// An 8-unit step from x 0 to 200: its riser and its top.
	step := quad(geom.Vec3{X: 0, Y: -100, Z: 0}, geom.Vec3{X: 0, Y: 100, Z: 0},
		geom.Vec3{X: 0, Y: 100, Z: 8}, geom.Vec3{X: 0, Y: -100, Z: 8})
	step = append(step, quad(geom.Vec3{X: 0, Y: -100, Z: 8}, geom.Vec3{X: 0, Y: 100, Z: 8},
		geom.Vec3{X: 200, Y: 100, Z: 8}, geom.Vec3{X: 200, Y: -100, Z: 8})...)
	for _, fast := range []bool{false, true} {
		w := play.NewWorld(append(ground(0), step...))
		s := play.NewSession(w, geom.Vec3{X: -50, Z: play.EyeHeight}, 0, 0)
		for i := range 75 {
			s.Step(play.Input{Seconds: frame, Forward: 1, Fast: fast})
			if z := s.Feet().Z; z > 8.2 {
				t.Fatalf("fast %v, frame %d: the step launched the feet to z %v", fast, i, z)
			}
			if m := s.Motion(); m != play.Grounded {
				t.Fatalf("fast %v, frame %d: motion %v on the step", fast, i, m)
			}
		}
		if x := s.Feet().X; x < 240 {
			t.Fatalf("fast %v: did not cross the step, x %v", fast, x)
		}
	}
}

// Flight crosses a wall the capsule could not walk through. Catches
// collision applied in flight, which would trap a capsule that flies out of
// a hole.
func TestFlightCrossesTheWall(t *testing.T) {
	w := play.NewWorld(append(ground(0), wall(300)...))
	s := play.NewSession(w, geom.Vec3{Z: play.EyeHeight}, 0, 0)
	run(s, 1, play.Input{ToggleFlight: true})
	if s.Motion() != play.Flying {
		t.Fatalf("motion %v after F, want Flying", s.Motion())
	}
	// Pitch the view to the horizon so the flight stays at the wall's height.
	_, pitch := s.View()
	run(s, 1, play.Input{LookY: pitch / camera.LookSensitivity})
	if _, p := s.View(); math.Abs(float64(p)) > 1e-4 {
		t.Fatalf("view pitch %v, want level", p)
	}
	run(s, 60, play.Input{Seconds: frame, Forward: 1})
	if x := s.Feet().X; x < 800 {
		t.Fatalf("flight stopped at x %v, want past the wall at 300", x)
	}
}

// The camera stands 240 units behind the eye, and a wall behind the capsule
// pulls it in to 6 units (the swept sphere's radius) in front of the wall.
// Catches a camera arm that ignores collision and shows the scene from
// behind the wall.
func TestCameraArmShortensBehindAWall(t *testing.T) {
	open := play.NewSession(play.NewWorld(ground(0)), geom.Vec3{Z: play.EyeHeight}, 0, 0)
	if d := open.Camera().Sub(open.Eye()).Length(); math.Abs(float64(d-240)) > 0.5 {
		t.Fatalf("open arm is %v long, want 240", d)
	}
	if open.Camera().X > -200 {
		t.Fatalf("camera at %v, want behind the eye looking along +X", open.Camera())
	}

	w := play.NewWorld(append(ground(0), wall(-100)...))
	s := play.NewSession(w, geom.Vec3{Z: play.EyeHeight}, 0, 0)
	run(s, 30, play.Input{Seconds: frame})
	if x := s.Camera().X; x < -94.5 || x > -93.5 {
		t.Fatalf("camera at x %v, want 6 units in front of the wall at -100", x)
	}
	if !s.BodyVisible() {
		t.Fatal("body hidden with the camera 94 units away")
	}
}

func TestCollisionReplacementPreservesPlayerAndUsesLoadedGeometry(t *testing.T) {
	for _, flying := range []bool{false, true} {
		s := play.NewSession(play.NewWorld(ground(0)), geom.Vec3{Z: play.EyeHeight}, 0, -0.2)
		s.Step(play.Input{Seconds: frame, ToggleFlight: flying, Jump: !flying, Forward: 1})
		feet, velocity, motion := s.Feet(), s.Velocity(), s.Motion()
		yaw, pitch := s.View()
		s.ReplaceWorld(play.NewWorld(append(ground(0), wall(300)...)))
		y, p := s.View()
		if s.Feet() != feet || s.Velocity() != velocity || s.Motion() != motion || y != yaw || p != pitch {
			t.Fatal("rebuilding collision reset the player")
		}
		// Flight intentionally bypasses collision; return to walking before
		// checking that the newly loaded wall stops the capsule.
		if flying {
			s.Step(play.Input{ToggleFlight: true})
		}
		run(s, 180, play.Input{Seconds: frame, Forward: 1})
		if x := s.Feet().X; x > 284 {
			t.Fatalf("player crossed newly loaded wall: x=%v", x)
		}
	}
}
