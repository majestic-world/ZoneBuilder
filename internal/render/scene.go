package render

import (
	"fmt"
	"image"
	"slices"
	"time"
	"unsafe"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/render/gles"
	"zonebuilder/internal/scene"
)

// sceneVert takes absolute world positions (Unreal basis, Z up), subtracts
// the scene's rebase origin first, so the large map offset never enters a
// matrix product, then applies uViewProj, which includes the Y/Z swap into
// the Y-up render basis. gl_Position is invariant because terrain layers
// redraw the base's triangles from their own buffers and depth-test them
// with GEQUAL against it. vPos is the rebased position, for Water's view
// angle.
const sceneVert = `#version 300 es
layout(location = 0) in vec3 aPos;
layout(location = 1) in vec2 aUV;
layout(location = 2) in vec2 aMaskUV;
layout(location = 3) in float aAlpha;
uniform highp vec3 uOrigin;
uniform highp mat4 uViewProj;
out highp vec2 vUV;
out highp vec2 vMaskUV;
out highp vec3 vPos;
out mediump float vAlpha;
invariant gl_Position;
void main() {
	vUV = aUV;
	vMaskUV = aMaskUV;
	vAlpha = aAlpha;
	vPos = aPos - uOrigin;
	gl_Position = uViewProj * vec4(vPos, 1.0);
}
`

// sceneFrag is UE2-Studio's Textured view, unlit: the texture sample
// times the vertex alpha, shaded per render mode (uMode, the
// scene.RenderMode value) as gpu.rs's fs_main (Opaque, Brighten),
// fs_masked, fs_terrain_layer, fs_translucent (Translucent, Additive),
// fs_modulated, fs_water and fs_overlay. Water's normal is the triangle's,
// from the position derivatives: UE2-Studio's fresnel only reads its
// absolute cosine with the view. uOpaque takes the texture's alpha as 1.
// Sampling the sRGB texture yields linear colour, which the sRGB target
// encodes on write and blends in.
var sceneFrag = `#version 300 es
precision highp float;
uniform sampler2D uTexture;
uniform sampler2D uMask;
uniform int uMode;
uniform bool uOpaque;
uniform highp vec3 uEye;
in highp vec2 vUV;
in highp vec2 vMaskUV;
in highp vec3 vPos;
in mediump float vAlpha;
out vec4 oColor;
` + groundGLSL + `
void main() {
	vec4 s = texture(uTexture, vUV);
	if (uOpaque) {
		s.a = 1.0;
	}
	vec3 c = s.rgb;
	float a = vAlpha;
	if (uMode == ` + modeMasked + `) {
		if (s.a * vAlpha < 0.5) {
			discard;
		}
	} else if (uMode == ` + modeTerrainLayer + `) {
		a = texture(uMask, vMaskUV).r * s.a * vAlpha;
	} else if (uMode == ` + modeTranslucent + ` || uMode == ` + modeAdditive + `) {
		a = s.a * vAlpha;
	} else if (uMode == ` + modeModulated + `) {
		float coverage = max(max(s.r, s.g), s.b) * vAlpha;
		c = mix(vec3(1.0), s.rgb, coverage);
		a = 1.0;
	} else if (uMode == ` + modeWater + `) {
		vec3 n = normalize(cross(dFdx(vPos), dFdy(vPos)));
		vec3 v = normalize(uEye - vPos);
		float fresnel = pow(1.0 - abs(dot(n, v)), 2.0);
		c = mix(c, vec3(0.12, 0.38, 0.62), 0.28 + fresnel * 0.32);
		a = s.a * vAlpha;
	}
	if (uGround != 0 && (uMode == ` + modeOpaque + ` || uMode == ` + modeMasked + ` || uMode == ` + modeTerrainLayer + `)) {
		c = ground(c);
	}
	oColor = vec4(c, a);
}
`

