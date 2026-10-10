package model

import (
	"bytes"
	"errors"
	"image"
	"math"
	"testing"

	"zonebuilder/internal/geom"
)

// Catches any field of the UE2HUM01 format read or written wrong: a single
// misplaced byte, a dropped field or a lossy re-encode changes the output.
func TestHumanRoundTripsByteForByte(t *testing.T) {
	b, err := Decode(Human)
	if err != nil {
		t.Fatalf("Decode(Human): %v", err)
	}
	if got := Encode(b); !bytes.Equal(got, Human) {
		t.Fatalf("Encode(Decode(Human)) differs: %d bytes, want %d", len(got), len(Human))
	}
}

// Catches a partial read accepted as a bundle: a decoder that stops at the
// last part without checking for EOF, or reads past a short buffer.
func TestDecodeRefusesTruncatedAndTrailingBytes(t *testing.T) {
	for name, data := range map[string][]byte{
		"truncated":     Human[:len(Human)-1],
		"trailing byte": append(bytes.Clone(Human), 0),
	} {
		if _, err := Decode(data); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: Decode error = %v, want ErrInvalid", name, err)
		}
	}
}

// Catches a wrong palette or inverse bind (missing base or placement, wrong
// composition order, unconjugated root): the human no longer stands on Z = 0
// at the 80-unit height UE2-Studio's extractor scaled it to.
func TestHumanIdleFrameZeroStandsOnGroundAt80(t *testing.T) {
	b, err := Decode(Human)
	if err != nil {
		t.Fatal(err)
	}
	a := NewAnimator(b)
	a.Sample(0, 0, 1)
	minZ, maxZ := float32(math.Inf(1)), float32(math.Inf(-1))
	for i, part := range b.Parts {
		positions := make([]geom.Vec3, len(part.Vertices))
		a.Skin(i, positions, nil)
		for _, p := range positions {
			minZ, maxZ = min(minZ, p.Z), max(maxZ, p.Z)
		}
	}
	if math.Abs(float64(minZ)) > 0.5 || math.Abs(float64(maxZ-minZ-80)) > 1 {
		t.Fatalf("idle frame 0: min Z %v, height %v; want 0 ± 0.5 and 80 ± 1", minZ, maxZ-minZ)
	}
}

// twoBones is a bundle built from scratch: root at the origin, arm 10 units
// along +X, a vertex at (20, 0, 0) on the arm, and a part base turning the
// whole mesh 90° about Z and lifting it 5 units. Clips: 0 rest; 1 bends the
// arm 90° about Y at frame 1 (key times 0 and 1); 2 holds the bend.
func twoBones(t *testing.T) *Bundle {
	t.Helper()
	s := float32(math.Sqrt2 / 2)
	bent := Quat{Y: s, W: s} // ToAxis rows (0,0,1) (0,1,0) (-1,0,0): +X → +Z
	still := Track{KeyQuat: []Quat{IdentityQuat}, KeyPos: []geom.Vec3{{}}}
	armAt := func(q ...Quat) Track {
		times := []float32{0, 1}[:len(q)]
		return Track{KeyQuat: q, KeyPos: []geom.Vec3{{X: 10}}, KeyTime: times}
	}
	skin := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	skin.Pix = []byte{200, 100, 50, 255} // opaque
	b := &Bundle{
		Placement: IdentityAffine,
		JumpClips: [2]uint32{1, 2},
		Names:     []string{"ROOT", "Arm"},
		Clips: []Clip{
			{Frames: 2, Rate: 10, Looping: true, Tracks: []Track{still, armAt(IdentityQuat)}},
			{Frames: 2, Rate: 10, Tracks: []Track{still, armAt(IdentityQuat, bent)}},
			{Frames: 2, Rate: 10, Looping: true, Tracks: []Track{still, armAt(bent)}},
		},
		Parts: []Part{{
			Base: Affine{Origin: geom.Vec3{Z: 5}, Axis: [3]geom.Vec3{{Y: -1}, {X: 1}, {Z: 1}}},
			Bones: []Bone{
				{Name: "root", Parent: NoParent, Orientation: IdentityQuat},
				{Name: "arm", Parent: 0, Position: geom.Vec3{X: 10}, Orientation: IdentityQuat},
			},
			InverseBind: []Affine{IdentityAffine, {Origin: geom.Vec3{X: -10}, Axis: IdentityAffine.Axis}},
			Vertices: []Vertex{{
				Position: geom.Vec3{X: 20}, Normal: geom.Vec3{X: 1},
				Bones: [4]uint16{1, 0, 0, 0}, Weights: [4]float32{1, 0, 0, 0},
			}},
			Indices:  []uint32{0, 0, 0},
			Sections: []Section{{IndexCount: 3, Texture: "Test.Skin", PNG: EncodePNG(skin)}},
		}},
	}
	// A bundle built from scratch must survive Encode for zbmodel; an opaque
	// skin written by image/png would be RGB and refused.
	decoded, err := Decode(Encode(b))
	if err != nil {
		t.Fatalf("Decode(Encode(from scratch)): %v", err)
	}
	return decoded
}

func skinOne(a *Animator) geom.Vec3 {
	p := make([]geom.Vec3, 1)
	a.Skin(0, p, nil)
	return p[0]
}

func near(a, b geom.Vec3) bool { return a.Sub(b).Length() < 1e-3 }

// Catches wrong key interpolation, quaternion convention or composition
// order (parent ∘ local ∘ inverse bind, base first): the arm vertex lands
// somewhere else. Expected points are worked by hand from the ToAxis rows.
func TestTwoBoneRotationAtFrameOneMovesVertex(t *testing.T) {
	a := NewAnimator(twoBones(t))
	rest := geom.Vec3{X: 0, Y: -20, Z: 5} // (20,0,0) through the base
	if got := skinOne(a); !near(got, rest) {
		t.Fatalf("rest pose vertex %v, want %v", got, rest)
	}
	a.Sample(1, 1, 1)
	// Arm bent: joint (10,0,0) + 10·(0,0,1) = (10,0,10); base → (0,-10,15).
	if got, want := skinOne(a), (geom.Vec3{X: 0, Y: -10, Z: 15}); !near(got, want) {
		t.Fatalf("frame 1 vertex %v, want %v", got, want)
	}
	a.Sample(1, 0.5, 1)
	// Halfway between keys: 45° → (10+10·cos45, 0, 10·sin45) → base.
	c := float32(10 * math.Sqrt2 / 2)
	if got, want := skinOne(a), (geom.Vec3{X: 0, Y: -10 - c, Z: 5 + c}); !near(got, want) {
		t.Fatalf("frame 0.5 vertex %v, want %v", got, want)
	}
}

// Catches a transition that snaps, blends skinned positions instead of bone
// rotations, or ignores the smoothstep timing: halfway through the 0.2 s blend
// from rest into the held bend, the arm is at 45°, between the 2 poses.
func TestTransitionHalfwayIsBetweenPoses(t *testing.T) {
	a := NewAnimator(twoBones(t))
	a.Update(2, TransitionSeconds/2)
	c := float32(10 * math.Sqrt2 / 2)
	if got, want := skinOne(a), (geom.Vec3{X: 0, Y: -10 - c, Z: 5 + c}); !near(got, want) {
		t.Fatalf("halfway vertex %v, want %v (rest (0,-20,5), bent (0,-10,15))", got, want)
	}
	a.Update(2, TransitionSeconds/2)
	if got, want := skinOne(a), (geom.Vec3{X: 0, Y: -10, Z: 15}); !near(got, want) {
		t.Fatalf("after transition vertex %v, want %v", got, want)
	}
}
