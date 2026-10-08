package render

import (
	"fmt"
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
	oColor = vec4(c, a);
}
`

// The render modes the fragment shader branches on.
var (
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

// gpuBatch is one scene batch uploaded to the GPU.
type gpuBatch struct {
	mode          scene.RenderMode
	opaque        bool
	vao, vbo, ibo uint32
	count         int
	texture, mask uint32
}

// sceneRenderer draws a scene's batches.
type sceneRenderer struct {
	prog     uint32
	viewProj int32
	origin   int32
	mode     int32
	opaque   int32
	eye      int32
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
		mode:     gles.GetUniformLocation(p, "uMode"),
		opaque:   gles.GetUniformLocation(p, "uOpaque"),
		eye:      gles.GetUniformLocation(p, "uEye"),
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
			opaque:  b.OpaqueTexture,
			count:   len(b.Indices),
			texture: sr.textures.material(b.Texture, b.Mode == scene.Masked && !b.OpaqueTexture),
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
		gles.EnableVertexAttribArray(3)
		gles.VertexAttribPointer(3, 1, gles.FLOAT, false, stride, unsafe.Offsetof(scene.Vertex{}.Alpha))
		gles.BindVertexArray(0)
		gles.BindBuffer(gles.ARRAY_BUFFER, 0)
		gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, 0)
		sr.batches = append(sr.batches, g)
	}
}

// draw renders the batches with viewProj (rebased Unreal basis to clip)
// seen from eye (rebased, Unreal basis), one pass per render mode in
// UE2-Studio's order, each pass in scene order (a terrain's layers blend in
// TerrainInfo order). The depth test is on and GREATER when called;
// blending and depth writes are left as found (off and on).
func (sr *sceneRenderer) draw(viewProj mat4, eye geom.Vec3) {
	if len(sr.batches) == 0 {
		return
	}
	gles.UseProgram(sr.prog)
	gles.UniformMatrix4fv(sr.viewProj, (*[16]float32)(&viewProj))
	gles.Uniform3f(sr.origin, sr.rebase[0], sr.rebase[1], sr.rebase[2])
	gles.Uniform3f(sr.eye, eye.X, eye.Y, eye.Z)
	for _, mode := range passes {
		drawn := false
		for _, b := range sr.batches {
			if b.mode != mode {
				continue
			}
			if !drawn {
				passState(mode)
				gles.Uniform1i(sr.mode, int32(mode))
				drawn = true
			}
			opaque := int32(0)
			if b.opaque {
				opaque = 1
			}
			gles.Uniform1i(sr.opaque, opaque)
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
