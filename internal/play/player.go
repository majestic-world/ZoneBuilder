package play

import (
	"math"

	"zonebuilder/internal/geom"
)

// The capsule and its motion, the constants of UE2-Studio's play_map.rs.
const (
	// EyeHeight is the eye above the feet.
	EyeHeight = 64
	// Height is the capsule from the feet to the top of the head.
	Height = 80
	radius = 16
	// skin is the clearance the capsule keeps from what it touches.
	skin = 0.05

	gravity       = 980
	terminalSpeed = 1800
	jumpSpeed     = 360
	runSpeed      = 260
	flySpeed      = 900
	// fastMultiplier is the speed gear while Shift is held.
	fastMultiplier = 2
	// maxStep is the longest sub-step: small spatial steps keep a fast fall
	// or a slow frame from crossing a thin wall.
	maxStep           = 4
	resolveIterations = 6
	// floorNormalZ is the least normal Z of a walkable surface.
	floorNormalZ = 0.65
	// maxSeconds caps a frame's time step.
	maxSeconds = 0.1
	// respawnDrop is how far below its spawn the capsule falls before it is
	// put back there: leaving the map must not strand it below everything.
	respawnDrop = 20_000

	// Animation tolerances, independent of the collision contacts: a
	// transient contact loss on a stair must not select the falling clip.
	fallAnimationDelay       = 0.18
	groundAnimationClearance = 16
)

// Motion is the capsule's state, which picks the body's animation.
type Motion int

const (
	Grounded Motion = iota
	Jumping
	Falling
	Flying
)

func (m Motion) String() string {
	switch m {
	case Grounded:
		return "Grounded"
	case Jumping:
		return "Jumping"
	case Falling:
		return "Falling"
	case Flying:
		return "Flying"
	}
	return "Motion(?)"
}

// player is the capsule: feet at the bottom of the capsule.
type player struct {
	feet, velocity     geom.Vec3
	flying, grounded   bool
	jumpStarted        bool
	unsupportedSeconds float32
	spawn              geom.Vec3
}

// newPlayer stands the capsule on the floor under eye, or leaves it flying
// at eye level when there is no floor.
func newPlayer(w *World, eye geom.Vec3) player {
	floor, ok := w.Floor(eye)
	if !ok {
		floor = eye.Z - EyeHeight
	}
	p := player{
		feet:     geom.Vec3{X: eye.X, Y: eye.Y, Z: floor + skin},
		flying:   !ok,
		grounded: ok,
	}
	w.resolve(&p.feet, &p.velocity)
	p.spawn = p.feet
	return p
}

func (p *player) eye() geom.Vec3 { return p.feet.Add(geom.Vec3{Z: EyeHeight}) }

func (p *player) motion() Motion {
	switch {
	case p.flying:
		return Flying
	case p.jumpStarted && p.velocity.Z > 0:
		return Jumping
	case p.jumpStarted || p.unsupportedSeconds >= fallAnimationDelay:
		return Falling
	}
	return Grounded
}

func (p *player) toggleFlight() {
	p.flying = !p.flying
	p.velocity = geom.Vec3{}
	p.grounded = false
	p.jumpStarted = false
	p.unsupportedSeconds = 0
}

func (p *player) jump() {
	if p.grounded && !p.flying {
		p.velocity.Z = jumpSpeed
		p.grounded = false
		p.jumpStarted = true
	}
}

// advance moves the capsule for seconds towards wish, a direction whose
// length is the share of full speed.
func (p *player) advance(w *World, seconds float32, wish geom.Vec3, fast bool) {
	seconds = min(max(seconds, 0), maxSeconds)
	if wish.Length() > 1 {
		wish = wish.Normalize()
	}
	speed := float32(runSpeed)
	if p.flying {
		speed = flySpeed
	}
	if fast {
		speed *= fastMultiplier
	}
	if p.flying {
		p.feet = p.feet.Add(wish.Scale(speed * seconds))
		return
	}
	p.velocity.Z = max(p.velocity.Z-gravity*seconds, -terminalSpeed)
	moveVelocity := geom.Vec3{X: wish.X * speed, Y: wish.Y * speed, Z: p.velocity.Z}
	steps := max(1, int(math.Ceil(float64(moveVelocity.Length()*seconds/maxStep))))
	p.grounded = false
	for range steps {
		// Contacts constrain this sub-step, not the next walking command.
		// Reapply horizontal input: retaining its previous projection while
		// ascent is clamped would repeatedly erase uphill movement.
		p.velocity.X, p.velocity.Y = moveVelocity.X, moveVelocity.Y
		p.feet = p.feet.Add(p.velocity.Scale(seconds / float32(steps)))
		upwardLimit := max(p.velocity.Z, 0)
		if w.resolve(&p.feet, &p.velocity) {
			p.grounded = true
		}
		// Sliding against a step's rounded capsule contact can turn
		// horizontal speed into an upward launch. The position correction
		// already climbs the step; only a jump supplies ascent.
		p.velocity.Z = min(p.velocity.Z, upwardLimit)
	}
	if p.grounded {
		p.jumpStarted = false
	}
	_, nearFloor := w.floorBelow(p.feet.Add(geom.Vec3{Z: skin}), groundAnimationClearance+skin)
	if p.grounded || nearFloor {
		p.unsupportedSeconds = 0
	} else {
		p.unsupportedSeconds += seconds
	}
	if p.feet.Z < p.spawn.Z-respawnDrop {
		p.feet = p.spawn
		p.velocity = geom.Vec3{}
		p.jumpStarted = false
		p.unsupportedSeconds = 0
	}
}