// The render modes the fragment shader branches on.
var (
	modeOpaque       = fmt.Sprint(int(scene.Opaque))
	modeMasked       = fmt.Sprint(int(scene.Masked))
	modeTerrainLayer = fmt.Sprint(int(scene.TerrainLayer))
	modeTranslucent  = fmt.Sprint(int(scene.Translucent))
	modeModulated    = fmt.Sprint(int(scene.Modulated))
	modeAdditive     = fmt.Sprint(int(scene.Additive))
	modeWater        = fmt.Sprint(int(scene.Water))
)

// passes are the render modes in draw order (UE2-Studio gpu.rs).
var passes = [...]scene.RenderMode{
	scene.Opaque, scene.Masked, scene.TerrainLayer, scene.Translucent,
	scene.Brighten, scene.Modulated, scene.Additive, scene.Water, scene.Overlay,
}

// gpuScene is one scene on the GPU, or on its way there: prep holds what is
// left to upload, one unit (texture, index set, batch) at a time, and the
// scene is drawn once prep is nil.
type gpuScene struct {
	scene *scene.Scene
	prep  *Prepared
	// done counts the upload units finished.
	done     int
	sets     []gpuSet
	batches  []gpuBatch
	textures []textureKey
	// ids are the GL textures of the keys acquired so far.
	ids map[textureKey]uint32
	// hidden is the set of hidden actors the mesh index sets were last
	// cut for; cut are the index ranges each cut scene batch leaves out.
	hidden *scene.HiddenActors
	cut    map[int][]indexRange
}

// gpuSet is an index set on the GPU: its sectors, and the index ranges of
// the visible ones this frame, adjacent ranges merged.
type gpuSet struct {
	ibo     uint32
	sectors []sector
	visible []indexRange
}

type indexRange struct{ first, count int }

// gpuBatch is one scene batch on the GPU; its indices are those of its set.
type gpuBatch struct {
	mode          scene.RenderMode
	opaque        bool
	mesh          bool
	vao, vbo      uint32
	set           int
	texture, mask uint32
	// source is the batch's index in the scene's Batches.
	source int
}

// DrawStats count what the last frame drew.
type DrawStats struct {
	// Draws is the number of draw calls, Triangles what they drew.
	Draws, Triangles int
	// Sectors is the number of sectors of the uploaded scenes, Culled
	// those outside the view frustum.
	Sectors, Culled int
}

// sceneRenderer draws the scenes on the GPU, in the order they were added,
// and uploads the queued ones a few units per frame.
type sceneRenderer struct {
	prog     uint32
	viewProj int32
	origin   int32
	mode     int32
	opaque   int32
	eye      int32
	textures *textureCache
	scenes   []*gpuScene
	// warm is the 1×1 target of warmUp.
	warm viewTarget
	// dying are removed scenes whose GL resources upload frees, a step at
	// a time.
	dying []*gpuScene
	// rebase is the world's origin, subtracted in the vertex shader.
	rebase geom.Vec3
	stats  DrawStats
	// hideMeshes leaves the static mesh batches out of draw.
	hideMeshes bool
	// hidden are the static mesh actors cut out of their batches.
	hidden *scene.HiddenActors
	// ground marks the grid and the selected zone's footprint.
	ground groundShader
}

func newSceneRenderer(anisotropic bool) (*sceneRenderer, error) {
	p, err := newProgram(sceneVert, sceneFrag)
	if err != nil {
		return nil, fmt.Errorf("render: scene program: %w", err)
	}
	gles.UseProgram(p)
	gles.Uniform1i(gles.GetUniformLocation(p, "uTexture"), 0)
	gles.Uniform1i(gles.GetUniformLocation(p, "uMask"), 1)
	gles.UseProgram(0)
	return &sceneRenderer{
		prog:     p,
		viewProj: gles.GetUniformLocation(p, "uViewProj"),
		origin:   gles.GetUniformLocation(p, "uOrigin"),
		mode:     gles.GetUniformLocation(p, "uMode"),
		opaque:   gles.GetUniformLocation(p, "uOpaque"),
		eye:      gles.GetUniformLocation(p, "uEye"),
		textures: newTextureCache(anisotropic),
		ground:   newGroundShader(p),
	}, nil
}

