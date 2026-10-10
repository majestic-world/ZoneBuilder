package render

import (
	"fmt"
	"image"
	"slices"
	"unsafe"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/model"
	"zonebuilder/internal/render/gles"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/texture"
)

// modelVert places a CPU-skinned vertex (model space, Unreal axes, feet at
// the origin, facing +X) on one instance: turned by the instance's yaw
// about Z, then moved to its feet, which arrive in server coordinates and
// become client ones here (scene.FromServer). The feet are rebased before
// the model's offset is added, so the map's large coordinates never meet
// the small ones in float32.
var modelVert = `#version 300 es
layout(location = 0) in vec3 aPos;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in vec2 aUV;
layout(location = 3) in highp vec4 aInstance;
uniform highp vec3 uOrigin;
uniform highp mat4 uViewProj;
out highp vec2 vUV;
out mediump vec3 vNormal;
void main() {
	float c = cos(aInstance.w);
	float s = sin(aInstance.w);
	mat2 yaw = mat2(c, s, -s, c);
	highp vec3 feet = vec3(aInstance.xy, aInstance.z - ` + fmt.Sprintf("%.1f", float32(scene.ServerZOffset)) + `) - uOrigin;
	vUV = aUV;
	vNormal = vec3(yaw * aNormal.xy, aNormal.z);
	gl_Position = uViewProj * vec4(feet + vec3(yaw * aPos.xy, aPos.z), 1.0);
}
`

// modelFrag is UE2-Studio's lit skinned preview (gpu.rs vs_skinned with a
// map view mode): the skin times a fixed fill, 0.60 + 0.40·max(N·L, 0),
// with L its (0.45, 0.80, 0.40) of the Y-up renderer in Unreal axes.
// Masked sections cut out under alpha 0.5; the others ignore alpha.
const modelFrag = `#version 300 es
precision highp float;
uniform sampler2D uTexture;
uniform bool uMasked;
in highp vec2 vUV;
in mediump vec3 vNormal;
out vec4 oColor;
void main() {
	vec4 s = texture(uTexture, vUV);
	if (uMasked && s.a < 0.5) {
		discard;
	}
	vec3 l = normalize(vec3(0.45, 0.40, 0.80));
	float light = 0.60 + 0.40 * max(dot(normalize(vNormal), l), 0.0);
	oColor = vec4(s.rgb * light, 1.0);
}
`

// Instance places one copy of a model: its feet at Pos, in server
// coordinates, turned Yaw radians about Z from facing +X (counterclockwise
// seen from above). The layout is the instance attribute's: 4 floats.
type Instance struct {
	Pos geom.Vec3
	Yaw float32
}

// Model is a skinned model on the GPU (a model.Bundle): its skins,
// indices and UVs, uploaded once; the skinned vertices of the frame
// (Pose); and its instances (SetInstances), all in the one pose. It
// belongs to the Renderer that made it and dies with it: after a new
// Renderer, make the Model again.
type Model struct {
	r *Renderer
	// vao reads skinned (positions, then normals), static (UVs) and
	// instances (Instance), and holds ibo.
	vao, skinned, static, instances, ibo uint32
	// parts are the first vertex of each bundle part in skin.
	parts []int
	// skin is the CPU side of skinned: every part's positions, then
	// every part's normals.
	skin     []geom.Vec3
	sections []modelSection
	textures []uint32
	count    int
}

// modelSection is one draw: a range of the index buffer with one skin.
type modelSection struct {
	texture      uint32
	masked       bool
	first, count int
}

