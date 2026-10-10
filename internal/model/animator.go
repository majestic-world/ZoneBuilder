package model

import (
	"math"

	"zonebuilder/internal/geom"
)

// TransitionSeconds is how long Update blends from the displayed pose into a
// newly selected clip (UE2-Studio avatar.rs TRANSITION_SECONDS).
const TransitionSeconds = 0.2

// Clip indices of UE2-Studio's human bundle; the jumps are Bundle.JumpClips.
const (
	ClipIdle    = 0
	ClipRun     = 1
	ClipFalling = 2
)

// Advance moves frame forward by seconds at the authored rate: wrapping when
// the clip loops, holding the last frame when it does not.
func (c *Clip) Advance(frame, seconds float32) float32 {
	next := frame + seconds*c.Rate
	if c.Looping {
		r := float32(math.Mod(float64(next), float64(c.Frames)))
		if r < 0 {
			r += float32(c.Frames)
		}
		return r
	}
	return min(next, float32(c.Frames-1))
}

// Animator poses a Bundle and skins its parts on the CPU, in model space: the
// bundle's Placement and each part's Base are applied, so Skin yields Unreal
// Z-up coordinates with the feet on Z = 0, facing +X, ready for an instance
// transform T(x, y, z)·Rz(yaw). It is a port of UE2-Studio's Assets and the
// clip bookkeeping of Avatar::advance. Not safe for concurrent use; the Bundle
// must not change while an Animator uses it.
type Animator struct {
	bundle  *Bundle
	parts   []animatedPart
	clip    int
	frame   float32
	elapsed float32
}

type animatedPart struct {
	base    Affine  // Placement ∘ Base
	tracks  []int32 // track per bone, -1 = bind pose
	locals  []local // last sampled locals, after blending
	from    []local // locals captured by BeginTransition
	palette []Affine
}

// NewAnimator starts at the first frame of clip 0 with no transition pending,
// already posed.
func NewAnimator(b *Bundle) *Animator {
	a := &Animator{bundle: b, parts: make([]animatedPart, len(b.Parts)), elapsed: TransitionSeconds}
	for i := range b.Parts {
		p := &b.Parts[i]
		n := len(p.Bones)
		a.parts[i] = animatedPart{
			base:    b.Placement.Compose(p.Base),
			tracks:  bindTracks(p.Bones, b.Names),
			locals:  make([]local, n),
			from:    make([]local, n),
			palette: make([]Affine, n),
		}
	}
	a.Sample(0, 0, 1)
	return a
}

// Clip is the clip Update last played.
func (a *Animator) Clip() int { return a.clip }

// Update plays clip for seconds more: selecting a different clip captures the
// displayed pose and restarts at frame 0, blending into the new clip over
// TransitionSeconds with a smoothstep; then the pose is sampled. Playback
// follows the clip's own rate.
func (a *Animator) Update(clip int, seconds float32) {
	if clip != a.clip {
		a.BeginTransition()
		a.elapsed, a.frame, a.clip = 0, 0, clip
	}
	a.frame = a.bundle.Clips[clip].Advance(a.frame, seconds)
	a.elapsed = min(a.elapsed+seconds, TransitionSeconds)
	t := a.elapsed / TransitionSeconds
	a.Sample(clip, a.frame, t*t*(3-2*t))
}

// BeginTransition captures the pose last sampled (an interrupted transition
// included) as the pose the next Sample blends from.
func (a *Animator) BeginTransition() {
	for i := range a.parts {
		copy(a.parts[i].from, a.parts[i].locals)
	}
}

// Sample poses every part at frame (frame units) of clip, blended from the
// pose captured by BeginTransition by alpha (0 = captured pose, 1 = clip
// alone). The blend runs on bone-local translation and rotation, before
// composition, so limbs keep their length.
func (a *Animator) Sample(clip int, frame, alpha float32) {
	c := &a.bundle.Clips[clip]
	for i := range a.parts {
		ap, part := &a.parts[i], &a.bundle.Parts[i]
		for k := range part.Bones {
			bone := &part.Bones[k]
			target := local{bone.Position, bone.Orientation}
			if t := ap.tracks[k]; t >= 0 && int(t) < len(c.Tracks) {
				target = sampleTrack(&c.Tracks[t], bone, float32(c.Frames), frame, c.Looping)
			}
			if alpha < 1 {
				from := ap.from[k]
				target = local{
					from.position.Add(target.position.Sub(from.position).Scale(alpha)),
					from.rotation.slerp(target.rotation, alpha),
				}
			}
			ap.locals[k] = target
		}
		compose(ap.palette, part.Bones, part.InverseBind, ap.base, ap.locals)
	}
}

// Skin writes part's vertices in the last sampled pose into positions and,
// when normals is not nil, normals; both must hold len(Parts[part].Vertices).
// Each vertex blends its 4 palette entries by weight and transforms once.
// Normals take the blended basis without renormalizing.
func (a *Animator) Skin(part int, positions, normals []geom.Vec3) {
	palette := a.parts[part].palette
	for i, v := range a.bundle.Parts[part].Vertices {
		var m Affine
		for k, bone := range v.Bones {
			e, w := &palette[bone], v.Weights[k]
			m.Origin = m.Origin.Add(e.Origin.Scale(w))
			for r := range m.Axis {
				m.Axis[r] = m.Axis[r].Add(e.Axis[r].Scale(w))
			}
		}
		positions[i] = m.Point(v.Position)
		if normals != nil {
			normals[i] = m.Vector(v.Normal)
		}
	}
}