// gridTerrain is the terrain whose cells the ground grid follows: the
// first uploaded scene's first terrain (the tiles' terrains share the
// same cell lattice), nil when none is uploaded.
func (sr *sceneRenderer) gridTerrain() *scene.Terrain {
	for _, gs := range sr.scenes {
		if gs.prep == nil && len(gs.scene.Terrains) > 0 {
			return &gs.scene.Terrains[0]
		}
	}
	return nil
}

// add queues p's scene for upload.
func (sr *sceneRenderer) add(p *Prepared) {
	sr.scenes = append(sr.scenes, &gpuScene{scene: p.scene, prep: p, ids: map[textureKey]uint32{}})
}

// remove stops drawing s and queues its GL resources, uploaded or not, for
// freeing by upload.
func (sr *sceneRenderer) remove(s *scene.Scene) {
	sr.scenes = slices.DeleteFunc(sr.scenes, func(gs *gpuScene) bool {
		if gs.scene != s {
			return false
		}
		gs.scene, gs.prep = nil, nil
		sr.dying = append(sr.dying, gs)
		return true
	})
}

// upload frees the removed scenes and then works through the queued ones,
// in order, until budget has passed (at least one step per call: freeing a
// whole tile at once takes tens of milliseconds too). It returns the scenes
// it finished and whether work remains.
func (sr *sceneRenderer) upload(budget time.Duration) (finished []*scene.Scene, more bool) {
	start := time.Now()
	stepped := false
	for len(sr.dying) > 0 {
		if stepped && time.Since(start) >= budget {
			return nil, true
		}
		if sr.dying[0].releaseStep(sr.textures) {
			sr.dying = sr.dying[1:]
		}
		stepped = true
	}
	for _, gs := range sr.scenes {
		for gs.prep != nil {
			if stepped && time.Since(start) >= budget {
				return finished, true
			}
			sr.step(gs)
			stepped = true
			if gs.prep == nil {
				finished = append(finished, gs.scene)
			}
		}
	}
	return finished, false
}

// progress is how much of s is on the GPU, 0 to 1; false when s is not
// queued nor uploaded.
func (sr *sceneRenderer) progress(s *scene.Scene) (float32, bool) {
	for _, gs := range sr.scenes {
		if gs.scene == s {
			if gs.prep == nil {
				return 1, true
			}
			return float32(gs.done) / float32(gs.prep.units()), true
		}
	}
	return 0, false
}

// units is the number of upload steps p takes.
func (p *Prepared) units() int { return len(p.textures) + len(p.sets) + len(p.batches) }

// step uploads the next unit of gs: its textures first, then its index
// sets, then its batches, each batch warmed up (warmUp) as it lands.
func (sr *sceneRenderer) step(gs *gpuScene) {
	p := gs.prep
	switch i := gs.done; {
	case i < len(p.textures):
		pt := &p.textures[i]
		sr.textures.upload(pt)
		gs.ids[pt.key] = sr.textures.acquire(pt.key)
		gs.textures = append(gs.textures, pt.key)
	case i < len(p.textures)+len(p.sets):
		set := &p.sets[i-len(p.textures)]
		g := gpuSet{ibo: gles.GenBuffer(), sectors: set.sectors}
		gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, g.ibo)
		gles.BufferData(gles.ELEMENT_ARRAY_BUFFER, set.indices, gles.STATIC_DRAW)
		gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, 0)
		gs.sets = append(gs.sets, g)
	default:
		b := gs.uploadBatch(sr.textures, &p.batches[i-len(p.textures)-len(p.sets)])
		gs.batches = append(gs.batches, b)
		sr.warmUp(b)
	}
	gs.done++
	if gs.done == p.units() {
		gs.prep = nil
	}
}

