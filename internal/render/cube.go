package render

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	"zonebuilder/internal/render/gles"
)

// cube is the M0 spike's scene: a textured cube whose texture is a DXT1
// mip chain uploaded natively (ADR 0002). It stands in for the map scene
// until the scene renderer lands.
type cube struct {
	prog     uint32
	mvp      int32
	vao      uint32
	vbo, ibo uint32
	tex      uint32
}

type cubeVertex struct {
	pos    [3]float32
	normal [3]float32
	uv     [2]float32
}

const cubeVert = `#version 300 es
layout(location = 0) in vec3 aPos;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in vec2 aUV;
uniform mat4 uModel;
uniform mat4 uViewProj;
out vec2 vUV;
out vec3 vNormal;
void main() {
	vUV = aUV;
	vNormal = mat3(uModel) * aNormal;
	gl_Position = uViewProj * uModel * vec4(aPos, 1.0);
}
`

const cubeFrag = `#version 300 es
precision mediump float;
uniform sampler2D uTex;
in vec2 vUV;
in vec3 vNormal;
out vec4 oColor;
void main() {
	float light = 0.35 + 0.65 * max(dot(normalize(vNormal), normalize(vec3(0.4, 0.8, 0.6))), 0.0);
	oColor = vec4(texture(uTex, vUV).rgb * light, 1.0);
}
`

func newCube() (*cube, error) {
	p, err := newProgram(cubeVert, cubeFrag)
	if err != nil {
		return nil, fmt.Errorf("render: cube program: %w", err)
	}
	c := &cube{prog: p}
	gles.UseProgram(p)
	gles.Uniform1i(gles.GetUniformLocation(p, "uTex"), 0)
	gles.UseProgram(0)

	verts, idx := cubeMesh()
	c.vao = gles.GenVertexArray()
	gles.BindVertexArray(c.vao)
	c.vbo = gles.GenBuffer()
	gles.BindBuffer(gles.ARRAY_BUFFER, c.vbo)
	gles.BufferData(gles.ARRAY_BUFFER, verts, gles.STATIC_DRAW)
	c.ibo = gles.GenBuffer()
	gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, c.ibo)
	gles.BufferData(gles.ELEMENT_ARRAY_BUFFER, idx, gles.STATIC_DRAW)
	stride := int(unsafe.Sizeof(cubeVertex{}))
	gles.EnableVertexAttribArray(0)
	gles.VertexAttribPointer(0, 3, gles.FLOAT, false, stride, unsafe.Offsetof(cubeVertex{}.pos))
	gles.EnableVertexAttribArray(1)
	gles.VertexAttribPointer(1, 3, gles.FLOAT, false, stride, unsafe.Offsetof(cubeVertex{}.normal))
	gles.EnableVertexAttribArray(2)
	gles.VertexAttribPointer(2, 2, gles.FLOAT, false, stride, unsafe.Offsetof(cubeVertex{}.uv))
	gles.BindVertexArray(0)
	gles.BindBuffer(gles.ARRAY_BUFFER, 0)
	gles.BindBuffer(gles.ELEMENT_ARRAY_BUFFER, 0)

	c.tex, err = uploadCheckerDXT1(256)
	if err != nil {
		c.release()
		return nil, err
	}
	return c, nil
}

// draw renders the cube rotated by angle (radians) with the given
// view-projection matrix into the bound framebuffer.
func (c *cube) draw(viewProj mat4, angle float32) {
	model := mul(rotateY(angle), rotateX(angle*0.6))
	gles.UseProgram(c.prog)
	gles.UniformMatrix4fv(gles.GetUniformLocation(c.prog, "uModel"), (*[16]float32)(&model))
	gles.UniformMatrix4fv(gles.GetUniformLocation(c.prog, "uViewProj"), (*[16]float32)(&viewProj))
	gles.ActiveTexture(gles.TEXTURE0)
	gles.BindTexture(gles.TEXTURE_2D, c.tex)
	gles.BindVertexArray(c.vao)
	gles.DrawElements(gles.TRIANGLES, 36, gles.UNSIGNED_SHORT, 0)
	gles.BindVertexArray(0)
	gles.BindTexture(gles.TEXTURE_2D, 0)
	gles.UseProgram(0)
}

