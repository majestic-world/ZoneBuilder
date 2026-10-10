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

// floorRise is the rise per unit run of the steepest walkable slope: the
// most the ground can drop under a sub-step's run without leaving the
// walkable.
var floorRise = float32(math.Sqrt(1-floorNormalZ*floorNormalZ) / floorNormalZ)

// maxRest is the most the capsule's rounded bottom holds its feet above
// the ground under them, on the steepest walkable slope.
const maxRest = radius*(1/floorNormalZ-1) + skin

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

// player is the capsule: feet at the bottom of the capsule. On a slope the
// capsule's rounded bottom touches the ground uphill of its feet and holds
// them rest above the ground under them.
type player struct {
	feet, velocity     geom.Vec3
	rest               float32
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
	p.stand(w.floorBelow(p.feet.Add(geom.Vec3{Z: skin}), groundAnimationClearance+skin))
	return p
}

// stand measures rest from floor, the height of the walkable ground found
// under the feet, if any. A capsule off the ground, or held up by an edge
// past which its feet hang over lower ground, stands on its feet.
func (p *player) stand(floor float32, found bool) {
	p.rest = 0
	if gap := p.feet.Z - floor; p.grounded && found && gap > 0 && gap <= maxRest {
		p.rest = gap
	}
}

// ground is where the body stands: the ground under the feet.
func (p *player) ground() geom.Vec3 { return p.feet.Sub(geom.Vec3{Z: p.rest}) }

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
	p.rest = 0
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
	supported := p.grounded
	p.grounded = false
	for range steps {
		// Contacts constrain this sub-step, not the next walking command.
		// Reapply horizontal input: retaining its previous projection while
		// ascent is clamped would repeatedly erase uphill movement.
		p.velocity.X, p.velocity.Y = moveVelocity.X, moveVelocity.Y
		move := p.velocity.Scale(seconds / float32(steps))
		p.feet = p.feet.Add(move)
		upwardLimit := max(p.velocity.Z, 0)
		ground := w.resolve(&p.feet, &p.velocity)
		if !ground && supported && p.velocity.Z <= 0 {
			ground = p.snapDown(w, geom.Vec3{X: move.X, Y: move.Y}.Length())
		}
		supported = ground
		p.grounded = p.grounded || ground
		// Sliding against a step's rounded capsule contact can turn
		// horizontal speed into an upward launch. The position correction
		// already climbs the step; only a jump supplies ascent.
		p.velocity.Z = min(p.velocity.Z, upwardLimit)
	}
	if p.grounded {
		p.jumpStarted = false
	}
	floor, nearFloor := w.floorBelow(p.feet.Add(geom.Vec3{Z: skin}), groundAnimationClearance+skin)
	if p.grounded || nearFloor {
		p.unsupportedSeconds = 0
	} else {
		p.unsupportedSeconds += seconds
	}
	p.stand(floor, nearFloor)
	if p.feet.Z < p.spawn.Z-respawnDrop {
		p.feet = p.spawn
		p.velocity = geom.Vec3{}
		p.rest = 0
		p.jumpStarted = false
		p.unsupportedSeconds = 0
	}
}

// snapDown keeps a capsule that stood on the ground before a sub-step of
// run units on it: walking down a walkable slope moves the capsule off it
// faster than gravity brings it back, and it would bounce down the slope
// off the ground. The capsule drops as far as the steepest walkable slope
// falls under run; it stays where it is if no ground is there.
func (p *player) snapDown(w *World, run float32) bool {
	feet, velocity := p.feet, p.velocity
	feet.Z -= run*floorRise + 2*skin
	if !w.resolve(&feet, &velocity) {
		return false
	}
	p.feet, p.velocity = feet, velocity
	return true
}
