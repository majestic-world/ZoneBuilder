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
// the Y-up render basis.
const sceneVert = `#version 300 es
layout(location = 0) in vec3 aPos;
uniform highp vec3 uOrigin;
uniform highp mat4 uViewProj;
out highp vec3 vPos;
void main() {
	highp vec3 p = aPos - uOrigin;
	vPos = p;
	gl_Position = uViewProj * vec4(p, 1.0);
}
`

// sceneFrag shades untextured geometry: a flat grey lit by a fixed sun,
// with the face normal taken from screen-space derivatives (no normals are
// stored) and lit two-sided, since the derivative normal's sign follows the
// winding.
const sceneFrag = `#version 300 es
precision highp float;
in highp vec3 vPos;
out vec4 oColor;
void main() {
	vec3 n = normalize(cross(dFdx(vPos), dFdy(vPos)));
	// Unreal basis: Z is up.
	float sun = abs(dot(n, normalize(vec3(0.35, 0.25, 0.9))));
	vec3 base = vec3(0.42, 0.42, 0.40);
	oColor = vec4(base * (0.25 + 0.75 * sun), 1.0);
}
`

// gpuBatch is one scene batch uploaded to the GPU.
type gpuBatch struct {
	vao, vbo, ibo uint32
	count         int
}

// sceneRenderer draws a scene's batches.
type sceneRenderer struct {
	prog     uint32
	viewProj int32
	origin   int32
	batches  []gpuBatch
	// rebase is the scene's origin, subtracted in the vertex shader.
	rebase [3]float32
}

func newSceneRenderer() (*sceneRenderer, error) {
	p, err := newProgram(sceneVert, sceneFrag)
	if err != nil {
		return nil, fmt.Errorf("render: scene program: %w", err)
	}
	return &sceneRenderer{
		prog:     p,
		viewProj: gles.GetUniformLocation(p, "uViewProj"),
		origin:   gles.GetUniformLocation(p, "uOrigin"),
	}, nil
}

// upload replaces the GPU copy of the scene with s (nil: nothing drawn).
func (sr *sceneRenderer) upload(s *scene.Scene) {
	sr.releaseBatches()
	if s == nil {
		return
	}
	sr.rebase = [3]float32{s.Origin.X, s.Origin.Y, s.Origin.Z}
	for i := range s.Batches {
		b := &s.Batches[i]
		if len(b.Indices) == 0 {
			continue
		}
		g := gpuBatch{count: len(b.Indices)}
		g.vao = gles.GenVertexArray()
		gles.BindVertexArray(g.vao)
		g.vbo = gles.GenBuffer()
		gles.BindBuffer(gles.ARRAY_BUFFER, g.vbo)
		gles.BufferData(gles.ARRAY_BUFFER, b.Vertices, gles.STATIC_DRAW)
		g.ibo = gles.GenBuffer()
		gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, g.ibo)
		gles.BufferData(gles.ELEMENT_ARRAY_BUFFER, b.Indices, gles.STATIC_DRAW)
		stride := int(unsafe.Sizeof(scene.Vertex{}))
		gles.EnableVertexAttribArray(0)
		gles.VertexAttribPointer(0, 3, gles.FLOAT, false, stride, unsafe.Offsetof(scene.Vertex{}.Pos))
		gles.BindVertexArray(0)
		gles.BindBuffer(gles.ARRAY_BUFFER, 0)
		gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, 0)
		sr.batches = append(sr.batches, g)
	}
}

// draw renders every batch with viewProj (rebased Unreal basis to clip).
func (sr *sceneRenderer) draw(viewProj mat4) {
	if len(sr.batches) == 0 {
		return
	}
	gles.UseProgram(sr.prog)
	gles.UniformMatrix4fv(sr.viewProj, (*[16]float32)(&viewProj))
	gles.Uniform3f(sr.origin, sr.rebase[0], sr.rebase[1], sr.rebase[2])
	for _, b := range sr.batches {
		gles.BindVertexArray(b.vao)
		gles.DrawElements(gles.TRIANGLES, b.count, gles.UNSIGNED_INT, 0)
	}
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
}

func (sr *sceneRenderer) release() {
	sr.releaseBatches()
	gles.DeleteProgram(sr.prog)
}
