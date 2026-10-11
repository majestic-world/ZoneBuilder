package render

import (
	"slices"

	"zonebuilder/internal/render/gles"
	"zonebuilder/internal/scene"
)

// highlight is the static mesh actor drawn tinted over the scene, the
// target of the mesh menu while it is open. Its buffers are built the
// first frame its scene is uploaded and freed when the target changes,
// the highlight turns off or the scene leaves.
type highlight struct {
	key scene.ActorKey
	on  bool
	// gs is the scene the buffers were built from, nil until built.
	gs      *gpuScene
	batches []highlightBatch
}

// highlightBatch draws the actor's triangles of one mesh batch: an index
// buffer of its sections' ranges, in a vertex array over the batch's
// vertex buffer.
type highlightBatch struct {
	// batch is the index in gs.batches.
	batch    int
	vao, ibo uint32
	count    int
}

// sectionRanges are, per scene batch, the index ranges of the sections of
// the actors want accepts, sorted by first index.
func sectionRanges(actors []scene.MeshActor, want func(scene.ActorKey) bool) map[int][]indexRange {
	ranges := map[int][]indexRange{}
	for i := range actors {
		a := &actors[i]
		if !want(a.Key()) {
			continue
		}
		for _, sec := range a.Sections {
			if sec.Batch >= 0 && sec.Count > 0 {
				ranges[sec.Batch] = append(ranges[sec.Batch], indexRange{sec.First, sec.Count})
			}
		}
	}
	for _, r := range ranges {
		slices.SortFunc(r, func(x, y indexRange) int { return x.first - y.first })
	}
	return ranges
}

// gatherRanges are the indices of src within ranges, in range order.
func gatherRanges(src []uint32, ranges []indexRange) []uint32 {
	n := 0
	for _, r := range ranges {
		n += r.count
	}
	out := make([]uint32, 0, n)
	for _, r := range ranges {
		out = append(out, src[r.first:r.first+r.count]...)
	}
	return out
}

// set highlights the actor key (on) or nothing; a change frees the
// buffers of the previous target.
func (h *highlight) set(key scene.ActorKey, on bool) {
	if key == h.key && on == h.on {
		return
	}
	h.free()
	h.key, h.on = key, on
}

// free deletes the buffers; they are built again on the next draw that
// needs them.
func (h *highlight) free() {
	for _, b := range h.batches {
		gles.DeleteVertexArray(b.vao)
		gles.DeleteBuffer(b.ibo)
	}
	h.gs, h.batches = nil, nil
}

// build makes the buffers from the first uploaded scene that has the
// actor; none found leaves gs nil, to try again next frame.
func (h *highlight) build(scenes []*gpuScene) {
	for _, gs := range scenes {
		if gs.prep != nil {
			continue
		}
		ranges := sectionRanges(gs.scene.Actors, func(k scene.ActorKey) bool { return k == h.key })
		if len(ranges) == 0 {
			continue
		}
		for i, b := range gs.batches {
			r := ranges[b.source]
			if !b.mesh || len(r) == 0 {
				continue
			}
			indices := gatherRanges(gs.scene.Batches[b.source].Indices, r)
			hb := highlightBatch{batch: i, vao: gles.GenVertexArray(), ibo: gles.GenBuffer(), count: len(indices)}
			gles.BindVertexArray(hb.vao)
			gles.BindBuffer(gles.ARRAY_BUFFER, b.vbo)
			gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, hb.ibo)
			gles.BufferData(gles.ELEMENT_ARRAY_BUFFER, indices, gles.STATIC_DRAW)
			vertexAttribs()
			gles.BindVertexArray(0)
			gles.BindBuffer(gles.ARRAY_BUFFER, 0)
			gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, 0)
			h.batches = append(h.batches, hb)
		}
		h.gs = gs
		return
	}
}

// drawHighlight redraws the highlighted actor's triangles tinted, each
// batch in its render mode but without writing depth. The depth-writing
// modes (Opaque, Masked) test EQUAL, so only the fragments that won the
// depth test in the scene passes are tinted, with no z-fighting; the
// blended modes write no depth to match and keep their pass's test. Called
// by draw with the scene program in use; it leaves uHighlight off and the
// textures and vertex array bound.
func (sr *sceneRenderer) drawHighlight() {
	h := &sr.highlight
	if !h.on || sr.hideMeshes || sr.hidden.Has(h.key) {
		return
	}
	if h.gs == nil {
		h.build(sr.scenes)
		if h.gs == nil {
			return
		}
	}
	gles.Uniform1i(sr.highlightOn, 1)
	for _, hb := range h.batches {
		b := h.gs.batches[hb.batch]
		passState(b.mode)
		if b.mode == scene.Opaque || b.mode == scene.Masked {
			gles.DepthFunc(gles.EQUAL)
		}
		gles.DepthMask(false)
		gles.Uniform1i(sr.mode, int32(b.mode))
		gles.Uniform1i(sr.opaque, boolInt(b.opaque))
		gles.ActiveTexture(gles.TEXTURE1)
		gles.BindTexture(gles.TEXTURE_2D, b.mask)
		gles.ActiveTexture(gles.TEXTURE0)
		gles.BindTexture(gles.TEXTURE_2D, b.texture)
		gles.BindVertexArray(hb.vao)
		gles.DrawElements(gles.TRIANGLES, hb.count, gles.UNSIGNED_INT, 0)
		sr.stats.Draws++
		sr.stats.Triangles += hb.count / 3
	}
	gles.Uniform1i(sr.highlightOn, 0)
}