// NewModel puts b on the GPU: every section with indices, drawn with its
// skin, cut out at alpha 0.5 when masked (any non-Opaque section is: the
// pass writes depth and never blends). It starts in no pose: Pose before
// the first draw.
func (r *Renderer) NewModel(b *model.Bundle) (*Model, error) {
	m := &Model{r: r, parts: make([]int, len(b.Parts))}
	vertices, indices := 0, 0
	for i := range b.Parts {
		m.parts[i] = vertices
		vertices += len(b.Parts[i].Vertices)
		indices += len(b.Parts[i].Indices)
	}
	uvs := make([][2]float32, 0, vertices)
	all := make([]uint32, 0, indices)
	for i := range b.Parts {
		p := &b.Parts[i]
		for _, v := range p.Vertices {
			uvs = append(uvs, v.UV)
		}
		base := uint32(m.parts[i])
		first := len(all)
		for _, k := range p.Indices {
			all = append(all, base+k)
		}
		for j := range p.Sections {
			s := &p.Sections[j]
			if s.IndexCount == 0 {
				continue
			}
			masked := s.Masked || s.RenderMode != model.Opaque
			id, err := modelTexture(s, masked, r.Info.Anisotropic)
			if err != nil {
				m.Release()
				return nil, fmt.Errorf("render: model part %d section %d (%s): %w", i, j, s.Texture, err)
			}
			m.textures = append(m.textures, id)
			m.sections = append(m.sections, modelSection{texture: id, masked: masked, first: first + int(s.FirstIndex), count: int(s.IndexCount)})
		}
	}
	m.skin = make([]geom.Vec3, 2*vertices)

	m.vao = gles.GenVertexArray()
	gles.BindVertexArray(m.vao)
	m.skinned = gles.GenBuffer()
	gles.BindBuffer(gles.ARRAY_BUFFER, m.skinned)
	gles.BufferData(gles.ARRAY_BUFFER, m.skin, gles.DYNAMIC_DRAW)
	vec3 := int(unsafe.Sizeof(geom.Vec3{}))
	gles.EnableVertexAttribArray(0)
	gles.VertexAttribPointer(0, 3, gles.FLOAT, false, vec3, 0)
	gles.EnableVertexAttribArray(1)
	gles.VertexAttribPointer(1, 3, gles.FLOAT, false, vec3, uintptr(vertices*vec3))
	m.static = gles.GenBuffer()
	gles.BindBuffer(gles.ARRAY_BUFFER, m.static)
	gles.BufferData(gles.ARRAY_BUFFER, uvs, gles.STATIC_DRAW)
	gles.EnableVertexAttribArray(2)
	gles.VertexAttribPointer(2, 2, gles.FLOAT, false, int(unsafe.Sizeof(uvs[0])), 0)
	m.instances = gles.GenBuffer()
	gles.BindBuffer(gles.ARRAY_BUFFER, m.instances)
	gles.EnableVertexAttribArray(3)
	gles.VertexAttribPointer(3, 4, gles.FLOAT, false, int(unsafe.Sizeof(Instance{})), 0)
	gles.VertexAttribDivisor(3, 1)
	m.ibo = gles.GenBuffer()
	gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, m.ibo)
	gles.BufferData(gles.ELEMENT_ARRAY_BUFFER, all, gles.STATIC_DRAW)
	gles.BindVertexArray(0)
	gles.BindBuffer(gles.ARRAY_BUFFER, 0)
	gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, 0)
	r.models[m] = struct{}{}
	return m, nil
}

// modelTexture uploads s's skin: sRGB with its full mip chain, the masked
// chain (colour bled under the cutout) when masked, repeat, anisotropic.
func modelTexture(s *model.Section, masked, anisotropic bool) (uint32, error) {
	px, err := s.Pixels()
	if err != nil {
		return 0, err
	}
	img := &texture.Image{Width: px.Rect.Dx(), Height: px.Rect.Dy(), Pix: tightPix(px)}
	levels := img.Mipmaps()
	if masked {
		levels = img.MaskedMipmaps()
	}
	id := gles.GenTexture()
	gles.BindTexture(gles.TEXTURE_2D, id)
	for n, l := range levels {
		gles.TexImage2D(gles.TEXTURE_2D, n, gles.SRGB8_ALPHA8, l.Width, l.Height, gles.RGBA, gles.UNSIGNED_BYTE, l.Pix)
	}
	aniso := int32(1)
	if anisotropic {
		aniso = maxAnisotropy
	}
	sampling(len(levels)-1, gles.LINEAR_MIPMAP_LINEAR, gles.REPEAT, aniso)
	gles.BindTexture(gles.TEXTURE_2D, 0)
	return id, nil
}

// tightPix is px's pixels without row padding.
func tightPix(px *image.NRGBA) []byte {
	w, h := px.Rect.Dx(), px.Rect.Dy()
	if px.Stride == 4*w {
		return px.Pix[:4*w*h]
	}
	pix := make([]byte, 0, 4*w*h)
	for y := range h {
		pix = append(pix, px.Pix[y*px.Stride:y*px.Stride+4*w]...)
	}
	return pix
}

