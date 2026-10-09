package scene_test

import (
	"math"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
)

// With Giran (22_22) and its east neighbour (23_22) loaded apart into one
// World, as the app opens a tile with its neighbours, a pick lands on the
// server's ground on either side of the border x = 98304, and a slanted ray
// that starts over 22_22 lands in 23_22. The points are NPC spawns of the
// datapack (gameserver/data/spawn/22_22.xml and 23_22.xml, line in the
// name; spawn Z is the ground the server stands them on). Catches a pick
// that only searches the tile the camera opened, or the one under the
// ray's origin.
func TestWorldPicksAcrossTheTileBorder(t *testing.T) {
	root := clientRoot(t)
	w := scene.NewWorld(geom.Vec3{X: 2.5 * 32768, Y: 4.5 * 32768})
	for _, tile := range []scene.Tile{{X: 22, Y: 22}, {X: 23, Y: 22}} {
		s, err := scene.Load(root, []scene.Tile{tile})
		if err != nil {
			t.Fatalf("Load(%s): %v", tile.Name(), err)
		}
		w.Add(s)
	}
	for _, p := range []struct {
		name    string
		x, y, z float32
		// from is where the ray starts, relative to the spawn.
		from geom.Vec3
	}{
		{"22_22 atanas :387", 90498, 147535, -3528, geom.Vec3{Z: 500}},
		{"23_22 faf_herald_lokness :464", 102656, 157424, -3735, geom.Vec3{Z: 500}},
		{"23_22 summoner_brynthea :1343", 106345, 135523, -3415, geom.Vec3{Z: 500}},
		{"23_22 seen from 22_22", 102656, 157424, -3735, geom.Vec3{X: -5000, Z: 10000}},
	} {
		target := geom.Vec3{X: p.x, Y: p.y, Z: p.z}
		origin := scene.FromServer(target.Add(p.from))
		h, ok := w.Pick(scene.Ray{Origin: origin, Dir: p.from.Scale(-1)})
		if !ok {
			t.Errorf("%s: no hit", p.name)
			continue
		}
		d := h.Pos.Sub(target).Length()
		t.Logf("%s: server %v %v %v, pick %.1f %.1f %.1f (%v)", p.name, p.x, p.y, p.z, h.Pos.X, h.Pos.Y, h.Pos.Z, h.Surface)
		if math.IsNaN(float64(d)) || d >= 16 {
			t.Errorf("%s: pick %v is %.1f units from the server %v, want less than 16", p.name, h.Pos, d, target)
		}
	}
}

// A tile dropped from the World is no longer picked: the ray that hit its
// ground now meets nothing. Catches an unload that leaves the tile's
// geometry behind.
func TestWorldForgetsARemovedTile(t *testing.T) {
	root := clientRoot(t)
	w := scene.NewWorld(geom.Vec3{})
	east, err := scene.Load(root, []scene.Tile{{X: 23, Y: 22}})
	if err != nil {
		t.Fatal(err)
	}
	w.Add(east)
	r := scene.Ray{Origin: geom.Vec3{X: 106345, Y: 135523, Z: -2900}, Dir: geom.Vec3{Z: -1}}
	if _, ok := w.Pick(r); !ok {
		t.Fatal("no hit before removing the tile")
	}
	w.Remove(east)
	if h, ok := w.Pick(r); ok {
		t.Errorf("hit at %v after removing the tile", h.Pos)
	}
}

// With static meshes hidden, as the viewport's toggle hides them, the ray
// that hits the roof of a Giran house (Giran_house02, see
// TestPickHitsGiranRoof) goes through it to the fixed map geometry below.
// Catches a vertex placed on a roof the user cannot see.
func TestWorldWithMeshesHiddenPicksThroughThem(t *testing.T) {
	s, err := scene.Load(clientRoot(t), []scene.Tile{{X: 22, Y: 22}})
	if err != nil {
		t.Fatal(err)
	}
	w := scene.NewWorld(geom.Vec3{})
	w.Add(s)
	r := scene.Ray{Origin: geom.Vec3{X: 77833, Y: 149110, Z: 10000}, Dir: geom.Vec3{Z: -1}}
	roof, ok := w.Pick(r)
	if !ok || roof.Surface != scene.SurfaceMesh {
		t.Fatalf("meshes shown: hit %v (%v), want the roof mesh", roof.Pos, roof.Surface)
	}
	w.HideMeshes = true
	h, ok := w.Pick(r)
	if !ok {
		t.Fatal("meshes hidden: no hit under the roof")
	}
	if h.Surface == scene.SurfaceMesh || h.Pos.Z >= roof.Pos.Z-300 {
		t.Errorf("meshes hidden: hit at z %.1f (%v), want the terrain or BSP well below the roof at %.1f", h.Pos.Z, h.Surface, roof.Pos.Z)
	}
}
