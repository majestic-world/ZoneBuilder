// Package render draws the 3D viewport with GLES 3.0 through ANGLE, inside a
// Gio window that uses app.CustomRenderer. The window loop owns the EGL
// context (package egl); each frame it calls Renderer.DrawViewport for the
// viewport rectangle and then lets Gio draw its panels on top.
package render

import (
	"errors"
	"fmt"
	"image"
	"math"
	"slices"

	"zonebuilder/internal/render/gles"
)

// Info describes the GL implementation, for the startup log and the panel.
type Info struct {
	Version  string
	Renderer string
	// ClipControl is GL_EXT_clip_control, which reversed Z needs (ADR 0001).
	ClipControl bool
	// DXT reports native DXT1/3/5 in sRGB (ADR 0002): ANGLE splits what
	// desktop GL calls GL_EXT_texture_compression_s3tc into
	// GL_EXT_texture_compression_dxt1, GL_ANGLE_texture_compression_dxt3/5
	// and GL_EXT_texture_compression_s3tc_srgb.
	DXT bool
	// S3TC is the literal GL_EXT_texture_compression_s3tc string, logged for
	// the record; ANGLE on D3D11 does not list it.
	S3TC bool
}

func queryInfo() Info {
	exts := gles.Extensions()
	has := func(e string) bool { return slices.Contains(exts, e) }
	return Info{
		Version:     gles.GetString(gles.VERSION),
		Renderer:    gles.GetString(gles.RENDERER),
		ClipControl: has("GL_EXT_clip_control") && gles.HasClipControl(),
		DXT: has("GL_EXT_texture_compression_dxt1") &&
			has("GL_ANGLE_texture_compression_dxt3") &&
			has("GL_ANGLE_texture_compression_dxt5") &&
			has("GL_EXT_texture_compression_s3tc_srgb"),
		S3TC: has("GL_EXT_texture_compression_s3tc"),
	}
}

// Renderer owns the viewport's GL resources. Every method must run on the
// thread holding the current EGL context.
type Renderer struct {
	Info Info

	// encodeSRGB is set when the window surface is linear, so the
	// compositor converts the linear scene colours to sRGB itself.
	encodeSRGB bool
	target     viewTarget
	comp       *compositor
	cube       *cube
}

// New checks the extensions the renderer depends on and creates its GL
// resources. surfaceSRGB tells whether the window surface encodes sRGB on
// write (egl.Context.SRGB).
func New(surfaceSRGB bool) (*Renderer, error) {
	if err := gles.Load(); err != nil {
		return nil, err
	}
	r := &Renderer{Info: queryInfo(), encodeSRGB: !surfaceSRGB}
	var missing []error
	if !r.Info.ClipControl {
		missing = append(missing, errors.New("GL_EXT_clip_control (reversed Z, ADR 0001)"))
	}
	if !r.Info.DXT {
		missing = append(missing, errors.New("native DXT1/3/5 sRGB textures (ADR 0002)"))
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("render: ANGLE (%s) lacks %w", r.Info.Renderer, errors.Join(missing...))
	}
	var err error
	if r.comp, err = newCompositor(); err != nil {
		return nil, err
	}
	if r.cube, err = newCube(); err != nil {
		r.Release()
		return nil, err
	}
	return r, nil
}

// Camera framing for the spike cube.
const (
	fovY      = 60 * math.Pi / 180
	nearPlane = 0.1
	farPlane  = 100
)

// DrawViewport renders the scene into rect (window pixels, origin top-left)
// of the window framebuffer, which is window pixels in size. The projection
// uses rect's own aspect ratio, so resizing never stretches the scene. GL
// state the Gio renderer relies on is left as it was found: framebuffer 0,
// no depth test, no scissor, default clip control.
func (r *Renderer) DrawViewport(rect image.Rectangle, window image.Point, angle float32) error {
	rect = rect.Intersect(image.Rectangle{Max: window})
	if rect.Empty() {
		return nil
	}
	size := rect.Size()
	if err := r.target.bind(size); err != nil {
		return err
	}
	gles.Viewport(0, 0, size.X, size.Y)
	// Reversed Z: depth 1 at the near plane, 0 at the far plane, cleared
	// to 0 and tested with GREATER. No face culling: the back faces are
	// drawn too and only the depth test hides them.
	gles.ClipControlEXT(gles.LOWER_LEFT_EXT, gles.ZERO_TO_ONE_EXT)
	gles.ClearColor(0.01, 0.012, 0.016, 1) // linear; the target encodes sRGB
	gles.ClearDepthf(0)
	gles.Clear(gles.COLOR_BUFFER_BIT | gles.DEPTH_BUFFER_BIT)
	gles.Enable(gles.DEPTH_TEST)
	gles.DepthFunc(gles.GREATER)

	aspect := float32(size.X) / float32(size.Y)
	fov := float32(fovY)
	if aspect < 1 {
		// Portrait viewport: widen the vertical field of view so the
		// horizontal one stays fovY and the cube still fits across.
		fov = 2 * float32(math.Atan(math.Tan(fovY/2)/float64(aspect)))
	}
	viewProj := mul(reversedPerspective(fov, aspect, nearPlane, farPlane), translate(0, 0, -4.5))
	r.cube.draw(viewProj, angle)

	gles.Disable(gles.DEPTH_TEST)
	gles.ClipControlEXT(gles.LOWER_LEFT_EXT, gles.NEGATIVE_ONE_TO_ONE_EXT)
	r.comp.draw(&r.target, rect, window.Y, r.encodeSRGB)
	if e := gles.GetError(); e != gles.NO_ERROR {
		return fmt.Errorf("render: GL error 0x%x in viewport pass", e)
	}
	return nil
}

// Release frees the GL resources. The context must still be current.
func (r *Renderer) Release() {
	if r.cube != nil {
		r.cube.release()
	}
	if r.comp != nil {
		r.comp.release()
	}
	r.target.release()
}