// warmUp draws one triangle of b into a 1×1 target. ANGLE creates the
// Direct3D side of a buffer or texture at its first draw, tens of
// milliseconds for a whole tile: drawn here, that cost lands in the upload
// step, within the frame's upload budget, and not in the first frame that
// shows the tile. Called outside DrawViewport, with no depth test or
// blending on; it leaves framebuffer, program, textures and vertex array
// unbound.
func (sr *sceneRenderer) warmUp(b gpuBatch) {
	if err := sr.warm.bind(image.Pt(1, 1)); err != nil {
		gles.BindFramebuffer(gles.FRAMEBUFFER, 0)
		return
	}
	gles.Viewport(0, 0, 1, 1)
	gles.UseProgram(sr.prog)
	gles.ActiveTexture(gles.TEXTURE1)
	gles.BindTexture(gles.TEXTURE_2D, b.mask)
	gles.ActiveTexture(gles.TEXTURE0)
	gles.BindTexture(gles.TEXTURE_2D, b.texture)
	gles.BindVertexArray(b.vao)
	gles.DrawElements(gles.TRIANGLES, 3, gles.UNSIGNED_INT, 0)
	gles.BindVertexArray(0)
	gles.BindTexture(gles.TEXTURE_2D, 0)
	gles.ActiveTexture(gles.TEXTURE1)
	gles.BindTexture(gles.TEXTURE_2D, 0)
	gles.ActiveTexture(gles.TEXTURE0)
	gles.UseProgram(0)
	gles.BindFramebuffer(gles.FRAMEBUFFER, 0)
}

// uploadBatch puts b's vertices on the GPU, in a vertex array that also
// binds its index set. A texture key the scene did not acquire is a
// stand-in.
func (gs *gpuScene) uploadBatch(tc *textureCache, b *preparedBatch) gpuBatch {
	texture, ok := gs.ids[b.texture]
	if !ok {
		texture = tc.acquire(b.texture)
	}
	mask, ok := gs.ids[b.mask]
	if !ok {
		mask = tc.acquire(b.mask)
	}
	g := gpuBatch{mode: b.mode, opaque: b.opaque, mesh: b.mesh, set: b.set, texture: texture, mask: mask, source: b.source}
	stride := int(unsafe.Sizeof(scene.Vertex{}))
	g.vao = gles.GenVertexArray()
	gles.BindVertexArray(g.vao)
	g.vbo = gles.GenBuffer()
	gles.BindBuffer(gles.ARRAY_BUFFER, g.vbo)
	gles.BufferData(gles.ARRAY_BUFFER, b.vertices, gles.STATIC_DRAW)
	gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, gs.sets[b.set].ibo)
	gles.EnableVertexAttribArray(0)
	gles.VertexAttribPointer(0, 3, gles.FLOAT, false, stride, unsafe.Offsetof(scene.Vertex{}.Pos))
	gles.EnableVertexAttribArray(1)
	gles.VertexAttribPointer(1, 2, gles.FLOAT, false, stride, unsafe.Offsetof(scene.Vertex{}.UV))
	gles.EnableVertexAttribArray(2)
	gles.VertexAttribPointer(2, 2, gles.FLOAT, false, stride, unsafe.Offsetof(scene.Vertex{}.MaskUV))
	gles.EnableVertexAttribArray(3)
	gles.VertexAttribPointer(3, 1, gles.FLOAT, false, stride, unsafe.Offsetof(scene.Vertex{}.Alpha))
	gles.BindVertexArray(0)
	gles.BindBuffer(gles.ARRAY_BUFFER, 0)
	gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, 0)
	return g
}

// releaseStep frees part of gs: one batch's buffers, or once they are all
// gone its index sets, then one texture reference at a time. It reports
// whether gs is all freed.
func (gs *gpuScene) releaseStep(tc *textureCache) bool {
	if n := len(gs.batches); n > 0 {
		b := gs.batches[n-1]
		gles.DeleteVertexArray(b.vao)
		gles.DeleteBuffer(b.vbo)
		gs.batches = gs.batches[:n-1]
		return false
	}
	if len(gs.sets) > 0 {
		for _, s := range gs.sets {
			gles.DeleteBuffer(s.ibo)
		}
		gs.sets = nil
		return false
	}
	if n := len(gs.textures); n > 0 {
		tc.drop(gs.textures[n-1])
		gs.textures = gs.textures[:n-1]
		return false
	}
	gs.ids = nil
	return true
}

