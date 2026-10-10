package main

import (
	"bytes"
	"math"
	"os"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/model"
)

// deathKnight is the request of the regeneration command in
// internal/model/assets/README.md.
var deathKnight = request{
	Mesh:      "LineageMonsters15.death_knight_wizard_m00",
	Animation: "LineageMonsters15.death_knight_wizard_anim",
	Skins: []string{
		"LineageMonstersTex9.death_knight_wizard.death_knight_wizard_t00",
		"LineageMonstersTex9.death_knight_wizard.death_knight_wizard_t01",
	},
	Clips:     []string{"Wait"},
	DrawScale: 1,
}

func extractDeathKnight(t *testing.T) *result {
	t.Helper()
	root := os.Getenv("ZB_CLIENT")
	if root == "" {
		t.Skip("ZB_CLIENT not set: tests against the real client skipped")
	}
	res, err := extract(l2pkg.NewClient(root), deathKnight)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// Catches a skin not resolved through the material graph (a section left
// untextured, drawn pink or empty, or given the other slot's skin) and a
// wrong wedge assembly or pose (the feet not on Z = 0 in Wait frame 0).
func TestDeathKnightWaitHasBothSkinsAndFeetOnGround(t *testing.T) {
	res := extractDeathKnight(t)
	b, err := model.Decode(model.Encode(res.Bundle))
	if err != nil {
		t.Fatalf("Decode(Encode(death knight)): %v", err)
	}
	part := &b.Parts[0]
	want := []string{
		"LineageMonstersTex9.death_knight_wizard.death_knight_wizard_t00_ori",
		"LineageMonstersTex9.death_knight_wizard.death_knight_wizard_t01_ori",
	}
	if len(part.Sections) != len(want) {
		t.Fatalf("%d sections, want %d", len(part.Sections), len(want))
	}
	for i, s := range part.Sections {
		if s.IndexCount == 0 {
			t.Errorf("section %d draws nothing", i)
		}
		if s.Texture != want[i] {
			t.Errorf("section %d skin %q, want %q", i, s.Texture, want[i])
		}
		img, err := s.Pixels()
		if err != nil {
			t.Fatalf("section %d skin: %v", i, err)
		}
		if img.Rect.Dx() != 512 || img.Rect.Dy() != 512 {
			t.Errorf("section %d skin is %v, want 512×512", i, img.Rect.Size())
		}
		// A placeholder (pink) or empty skin is one colour; the real one
		// has thousands, and is mostly not magenta.
		colours := map[[3]byte]bool{}
		magenta := 0
		for p := 0; p < len(img.Pix); p += 4 {
			c := [3]byte{img.Pix[p], img.Pix[p+1], img.Pix[p+2]}
			colours[c] = true
			if c[0] > 200 && c[1] < 60 && c[2] > 200 {
				magenta++
			}
		}
		if len(colours) < 1000 || magenta > len(img.Pix)/4/10 {
			t.Errorf("section %d skin has %d colours, %d magenta texels: not the real skin", i, len(colours), magenta)
		}
	}

	a := model.NewAnimator(b)
	positions := make([]geom.Vec3, len(part.Vertices))
	a.Skin(0, positions, nil)
	box := geom.EmptyBox()
	for _, p := range positions {
		box.Include(p)
	}
	if math.Abs(float64(box.Min.Z)) > 0.01 {
		t.Errorf("Wait frame 0 lowest Z %v, want 0", box.Min.Z)
	}
	if got := box.Max.Z - box.Min.Z; math.Abs(float64(got-res.Height)) > 0.01 {
		t.Errorf("Wait frame 0 height %v, printed height %v", got, res.Height)
	}
}

// Catches an embedded monster.bin or constants that no longer come from the
// client: the README's regeneration command must give the embedded bytes,
// and the radius and height it prints must be the preview monster's
// constants.
func TestRegeneratingMonsterGivesEmbeddedBytes(t *testing.T) {
	res := extractDeathKnight(t)
	if !bytes.Equal(model.Encode(res.Bundle), model.Monster) {
		t.Error("the regeneration command no longer gives internal/model/assets/monster.bin")
	}
	if res.Radius != model.MonsterRadius || res.Height != model.MonsterHeight {
		t.Errorf("printed radius %v height %v, constants %v %v", res.Radius, res.Height, model.MonsterRadius, model.MonsterHeight)
	}
}
