package render

import (
	"math"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/texture"
)

// sectorSpan is the side of a sector, the unit of frustum culling: a
// quarter of a tile, so a tile's batches split into at most 4×4 draw ranges.
const sectorSpan = scene.TileSpan / 4

// Prepared is a scene made ready for the GPU off the context thread
// (Prepare): its index lists regrouped by sector and the textures that need
// decoding decoded, so that the upload on the context thread only copies
// buffers and pixels.
type Prepared struct {
	scene    *scene.Scene
	sets     []indexSet
	batches  []preparedBatch
	textures []preparedTexture
}

// indexSet is the index list of one or more batches (a terrain's layers
// share theirs) with its triangles grouped by sector: every sector's
// triangles are one contiguous range, in the scene's order within it.
type indexSet struct {
	indices []uint32
	sectors []sector
}

// sector is one range of an index set: indices [first, first+count) and
// the world box of their vertices.
type sector struct {
	first, count int
	bounds       geom.Box
}

// preparedBatch is one scene batch: its vertices, the index set it draws,
// and its textures by cache key.
type preparedBatch struct {
	mode          scene.RenderMode
	opaque        bool
	mesh          bool
	vertices      []scene.Vertex
	set           int
	texture, mask textureKey
	// source is the batch's index in the scene's Batches.
	source int
}

// textureKey names one GL texture of the cache: a texture export (its
// path, the same in every scene that uses it) in one role. A key with
// neither path nor own is the stand-in of its role, for a batch without
// that texture.
type textureKey struct {
	path string
	// own identifies a texture without a path, which no other scene
	// shares.
	own  *texture.Texture
	role textureRole
}

type textureRole uint8

const (
	// roleMaterial is a material texture: sRGB, mipmapped.
	roleMaterial textureRole = iota + 1
	// roleMasked is a material texture prepared for the alpha cutout.
	roleMasked
	// roleMask is a terrain layer's alpha map: linear, one level.
	roleMask
)

// preparedTexture is what uploading one texture needs: the package's DXT
// levels as stored (compressed set), or the levels decoded on the CPU.
// Neither means it cannot be drawn and the stand-in is used.
type preparedTexture struct {
	key        textureKey
	t          *texture.Texture
	compressed bool
	levels     []*texture.Image
}

// Prepare regroups s's batches by sector and decodes the textures the GPU
// cannot take as stored. It does no GL call and may run on any goroutine;
// s must not change meanwhile.
func Prepare(s *scene.Scene) *Prepared {
	p := &Prepared{scene: s}
	sets := map[*uint32]int{}
	textures := map[textureKey]bool{}
	for i := range s.Batches {
		b := &s.Batches[i]
		if len(b.Indices) == 0 {
			continue
		}
		set, ok := sets[&b.Indices[0]]
		if !ok {
			set = len(p.sets)
			sets[&b.Indices[0]] = set
			p.sets = append(p.sets, sectorize(b.Vertices, b.Indices))
		}
		pb := preparedBatch{mode: b.Mode, opaque: b.OpaqueTexture, mesh: b.Mesh, vertices: b.Vertices, set: set, source: i}
		role := roleMaterial
		if b.Mode == scene.Masked && !b.OpaqueTexture {
			role = roleMasked
		}
		pb.texture = p.texture(textures, b.Texture, role)
		pb.mask = p.texture(textures, b.Mask, roleMask)
		p.batches = append(p.batches, pb)
	}
	return p
}

// texture is the cache key of t in role, preparing t the first time; nil
// is the role's stand-in.
func (p *Prepared) texture(seen map[textureKey]bool, t *texture.Texture, role textureRole) textureKey {
	if t == nil {
		return textureKey{role: role}
	}
	k := textureKey{path: t.Path, role: role}
	if t.Path == "" {
		k.own = t
	}
	if seen[k] {
		return k
	}
	seen[k] = true
	pt := preparedTexture{key: k, t: t}
	_, isDXT := dxtFormat(t.Format, true)
	switch {
	case isDXT && role != roleMasked && nativeDXT(t):
		pt.compressed = true
	default:
		img, err := t.RGBA()
		if err != nil {
			break
		}
		switch role {
		case roleMaterial:
			pt.levels = img.Mipmaps()
		case roleMasked:
			pt.levels = img.MaskedMipmaps()
		case roleMask:
			pt.levels = []*texture.Image{img}
		}
	}
	p.textures = append(p.textures, pt)
	return k
}

// sectorize groups the triangles of indices by the sector their centroid
// falls in.
func sectorize(vertices []scene.Vertex, indices []uint32) indexSet {
	tris := len(indices) / 3
	ids := map[[2]int32]int{}
	var keys [][2]int32
	of := make([]int32, tris)
	for t := range tris {
		a, b, c := vertices[indices[3*t]].Pos, vertices[indices[3*t+1]].Pos, vertices[indices[3*t+2]].Pos
		k := [2]int32{
			int32(math.Floor(float64((a.X + b.X + c.X) / (3 * sectorSpan)))),
			int32(math.Floor(float64((a.Y + b.Y + c.Y) / (3 * sectorSpan)))),
		}
		id, ok := ids[k]
		if !ok {
			id = len(keys)
			ids[k] = id
			keys = append(keys, k)
		}
		of[t] = int32(id)
	}
	set := indexSet{indices: make([]uint32, 0, 3*tris), sectors: make([]sector, len(keys))}
	counts := make([]int, len(keys))
	for _, id := range of {
		counts[id]++
	}
	next := make([]int, len(keys))
	first := 0
	for id, n := range counts {
		set.sectors[id] = sector{first: first, count: 3 * n, bounds: geom.EmptyBox()}
		next[id] = first
		first += 3 * n
	}
	set.indices = set.indices[:3*tris]
	for t, id := range of {
		sec := &set.sectors[id]
		at := next[id]
		for k := range 3 {
			v := indices[3*t+k]
			set.indices[at+k] = v
			sec.bounds.Include(vertices[v].Pos)
		}
		next[id] = at + 3
	}
	return set
}
