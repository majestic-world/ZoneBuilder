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

// Catches an embedded monster.bin that Decode refuses, or that drifted from
// its constants (regenerated without updating MonsterHeight, or with a
// placement that no longer lifts the feet to Z = 0 in Wait frame 0).
func TestMonsterWaitStandsOnGroundAtItsHeight(t *testing.T) {
	b, err := Decode(Monster)
	if err != nil {
		t.Fatal(err)
	}
	positions := make([]geom.Vec3, len(b.Parts[0].Vertices))
	NewAnimator(b).Skin(0, positions, nil)
	box := geom.EmptyBox()
	for _, p := range positions {
		box.Include(p)
	}
	if math.Abs(float64(box.Min.Z)) > 0.01 || math.Abs(float64(box.Max.Z-box.Min.Z-MonsterHeight)) > 0.01 {
		t.Fatalf("Wait frame 0: min Z %v, height %v; want 0 and MonsterHeight %v", box.Min.Z, box.Max.Z-box.Min.Z, MonsterHeight)
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

// Catches an inverse bind built from the wrong bind pose or inverted wrong:
// the root's stored quaternion not conjugated (the mesh mirrors), the parent
// composed on the wrong side, or the basis not transposed. The root stores
// a 90° turn about Z that the conjugation makes take +X to +Y; its child sits
// 10 units along the root's X, so at model (0, 10, 0). Worked by hand: the
// child's inverse bind takes model (0, 20, 0) to bone-local (10, 0, 0), and
// the root's takes model (0, 1, 0) to local (1, 0, 0).
func TestInverseBindUndoesTheBindPose(t *testing.T) {
	s := float32(math.Sqrt2 / 2)
	bones := []Bone{
		{Name: "root", Parent: NoParent, Orientation: Quat{Z: s, W: s}},
		{Name: "child", Parent: 0, Position: geom.Vec3{X: 10}, Orientation: IdentityQuat},
	}
	inverse := InverseBind(bones)
	if len(inverse) != 2 {
		t.Fatalf("InverseBind gave %d transforms, want 2", len(inverse))
	}
	if got, want := inverse[0].Point(geom.Vec3{Y: 1}), (geom.Vec3{X: 1}); !near(got, want) {
		t.Errorf("root inverse bind of (0,1,0) = %v, want %v", got, want)
	}
	if got, want := inverse[1].Point(geom.Vec3{Y: 20}), (geom.Vec3{X: 10}); !near(got, want) {
		t.Errorf("child inverse bind of (0,20,0) = %v, want %v", got, want)
	}
}

// The same bugs against UE2-Studio's own output: every part of human.bin
// carries the inverse binds skin.rs computed from those bones.
func TestInverseBindMatchesHumanBin(t *testing.T) {
	b, err := Decode(Human)
	if err != nil {
		t.Fatal(err)
	}
	for p, part := range b.Parts {
		for i, got := range InverseBind(part.Bones) {
			want := part.InverseBind[i]
			worst := got.Origin.Sub(want.Origin).Length()
			for k := range got.Axis {
				worst = max(worst, got.Axis[k].Sub(want.Axis[k]).Length())
			}
			if worst > 1e-3 {
				t.Fatalf("part %d bone %d (%s): InverseBind %v, human.bin %v", p, i, part.Bones[i].Name, got, want)
			}
		}
	}
}
