package unreal_test

import (
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/unreal"
)

// TestBrushTransformAppliesTheABrushOrder places one vertex with every part
// of the brush transform set to a distinct value. The expected point is
// worked by hand: v - PrePivot = (1, 1, 2); MainScale (2, 3, 4) gives
// (2, 3, 8); a 90° Yaw turns X into Y, giving (-3, 2, 8); PostScale
// (5, 6, 7) gives (-15, 12, 56); Location adds (1000, 2000, 3000).
// Catches the operations applied in the wrong order (PostScale before the
// rotation, MainScale after it, PrePivot added), a Yaw turning the wrong
// way, or DrawScale-style scaling of the location.
func TestBrushTransformAppliesTheABrushOrder(t *testing.T) {
	b := unreal.Brush{
		Location:  geom.Vec3{X: 1000, Y: 2000, Z: 3000},
		PrePivot:  geom.Vec3{X: 10, Y: 20, Z: 30},
		Rotation:  l2pkg.Rotator{Yaw: 16384},
		MainScale: unreal.Scale{Scale: geom.Vec3{X: 2, Y: 3, Z: 4}},
		PostScale: unreal.Scale{Scale: geom.Vec3{X: 5, Y: 6, Z: 7}},
	}
	got := b.Transform().Point(geom.Vec3{X: 11, Y: 21, Z: 32})
	want := geom.Vec3{X: 985, Y: 2012, Z: 3056}
	if d := got.Sub(want).Length(); d > 1e-3 {
		t.Errorf("Point = %v, want %v", got, want)
	}
}