// Pose skins every part with a, which must animate the bundle m was made
// from, and uploads the result: the pose every instance takes until the
// next Pose. Call it once per frame, after a.Update.
func (m *Model) Pose(a *model.Animator) {
	n := len(m.skin) / 2
	for i, first := range m.parts {
		end := n
		if i+1 < len(m.parts) {
			end = m.parts[i+1]
		}
		a.Skin(i, m.skin[first:end], m.skin[n+first:n+end])
	}
	gles.BindBuffer(gles.ARRAY_BUFFER, m.skinned)
	gles.BufferData(gles.ARRAY_BUFFER, m.skin, gles.DYNAMIC_DRAW)
	gles.BindBuffer(gles.ARRAY_BUFFER, 0)
}

// SetInstances replaces the copies of m drawn (none: m draws nothing).
// in is only read during the call.
func (m *Model) SetInstances(in []Instance) {
	gles.BindBuffer(gles.ARRAY_BUFFER, m.instances)
	gles.BufferData(gles.ARRAY_BUFFER, in, gles.DYNAMIC_DRAW)
	gles.BindBuffer(gles.ARRAY_BUFFER, 0)
	m.count = len(in)
}

// Release frees m's GL resources; m must not be used again.
func (m *Model) Release() {
	for _, id := range m.textures {
		gles.DeleteTexture(id)
	}
	gles.DeleteVertexArray(m.vao)
	gles.DeleteBuffer(m.skinned)
	gles.DeleteBuffer(m.static)
	gles.DeleteBuffer(m.instances)
	gles.DeleteBuffer(m.ibo)
	delete(m.r.models, m)
	m.r.queue = slices.DeleteFunc(m.r.queue, func(q *Model) bool { return q == m })
	*m = Model{r: m.r}
}

// modelPass draws the models queued for a frame, every instance of a model
// in one call per section, in the Masked pass's state: depth GREATER and
// written, no blending.
type modelPass struct {
	prog             uint32
	viewProj, origin int32
	masked           int32
}

func newModelPass() (*modelPass, error) {
	p, err := newProgram(modelVert, modelFrag)
	if err != nil {
		return nil, fmt.Errorf("render: model program: %w", err)
	}
	gles.UseProgram(p)
	gles.Uniform1i(gles.GetUniformLocation(p, "uTexture"), 0)
	gles.UseProgram(0)
	return &modelPass{
		prog:     p,
		viewProj: gles.GetUniformLocation(p, "uViewProj"),
		origin:   gles.GetUniformLocation(p, "uOrigin"),
		masked:   gles.GetUniformLocation(p, "uMasked"),
	}, nil
}

// draw renders models with viewProj (rebased Unreal basis to clip) and the
// rebase origin, adding to stats. It leaves texture unit 0 active with no
// texture, no VAO bound, and the program in use for the caller to replace.
func (mp *modelPass) draw(viewProj mat4, origin geom.Vec3, models []*Model, stats *DrawStats) {
	if len(models) == 0 {
		return
	}
	gles.UseProgram(mp.prog)
	gles.UniformMatrix4fv(mp.viewProj, (*[16]float32)(&viewProj))
	gles.Uniform3f(mp.origin, origin.X, origin.Y, origin.Z)
	passState(scene.Masked)
	gles.ActiveTexture(gles.TEXTURE0)
	masked := int32(-1)
	for _, m := range models {
		if m.count == 0 {
			continue
		}
		gles.BindVertexArray(m.vao)
		for _, s := range m.sections {
			if k := boolInt(s.masked); k != masked {
				gles.Uniform1i(mp.masked, k)
				masked = k
			}
			gles.BindTexture(gles.TEXTURE_2D, s.texture)
			gles.DrawElementsInstanced(gles.TRIANGLES, s.count, gles.UNSIGNED_INT, uintptr(s.first*4), m.count)
			stats.Draws++
			stats.Triangles += s.count / 3 * m.count
		}
	}
	gles.BindTexture(gles.TEXTURE_2D, 0)
	gles.BindVertexArray(0)
}

func (mp *modelPass) release() {
	gles.DeleteProgram(mp.prog)
}
