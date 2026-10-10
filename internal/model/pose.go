package model

import (
	"math"

	"zonebuilder/internal/geom"
)

// Port of UE2-Studio crates/animation-engine/src/pose.rs: track binding, key
// sampling and palette composition.

// bindTracks gives each bone the first track whose name matches it ASCII
// case-insensitively, or -1. Indices past u16 stay unbound, as in BoundAnim.
func bindTracks(bones []Bone, names []string) []int32 {
	tracks := make([]int32, len(bones))
	for i, bone := range bones {
		tracks[i] = -1
		for t, name := range names {
			if t > math.MaxUint16 {
				break
			}
			if asciiEqualFold(name, bone.Name) {
				tracks[i] = int32(t)
				break
			}
		}
	}
	return tracks
}

func bound(tracks []int32) int {
	n := 0
	for _, t := range tracks {
		if t >= 0 {
			n++
		}
	}
	return n
}

// asciiEqualFold is Rust's eq_ignore_ascii_case (strings.EqualFold also folds
// non-ASCII letters).
func asciiEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

// local is one bone's bone-local translation and rotation, before the root
// conjugation.
type local struct {
	position geom.Vec3
	rotation Quat
}

// sampleTrack is one bone's animated local transform at frame (frame units),
// CAnimTrack::GetBonePosition's branch order. Empty or non-finite channels
// fall back to the bind pose.
func sampleTrack(t *Track, bone *Bone, numFrames, frame float32, looping bool) local {
	if len(t.KeyTime) == 1 || numFrames <= 1 || frame == 0 {
		pos, rot := bone.Position, bone.Orientation
		if len(t.KeyPos) > 0 {
			pos = t.KeyPos[0]
		}
		if len(t.KeyQuat) > 0 {
			rot = t.KeyQuat[0]
		}
		return local{finitePosition(pos, bone.Position), finiteOrientation(rot, bone.Orientation)}
	}
	var timed *blend
	if len(t.KeyTime) > 0 {
		b := timedBlend(t.KeyTime, numFrames, frame, looping)
		timed = &b
	}
	pos := bone.Position
	switch n := len(t.KeyPos); n {
	case 0:
	case 1:
		pos = t.KeyPos[0]
	default:
		b := blendFor(timed, n, numFrames, frame, looping)
		from, to := t.KeyPos[b.x], t.KeyPos[b.y]
		pos = from.Add(to.Sub(from).Scale(b.f))
	}
	rot := bone.Orientation
	switch n := len(t.KeyQuat); n {
	case 0:
	case 1:
		rot = t.KeyQuat[0]
	default:
		b := blendFor(timed, n, numFrames, frame, looping)
		rot = t.KeyQuat[b.x].slerp(t.KeyQuat[b.y], b.f)
	}
	return local{finitePosition(pos, bone.Position), finiteOrientation(rot, bone.Orientation)}
}

func finite(f float32) bool { return !math.IsNaN(float64(f)) && !math.IsInf(float64(f), 0) }

func finitePosition(sampled, bind geom.Vec3) geom.Vec3 {
	if finite(sampled.X) && finite(sampled.Y) && finite(sampled.Z) {
		return sampled
	}
	return bind
}

func finiteOrientation(sampled, bind Quat) Quat {
	switch {
	case usableQuat(sampled):
		return sampled
	case usableQuat(bind):
		return bind
	}
	return IdentityQuat
}

func usableQuat(q Quat) bool {
	l := q.dot(q)
	return finite(l) && l > minPositive
}

// blend is a key pair and the fraction between them.
type blend struct {
	x, y int
	f    float32
}

func blendFor(timed *blend, count int, numFrames, frame float32, looping bool) blend {
	if timed == nil {
		return uniformBlend(count, numFrames, frame, looping)
	}
	last := count - 1
	return blend{min(timed.x, last), min(timed.y, last), timed.f}
}

func timedBlend(keyTime []float32, numFrames, frame float32, looping bool) blend {
	last := len(keyTime) - 1
	low, high := 0, last
	for low+1 < high {
		mid := (low + high) / 2
		if frame < keyTime[mid] {
			high = mid
		} else {
			low = mid
		}
	}
	x := low
	for x < last && frame >= keyTime[x+1] {
		x++
	}
	switch y := x + 1; {
	case y <= last:
		return blend{x, y, fraction(frame-keyTime[x], keyTime[y]-keyTime[x])}
	case looping:
		return blend{x, 0, fraction(frame-keyTime[x], numFrames-keyTime[x])}
	default:
		return blend{x, last, 0}
	}
}

func uniformBlend(count int, numFrames, frame float32, looping bool) blend {
	last := count - 1
	position := frame / numFrames * float32(count)
	x := min(int(max(position, 0)), last)
	f := min(max(position-float32(x), 0), 1)
	switch y := x + 1; {
	case y <= last:
		return blend{x, y, f}
	case looping:
		return blend{x, 0, f}
	default:
		return blend{x, last, 0}
	}
}

func fraction(numerator, denominator float32) float32 {
	if denominator > 0 {
		return numerator / denominator
	}
	return 0
}

// compose fills palette with base ∘ bone model space ∘ inverse bind, bones in
// index order so parents come first. Bone 0's rotation is conjugated
// (SkelMeshInstance.cpp:655); no other bone's is.
func compose(palette []Affine, bones []Bone, inverseBind []Affine, base Affine, locals []local) {
	for i := range bones {
		rot := locals[i].rotation
		if i == 0 {
			rot = rot.conjugate()
		}
		l := newAffine(locals[i].position, rot)
		if p := int(bones[i].Parent); bones[i].Parent != NoParent && p < i {
			palette[i] = palette[p].Compose(l)
		} else {
			palette[i] = base.Compose(l)
		}
	}
	for i := range palette {
		inverse := IdentityAffine
		if i < len(inverseBind) {
			inverse = inverseBind[i]
		}
		palette[i] = palette[i].Compose(inverse)
	}
}
