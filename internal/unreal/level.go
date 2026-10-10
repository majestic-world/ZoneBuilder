package unreal

import (
	"fmt"

	"zonebuilder/internal/l2pkg"
)

// Level is a map's Level object: the actors Unreal instantiates and the
// BSP model. Port of UE2-Studio's Level::read (src/unreal/objects.rs).
type Level struct {
	// Actors are object references (positive = local export ref-1).
	Actors []int32
	// Model is the object reference of the BSP Model.
	Model int32
}

// FindLevel is the 0-based index of the first Level export of p, or -1.
func FindLevel(p *l2pkg.Package) int {
	for i := range p.Exports {
		if p.Exports[i].ClassName == "Level" {
			return i
		}
	}
	return -1
}

// ReadLevel decodes export i (0-based) of p as a Level. Licensees above 20
// serialize a second actor array, which replaces the first.
func ReadLevel(p *l2pkg.Package, i int) (*Level, error) {
	r, err := p.ExportReader(i)
	if err != nil {
		return nil, err
	}
	if _, err := l2pkg.ReadProperties(p, r, p.Exports[i].Flags); err != nil {
		return nil, fmt.Errorf("Level: %w", err)
	}
	l := &Level{Actors: readLevelObjects(r)}
	if p.Header.LicenseeVersion > 20 {
		l.Actors = readLevelObjects(r)
	}
	for range 4 { // URL Protocol, Host, Map, Portal
		r.SkipString()
	}
	opts := readCount(r, "opções da URL")
	for range opts {
		r.SkipString()
	}
	r.I32() // URL Port
	r.U8()  // URL Valid
	r.Skip(2)
	reach := readCount(r, "reach specs")
	for range reach {
		r.I32()
		r.Index()
		r.Index()
		r.I32()
		r.I32()
		r.I32()
		r.U8()
	}
	l.Model = r.Index()
	if err := r.Err(); err != nil {
		return nil, fmt.Errorf("Level: %w", err)
	}
	return l, nil
}

func readLevelObjects(r *l2pkg.Reader) []int32 {
	n, dup := r.I32(), r.I32()
	if r.Err() != nil {
		return nil
	}
	if n < 0 || n > maxCount || dup != n {
		r.Fail(fmt.Errorf("array de objetos do Level com contagens %d e %d", n, dup))
		return nil
	}
	out := make([]int32, 0, n)
	for range n {
		out = append(out, r.Index())
	}
	return out
}

// maxCount is UE2-Studio's check_count limit on serialized array counts.
const maxCount = 10_000_000

// readCount reads a compact array count, failing r when it is out of range.
func readCount(r *l2pkg.Reader, what string) int {
	n := r.Index()
	if r.Err() == nil && (n < 0 || n > maxCount) {
		r.Fail(fmt.Errorf("contagem inválida de %s: %d", what, n))
	}
	if r.Err() != nil {
		return 0
	}
	return int(n)
}

// PolyFlags bits of a BSP surface (FBspSurf::PolyFlags).
const (
	PFInvisible    = 0x0000_0001
	PFMasked       = 0x0000_0002
	PFTranslucent  = 0x0000_0004
	PFNotSolid     = 0x0000_0008
	PFModulated    = 0x0000_0040
	PFFakeBackdrop = 0x0000_0080
	PFPortal       = 0x0400_0000
	PFAntiPortal   = 0x2000_0000
	// PFNotVisible are editor helpers rather than visible geometry. Collision
	// flags such as NotSolid are absent on purpose: water stays visible.
	PFNotVisible = PFInvisible | PFFakeBackdrop | PFPortal | PFAntiPortal
)

// Model is a BSP model (Level.Model). Positions are world space, Unreal basis.
type Model struct {
	Vectors, Points [][3]float32
	Nodes           []BSPNode
	Surfs           []BSPSurf
	Verts           []BSPVert
}

// BSPNode is the part of an FBspNode the geometry needs: a convex polygon
// of NumVertices entries of Verts starting at VertPool.
type BSPNode struct {
	VertPool    int32
	Surf        int32
	NumVertices uint8
}

// BSPSurf is the part of an FBspSurf the geometry needs.
type BSPSurf struct {
	// Material is an object reference in the map's index space; 0 is none.
	Material  int32
	PolyFlags uint32
	// Base, Normal, TextureU and TextureV index Points/Vectors: the UV
	// basis (texels) and the plane normal.
	Base, Normal, TextureU, TextureV int32
}

// BSPVert is a Verts entry: an index into Points.
type BSPVert struct {
	Point int32
}

