package scene_test

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
)

// clientRoot is the real client the tests read (the folder above Maps),
// named by ZB_CLIENT. The tests skip without it.
func clientRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("ZB_CLIENT")
	if root == "" {
		t.Skip("ZB_CLIENT não definido: testes contra o cliente real pulados")
	}
	return root
}

func loadTile(t *testing.T, name string) *scene.Scene {
	t.Helper()
	tile, err := scene.ParseTile(name)
	if err != nil {
		t.Fatal(err)
	}
	s, err := scene.Load(clientRoot(t), []scene.Tile{tile})
	if err != nil {
		t.Fatalf("Load(%s): %v", name, err)
	}
	if len(s.Terrains) != 1 {
		t.Fatalf("Load(%s): %d terrenos, quero 1 (avisos: %q)", name, len(s.Terrains), s.Warnings)
	}
	return s
}

// Giran's terrain lies on its own tile: tile X_Y starts at world
// ((X-20)*32768, (Y-18)*32768), and its 256-sample grid ends one 128-unit
// cell short of the next tile. The height range is the one UE2-Studio's
// TerrainGrid measures for this map (load_visual_scene on the Fafurion
// client, 2026-10-08). Catches a wrong TerrainScale, Location or Z scale.
func TestGiranTerrainCoversItsTile(t *testing.T) {
	want := geom.Box{
		Min: geom.Vec3{X: (22 - 20) * 32768, Y: (22 - 18) * 32768, Z: -4413.8955},
		Max: geom.Vec3{X: (22-20)*32768 + 255*128, Y: (22-18)*32768 + 255*128, Z: -581.833},
	}
	for _, name := range []string{"22_22", "22_22_Classic"} {
		s := loadTile(t, name)
		got := s.Terrains[0].Bounds
		for axis := range 3 {
			if d := math.Abs(float64(got.Min.Axis(axis) - want.Min.Axis(axis))); d > 0.5 {
				t.Errorf("%s: mínimo do eixo %d = %v, quero %v", name, axis, got.Min.Axis(axis), want.Min.Axis(axis))
			}
			if d := math.Abs(float64(got.Max.Axis(axis) - want.Max.Axis(axis))); d > 0.5 {
				t.Errorf("%s: máximo do eixo %d = %v, quero %v", name, axis, got.Max.Axis(axis), want.Max.Axis(axis))
			}
		}
	}
}

// The terrain draws exactly the triangles UE2-Studio's TerrainGrid draws:
// invisible quads missing, edge turns flipping the split diagonal. The
// expected index count and order checksum (sum of (i+1)*(index+1) over the
// index list, wrapping at 2^64) were measured with UE2-Studio's
// load_visual_scene on the Fafurion client on 2026-10-08. Catches a wrong
// bitmap stride or bit order, a dropped edge turn, or a winding change.
func TestTerrainDrawsTheQuadsUE2StudioDraws(t *testing.T) {
	for _, c := range []struct {
		name    string
		indices int
		sum     uint64
	}{
		{"22_22", 370284, 3002590888048257},
		{"23_22", 388320, 3294602379370683},
		{"17_22_Classic", 375810, 3062727014953802},
	} {
		s := loadTile(t, c.name)
		idx := s.Batches[s.Terrains[0].Batch].Indices
		var sum uint64
		for i, v := range idx {
			sum += uint64(i+1) * (uint64(v) + 1)
		}
		if len(idx) != c.indices || sum != c.sum {
			t.Errorf("%s: %d índices com soma %d, quero %d com soma %d", c.name, len(idx), sum, c.indices, c.sum)
		}
	}
}

// Every tile map of the client loads: its TerrainInfo properties, its
// heightmap's material block and mip chain read under whatever
// ArVer/licensee pair its packages carry. Catches a property or version
// gate read wrong on any map, not just Giran.
func TestEveryTileMapLoads(t *testing.T) {
	root := clientRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, "Maps"))
	if err != nil {
		t.Fatal(err)
	}
	var loaded, fallback int
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".unr")
		if !ok {
			continue
		}
		tile, err := scene.ParseTile(name)
		if err != nil {
			continue // Entry, Lobby01, ...: not a tile
		}
		s, err := scene.Load(root, []scene.Tile{tile})
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		loaded++
		for _, w := range s.Warnings {
			t.Logf("%s: %s", name, w)
		}
		for _, ter := range s.Terrains {
			if ter.FallbackScale {
				fallback++
				t.Logf("%s: TerrainScale quebrado, terreno posto por MapX/MapY em %v", name, ter.Bounds)
			}
		}
	}
	t.Logf("%d mapas carregados, %d com fallback de escala", loaded, fallback)
}
