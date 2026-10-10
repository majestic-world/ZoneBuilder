package scene

import (
	"bytes"
	"encoding/binary"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/unreal"
)

// CollisionFixture decodes synthetic serialized UE2 objects, then uses the
// production scene assembly path. The fixture has a hidden mesh at x=300
// and an invisible BSP wall at x=600, with independent blocking flags.
func CollisionFixture(tb testing.TB, meshBlocks, bspSolid bool) *Scene {
	tb.Helper()
	p := &l2pkg.Package{Name: "fixture", Names: []string{"None", "bHidden", "bCollideActors", "bBlockActors", "bBlockPlayers", "bWorldGeometry", "bBlockNonZeroExtentTraces"}}
	add := func(class string, data []byte) int {
		i := len(p.Exports)
		p.Exports = append(p.Exports, l2pkg.Export{ClassName: class, ObjectName: class, SerialOffset: int32(len(p.Data)), SerialSize: int32(len(data))})
		p.Data = append(p.Data, data...)
		return i
	}
	var b bytes.Buffer
	put := func(v any) {
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
			tb.Fatal(err)
		}
	}
	zeros := func(n int) { b.Write(make([]byte, n)) }
	// Level with Model reference 2, without URL/reach specs.
	put(uint8(0))
	put(int32(0))
	put(int32(0))
	zeros(4)
	put(uint8(0))
	put(int32(0))
	zeros(3)
	put(uint8(0))
	put(uint8(2))
	add("Level", bytes.Clone(b.Bytes()))
	b.Reset()
	// Model: one invisible triangular wall.
	put(uint8(0))
	zeros(41)
	put(uint8(1))
	put([3]float32{1, 0, 0})
	put(uint8(3))
	for _, v := range [][3]float32{{600, -500, -500}, {600, 500, -500}, {600, 0, 1000}} {
		put(v)
	}
	put(uint8(1))
	zeros(16 + 8 + 1)
	zeros(7)
	zeros(12 + 4 + 8 + 8)
	zeros(2)
	put(uint8(3))
	zeros(4 + 4 + 12)
	put(uint8(1))
	put(uint8(0))
	flags := uint32(unreal.PFInvisible)
	if !bspSolid {
		flags |= unreal.PFNotSolid
	}
	put(flags)
	zeros(6 + 16 + 4)
	put(uint8(3))
	for i := uint8(0); i < 3; i++ {
		put(i)
		put(uint8(0))
	}
	add("Model", bytes.Clone(b.Bytes()))
	b.Reset()
	// StaticMesh with one triangle, no material or colour/UV streams.
	put(uint8(0))
	zeros(41)
	put(uint8(1))
	put(uint32(0))
	for _, n := range []uint16{0, 0, 0, 0, 1} {
		put(n)
	}
	zeros(25)
	put(uint8(3))
	for _, v := range [][3]float32{{300, -500, -500}, {300, 500, -500}, {300, 0, 1000}} {
		put(v)
		zeros(12)
	}
	put(uint32(0))
	put(uint8(0))
	put(uint32(0))
	put(uint8(0))
	put(uint32(0))
	put(uint8(0))
	put(uint8(3))
	put([3]uint16{2, 1, 0})
	put(uint32(0))
	mi := add("StaticMesh", bytes.Clone(b.Bytes()))
	b.Reset()
	put(uint8(1))
	put(uint8(0x83))
	for i := uint8(2); i <= 6; i++ {
		put(i)
		info := uint8(3)
		if meshBlocks {
			info = 0x83
		}
		put(info)
	}
	put(uint8(0))
	ai := add("StaticMeshActor", bytes.Clone(b.Bytes()))
	s := &Scene{}
	ld := newLoader(l2pkg.NewClient(""))
	if err := s.addBSP(ld, p, Tile{}, nil); err != nil {
		tb.Fatal(err)
	}
	mesh, err := unreal.ReadStaticMesh(p, mi)
	if err != nil {
		tb.Fatal(err)
	}
	actor, err := unreal.ReadActor(p, ai)
	if err != nil {
		tb.Fatal(err)
	}
	ma := MeshActor{Actor: *actor, Hidden: actor.Hidden, Collision: actor.Collision.Over(unreal.Collision{}), Bounds: geom.EmptyBox()}
	if _, err := s.placeMesh(ld, &ma, p, p, mesh, nil); err != nil {
		tb.Fatal(err)
	}
	s.Actors = append(s.Actors, ma)
	return s
}
