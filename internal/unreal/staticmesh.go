package unreal

import (
	"fmt"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/l2pkg"
)

// StaticMesh is the drawable part of a UStaticMesh: its vertex and index
// streams and its sections. Port of UE2-Studio's StaticMesh::read
// (crates/static-mesh-engine/src/static_mesh.rs).
type StaticMesh struct {
	// Positions are the vertex positions in the mesh's local space,
	// Unreal basis.
	Positions []geom.Vec3
	// UVs is the first UV stream, parallel to Positions, or empty when the
	// mesh has none.
	UVs [][2]float32
	// Alpha is the alpha of the ColorStream, by vertex index, or empty when
	// every entry is 255; a vertex past its end is opaque. A material whose
	// Opacity is a VertexColor uses it as coverage.
	Alpha    []uint8
	Indices  []uint16
	Sections []MeshSection
	// Materials holds Materials[i].Material per slot: an object reference in
	// the mesh package's index space, 0 when the slot names none.
	Materials []int32
}

// MeshSection is a run of triangles drawn with one material slot: indices
// Indices[FirstIndex : FirstIndex+3*Triangles].
type MeshSection struct {
	FirstIndex int
	Triangles  int
}

// primitiveTail is the UPrimitive data after the properties of a Model or
// StaticMesh: a bounding box (min, max, valid byte) and a sphere.
const primitiveTail = 25 + 16

// ReadStaticMesh decodes export i (0-based) of p as a StaticMesh.
func ReadStaticMesh(p *l2pkg.Package, i int) (*StaticMesh, error) {
	r, err := p.ExportReader(i)
	if err != nil {
		return nil, err
	}
	name := p.Exports[i].ObjectName
	props, err := l2pkg.ReadProperties(p, r, p.Exports[i].Flags)
	if err != nil {
		return nil, fmt.Errorf("StaticMesh %s: %w", name, err)
	}
	m := &StaticMesh{}
	for _, slot := range props.Maps("Materials") {
		ref, _ := slot.Index("Material")
		m.Materials = append(m.Materials, ref)
	}
	r.Skip(primitiveTail)
	n := readCount(r, "seções")
	m.Sections = make([]MeshSection, 0, n)
	for range n {
		r.U32()
		first := r.U16()
		r.U16()
		r.U16()
		r.U16()
		m.Sections = append(m.Sections, MeshSection{FirstIndex: int(first), Triangles: int(r.U16())})
	}
	r.Skip(25) // bounding box
	n = readCount(r, "vértices")
	m.Positions = make([]geom.Vec3, 0, n)
	for range n {
		m.Positions = append(m.Positions, geom.Vec3{X: r.F32(), Y: r.F32(), Z: r.F32()})
		r.Skip(12) // normal
	}
	r.U32() // revision
	// FColor is stored B, G, R, A: alpha is the top byte of the u32.
	n = readCount(r, "cores")
	for k := range n {
		a := uint8(r.U32() >> 24)
		if a != 255 && m.Alpha == nil {
			m.Alpha = make([]uint8, k, n)
			for j := range m.Alpha {
				m.Alpha[j] = 255
			}
		}
		if m.Alpha != nil {
			m.Alpha = append(m.Alpha, a)
		}
	}
	r.U32()
	n = readCount(r, "cores alfa")
	r.Skip(n * 4)
	r.U32()
	streams := readCount(r, "streams de UV")
	for s := range streams {
		n = readCount(r, "UVs")
		if s == 0 {
			m.UVs = make([][2]float32, 0, n)
			for range n {
				m.UVs = append(m.UVs, [2]float32{r.F32(), r.F32()})
			}
		} else {
			r.Skip(n * 8)
		}
		r.U32()
		r.U32()
	}
	n = readCount(r, "índices")
	m.Indices = make([]uint16, 0, n)
	for range n {
		m.Indices = append(m.Indices, r.U16())
	}
	r.U32()
	// The wireframe index stream, its revision and CollisionModel follow;
	// nothing drawn needs them.
	if err := r.Err(); err != nil {
		return nil, fmt.Errorf("StaticMesh %s: %w", name, err)
	}
	for k, s := range m.Sections {
		if s.FirstIndex+3*s.Triangles > len(m.Indices) {
			return nil, fmt.Errorf("StaticMesh %s: seção %d passa do fim de %s", name, k, inflect.Count(len(m.Indices), "índice", "índices"))
		}
	}
	return m, nil
}

// Triangle is the three vertex indices (into Positions) of triangle t of
// section s, in the reversed (i2, i1, i0) order UE2-Studio draws them in;
// false when a vertex index is out of bounds (UE2-Studio drops such a
// corner; dropping the triangle keeps the rest aligned).
func (m *StaticMesh) Triangle(s MeshSection, t int) ([3]uint16, bool) {
	k := s.FirstIndex + 3*t
	tri := [3]uint16{m.Indices[k+2], m.Indices[k+1], m.Indices[k]}
	for _, v := range tri {
		if int(v) >= len(m.Positions) {
			return [3]uint16{}, false
		}
	}
	return tri, true
}