// cutHidden rebuilds the index set of every mesh batch whose triangles
// the actors in hidden take out, or gave back, since the last cut: the
// batch's scene indices minus the hidden actors' sections, sectorized
// again. A batch no hidden actor touches keeps its set.
func (gs *gpuScene) cutHidden(hidden *scene.HiddenActors) {
	cut := map[int][]indexRange{}
	if hidden.Len() > 0 {
		for i := range gs.scene.Actors {
			a := &gs.scene.Actors[i]
			if !hidden.Has(a.Key()) {
				continue
			}
			for _, sec := range a.Sections {
				if sec.Batch >= 0 && sec.Count > 0 {
					cut[sec.Batch] = append(cut[sec.Batch], indexRange{sec.First, sec.Count})
				}
			}
		}
	}
	for _, ranges := range cut {
		slices.SortFunc(ranges, func(x, y indexRange) int { return x.first - y.first })
	}
	for _, b := range gs.batches {
		ranges, old := cut[b.source], gs.cut[b.source]
		if !b.mesh || slices.Equal(ranges, old) {
			continue
		}
		src := &gs.scene.Batches[b.source]
		indices := src.Indices
		if len(ranges) > 0 {
			indices = make([]uint32, 0, len(src.Indices))
			at := 0
			for _, r := range ranges {
				indices = append(indices, src.Indices[at:r.first]...)
				at = r.first + r.count
			}
			indices = append(indices, src.Indices[at:]...)
		}
		set := &gs.sets[b.set]
		next := sectorize(src.Vertices, indices)
		gles.BindVertexArray(0)
		gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, set.ibo)
		gles.BufferData(gles.ELEMENT_ARRAY_BUFFER, next.indices, gles.STATIC_DRAW)
		gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, 0)
		set.sectors, set.visible = next.sectors, set.visible[:0]
	}
	gs.hidden, gs.cut = hidden, cut
}

// cull finds the sectors of every uploaded scene that the frustum of
// viewProj (rebased Unreal basis to clip) can see.
func (sr *sceneRenderer) cull(viewProj mat4) {
	f := newFrustum(viewProj)
	for _, gs := range sr.scenes {
		if gs.prep != nil {
			continue
		}
		if gs.hidden != sr.hidden {
			gs.cutHidden(sr.hidden)
		}
		for i := range gs.sets {
			set := &gs.sets[i]
			set.visible = set.visible[:0]
			for _, sec := range set.sectors {
				sr.stats.Sectors++
				box := geom.Box{Min: sec.bounds.Min.Sub(sr.rebase), Max: sec.bounds.Max.Sub(sr.rebase)}
				if !f.sees(box) {
					sr.stats.Culled++
					continue
				}
				if n := len(set.visible); n > 0 && set.visible[n-1].first+set.visible[n-1].count == sec.first {
					set.visible[n-1].count += sec.count
					continue
				}
				set.visible = append(set.visible, indexRange{sec.first, sec.count})
			}
		}
	}
}