func (c *cube) release() {
	gles.DeleteProgram(c.prog)
	gles.DeleteVertexArray(c.vao)
	gles.DeleteBuffer(c.vbo)
	gles.DeleteBuffer(c.ibo)
	gles.DeleteTexture(c.tex)
}

func cubeMesh() ([]cubeVertex, []uint16) {
	// Each face: normal, and the two axes spanning it (u, v).
	faces := [6][3][3]float32{
		{{0, 0, 1}, {1, 0, 0}, {0, 1, 0}},
		{{0, 0, -1}, {-1, 0, 0}, {0, 1, 0}},
		{{1, 0, 0}, {0, 0, -1}, {0, 1, 0}},
		{{-1, 0, 0}, {0, 0, 1}, {0, 1, 0}},
		{{0, 1, 0}, {1, 0, 0}, {0, 0, -1}},
		{{0, -1, 0}, {1, 0, 0}, {0, 0, 1}},
	}
	corners := [4][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}}
	verts := make([]cubeVertex, 0, 24)
	idx := make([]uint16, 0, 36)
	for _, f := range faces {
		n, u, v := f[0], f[1], f[2]
		base := uint16(len(verts))
		for _, c := range corners {
			var p [3]float32
			for i := range 3 {
				p[i] = n[i] + c[0]*u[i] + c[1]*v[i]
			}
			verts = append(verts, cubeVertex{pos: p, normal: n, uv: [2]float32{(c[0] + 1) / 2, (1 - c[1]) / 2}})
		}
		idx = append(idx, base, base+1, base+2, base, base+2, base+3)
	}
	return verts, idx
}

// uploadCheckerDXT1 builds a size×size checkerboard as DXT1 blocks for every
// mip level down to 1×1 and uploads it with the sRGB DXT1 format, checking
// that ANGLE accepts each level, including the sub-block 2×2 and 1×1 tails.
func uploadCheckerDXT1(size int) (uint32, error) {
	tex := gles.GenTexture()
	gles.BindTexture(gles.TEXTURE_2D, tex)
	level := 0
	for s := size; s >= 1; s /= 2 {
		gles.CompressedTexImage2D(gles.TEXTURE_2D, level, gles.COMPRESSED_SRGB_ALPHA_S3TC_DXT1_EXT, s, s, checkerDXT1(s))
		if e := gles.GetError(); e != gles.NO_ERROR {
			gles.DeleteTexture(tex)
			return 0, fmt.Errorf("render: DXT1 upload of mip %d (%dx%d): GL error 0x%x", level, s, s, e)
		}
		level++
	}
	gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_MAX_LEVEL, int32(level-1))
	gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_MIN_FILTER, gles.LINEAR_MIPMAP_LINEAR)
	gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_MAG_FILTER, gles.LINEAR)
	gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_WRAP_S, gles.REPEAT)
	gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_WRAP_T, gles.REPEAT)
	gles.BindTexture(gles.TEXTURE_2D, 0)
	return tex, nil
}

// checkerDXT1 encodes one mip level (s×s) of a checkerboard with 8 cells per
// side. Every 4×4 block is a single colour: colour0 holds it and all indices
// select colour0. Levels whose cells are smaller than a block get the
// checker's average colour, which is what a box-filtered mip would hold.
func checkerDXT1(s int) []byte {
	const (
		amber   = 0xFD20 // RGB565 of (255,164,0)
		slate   = 0x31A8 // RGB565 of (48,52,64)
		average = 0x9364 // RGB565 of (151,108,32)
	)
	blocks := (s + 3) / 4
	cell := s / 8 // cell side in pixels at this level
	out := make([]byte, 0, blocks*blocks*8)
	for by := range blocks {
		for bx := range blocks {
			color := uint16(average)
			if cell >= 4 {
				color = slate
				if (bx*4/cell+by*4/cell)%2 == 0 {
					color = amber
				}
			}
			out = binary.LittleEndian.AppendUint16(out, color)
			out = binary.LittleEndian.AppendUint16(out, 0)
			out = binary.LittleEndian.AppendUint32(out, 0)
		}
	}
	return out
}