// ReadModel decodes export i (0-based) of p as a Model. Port of
// UE2-Studio's Model::read: only the byte widths of the skipped fields
// matter. Licensees above 20 add a u32 to every surface.
func ReadModel(p *l2pkg.Package, i int) (*Model, error) {
	if cls := p.Exports[i].ClassName; cls != "Model" {
		return nil, fmt.Errorf("export %s é %s, não Model", p.Exports[i].ObjectName, cls)
	}
	r, err := p.ExportReader(i)
	if err != nil {
		return nil, err
	}
	if _, err := l2pkg.ReadProperties(p, r, p.Exports[i].Flags); err != nil {
		return nil, fmt.Errorf("Model: %w", err)
	}
	r.Skip(41) // UPrimitive tail: bounding box and sphere
	m := &Model{}
	m.Vectors = readVectors(r, "vectors")
	m.Points = readVectors(r, "points")
	n := readCount(r, "nós BSP")
	m.Nodes = make([]BSPNode, 0, n)
	for range n {
		r.Skip(16) // plane
		r.U64()
		r.U8() // node flags
		node := BSPNode{VertPool: r.Index(), Surf: r.Index()}
		for range 5 {
			r.Index()
		}
		r.Skip(12) // vector
		r.I32()
		r.U64()
		r.U64()
		r.Index()
		r.Index()
		node.NumVertices = r.U8()
		r.I32()
		r.I32()
		r.Skip(12)
		m.Nodes = append(m.Nodes, node)
	}
	extra := p.Header.LicenseeVersion > 20
	n = readCount(r, "superfícies BSP")
	m.Surfs = make([]BSPSurf, 0, n)
	for range n {
		s := BSPSurf{Material: r.Index(), PolyFlags: r.U32()}
		s.Base, s.Normal, s.TextureU, s.TextureV = r.Index(), r.Index(), r.Index(), r.Index()
		r.Index()
		r.Index()
		r.Skip(16)
		r.U32()
		if extra {
			r.U32()
		}
		m.Surfs = append(m.Surfs, s)
	}
	n = readCount(r, "vértices BSP")
	m.Verts = make([]BSPVert, 0, n)
	for range n {
		m.Verts = append(m.Verts, BSPVert{Point: r.Index()})
		r.Index() // side
	}
	if err := r.Err(); err != nil {
		return nil, fmt.Errorf("Model: %w", err)
	}
	return m, nil
}

func readVectors(r *l2pkg.Reader, what string) [][3]float32 {
	n := readCount(r, what)
	out := make([][3]float32, 0, n)
	for range n {
		out = append(out, [3]float32{r.F32(), r.F32(), r.F32()})
	}
	return out
}

// Polygon is one BSP node's convex polygon. Points is reused between
// visits: copy it to keep it.
type Polygon struct {
	Surf   int
	Points [][3]float32
}

// Polygons visits every node polygon of m that has at least 3 points, in
// node order, whatever its surface's flags: a brush's Model is the
// volume's whole shape. Out-of-range indices are errors.
func (m *Model) Polygons(visit func(Polygon)) error {
	var pts [][3]float32
	for k, node := range m.Nodes {
		if node.Surf < 0 || int(node.Surf) >= len(m.Surfs) {
			return fmt.Errorf("nó BSP %d: superfície %d fora do intervalo", k, node.Surf)
		}
		start, count := int(node.VertPool), int(node.NumVertices)
		if start < 0 || start+count > len(m.Verts) {
			return fmt.Errorf("nó BSP %d: vértices %d+%d fora do intervalo (%d)", k, start, count, len(m.Verts))
		}
		pts = pts[:0]
		for _, v := range m.Verts[start : start+count] {
			if v.Point < 0 || int(v.Point) >= len(m.Points) {
				return fmt.Errorf("nó BSP %d: ponto %d fora do intervalo", k, v.Point)
			}
			pts = append(pts, m.Points[v.Point])
		}
		if len(pts) < 3 {
			continue
		}
		if s := m.Surfs[node.Surf].Normal; s < 0 || int(s) >= len(m.Vectors) {
			return fmt.Errorf("superfície BSP %d: normal %d fora do intervalo", node.Surf, s)
		}
		visit(Polygon{Surf: int(node.Surf), Points: pts})
	}
	return nil
}

// VisiblePolygons is Polygons without the polygons of invisible surfaces
// (PFNotVisible): the input of UE2-Studio's Model::visual_surfaces, which
// fans each one (0, i-1, i) and groups them by surface.
func (m *Model) VisiblePolygons(visit func(Polygon)) error {
	return m.Polygons(func(p Polygon) {
		if m.Surfs[p.Surf].PolyFlags&PFNotVisible == 0 {
			visit(p)
		}
	})
}
