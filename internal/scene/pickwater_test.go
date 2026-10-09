package scene_test

import (
	"math"
	"slices"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/unreal"
)

// boxCorners are the corners of the axis-aligned box lo..hi in hexahedron's
// order.
func boxCorners(lo, hi geom.Vec3) [8]geom.Vec3 {
	return [8]geom.Vec3{
		{X: lo.X, Y: lo.Y, Z: lo.Z}, {X: hi.X, Y: lo.Y, Z: lo.Z}, {X: hi.X, Y: hi.Y, Z: lo.Z}, {X: lo.X, Y: hi.Y, Z: lo.Z},
		{X: lo.X, Y: lo.Y, Z: hi.Z}, {X: hi.X, Y: lo.Y, Z: hi.Z}, {X: hi.X, Y: hi.Y, Z: hi.Z}, {X: lo.X, Y: hi.Y, Z: hi.Z},
	}
}

// waterBox is the water volume export of tile 22_22 filling box lo..hi,
// built by b.
func waterBox(t *testing.T, export int, b *unreal.Brush, lo, hi geom.Vec3) scene.WaterVolume {
	t.Helper()
	v, err := scene.NewWaterVolume(scene.Tile{X: 22, Y: 22}, export, "WaterVolume", b, hexahedron(boxCorners(lo, hi)))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func vec(x, y, z float32) geom.Vec3 { return geom.Vec3{X: x, Y: y, Z: z} }

var noFloor = float32(math.Inf(1))

// TestPickWaterClipsTheRayByTheVolumePlanes casts rays at a 100³ box of
// water and a second box straight below it. Catches an entry measured to
// the box instead of the faces, a ray beside the volume taken as a hit, a
// camera inside the water not selecting it, a ray grazing an edge lost to
// float error, a farther volume chosen over the nearer one, and a volume
// behind the floor (entry past maxDist) selected through the ground.
func TestPickWaterClipsTheRayByTheVolumePlanes(t *testing.T) {
	w := scene.NewWorld(geom.Vec3{})
	w.Add(&scene.Scene{WaterVolumes: []scene.WaterVolume{
		waterBox(t, 1, unitBrush(), vec(0, 0, 0), vec(100, 100, 100)),
		waterBox(t, 2, unitBrush(), vec(0, 0, -300), vec(100, 100, -200)),
	}})
	diag := float32(1 / math.Sqrt2)
	for _, c := range []struct {
		name    string
		ray     scene.Ray
		maxDist float32
		export  int // 0: no hit
		entry   float32
	}{
		{"enters through the top", scene.Ray{Origin: vec(50, 50, 300), Dir: vec(0, 0, -1)}, noFloor, 1, 200},
		{"passes beside", scene.Ray{Origin: vec(150, 50, 300), Dir: vec(0, 0, -1)}, noFloor, 0, 0},
		{"born inside", scene.Ray{Origin: vec(50, 50, 50), Dir: vec(1, 0, 0)}, noFloor, 1, 0},
		{"grazes the top +X edge", scene.Ray{Origin: vec(150, 50, 50), Dir: vec(-diag, 0, diag)}, noFloor, 1, 50 * math.Sqrt2},
		{"volume behind the floor", scene.Ray{Origin: vec(50, 50, 300), Dir: vec(0, 0, -1)}, 150, 0, 0},
		{"lower volume seen from under the upper one", scene.Ray{Origin: vec(50, 50, -100), Dir: vec(0, 0, -1)}, noFloor, 2, 100},
	} {
		v, entry, ok := w.PickWater(c.ray, c.maxDist)
		switch {
		case c.export == 0 && ok:
			t.Errorf("%s: hit %s export %d at %g, want no hit", c.name, v.Name, v.Export, entry)
		case c.export != 0 && !ok:
			t.Errorf("%s: no hit, want export %d", c.name, c.export)
		case c.export != 0 && (v.Export != c.export || math.Abs(float64(entry-c.entry)) > 0.01):
			t.Errorf("%s: export %d at %g, want export %d at %g", c.name, v.Export, entry, c.export, c.entry)
		}
	}
}

// TestWaterBodyJoinsTouchingVolumesWithTheSameTop lays 3 boxes side by
// side with the same top, the third in another tile's scene and 0.5 apart
// (within the 1 unit gap), and a 4th touching the first with its top 100
// higher. Catches a body that stops at the first neighbour or at the
// tile's scene, or that swallows water of another level, which would
// compile into one zone breathing at the higher top.
func TestWaterBodyJoinsTouchingVolumesWithTheSameTop(t *testing.T) {
	w := scene.NewWorld(geom.Vec3{})
	w.Add(&scene.Scene{WaterVolumes: []scene.WaterVolume{
		waterBox(t, 1, unitBrush(), vec(0, 0, 0), vec(100, 100, 100)),
		waterBox(t, 2, unitBrush(), vec(100, 0, 0), vec(200, 100, 100)),
		waterBox(t, 4, unitBrush(), vec(0, 100, 0), vec(100, 200, 200)),
	}})
	w.Add(&scene.Scene{WaterVolumes: []scene.WaterVolume{
		waterBox(t, 3, unitBrush(), vec(200.5, 0, 50), vec(300, 100, 100)),
	}})
	first, _, ok := w.PickWater(scene.Ray{Origin: vec(50, 50, 300), Dir: vec(0, 0, -1)}, noFloor)
	if !ok {
		t.Fatal("no volume under the ray")
	}
	var got []int
	for _, v := range w.WaterBody(first) {
		got = append(got, v.Export)
	}
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Errorf("body exports %v, want [1 2 3]", got)
	}
}

// TestPickWaterNeverChoosesAnUnsupportedVolume puts a sheared volume (its
// shape is not the client's) above a plain one and beside it, same top.
// Catches the unsupported volume selected by the click, or pulled into the
// body of its neighbour, and then compiled.
func TestPickWaterNeverChoosesAnUnsupportedVolume(t *testing.T) {
	sheared := unitBrush()
	sheared.MainScale.SheerRate = 0.5
	w := scene.NewWorld(geom.Vec3{})
	w.Add(&scene.Scene{WaterVolumes: []scene.WaterVolume{
		waterBox(t, 1, sheared, vec(0, 0, 0), vec(100, 100, 100)),
		waterBox(t, 2, unitBrush(), vec(0, 0, -200), vec(100, 100, -100)),
		waterBox(t, 3, sheared, vec(100, 0, -200), vec(200, 100, -100)),
	}})
	v, _, ok := w.PickWater(scene.Ray{Origin: vec(50, 50, 300), Dir: vec(0, 0, -1)}, noFloor)
	if !ok {
		t.Fatal("no hit, want export 2 under the sheared one")
	}
	if v.Export != 2 {
		t.Fatalf("picked export %d, want export 2 under the sheared one", v.Export)
	}
	if body := w.WaterBody(v); len(body) != 1 {
		t.Errorf("body of export 2 has %d volumes, want only itself", len(body))
	}
	if v, _, ok := w.PickWater(scene.Ray{Origin: vec(150, 50, 300), Dir: vec(0, 0, -1)}, noFloor); ok {
		t.Errorf("picked sheared export %d, want no hit", v.Export)
	}
}
