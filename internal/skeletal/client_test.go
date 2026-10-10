package skeletal_test

import (
	"math"
	"os"
	"testing"

	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/skeletal"
)

// monsters opens the package holding the preview monster in the real client
// named by ZB_CLIENT. The tests skip without it.
func monsters(t *testing.T) *l2pkg.Package {
	t.Helper()
	root := os.Getenv("ZB_CLIENT")
	if root == "" {
		t.Skip("ZB_CLIENT not set: tests against the real client skipped")
	}
	p, err := l2pkg.NewClient(root).Package("LineageMonsters15")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func readMesh(t *testing.T, p *l2pkg.Package) *skeletal.Mesh {
	t.Helper()
	i := p.ExportNamed("death_knight_wizard_m00", "SkeletalMesh")
	if i < 0 {
		t.Fatal("death_knight_wizard_m00 not found")
	}
	m, err := skeletal.ReadMesh(p, i)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// The preview monster's LOD 0 is self-consistent and has the counts
// UE2-Studio's reader gives the same export (99 bones, one vertex per wedge,
// 2741 wedges, 3880 faces, 2 skinned sections). Catches a field read out of
// step in the 132/40 layout, the wrong wedge assembly (vertices pointing at
// the wrong point or bone), and influences packed without renormalising.
func TestDeathKnightMeshLOD0(t *testing.T) {
	p := monsters(t)
	m := readMesh(t, p)

	if len(m.Bones) != 99 {
		t.Errorf("bones = %d, want 99", len(m.Bones))
	}
	if len(m.Vertices) != 2741 {
		t.Errorf("vertices = %d, want 2741 (one per wedge)", len(m.Vertices))
	}
	if len(m.Indices) != 3*3880 {
		t.Errorf("indices = %d, want %d (3880 faces)", len(m.Indices), 3*3880)
	}
	for k, idx := range m.Indices {
		if int(idx) >= len(m.Vertices) {
			t.Fatalf("index %d names vertex %d of %d", k, idx, len(m.Vertices))
		}
	}
	if m.Unbound != 0 {
		t.Errorf("%d vertices have no influence", m.Unbound)
	}
	for k, v := range m.Vertices {
		var sum float32
		for s, w := range v.Weights {
			sum += w
			if int(v.Bones[s]) >= len(m.Bones) {
				t.Fatalf("vertex %d slot %d names bone %d of %d", k, s, v.Bones[s], len(m.Bones))
			}
		}
		if math.Abs(float64(sum)-1) > 1e-5 {
			t.Fatalf("vertex %d weights sum to %v", k, sum)
		}
		if l := v.Normal.Length(); math.Abs(float64(l)-1) > 1e-3 {
			t.Fatalf("vertex %d normal length %v", k, l)
		}
	}

	if len(m.Sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(m.Sections))
	}
	covered := 0
	for k, s := range m.Sections {
		if s.FirstIndex != covered || s.IndexCount <= 0 {
			t.Errorf("section %d covers [%d, +%d), want to start at %d with faces", k, s.FirstIndex, s.IndexCount, covered)
		}
		covered += s.IndexCount
		path, err := p.ObjectPath(s.Material)
		if err != nil || path == "" {
			t.Errorf("section %d material %d: path %q, %v", k, s.Material, path, err)
		}
	}
	if covered != len(m.Indices) {
		t.Errorf("sections cover %d of %d indices", covered, len(m.Indices))
	}
}

// The preview monster's animation set has the Wait idle UE2-Studio reads
// (60 frames at 30 fps) with keys for every bone of the mesh. Catches a
// sequence or move read out of step in the 132/40 layout, sequences paired
// with the wrong key chunk, and bones left unbound (frozen in bind pose).
func TestDeathKnightWaitAnimation(t *testing.T) {
	p := monsters(t)
	m := readMesh(t, p)
	i := p.ExportNamed("death_knight_wizard_anim", "MeshAnimation")
	if i < 0 {
		t.Fatal("death_knight_wizard_anim not found")
	}
	a, err := skeletal.ReadAnimation(p, i)
	if err != nil {
		t.Fatal(err)
	}
	seq := a.SequenceNamed("Wait")
	if seq < 0 {
		t.Fatal("no Wait sequence")
	}
	if s := a.Sequences[seq]; s.Frames != 60 || s.Rate != 30 {
		t.Errorf("Wait = %d frames at %v fps, want 60 at 30", s.Frames, s.Rate)
	}
	tracks, err := a.Tracks(seq)
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != len(a.Bones) {
		t.Fatalf("Wait has %d tracks for %d animated bones", len(tracks), len(a.Bones))
	}
	for b, track := range a.Bind(m.Bones) {
		if track < 0 {
			t.Errorf("mesh bone %q has no track", m.Bones[b].Name)
			continue
		}
		k := tracks[track]
		if len(k.Quats) == 0 || len(k.Positions) == 0 || len(k.Times) == 0 {
			t.Errorf("track of %q has %d/%d/%d keys", m.Bones[b].Name, len(k.Quats), len(k.Positions), len(k.Times))
		}
		for _, time := range k.Times {
			if time < 0 || time >= 60 {
				t.Fatalf("track of %q has key time %v outside the 60 frames", m.Bones[b].Name, time)
			}
		}
	}
}
