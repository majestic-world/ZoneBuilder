package scene_test

import (
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/play"
	"zonebuilder/internal/scene"
)

func TestHiddenSolidGeometryBlocksWithoutBecomingVisibleFloor(t *testing.T) {
	box := geom.Box{Min: geom.Vec3{X: -2000, Y: -2000, Z: -2000}, Max: geom.Vec3{X: 2000, Y: 2000, Z: 2000}}
	for _, c := range []struct {
		mesh, bsp bool
		stop      float32
	}{{true, true, 284}, {false, true, 584}, {false, false, 1000}} {
		s := scene.CollisionFixture(t, c.mesh, c.bsp)
		w := scene.NewWorld(geom.Vec3{})
		w.Add(s)
		w.HideMeshes = true
		visible := 0
		w.Geometry(box, func(scene.Triangle) { visible++ }, nil)
		if visible != 0 || len(s.Batches) != 0 {
			t.Fatal("hidden collision became rendered/placement geometry")
		}
		if _, ok := s.Pick(scene.Ray{Origin: geom.Vec3{Z: 64}, Dir: geom.Vec3{X: 1}}); ok {
			t.Fatal("hidden geometry became pickable")
		}
		tris := []play.Triangle{
			{geom.Vec3{X: -2000, Y: -2000}, geom.Vec3{X: 2000, Y: -2000}, geom.Vec3{X: 2000, Y: 2000}},
			{geom.Vec3{X: -2000, Y: -2000}, geom.Vec3{X: 2000, Y: 2000}, geom.Vec3{X: -2000, Y: 2000}},
		}
		w.Collision(box, func(t scene.Triangle) {
			tris = append(tris, play.Triangle{scene.FromServer(t.A), scene.FromServer(t.B), scene.FromServer(t.C)})
		})
		session := play.NewSession(play.NewWorld(tris), geom.Vec3{Z: play.EyeHeight}, 0, 0)
		for range 240 {
			session.Step(play.Input{Seconds: 1.0 / 60, Forward: 1})
		}
		x := session.Feet().X
		if c.mesh || c.bsp {
			if x > c.stop || x < c.stop-2 {
				t.Fatalf("blocking mesh=%v BSP=%v: feet x=%v, want near %v", c.mesh, c.bsp, x, c.stop)
			}
		} else if x < c.stop {
			t.Fatalf("nonblocking hidden geometry stopped player at %v", x)
		}
	}
}
