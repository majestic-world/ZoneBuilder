package render

import (
	"fmt"
	"image"

	"zonebuilder/internal/render/gles"
)

// viewTarget is the framebuffer the 3D viewport renders into: an sRGB colour
// texture plus a 32-bit float depth buffer for reversed Z (ADR 0001). The
// window surface ANGLE gives us has neither a float depth format nor an sRGB
// colour space, so the scene cannot be drawn into it directly.
type viewTarget struct {
	fbo, color, depth uint32
	size              image.Point
}

// bind (re)allocates the attachments for size and binds the framebuffer.
func (t *viewTarget) bind(size image.Point) error {
	if t.fbo == 0 {
		t.fbo = gles.GenFramebuffer()
		t.color = gles.GenTexture()
		t.depth = gles.GenRenderbuffer()
	}
	gles.BindFramebuffer(gles.FRAMEBUFFER, t.fbo)
	if size == t.size {
		return nil
	}
	t.size = size
	gles.BindTexture(gles.TEXTURE_2D, t.color)
	gles.TexImage2D(gles.TEXTURE_2D, 0, gles.SRGB8_ALPHA8, size.X, size.Y, gles.RGBA, gles.UNSIGNED_BYTE, nil)
	gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_MIN_FILTER, gles.NEAREST)
	gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_MAG_FILTER, gles.NEAREST)
	gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_WRAP_S, gles.CLAMP_TO_EDGE)
	gles.TexParameteri(gles.TEXTURE_2D, gles.TEXTURE_WRAP_T, gles.CLAMP_TO_EDGE)
	gles.BindTexture(gles.TEXTURE_2D, 0)
	gles.BindRenderbuffer(gles.RENDERBUFFER, t.depth)
	gles.RenderbufferStorage(gles.RENDERBUFFER, gles.DEPTH_COMPONENT32F, size.X, size.Y)
	gles.BindRenderbuffer(gles.RENDERBUFFER, 0)
	gles.FramebufferTexture2D(gles.FRAMEBUFFER, gles.COLOR_ATTACHMENT0, gles.TEXTURE_2D, t.color, 0)
	gles.FramebufferRenderbuffer(gles.FRAMEBUFFER, gles.DEPTH_ATTACHMENT, gles.RENDERBUFFER, t.depth)
	if st := gles.CheckFramebufferStatus(gles.FRAMEBUFFER); st != gles.FRAMEBUFFER_COMPLETE {
		return fmt.Errorf("render: viewport framebuffer incomplete (0x%x)", st)
	}
	return nil
}

func (t *viewTarget) release() {
	if t.fbo != 0 {
		gles.DeleteFramebuffer(t.fbo)
		gles.DeleteTexture(t.color)
		gles.DeleteRenderbuffer(t.depth)
	}
	*t = viewTarget{}
}

// compositor copies the viewport's colour texture into its rectangle of the
// window framebuffer. A shader pass rather than glBlitFramebuffer: a blit
// from an sRGB source into ANGLE's linear window surface linearises the
// colours on the way (GLES 3.0 §4.3.3), darkening the image; here the
// shader re-encodes them to sRGB when the window surface is linear.
type compositor struct {
	prog   uint32
	vao    uint32
	encode int32
}

const compositeVert = `#version 300 es
out vec2 vUV;
void main() {
	// One triangle covering the viewport rectangle.
	vec2 p = vec2(float((gl_VertexID << 1) & 2), float(gl_VertexID & 2));
	vUV = p;
	gl_Position = vec4(p * 2.0 - 1.0, 0.0, 1.0);
}
`

const compositeFrag = `#version 300 es
precision mediump float;
uniform sampler2D uColor;
uniform bool uEncodeSRGB;
in vec2 vUV;
out vec4 oColor;
void main() {
	vec4 c = texture(uColor, vUV);
	if (uEncodeSRGB) {
		vec3 lo = c.rgb * 12.92;
		vec3 hi = 1.055 * pow(c.rgb, vec3(1.0 / 2.4)) - 0.055;
		c.rgb = mix(lo, hi, step(vec3(0.0031308), c.rgb));
	}
	oColor = vec4(c.rgb, 1.0);
}
`

func newCompositor() (*compositor, error) {
	p, err := newProgram(compositeVert, compositeFrag)
	if err != nil {
		return nil, fmt.Errorf("render: composite program: %w", err)
	}
	gles.UseProgram(p)
	gles.Uniform1i(gles.GetUniformLocation(p, "uColor"), 0)
	gles.UseProgram(0)
	return &compositor{prog: p, vao: gles.GenVertexArray(), encode: gles.GetUniformLocation(p, "uEncodeSRGB")}, nil
}

// draw copies t into dst (window pixels, origin top-left) of the default
// framebuffer whose height is windowHeight.
func (c *compositor) draw(t *viewTarget, dst image.Rectangle, windowHeight int, encodeSRGB bool) {
	gles.BindFramebuffer(gles.FRAMEBUFFER, 0)
	gles.Viewport(dst.Min.X, windowHeight-dst.Max.Y, dst.Dx(), dst.Dy())
	gles.UseProgram(c.prog)
	enc := int32(0)
	if encodeSRGB {
		enc = 1
	}
	gles.Uniform1i(c.encode, enc)
	gles.ActiveTexture(gles.TEXTURE0)
	gles.BindTexture(gles.TEXTURE_2D, t.color)
	gles.BindVertexArray(c.vao)
	gles.DrawArrays(gles.TRIANGLES, 0, 3)
	gles.BindVertexArray(0)
	gles.BindTexture(gles.TEXTURE_2D, 0)
	gles.UseProgram(0)
}

func (c *compositor) release() {
	gles.DeleteProgram(c.prog)
	gles.DeleteVertexArray(c.vao)
}
