package skeletal

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/l2pkg"
)

// weightEpsilon is the smallest influence weight that counts (skin.rs
// WEIGHT_EPSILON).
const weightEpsilon = 1e-6

// noBone marks an unused Lineage wedge bone slot.
const noBone = 255

// section is one FSkelMeshSection of a LOD model.
type section struct {
	material         uint16
	firstFace, faces uint16
	// boneMap maps a Lineage wedge's local bone slot to a skeleton bone.
	boneMap []int32
}

type wedge struct {
	point uint16
	uv    [2]float32
}

type face struct {
	wedges   [3]uint16
	material uint16
}

type influence struct {
	weight      float32
	point, bone uint16
}

type lineageWedge struct {
	point, normal geom.Vec3
	uv            [2]float32
	bones         [Influences]uint8
	weights       [Influences]float32
}

// lodModel is the part of one FStaticLODModel the assembly reads.
type lodModel struct {
	soft          []section
	rigidSections int
	softIndices   []uint16
	// stream counts the rigid vertex stream's entries.
	stream int
	// The classic arrays: points, wedges naming them, faces naming wedges,
	// and per-point influences.
	influences []influence
	wedges     []wedge
	faces      []face
	points     []geom.Vec3
	lineage    []lineageWedge
}

// readLodModel reads one FStaticLODModel (skeletal.rs read_lod_model).
func readLodModel(r *l2pkg.Reader) *lodModel {
	l := &lodModel{}
	skipArray(r, "SkinningData", 4)
	skipArray(r, "SkinPoints", 16)
	r.I32() // NumSoftWedges
	l.soft = readSections(r)
	l.rigidSections = len(readSections(r))
	l.softIndices = make([]uint16, count(r, "SoftIndices"))
	for k := range l.softIndices {
		l.softIndices[k] = r.U16()
	}
	r.I32() // revision
	skipArray(r, "RigidIndices", 2)
	r.I32()       // revision
	r.Skip(3 * 4) // FSkinVertexStream header
	l.stream = skipArray(r, "SkinVertexStream", 32)
	lazyArray(r, "VertInfluences", func() {
		l.influences = append(l.influences, influence{weight: r.F32(), point: r.U16(), bone: r.U16()})
	})
	lazyArray(r, "Wedges", func() {
		l.wedges = append(l.wedges, wedge{point: r.U16(), uv: [2]float32{r.F32(), r.F32()}})
	})
	lazyArray(r, "Faces", func() {
		l.faces = append(l.faces, face{wedges: [3]uint16{r.U16(), r.U16(), r.U16()}, material: r.U16()})
	})
	lazyArray(r, "Points", func() { l.points = append(l.points, vector(r)) })
	// LODDistanceFactor, LODHysteresis, NumSharedVerts, LODMaxInfluences,
	// f114, f118, bUseNewWedges (licensee >= 0x1C).
	r.Skip(7 * 4)
	l.lineage = make([]lineageWedge, count(r, "LineageWedges"))
	for k := range l.lineage {
		w := &l.lineage[k]
		w.point, w.normal = vector(r), vector(r)
		w.uv = [2]float32{r.F32(), r.F32()}
		w.bones = [4]uint8{r.U8(), r.U8(), r.U8(), r.U8()}
		w.weights = [4]float32{r.F32(), r.F32(), r.F32(), r.F32()}
	}
	return l
}

// readSections reads a TArray of sections: nine u16 fields, then (licensee
// >= 0x1C) the Lineage bone map.
func readSections(r *l2pkg.Reader) []section {
	out := make([]section, count(r, "seções"))
	for k := range out {
		s := &out[k]
		s.material = r.U16()
		r.Skip(4 * 2) // MinStreamIndex, MinWedgeIndex, MaxWedgeIndex, NumStreamIndices
		r.Skip(2 * 2) // BoneIndex, fE
		s.firstFace, s.faces = r.U16(), r.U16()
		s.boneMap = make([]int32, count(r, "LineageBoneMap"))
		for b := range s.boneMap {
			s.boneMap[b] = r.I32()
		}
	}
	return out
}

// rawMesh is a LOD in the classic shape: points with optional normals,
// wedges naming points, faces naming wedges, influences naming points.
type rawMesh struct {
	points     []geom.Vec3
	normals    []geom.Vec3 // parallel to points, or empty
	wedges     []wedge
	faces      []face
	influences []influence
}

