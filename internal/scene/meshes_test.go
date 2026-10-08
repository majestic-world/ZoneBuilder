package scene_test

import (
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
)

// A map places the static mesh actors UE2-Studio places, each where
// UE2-Studio puts it. The expected actor and triangle counts and the
// world-space bounds are UE2-Studio's load_visual_scene on the Fafurion
// client (geometry actors, 2026-10-08). The sampled actors cover a
// pitch/yaw/roll rotation with a mirrored DrawScale3D and a DrawScale, a
// Mover with a non-uniform DrawScale3D, an L2MovableStaticMeshActor, and a
// PrePivot. Catches a wrong rotation convention, scale order, pivot sign,
// section winding, or an actor class or Level entry left out.
func TestMeshActorsMatchUE2Studio(t *testing.T) {
	type sample struct {
		export   int
		class    string
		min, max geom.Vec3
	}
	for _, c := range []struct {
		tile         string
		actors, tris int
		samples      []sample
	}{
		{"22_22", 2649, 541568, []sample{
			{601, "StaticMeshActor", geom.Vec3{X: 88902.52, Y: 154593.42, Z: 1904.46}, geom.Vec3{X: 89073.27, Y: 155061.38, Z: 2022.39}},
			{1432, "Mover", geom.Vec3{X: 81194.70, Y: 151556.22, Z: -3546.87}, geom.Vec3{X: 81198.47, Y: 151606.88, Z: -3434.33}},
			{5779, "L2MovableStaticMeshActor", geom.Vec3{X: 83703.20, Y: 155794.70, Z: 1629.08}, geom.Vec3{X: 84028.45, Y: 156119.95, Z: 1863.57}},
			{1032, "StaticMeshActor", geom.Vec3{X: 77449.24, Y: 148851.62, Z: -3626.62}, geom.Vec3{X: 78216.82, Y: 149368.38, Z: -2648.30}},
		}},
		{"22_22_Classic", 1944, 429199, nil},
		{"18_16", 1108, 449772, []sample{
			{110, "StaticMeshActor", geom.Vec3{X: -52535.64, Y: -53732.89, Z: -2670.78}, geom.Vec3{X: -52490.03, Y: -53702.14, Z: -2636.74}},
		}},
	} {
		s := loadTile(t, c.tile)
		tris := 0
		byExport := map[int]*scene.MeshActor{}
		for i := range s.Actors {
			a := &s.Actors[i]
			byExport[a.Export] = a
			tris += a.Triangles()
		}
		if len(s.Actors) != c.actors || tris != c.tris {
			t.Errorf("%s: %d atores com %d triângulos, quero %d com %d", c.tile, len(s.Actors), tris, c.actors, c.tris)
		}
		for _, w := range s.Warnings {
			if strings.Contains(w, "static mesh") {
				t.Errorf("%s: aviso inesperado: %s", c.tile, w)
			}
		}
		for _, sm := range c.samples {
			a := byExport[sm.export]
			if a == nil {
				t.Errorf("%s: export %d não está entre os atores", c.tile, sm.export)
				continue
			}
			if a.Class != sm.class {
				t.Errorf("%s: export %d é %s, quero %s", c.tile, sm.export, a.Class, sm.class)
			}
			for axis := range 3 {
				if math.Abs(float64(a.Bounds.Min.Axis(axis)-sm.min.Axis(axis))) > 0.05 ||
					math.Abs(float64(a.Bounds.Max.Axis(axis)-sm.max.Axis(axis))) > 0.05 {
					t.Errorf("%s: %s (export %d): caixa %v, quero %v–%v", c.tile, a.Name, sm.export, a.Bounds, sm.min, sm.max)
					break
				}
			}
		}
	}
}

// A ray dropped onto the roof of a Giran house (Giran_Village_S.
// Giran_house02, StaticMeshActor349, whose world box UE2-Studio measures as
// x 77449–78217, y 148852–149368, z -3627–-2648) hits the mesh, at a server
// Z well above the terrain under the house and inside the house's height.
// Catches meshes missing from Pick, or a pick that takes the farther
// surface instead of the nearest.
func TestPickHitsGiranRoof(t *testing.T) {
	s := loadTile(t, "22_22")
	x, y := float32(77833), float32(149110)
	h, ok := s.Pick(down(x, y))
	if !ok {
		t.Fatal("sem acerto no telhado")
	}
	ter := &s.Terrains[0]
	ground := ter.Vertex(int((x-ter.Position.X)/ter.Scale.X+0.5), int((y-ter.Position.Y)/ter.Scale.Y+0.5)).Z
	t.Logf("telhado: pick %.1f %.1f %.1f (%v), terreno abaixo %.1f", h.Pos.X, h.Pos.Y, h.Pos.Z, h.Surface, ground)
	if h.Surface != scene.SurfaceMesh {
		t.Errorf("superfície %v, quero static mesh", h.Surface)
	}
	roof := h.Pos.Z - scene.ServerZOffset
	if roof < ground+300 || roof > -2648.30+0.5 {
		t.Errorf("telhado em z %.1f (cliente): quero entre %.1f (300 acima do terreno) e o topo da casa -2648.3", roof, ground+300)
	}
}

// A client that lacks the static mesh packages still opens the map: the
// terrain loads, and each missing .usx becomes a warning that names it.
// The client here is a temporary copy of Giran's map and heightmap only.
func TestMissingMeshPackageWarnsAndLoadsTheRest(t *testing.T) {
	root := clientRoot(t)
	tmp := t.TempDir()
	for _, f := range []string{"Maps/22_22.unr", "Textures/T_22_22.utx"} {
		copyFile(t, filepath.Join(root, f), filepath.Join(tmp, f))
	}
	s, err := scene.Load(tmp, []scene.Tile{{X: 22, Y: 22}})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Terrains) != 1 {
		t.Errorf("%d terrenos, quero 1", len(s.Terrains))
	}
	if len(s.Actors) != 0 {
		t.Errorf("%d atores sem nenhum pacote de mesh, quero 0", len(s.Actors))
	}
	var named bool
	for _, w := range s.Warnings {
		t.Log(w)
		named = named || strings.Contains(w, "Giran_Village_S")
	}
	if !named {
		t.Errorf("nenhum aviso cita o pacote Giran_Village_S: %q", s.Warnings)
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	src, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	dst, err := os.Create(to)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		t.Fatal(err)
	}
	if err := dst.Close(); err != nil {
		t.Fatal(err)
	}
}
