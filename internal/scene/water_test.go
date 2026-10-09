package scene_test

import (
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/scene/scenetest"
)

// TestGiranWaterVolumesComeFromTheBrushFaces loads Giran and checks its
// live WaterVolumes against the probe of the water-zone spec (Fafurion
// client, 2026-10-09): 8 of them, and WaterVolume0's faces spanning
// x 86757…98304, y 152256…153600, z -8778…-3780. Catches the box taken
// from Model.Points (x starts at 69989 there), a wrong brush transform, or
// a deleted or unplaced volume read.
func TestGiranWaterVolumesComeFromTheBrushFaces(t *testing.T) {
	s := loadTile(t, "22_22")
	if len(s.WaterVolumes) != 8 {
		t.Errorf("%s, want 8", inflect.Count(len(s.WaterVolumes), "water volume", "water volumes"))
	}
	want := geom.Box{
		Min: geom.Vec3{X: 86757, Y: 152256, Z: -8778},
		Max: geom.Vec3{X: 98304, Y: 153600, Z: -3780},
	}
	for _, v := range s.WaterVolumes {
		if v.Name != "WaterVolume0" {
			continue
		}
		if !within1(v.Bounds.Min, want.Min) || !within1(v.Bounds.Max, want.Max) {
			t.Errorf("WaterVolume0 box %v, want %v", v.Bounds, want)
		}
		if v.Tile != (scene.Tile{X: 22, Y: 22}) || v.Unsupported != "" || len(v.Faces) != 6 {
			t.Errorf("WaterVolume0: tile %v, unsupported %q, %s; want 22_22, supported, 6", v.Tile, v.Unsupported, inflect.Count(len(v.Faces), "face", "faces"))
		}
		return
	}
	t.Error("no WaterVolume0 in 22_22")
}

func within1(a, b geom.Vec3) bool { return a.Sub(b).Length() < 1 }

// cube is an axis-aligned 100³ cube's corners from the origin.
func cube() [8]geom.Vec3 { return scenetest.Box(geom.Vec3{}, geom.Vec3{X: 100, Y: 100, Z: 100}) }

// TestWaterVolumeExactOnlyForVerticalWallsAndFlatCaps builds an aligned
// box and the same box with its +X wall leaning out at the top. Catches a
// slanted wall taken as exact, which would compile a server prism that
// misses part of the water, or a plain box flagged approximate.
func TestWaterVolumeExactOnlyForVerticalWallsAndFlatCaps(t *testing.T) {
	aligned, err := scene.NewWaterVolume(scene.Tile{X: 22, Y: 22}, 0, "WaterVolume0", scenetest.UnitBrush(), scenetest.Hexahedron(cube()))
	if err != nil {
		t.Fatal(err)
	}
	if !aligned.Exact {
		t.Error("aligned box: Exact = false, want true")
	}
	c := cube()
	c[5].X, c[6].X = 150, 150
	slanted, err := scene.NewWaterVolume(scene.Tile{X: 22, Y: 22}, 1, "WaterVolume1", scenetest.UnitBrush(), scenetest.Hexahedron(c))
	if err != nil {
		t.Fatal(err)
	}
	if slanted.Exact {
		t.Error("box with a slanted wall: Exact = true, want false")
	}
}

// TestWaterVolumeWithShearIsUnsupported gives a box a sheared MainScale,
// which BrushTransform does not apply. Catches a sheared volume offered
// for selection with geometry the client does not have.
func TestWaterVolumeWithShearIsUnsupported(t *testing.T) {
	b := scenetest.UnitBrush()
	b.MainScale.SheerRate = 0.5
	v, err := scene.NewWaterVolume(scene.Tile{X: 22, Y: 22}, 0, "WaterVolume0", b, scenetest.Hexahedron(cube()))
	if err != nil {
		t.Fatal(err)
	}
	if v.Unsupported == "" {
		t.Error("sheared volume: Unsupported empty, want a reason")
	}
}