// draw renders the uploaded scenes with viewProj (rebased Unreal basis to
// clip) seen from eye (rebased, Unreal basis), one pass per render mode in
// UE2-Studio's order, each pass in scene and batch order (a terrain's
// layers blend in TerrainInfo order), each batch only over its visible
// sectors. models runs right before the Translucent pass, whether or not
// any batch is translucent, and leaves texture unit 0 active and empty and
// no VAO bound. The depth test is on and GREATER when called; blending and
// depth writes are left as found (off and on).
func (sr *sceneRenderer) draw(viewProj mat4, eye geom.Vec3, models func()) {
	sr.stats = DrawStats{}
	sr.cull(viewProj)
	gles.UseProgram(sr.prog)
	gles.UniformMatrix4fv(sr.viewProj, (*[16]float32)(&viewProj))
	gles.Uniform3f(sr.origin, sr.rebase.X, sr.rebase.Y, sr.rebase.Z)
	gles.Uniform3f(sr.eye, eye.X, eye.Y, eye.Z)
	sr.ground.upload(sr.rebase, sr.gridTerrain())
	// What is bound already, to skip the calls that would change nothing.
	var texture, mask uint32
	opaque := int32(-1)
	for _, mode := range passes {
		if mode == scene.Translucent {
			models()
			gles.UseProgram(sr.prog)
			texture = 0
		}
		started := false
		for _, gs := range sr.scenes {
			if gs.prep != nil {
				continue
			}
			for _, b := range gs.batches {
				visible := gs.sets[b.set].visible
				if b.mode != mode || len(visible) == 0 || b.mesh && sr.hideMeshes {
					continue
				}
				if !started {
					passState(mode)
					gles.Uniform1i(sr.mode, int32(mode))
					started = true
				}
				if o := boolInt(b.opaque); o != opaque {
					gles.Uniform1i(sr.opaque, o)
					opaque = o
				}
				if b.texture != texture {
					gles.ActiveTexture(gles.TEXTURE0)
					gles.BindTexture(gles.TEXTURE_2D, b.texture)
					texture = b.texture
				}
				if b.mask != mask {
					gles.ActiveTexture(gles.TEXTURE1)
					gles.BindTexture(gles.TEXTURE_2D, b.mask)
					mask = b.mask
				}
				gles.BindVertexArray(b.vao)
				for _, r := range visible {
					gles.DrawElements(gles.TRIANGLES, r.count, gles.UNSIGNED_INT, uintptr(r.first*4))
					sr.stats.Draws++
					sr.stats.Triangles += r.count / 3
				}
			}
		}
	}
	gles.Disable(gles.BLEND)
	gles.DepthMask(true)
	gles.DepthFunc(gles.GREATER)
	gles.ActiveTexture(gles.TEXTURE1)
	gles.BindTexture(gles.TEXTURE_2D, 0)
	gles.ActiveTexture(gles.TEXTURE0)
	gles.BindTexture(gles.TEXTURE_2D, 0)
	gles.BindVertexArray(0)
	gles.UseProgram(0)
}

func boolInt(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// passState sets the depth and blend state of mode's pass, UE2-Studio's
// pipeline for it (gpu.rs): Opaque and Masked write depth unblended;
// TerrainLayer tests GEQUAL, since the layers lie on the base's triangles;
// the blended passes test GREATER without writing; Overlay skips the test.
// Blending is straight alpha except Brighten (one, one minus source colour),
// Modulated (destination colour, zero) and Additive (one, one).
func passState(mode scene.RenderMode) {
	depthFunc, write, blend := uint32(gles.GREATER), false, true
	switch mode {
	case scene.Opaque, scene.Masked:
		write, blend = true, false
	case scene.TerrainLayer:
		depthFunc = gles.GEQUAL
	case scene.Overlay:
		depthFunc, blend = gles.ALWAYS, false
	}
	gles.DepthFunc(depthFunc)
	gles.DepthMask(write)
	if !blend {
		gles.Disable(gles.BLEND)
		return
	}
	gles.Enable(gles.BLEND)
	switch mode {
	case scene.Brighten:
		gles.BlendFuncSeparate(gles.ONE, gles.ONE_MINUS_SRC_COLOR, gles.ONE, gles.ONE_MINUS_SRC_ALPHA)
	case scene.Modulated:
		gles.BlendFuncSeparate(gles.DST_COLOR, gles.ZERO, gles.ZERO, gles.ONE)
	case scene.Additive:
		gles.BlendFuncSeparate(gles.ONE, gles.ONE, gles.ONE, gles.ONE)
	default:
		gles.BlendFuncSeparate(gles.SRC_ALPHA, gles.ONE_MINUS_SRC_ALPHA, gles.ONE, gles.ONE_MINUS_SRC_ALPHA)
	}
}

// release frees every scene's GL resources and the program.
func (sr *sceneRenderer) release() {
	for _, gs := range append(sr.scenes, sr.dying...) {
		for !gs.releaseStep(sr.textures) {
		}
	}
	sr.scenes, sr.dying = nil, nil
	sr.textures.release()
	sr.warm.release()
	gles.DeleteProgram(sr.prog)
}