// reassemble is skin.rs reassemble, the port of FStaticLODModel::
// RestoreLineageMesh, for the soft path.
//
// Its guard decides which wedges the mesh is drawn from: a LOD whose classic
// lazy arrays are populated is used as it stands, and only a LOD without them
// is rebuilt from the Lineage wedges. The death knight's LOD 0 carries both
// (2741 classic wedges and 2741 Lineage wedges), so UE2-Studio draws it from
// the classic arrays with generated normals; following the same guard keeps
// this port's vertices identical to what UE2-Studio renders. Measured on that
// mesh, the Lineage path gives the same positions, UVs, influences and
// indices and differs only in the normals (the stored ones); it is what a
// LOD without classic arrays needs.
func reassemble(l *lodModel) (rawMesh, error) {
	if len(l.wedges) > 0 {
		if len(l.points) == 0 || len(l.influences) == 0 {
			return rawMesh{}, fmt.Errorf("o LOD tem wedges, mas não tem pontos ou influências")
		}
		return rawMesh{points: l.points, wedges: l.wedges, faces: l.faces, influences: l.influences}, nil
	}
	// The legacy SkinningData stream and the rigid vertex stream are other
	// Lineage layouts this reader does not take.
	if len(l.lineage) == 0 {
		return rawMesh{}, fmt.Errorf("o LOD não tem wedges clássicas nem do Lineage")
	}
	if l.stream > 0 || l.rigidSections > 0 {
		return rawMesh{}, fmt.Errorf("o LOD mistura wedges do Lineage e geometria rígida")
	}

	// Faces first: they also say which section owns each wedge, which the
	// influences need to resolve a bone.
	var out rawMesh
	owner := make([]int, len(l.lineage))
	for k := range owner {
		owner[k] = -1
	}
	for slot, s := range l.soft {
		end := (int(s.firstFace) + int(s.faces)) * 3
		if end > len(l.softIndices) {
			return rawMesh{}, fmt.Errorf("a seção %d quer %s, mas o buffer tem %d", slot, inflect.Count(end, "índice", "índices"), len(l.softIndices))
		}
		for f := range int(s.faces) {
			at := (int(s.firstFace) + f) * 3
			w := [3]uint16{l.softIndices[at], l.softIndices[at+1], l.softIndices[at+2]}
			for _, x := range w {
				if int(x) >= len(l.lineage) {
					return rawMesh{}, fmt.Errorf("uma face aponta a wedge %d de %d", x, len(l.lineage))
				}
				owner[x] = slot
			}
			out.faces = append(out.faces, face{wedges: w, material: s.material})
		}
	}
	if len(out.faces) == 0 {
		return rawMesh{}, fmt.Errorf("o LOD não tem faces")
	}

	// Wedges weld onto points by the exact bits of position and normal; the
	// first wedge on a point gives it its influences.
	seen := map[[6]uint32]uint16{}
	for k, w := range l.lineage {
		key := [6]uint32{
			math.Float32bits(w.point.X), math.Float32bits(w.point.Y), math.Float32bits(w.point.Z),
			math.Float32bits(w.normal.X), math.Float32bits(w.normal.Y), math.Float32bits(w.normal.Z),
		}
		point, ok := seen[key]
		if !ok {
			if len(out.points) > math.MaxUint16 {
				return rawMesh{}, fmt.Errorf("o LOD precisa de mais de 65536 pontos")
			}
			point = uint16(len(out.points))
			seen[key] = point
			out.points = append(out.points, w.point)
			out.normals = append(out.normals, normalised(w.normal))
			if err := softInfluences(&out, w, point, owner[k], l.soft); err != nil {
				return rawMesh{}, err
			}
		}
		out.wedges = append(out.wedges, wedge{point: point, uv: w.uv})
	}
	return out, nil
}

// softInfluences appends the influences of a Lineage wedge's point. A bone
// slot indexes the owning section's bone map, not the skeleton; a wedge no
// face names has no section and therefore no influence.
func softInfluences(out *rawMesh, w lineageWedge, point uint16, owner int, sections []section) error {
	if owner < 0 {
		return nil
	}
	boneMap := sections[owner].boneMap
	for s := range Influences {
		bone, weight := w.bones[s], w.weights[s]
		if bone == noBone || weight < weightEpsilon {
			continue
		}
		if int(bone) >= len(boneMap) {
			return fmt.Errorf("uma wedge do Lineage aponta o osso local %d, e a seção mapeia %s", bone, inflect.Count(len(boneMap), "osso", "ossos"))
		}
		mapped := boneMap[bone]
		if mapped < 0 || mapped > math.MaxUint16 {
			return fmt.Errorf("o mapa de ossos de uma seção tem %d", mapped)
		}
		out.influences = append(out.influences, influence{weight: weight, point: point, bone: uint16(mapped)})
	}
	return nil
}

func normalised(v geom.Vec3) geom.Vec3 {
	l := v.Length()
	if l > 1e-6 {
		return geom.Vec3{X: v.X / l, Y: v.Y / l, Z: v.Z / l}
	}
	return geom.Vec3{}
}

