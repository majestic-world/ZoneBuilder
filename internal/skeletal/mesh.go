package skeletal

import (
	"fmt"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
)

// Mesh is a SkeletalMesh's reference skeleton and its LOD 0, ready to skin:
// UE2-Studio's skin::build without the bind pose and the base transform,
// which are internal/model's math applied to Bones, Scale, Origin and
// Rotation.
type Mesh struct {
	// Bones is the reference skeleton in package order; a parent always
	// precedes its child.
	Bones []Bone
	// Scale, Origin and Rotation are ULodMesh's MeshScale, MeshOrigin and
	// RotOrigin. The mesh-to-actor base transform is
	// axis = rotator_to_axis(Rotation) with row k scaled by Scale[k], and
	// origin = axis applied to -Origin (skin.rs base_transform).
	Scale, Origin geom.Vec3
	Rotation      l2pkg.Rotator
	// Animation is the mesh's default MeshAnimation, an object reference in
	// the mesh package's index space; 0 when it names none.
	Animation int32

	// Vertices holds one vertex per wedge of LOD 0, in the mesh's own space.
	Vertices []Vertex
	// Indices are triangles over Vertices, wound for the renderer (Unreal's
	// winding reversed), grouped by section.
	Indices []uint32
	// Sections has one entry per material slot, in slot order.
	Sections []Section
	// Unbound counts vertices whose point had no influence and were bound
	// to bone 0 with full weight: the file disagreeing with itself.
	Unbound int
}

// Bone is one FMeshBone of the reference skeleton.
type Bone struct {
	Name string
	// Orientation is the stored quaternion x, y, z, w. Bone 0's is used
	// conjugated by the bind and sampled poses (skin.rs bind_pose); it is
	// kept here as the file wrote it.
	Orientation [4]float32
	Position    geom.Vec3
	Length      float32
	Size        geom.Vec3
	// Parent is the parent bone's index; -1 for bone 0.
	Parent int
}

// Influences is how many weighted bones a vertex carries.
const Influences = 4

// Vertex is one skinned wedge.
type Vertex struct {
	Position geom.Vec3
	// Normal is unit length.
	Normal geom.Vec3
	UV     [2]float32
	// Bones and Weights are the point's influences, heaviest first,
	// renormalised so the weights sum to 1. Unused slots repeat Bones[0]
	// with weight 0.
	Bones   [Influences]uint16
	Weights [Influences]float32
}

// Section is a run of Indices drawn with one material slot.
type Section struct {
	FirstIndex, IndexCount int
	PolyFlags              uint32
	// Material is the slot's skin, Textures[Materials[slot].TextureIndex], as
	// an object reference in the mesh package's index space (resolve it with
	// l2pkg.Client.Resolve); 0 when the slot names none.
	Material int32
}

// maxBones is the renderer's palette size (skin.rs MAX_BONES).
const maxBones = 1024

// primitiveTail is UPrimitive's bounding box (min, max, valid byte) and
// sphere after the property list.
const primitiveTail = 25 + 16

// material is one ULodMesh Materials entry.
type material struct {
	polyFlags    uint32
	textureIndex int32
}

// ReadMesh decodes export i (0-based) of p as a SkeletalMesh. Port of
// skeletal.rs SkeletalMesh::read and skin.rs build at LOD 0.
func ReadMesh(p *l2pkg.Package, i int) (*Mesh, error) {
	if err := checkVersion(p); err != nil {
		return nil, err
	}
	r, err := p.ExportReader(i)
	if err != nil {
		return nil, err
	}
	m := &Mesh{}
	var lod *lodModel
	fail := func(err error) (*Mesh, error) {
		return nil, fmt.Errorf("SkeletalMesh %s: %w", p.Exports[i].ObjectName, err)
	}
	if _, err := l2pkg.ReadProperties(p, r, p.Exports[i].Flags); err != nil {
		return fail(err)
	}
	r.Skip(primitiveTail)
	textures, materials := readLodMesh(r, m)
	skipArray(r, "Points2", 12)
	m.Bones = make([]Bone, count(r, "ossos"))
	for k := range m.Bones {
		if r.Err() != nil {
			break
		}
		m.Bones[k] = readBone(p, r)
	}
	m.Animation = r.Index()
	r.I32() // SkeletalDepth
	for range count(r, "WeightIndices") {
		skipArray(r, "WeightIndices", 2)
		r.I32()
	}
	skipArray(r, "BoneInfluences", 4)
	for range count(r, "AttachAliases") {
		r.Index()
	}
	for range count(r, "AttachBoneNames") {
		r.Index()
	}
	skipArray(r, "AttachCoords", 48)
	// Every LOD model is serialized in sequence, so all are walked; only
	// LOD 0 is kept.
	for k := range count(r, "LODs") {
		if r.Err() != nil {
			break
		}
		if l := readLodModel(r); k == 0 {
			lod = l
		}
	}
	// The top-level geometry: the base mesh UE2-Studio falls back to when a
	// LOD is unusable. Walked for the end-of-export check, not kept.
	r.Index() // f224
	lazyArray(r, "Points", func() { r.Skip(12) })
	lazyArray(r, "Wedges", func() { r.Skip(10) })
	lazyArray(r, "Triangles", func() { r.Skip(12) })
	lazyArray(r, "VertInfluences", func() { r.Skip(8) })
	lazyArray(r, "CollapseWedgeThus", func() { r.Skip(2) })
	lazyArray(r, "f1C8", func() { r.Skip(2) })
	// Lineage tail: [ArVer>=118 && lic>=3] i32, [ArVer>=123 && lic>=0x12]
	// TArray<f32>, [ArVer>=120] AuthKey, [lic>=0x23] i32.
	r.I32()
	skipArray(r, "tail", 4)
	r.I32()
	r.I32()
	checkEnd(p, r, i)
	if err := r.Err(); err != nil {
		return fail(err)
	}

	if err := checkSkeleton(m.Bones); err != nil {
		return fail(err)
	}
	if lod == nil {
		return fail(fmt.Errorf("a mesh não tem LOD"))
	}
	raw, err := reassemble(lod)
	if err != nil {
		return fail(fmt.Errorf("LOD 0: %w", err))
	}
	if m.Vertices, m.Unbound, err = convertWedges(raw, len(m.Bones)); err != nil {
		return fail(fmt.Errorf("LOD 0: %w", err))
	}
	if m.Indices, m.Sections, err = buildIndices(raw, textures, materials); err != nil {
		return fail(fmt.Errorf("LOD 0: %w", err))
	}
	generateNormals(m.Vertices, m.Indices)
	return m, nil
}

