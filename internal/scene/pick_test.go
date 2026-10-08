package scene_test

import (
	"math"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
)

// down is a vertical ray straight down from well above every Giran height
// at server point (x, y).
func down(x, y float32) scene.Ray {
	return scene.Ray{Origin: geom.Vec3{X: x, Y: y, Z: 20000}, Dir: geom.Vec3{Z: -1}}
}

// A vertical ray at a Giran reference point lands on the server's ground,
// within 16 units of the x y z //pos prints there. The points are NPCs the
// datapack spawns on open terrain around Giran's gates
// (majestic-datapack gameserver/data/spawn/22_22.xml, line in the name;
// spawn Z is the ground the server stands them on). At each one the
// server's geodata (geodata/22_22.l2j) has a single layer within 8 units of
// the spawn Z, so nothing (bridge, floor, BSP) lies over the terrain there
// near the ground; the ray starts 500 above the spawn, under the floating
// Superion island meshes thousands of units overhead.
// Catches a wrong terrain transform, scale, tile origin or server Z offset.
func TestPickLandsOnGiranServerGround(t *testing.T) {
	s := loadTile(t, "22_22")
	for _, p := range []struct {
		name    string
		x, y, z float32
	}{
		{"guard_kurtys :399", 77026, 148813, -3598},
		{"sir_ortho_lancer :400", 77029, 148447, -3599},
		{"vesa :395", 81424, 143472, -3529},
		{"zerome :396", 81616, 143472, -3529},
		{"jeronin :403", 81436, 152950, -3532},
		{"blas :404", 81642, 152955, -3526},
		{"beltkem :392", 83705, 141439, -3530},
		{"rarshints :391", 84064, 141438, -3526},
		{"atanas :387", 90498, 147535, -3528},
		{"guard_reikin :388", 90501, 147180, -3530},
	} {
		h, ok := s.Pick(scene.Ray{Origin: geom.Vec3{X: p.x, Y: p.y, Z: p.z + 500}, Dir: geom.Vec3{Z: -1}})
		if !ok {
			t.Errorf("%s (%v %v %v): sem acerto", p.name, p.x, p.y, p.z)
			continue
		}
		if h.Surface != scene.SurfaceTerrain {
			t.Errorf("%s: superfície %v, quero terreno", p.name, h.Surface)
		}
		d := math.Sqrt(float64(sq(h.Pos.X-p.x) + sq(h.Pos.Y-p.y) + sq(h.Pos.Z-p.z)))
		t.Logf("%s: servidor %v %v %v, pick %.1f %.1f %.1f, Δz %+.1f", p.name, p.x, p.y, p.z, h.Pos.X, h.Pos.Y, h.Pos.Z, h.Pos.Z-p.z)
		if d >= 16 {
			t.Errorf("%s: pick %v está a %.1f unidades do servidor %v %v %v, quero menos de 16", p.name, h.Pos, d, p.x, p.y, p.z)
		}
	}
}

// A ray that never meets the terrain, beside the tile or pointing at the
// sky, returns no hit.
func TestPickOffTerrainMisses(t *testing.T) {
	s := loadTile(t, "22_22")
	for name, r := range map[string]scene.Ray{
		"fora do tile":    down(50000, 147000),
		"para o céu":      {Origin: geom.Vec3{X: 83400, Y: 147943, Z: 0}, Dir: geom.Vec3{Z: 1}},
		"horizontal alto": {Origin: geom.Vec3{X: 60000, Y: 147943, Z: 5000}, Dir: geom.Vec3{X: 1}},
		"direção nula":    {Origin: geom.Vec3{X: 83400, Y: 147943, Z: 0}},
	} {
		if h, ok := s.Pick(r); ok {
			t.Errorf("%s: acerto inesperado em %v", name, h.Pos)
		}
	}
}

func sq(v float32) float32 { return v * v }
