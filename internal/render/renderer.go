// Package render draws the 3D viewport with GLES 3.0 through ANGLE, inside a
// Gio window that uses app.CustomRenderer. The window loop owns the EGL
// context (package egl); each frame it calls Renderer.DrawViewport for the
// viewport rectangle and then lets Gio draw its panels on top.
package render

import (
	"errors"
	"fmt"
	"image"
	"slices"
	"time"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/render/gles"
	"zonebuilder/internal/scene"
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
	// Anisotropic is GL_EXT_texture_filter_anisotropic; without it material
	// textures are sampled trilinear only.
	Anisotropic bool
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
		S3TC:        has("GL_EXT_texture_compression_s3tc"),
		Anisotropic: has("GL_EXT_texture_filter_anisotropic"),
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
	scene      *sceneRenderer
	zones      *zoneOverlay
	modelPass  *modelPass
	// models are the Models made and not released; queue the ones the
	// next DrawViewport draws (DrawModel).
	models map[*Model]struct{}
	queue  []*Model
}

// New checks the extensions the renderer depends on and creates its GL
// resources. surfaceSRGB tells whether the window surface encodes sRGB on
// write (egl.Context.SRGB).
func New(surfaceSRGB bool) (*Renderer, error) {
	if err := gles.Load(); err != nil {
		return nil, err
	}
	r := &Renderer{Info: queryInfo(), encodeSRGB: !surfaceSRGB, models: make(map[*Model]struct{})}
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
	if r.scene, err = newSceneRenderer(r.Info.Anisotropic); err != nil {
		r.Release()
		return nil, err
	}
	if r.zones, err = newZoneOverlay(); err != nil {
		r.Release()
		return nil, err
	}
	if r.modelPass, err = newModelPass(); err != nil {
		r.Release()
		return nil, err
	}
	return r, nil
}

// SetOrigin sets the rebase origin of the scenes drawn (scene.World's
// Origin): the camera's render space has it at zero.
func (r *Renderer) SetOrigin(origin geom.Vec3) {
	r.scene.rebase = origin
}

// SetMeshesHidden hides (or shows again) the static mesh actors, leaving
// only the map's fixed geometry, terrain and BSP, as UnrealEd's W does.
func (r *Renderer) SetMeshesHidden(hidden bool) {
	r.scene.hideMeshes = hidden
}

// SetHiddenActors leaves the static mesh actors in hidden out of the view,
// one by one, whatever SetMeshesHidden says. The next frame cuts their
// triangles out of the batches they share with the other actors.
func (r *Renderer) SetHiddenActors(hidden *scene.HiddenActors) {
	r.scene.hidden = hidden
}

// AddScene queues the scene p was prepared from for upload. Upload puts it
// on the GPU a little per frame; it is drawn once it is all there.
func (r *Renderer) AddScene(p *Prepared) {
	r.scene.add(p)
}

// RemoveScene stops drawing s, uploaded or still queued, and leaves its
// GPU copy for Upload to free; a texture another scene uses stays.
func (r *Renderer) RemoveScene(s *scene.Scene) {
	r.scene.remove(s)
}

// Upload spends about budget, at least one step, freeing the removed
// scenes and then uploading the queued ones. It returns the scenes that
// became complete (and drawn), and whether work remains for later frames.
func (r *Renderer) Upload(budget time.Duration) (done []*scene.Scene, more bool) {
	return r.scene.upload(budget)
}

// UploadProgress is how much of s is on the GPU, 0 to 1; false when s was
// never added or was removed.
func (r *Renderer) UploadProgress(s *scene.Scene) (float32, bool) {
	return r.scene.progress(s)
}

// Stats are the counts of the last DrawViewport.
func (r *Renderer) Stats() DrawStats {
	return r.scene.stats
}

// SetZones replaces the zone shapes drawn over the scene (nil: none).
// shapes is only read during the call.
func (r *Renderer) SetZones(shapes []ZoneShape) {
	r.zones.set(shapes)
}

// SetGround replaces the ground marking: the grid and the selected zone's
// footprint. g is only read during the call.
func (r *Renderer) SetGround(g Ground) {
	r.scene.ground.set(g)
}

// DrawModel queues m's instances for the next DrawViewport only: whoever
// shows a model calls it every frame, so a model nobody queues is gone
// from the next frame on. Models draw in a pass of their own between the
// scene's Masked (and terrain layer) and Translucent passes, in queue
// order.
func (r *Renderer) DrawModel(m *Model) {
	r.queue = append(r.queue, m)
}

// DrawViewport renders the scene seen by cam into rect (window pixels,
// origin top-left) of the window framebuffer, which is window pixels in
// size. The projection uses rect's own aspect ratio, so resizing never
// stretches the scene. GL state the Gio renderer relies on is left as it
// was found: framebuffer 0, no depth test, no scissor, default clip control.
func (r *Renderer) DrawViewport(rect image.Rectangle, window image.Point, cam *camera.Camera) error {
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

	// The camera lives in the rebased Y-up render basis; the scene's
	// vertices are rebased in the shader and swapped by unrealToRender.
	aspect := float32(size.X) / float32(size.Y)
	right, up := cam.Basis()
	view := lookAt(cam.Position, cam.Forward(), right, up)
	proj := reversedPerspective(camera.FovY, aspect, camera.Near, cam.Far)
	viewProj := mul(proj, mul(view, unrealToRender))
	r.scene.draw(viewProj, scene.ToRender(cam.Position), func() {
		r.modelPass.draw(viewProj, r.scene.rebase, r.queue, &r.scene.stats)
	})
	r.queue = r.queue[:0]
	r.zones.draw(viewProj, [3]float32{r.scene.rebase.X, r.scene.rebase.Y, r.scene.rebase.Z})

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
	for m := range r.models {
		m.Release()
	}
	if r.modelPass != nil {
		r.modelPass.release()
	}
	if r.scene != nil {
		r.scene.release()
	}
	if r.zones != nil {
		r.zones.release()
	}
	if r.comp != nil {
		r.comp.release()
	}
	r.target.release()
}