// readLodMesh reads the ULodMesh part (skeletal.rs read_lod_mesh) into m and
// returns its Textures and Materials.
func readLodMesh(r *l2pkg.Reader, m *Mesh) ([]int32, []material) {
	lodVersion := r.I32()
	if r.Err() == nil && lodVersion <= 1 {
		r.Fail(fmt.Errorf("LodVersion %d não suportada", lodVersion))
		return nil, nil
	}
	r.I32()                  // VertexCount
	skipArray(r, "Verts", 4) // ArVer < 133: u32 elements
	textures := make([]int32, count(r, "Textures"))
	for k := range textures {
		textures[k] = r.Index()
	}
	m.Scale, m.Origin = vector(r), vector(r)
	m.Rotation = l2pkg.Rotator{Pitch: r.I32(), Yaw: r.I32(), Roll: r.I32()}
	skipArray(r, "FaceLevel", 2)
	skipArray(r, "Faces", 8)
	skipArray(r, "CollapseWedgeThus", 2)
	skipArray(r, "Wedges", 10)
	materials := make([]material, count(r, "Materials"))
	for k := range materials {
		materials[k] = material{polyFlags: r.U32(), textureIndex: r.I32()}
	}
	// ScaleMax, LODHysteresis, LODStrength, LODMinVerts, LODMorph,
	// LODZDisplace.
	r.Skip(6 * 4)
	if lodVersion >= 3 {
		// Impostor: present, SpriteMaterial, location, rotation, scale,
		// color, SpaceMode, DrawMode, LightMode.
		r.I32()
		r.Index()
		r.Skip(12 + 12 + 12 + 4 + 3*4)
	}
	if lodVersion >= 4 {
		r.F32() // SkinTesselationFactor
	}
	// Lineage tail, licensee >= 1 and LodVersion >= 5.
	if lodVersion >= 5 {
		r.I32()
	}
	if lodVersion >= 6 {
		r.U8()
	}
	if lodVersion >= 7 {
		r.U8()
	}
	return textures, materials
}

// readBone reads one FMeshBone: NumChildren comes before ParentIndex.
func readBone(p *l2pkg.Package, r *l2pkg.Reader) Bone {
	b := Bone{Name: name(p, r)}
	r.U32() // Flags
	b.Orientation = [4]float32{r.F32(), r.F32(), r.F32(), r.F32()}
	b.Position, b.Length, b.Size = vector(r), r.F32(), vector(r)
	r.I32() // NumChildren
	b.Parent = int(r.I32())
	return b
}

// checkSkeleton is skin.rs build_skeleton's verification: at least one bone,
// no more than the palette holds, and every parent before its child. Bone 0's
// stored parent is replaced by -1.
func checkSkeleton(bones []Bone) error {
	if len(bones) == 0 {
		return fmt.Errorf("a mesh não tem ossos")
	}
	if len(bones) > maxBones {
		return fmt.Errorf("%d ossos passam do limite de %d", len(bones), maxBones)
	}
	bones[0].Parent = -1
	for k, b := range bones[1:] {
		if b.Parent < 0 || b.Parent > k {
			return fmt.Errorf("o osso %q (%d) tem pai %d, que não vem antes dele", b.Name, k+1, b.Parent)
		}
	}
	return nil
}