// convertWedges makes one vertex per wedge with its point's influences packed
// to four (skin.rs convert_wedges). It returns how many vertices had no
// influence.
func convertWedges(raw rawMesh, bones int) ([]Vertex, int, error) {
	type weighted struct {
		weight float32
		bone   uint16
	}
	perPoint := make([][]weighted, len(raw.points))
	for _, in := range raw.influences {
		if int(in.point) >= len(raw.points) {
			return nil, 0, fmt.Errorf("uma influência aponta o ponto %d de %d", in.point, len(raw.points))
		}
		if int(in.bone) >= bones {
			return nil, 0, fmt.Errorf("uma influência aponta o osso %d de %d", in.bone, bones)
		}
		perPoint[in.point] = append(perPoint[in.point], weighted{in.weight, in.bone})
	}

	unbound := 0
	vertices := make([]Vertex, len(raw.wedges))
	for k, w := range raw.wedges {
		if int(w.point) >= len(raw.points) {
			return nil, 0, fmt.Errorf("uma wedge aponta o ponto %d de %d", w.point, len(raw.points))
		}
		v := &vertices[k]
		v.Position, v.UV = raw.points[w.point], w.uv
		if len(raw.normals) > 0 {
			v.Normal = raw.normals[w.point]
		}

		// Heaviest first, so the cap drops the smallest; stable, as Rust's
		// sort_by is.
		kept := make([]weighted, 0, len(perPoint[w.point]))
		for _, in := range perPoint[w.point] {
			if in.weight >= weightEpsilon {
				kept = append(kept, in)
			}
		}
		if len(kept) == 0 {
			v.Weights[0] = 1
			unbound++
			continue
		}
		slices.SortStableFunc(kept, func(a, b weighted) int { return cmp.Compare(b.weight, a.weight) })
		kept = kept[:min(len(kept), Influences)]
		var total float32
		for _, in := range kept {
			total += in.weight
		}
		for s := range Influences {
			v.Bones[s] = kept[0].bone
		}
		for s, in := range kept {
			v.Bones[s] = in.bone
			if total > 0 {
				v.Weights[s] = in.weight / total
			}
		}
		// The division's leftover goes on the heaviest bone, so the four
		// weights sum to exactly one.
		var sum float32
		for _, x := range v.Weights {
			sum += x
		}
		v.Weights[0] += 1 - sum
	}
	return vertices, unbound, nil
}

// buildIndices groups the faces into one section per material slot, at least
// one per Materials entry, and reverses each face's winding for the renderer
// (skin.rs build_indices).
func buildIndices(raw rawMesh, textures []int32, materials []material) ([]uint32, []Section, error) {
	slots := len(materials)
	for _, f := range raw.faces {
		slots = max(slots, int(f.material)+1)
	}
	buckets := make([][]uint32, slots)
	for _, f := range raw.faces {
		for _, w := range f.wedges {
			if int(w) >= len(raw.wedges) {
				return nil, nil, fmt.Errorf("uma face aponta a wedge %d de %d", w, len(raw.wedges))
			}
		}
		buckets[f.material] = append(buckets[f.material], uint32(f.wedges[2]), uint32(f.wedges[1]), uint32(f.wedges[0]))
	}
	indices := make([]uint32, 0, 3*len(raw.faces))
	sections := make([]Section, slots)
	for slot, bucket := range buckets {
		s := &sections[slot]
		s.FirstIndex, s.IndexCount = len(indices), len(bucket)
		if slot < len(materials) {
			m := materials[slot]
			s.PolyFlags = m.polyFlags
			if m.textureIndex >= 0 && int(m.textureIndex) < len(textures) {
				s.Material = textures[m.textureIndex]
			}
		}
		indices = append(indices, bucket...)
	}
	return indices, sections, nil
}

// generateNormals gives every vertex whose normal is zero the area-weighted
// sum of its faces' normals (skin.rs with_generated_normals).
func generateNormals(vertices []Vertex, indices []uint32) {
	missing := false
	for _, v := range vertices {
		if v.Normal == (geom.Vec3{}) {
			missing = true
			break
		}
	}
	if !missing {
		return
	}
	sums := make([]geom.Vec3, len(vertices))
	for t := 0; t+2 < len(indices); t += 3 {
		a, b, c := indices[t], indices[t+1], indices[t+2]
		// Not normalised: the cross product's length is twice the
		// triangle's area, the weight a vertex normal wants.
		pa := vertices[a].Position
		n := vertices[b].Position.Sub(pa).Cross(vertices[c].Position.Sub(pa))
		sums[a], sums[b], sums[c] = sums[a].Add(n), sums[b].Add(n), sums[c].Add(n)
	}
	for k := range vertices {
		if vertices[k].Normal == (geom.Vec3{}) {
			vertices[k].Normal = normalised(sums[k])
		}
	}
}
