// Package scenetest builds brush Models and water volumes for tests. Only
// tests import it.
package scenetest

import (
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/unreal"
)

// Model is a brush Model with one BSP node per face, each face the
// polygon of pts listed by index, in order.
func Model(pts []geom.Vec3, faces [][]int32) *unreal.Model {
	m := &unreal.Model{Vectors: [][3]float32{{0, 0, 1}}, Surfs: []unreal.BSPSurf{{}}}
	for _, p := range pts {
		m.Points = append(m.Points, [3]float32{p.X, p.Y, p.Z})
	}
	for _, f := range faces {
		m.Nodes = append(m.Nodes, unreal.BSPNode{VertPool: int32(len(m.Verts)), NumVertices: uint8(len(f))})
		for _, k := range f {
			m.Verts = append(m.Verts, unreal.BSPVert{Point: k})
		}
	}
	return m
}

// Hexahedron is the brush Model with the 6 quad faces of the hexahedron
// whose bottom corners are c[0..3] and top corners c[4..7],
// counter-clockwise seen from above.
func Hexahedron(c [8]geom.Vec3) *unreal.Model {
	return Model(c[:], [][]int32{
		{3, 2, 1, 0}, {4, 5, 6, 7}, // bottom, top
		{0, 1, 5, 4}, {1, 2, 6, 5}, {2, 3, 7, 6}, {3, 0, 4, 7},
	})
}

// Box is the corners of the axis-aligned box lo..hi in Hexahedron's order.
func Box(lo, hi geom.Vec3) [8]geom.Vec3 {
	return [8]geom.Vec3{
		{X: lo.X, Y: lo.Y, Z: lo.Z}, {X: hi.X, Y: lo.Y, Z: lo.Z}, {X: hi.X, Y: hi.Y, Z: lo.Z}, {X: lo.X, Y: hi.Y, Z: lo.Z},
		{X: lo.X, Y: lo.Y, Z: hi.Z}, {X: hi.X, Y: lo.Y, Z: hi.Z}, {X: hi.X, Y: hi.Y, Z: hi.Z}, {X: lo.X, Y: hi.Y, Z: hi.Z},
	}
}

// UnitBrush is a brush that leaves its Model as it is: unit scales, no
// rotation, at the origin.
func UnitBrush() *unreal.Brush {
	one := unreal.Scale{Scale: geom.Vec3{X: 1, Y: 1, Z: 1}}
	return &unreal.Brush{MainScale: one, PostScale: one}
}

// Volume is water volume export (name) of tile, built by
// scene.NewWaterVolume from brush b and Model m.
func Volume(tb testing.TB, tile scene.Tile, export int, name string, b *unreal.Brush, m *unreal.Model) scene.WaterVolume {
	tb.Helper()
	v, err := scene.NewWaterVolume(tile, export, name, b, m)
	if err != nil {
		tb.Fatal(err)
	}
	return v
}
