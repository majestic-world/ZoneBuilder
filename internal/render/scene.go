package render

import (
	"fmt"
	"unsafe"

	"zonebuilder/internal/render/gles"
	"zonebuilder/internal/scene"
)

// sceneVert takes absolute world positions (Unreal basis, Z up), subtracts
// the scene's rebase origin first, so the large map offset never enters a
// matrix product, then applies uViewProj, which includes the Y/Z swap into
// the Y-up render basis. gl_Position is invariant because terrain layers
// redraw the base's triangles from their own buffers and depth-test them
// with GEQUAL against it.
const sceneVert = `#version 300 es
layout(location = 0) in vec3 aPos;
layout(location = 1) in vec2 aUV;
layout(location = 2) in vec2 aMaskUV;
uniform highp vec3 uOrigin;
uniform highp mat4 uViewProj;
out highp vec2 vUV;
out highp vec2 vMaskUV;
invariant gl_Position;
void main() {
	vUV = aUV;
	vMaskUV = aMaskUV;
	gl_Position = uViewProj * vec4(aPos - uOrigin, 1.0);
}
`

// sceneFrag is UE2-Studio's Textured view: the texture sample, unlit
// (fs_main for Opaque, fs_terrain_layer for TerrainLayer, whose alpha is
// the mask's R times the sample's alpha). Sampling the sRGB texture yields
// linear colour, which the sRGB target encodes on write and blends in.
const sceneFrag = `#version 300 es
precision highp float;
uniform sampler2D uTexture;
uniform sampler2D uMask;
uniform bool uLayer;
in highp vec2 vUV;
in highp vec2 vMaskUV;
out vec4 oColor;
void main() {
	vec4 s = texture(uTexture, vUV);
	float a = 1.0;
	if (uLayer) {
		a = texture(uMask, vMaskUV).r * s.a;
	}
	oColor = vec4(s.rgb, a);
}
`

// gpuBatch is one scene batch uploaded to the GPU.
type gpuBatch struct {
	mode          scene.RenderMode
	vao, vbo, ibo uint32
	count         int
	texture, mask uint32
}

// sceneRenderer draws a scene's batches.
type sceneRenderer struct {
	prog     uint32
	viewProj int32
	origin   int32
	layer    int32
	textures *textureCache
	batches  []gpuBatch
	// rebase is the scene's origin, subtracted in the vertex shader.
	rebase [3]float32
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
		layer:    gles.GetUniformLocation(p, "uLayer"),
		textures: newTextureCache(anisotropic),
	}, nil
}

// upload replaces the GPU copy of the scene with s (nil: nothing drawn).
func (sr *sceneRenderer) upload(s *scene.Scene) {
	sr.releaseBatches()
	if s == nil {
		return
	}
	sr.rebase = [3]float32{s.Origin.X, s.Origin.Y, s.Origin.Z}
	stride := int(unsafe.Sizeof(scene.Vertex{}))
	for i := range s.Batches {
		b := &s.Batches[i]
		if len(b.Indices) == 0 {
			continue
		}
		g := gpuBatch{
			mode:    b.Mode,
			count:   len(b.Indices),
			texture: sr.textures.material(b.Texture),
			mask:    sr.textures.mask(b.Mask),
		}
		g.vao = gles.GenVertexArray()
		gles.BindVertexArray(g.vao)
		g.vbo = gles.GenBuffer()
		gles.BindBuffer(gles.ARRAY_BUFFER, g.vbo)
		gles.BufferData(gles.ARRAY_BUFFER, b.Vertices, gles.STATIC_DRAW)
		g.ibo = gles.GenBuffer()
		gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, g.ibo)
		gles.BufferData(gles.ELEMENT_ARRAY_BUFFER, b.Indices, gles.STATIC_DRAW)
		gles.EnableVertexAttribArray(0)
		gles.VertexAttribPointer(0, 3, gles.FLOAT, false, stride, unsafe.Offsetof(scene.Vertex{}.Pos))
		gles.EnableVertexAttribArray(1)
		gles.VertexAttribPointer(1, 2, gles.FLOAT, false, stride, unsafe.Offsetof(scene.Vertex{}.UV))
		gles.EnableVertexAttribArray(2)
		gles.VertexAttribPointer(2, 2, gles.FLOAT, false, stride, unsafe.Offsetof(scene.Vertex{}.MaskUV))
		gles.BindVertexArray(0)
		gles.BindBuffer(gles.ARRAY_BUFFER, 0)
		gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, 0)
		sr.batches = append(sr.batches, g)
	}
}

// draw renders the batches with viewProj (rebased Unreal basis to clip),
// one pass per render mode in UE2-Studio's order, each pass in scene order
// (a terrain's layers blend in TerrainInfo order). The depth test is on
// and GREATER when called; blending and depth writes are left as found
// (off and on).
func (sr *sceneRenderer) draw(viewProj mat4) {
	if len(sr.batches) == 0 {
		return
	}
	gles.UseProgram(sr.prog)
	gles.UniformMatrix4fv(sr.viewProj, (*[16]float32)(&viewProj))
	gles.Uniform3f(sr.origin, sr.rebase[0], sr.rebase[1], sr.rebase[2])
	for _, mode := range []scene.RenderMode{scene.Opaque, scene.TerrainLayer} {
		switch mode {
		case scene.Opaque:
			gles.Uniform1i(sr.layer, 0)
		case scene.TerrainLayer:
			// Layers lie on the base's triangles: GEQUAL lets them through
			// at equal depth, and they must not write it.
			gles.Uniform1i(sr.layer, 1)
			gles.DepthFunc(gles.GEQUAL)
			gles.DepthMask(false)
			gles.Enable(gles.BLEND)
			gles.BlendFunc(gles.SRC_ALPHA, gles.ONE_MINUS_SRC_ALPHA)
		}
		for _, b := range sr.batches {
			if b.mode != mode {
				continue
			}
			gles.ActiveTexture(gles.TEXTURE0)
			gles.BindTexture(gles.TEXTURE_2D, b.texture)
			gles.ActiveTexture(gles.TEXTURE1)
			gles.BindTexture(gles.TEXTURE_2D, b.mask)
			gles.BindVertexArray(b.vao)
			gles.DrawElements(gles.TRIANGLES, b.count, gles.UNSIGNED_INT, 0)
		}
	}
	gles.Disable(gles.BLEND)
	gles.DepthMask(true)
	gles.DepthFunc(gles.GREATER)
	gles.BindTexture(gles.TEXTURE_2D, 0)
	gles.ActiveTexture(gles.TEXTURE0)
	gles.BindTexture(gles.TEXTURE_2D, 0)
	gles.BindVertexArray(0)
	gles.UseProgram(0)
}

func (sr *sceneRenderer) releaseBatches() {
	for _, b := range sr.batches {
		gles.DeleteVertexArray(b.vao)
		gles.DeleteBuffer(b.vbo)
		gles.DeleteBuffer(b.ibo)
	}
	sr.batches = sr.batches[:0]
	sr.textures.clear()
}

func (sr *sceneRenderer) release() {
	sr.releaseBatches()
	sr.textures.release()
	gles.DeleteProgram(sr.prog)
}
