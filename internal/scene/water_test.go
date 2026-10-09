package scene_test

import (
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/unreal"
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
			t.Errorf("WaterVolume0: tile %v, unsupported %q, %d faces; want 22_22, supported, 6", v.Tile, v.Unsupported, len(v.Faces))
		}
		return
	}
	t.Error("no WaterVolume0 in 22_22")
}

func within1(a, b geom.Vec3) bool { return a.Sub(b).Length() < 1 }

// hexahedron is a brush Model with the 6 quad faces of the hexahedron whose
// bottom corners are c[0..3] and top corners c[4..7], counter-clockwise
// seen from above, each face listed once as one BSP node.
func hexahedron(c [8]geom.Vec3) *unreal.Model {
	m := &unreal.Model{Vectors: [][3]float32{{0, 0, 1}}, Surfs: []unreal.BSPSurf{{}}}
	for _, p := range c {
		m.Points = append(m.Points, [3]float32{p.X, p.Y, p.Z})
	}
	for _, f := range [][4]int32{
		{3, 2, 1, 0}, {4, 5, 6, 7}, // bottom, top
		{0, 1, 5, 4}, {1, 2, 6, 5}, {2, 3, 7, 6}, {3, 0, 4, 7},
	} {
		m.Nodes = append(m.Nodes, unreal.BSPNode{VertPool: int32(len(m.Verts)), NumVertices: 4})
		for _, k := range f {
			m.Verts = append(m.Verts, unreal.BSPVert{Point: k})
		}
	}
	return m
}

// box is an axis-aligned 100³ cube's corners from the origin.
func box() [8]geom.Vec3 {
	return [8]geom.Vec3{
		{X: 0, Y: 0, Z: 0}, {X: 100, Y: 0, Z: 0}, {X: 100, Y: 100, Z: 0}, {X: 0, Y: 100, Z: 0},
		{X: 0, Y: 0, Z: 100}, {X: 100, Y: 0, Z: 100}, {X: 100, Y: 100, Z: 100}, {X: 0, Y: 100, Z: 100},
	}
}

func unitBrush() *unreal.Brush {
	one := unreal.Scale{Scale: geom.Vec3{X: 1, Y: 1, Z: 1}}
	return &unreal.Brush{MainScale: one, PostScale: one}
}

// TestWaterVolumeExactOnlyForVerticalWallsAndFlatCaps builds an aligned
// box and the same box with its +X wall leaning out at the top. Catches a
// slanted wall taken as exact, which would compile a server prism that
// misses part of the water, or a plain box flagged approximate.
func TestWaterVolumeExactOnlyForVerticalWallsAndFlatCaps(t *testing.T) {
	aligned, err := scene.NewWaterVolume(scene.Tile{X: 22, Y: 22}, 0, "WaterVolume0", unitBrush(), hexahedron(box()))
	if err != nil {
		t.Fatal(err)
	}
	if !aligned.Exact {
		t.Error("aligned box: Exact = false, want true")
	}
	c := box()
	c[5].X, c[6].X = 150, 150
	slanted, err := scene.NewWaterVolume(scene.Tile{X: 22, Y: 22}, 1, "WaterVolume1", unitBrush(), hexahedron(c))
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
	b := unitBrush()
	b.MainScale.SheerRate = 0.5
	v, err := scene.NewWaterVolume(scene.Tile{X: 22, Y: 22}, 0, "WaterVolume0", b, hexahedron(box()))
	if err != nil {
		t.Fatal(err)
	}
	if v.Unsupported == "" {
		t.Error("sheared volume: Unsupported empty, want a reason")
	}
}
