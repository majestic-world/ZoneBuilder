package main

import (
	"log"
	"math"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/model"
	"zonebuilder/internal/play"
	"zonebuilder/internal/render"
	"zonebuilder/internal/scene"
)

// playMode is the game mode (spec D8): the embedded human walking the open
// map with its collision. It sits in front of the active editing mode: while
// it runs, the viewport's events and keys come here and the mode gets none,
// so its selection and history stay as they were; the mode still presents
// and syncs, so its overlay (the zone outlines) stays drawn. Esc ends it and
// puts the edit camera back where it was.
type playMode struct {
	w *app.Window
	// gen tells a preparation still wanted from one abandoned by Esc.
	gen      int
	results  chan playPrepared
	starting bool
	// saved is the edit camera, restored on stop.
	saved camera.Camera

	s           *play.Session
	origin      geom.Vec3 // the World's origin the camera is rebased on
	firstPerson bool
	body        *humanBody

	held           map[key.Name]bool
	shift          bool
	jump, flight   bool
	dragging       bool
	last           f32.Point
	lookX, lookY   float32
	stepped        time.Time
	triangles      int
	bundle         *model.Bundle
	// gpu is the human on owner, the renderer it was made for.
	gpu      *render.Model
	owner    *render.Renderer
	instance [1]render.Instance
}

// playPrepared is a finished preparation: the collision world and the
// human, built off the event loop.
type playPrepared struct {
	gen       int
	world     *play.World
	triangles int
	bundle    *model.Bundle
	err       error
	eye       geom.Vec3
	yaw       float32
	pitch     float32
	took      time.Duration
}

func newPlayMode(w *app.Window) *playMode {
	return &playMode{w: w, results: make(chan playPrepared, 1)}
}

// active reports whether the game mode holds the viewport: preparing or
// playing.
func (p *playMode) active() bool { return p.starting || p.s != nil }

// playing reports whether the human walks (the preparation is over).
func (p *playMode) playing() bool { return p.s != nil }

// collisionBox spans every loaded tile: the World's X/Y test is all that
// matters, and terrain cells are clamped to the grid.
var collisionBox = geom.Box{Min: geom.Vec3{X: -1e8, Y: -1e8, Z: -1e8}, Max: geom.Vec3{X: 1e8, Y: 1e8, Z: 1e8}}

// start saves the edit camera and prepares the collision of the open map
// off the event loop; the human then stands on the ground under the
// screen's centre (the camera position when the centre ray hits nothing).
func (p *playMode) start(ws *workspace) locale.Message {
	w := ws.world()
	if w == nil {
		return locale.Message{Key: "spawn.play.no_map"}
	}
	p.gen++
	p.starting = true
	p.saved = ws.cam
	p.origin = w.Origin
	vp := ws.viewport()
	eye := worldPosition(w, ws.cam.Position)
	if hit, ok := pickAt(w, &ws.cam, f32.Pt(float32(vp.X)/2, float32(vp.Y)/2), vp); ok {
		eye = scene.FromServer(hit.Pos).Add(geom.Vec3{Z: play.EyeHeight})
	}
	// A snapshot of the scenes: the loop may add or remove tiles meanwhile.
	// Collision is the map's: meshes hidden from view still block.
	snap := scene.NewWorld(w.Origin)
	for _, s := range w.Scenes() {
		snap.Add(s)
	}
	gen, yaw, pitch, bundle := p.gen, ws.cam.Yaw, ws.cam.Pitch, p.bundle
	go func() {
		t0 := time.Now()
		var tris []play.Triangle
		snap.Geometry(collisionBox, func(t scene.Triangle) {
			if t.Blocks {
				tris = append(tris, play.Triangle{scene.FromServer(t.A), scene.FromServer(t.B), scene.FromServer(t.C)})
			}
		}, nil)
		n := len(tris)
		res := playPrepared{gen: gen, world: play.NewWorld(tris), triangles: n, eye: eye, yaw: yaw, pitch: pitch, bundle: bundle}
		if res.bundle == nil {
			res.bundle, res.err = model.Decode(model.Human)
		}
		res.took = time.Since(t0)
		p.results <- res
		p.w.Invalidate()
	}()
	log.Printf("jogo: preparando colisões a partir de %.0f %.0f %.0f", eye.X, eye.Y, eye.Z)
	return locale.Message{Key: "spawn.play.preparing"}
}

// receive installs a finished preparation, if any.
func (p *playMode) receive(ws *workspace) {
	select {
	case r := <-p.results:
		if r.gen != p.gen || !p.starting {
			return
		}
		p.starting = false
		if r.err != nil {
			log.Printf("jogo: humano indisponível: %v", r.err)
			ws.cam = p.saved
			ws.status = actionError(locale.Message{Key: "spawn.play.no_human"}, r.err, nil)
			return
		}
		p.bundle = r.bundle
		p.triangles = r.triangles
		p.s = play.NewSession(r.world, r.eye, r.yaw, r.pitch)
		p.body = newHumanBody(r.bundle, r.yaw)
		p.body.advance(0, geom.Vec3{}, p.s.Motion())
		p.firstPerson = false
		p.held = map[key.Name]bool{}
		p.shift, p.jump, p.flight, p.dragging = false, false, false, false
		p.lookX, p.lookY = 0, 0
		p.stepped = time.Time{}
		f := p.s.Feet()
		log.Printf("jogo: %s de colisão em %v, pés em %.0f %.0f %.0f (%s)",
			inflect.Count(r.triangles, "triângulo", "triângulos"), r.took.Round(time.Millisecond), f.X, f.Y, f.Z, p.s.Motion())
		p.place(ws)
	default:
	}
}

// stop ends the game mode and restores the edit camera.
func (p *playMode) stop(ws *workspace) {
	if !p.active() {
		return
	}
	p.gen++
	p.starting = false
	p.s = nil
	p.body = nil
	ws.cam = p.saved
	log.Printf("jogo: fim, câmera %s", formatPoseOr(&ws.cam, ws.world()))
	ws.status = action(locale.Message{Key: "spawn.play.stopped"})
}

// event takes one viewport event while the game mode is active; Esc stops
// it.
func (p *playMode) event(ws *workspace, ev event.Event) {
	switch e := ev.(type) {
	case key.FocusEvent:
		if !e.Focus {
			clear(p.held)
			p.shift = false
			p.dragging = false
		}
	case key.Event:
		if e.Name == key.NameEscape {
			if e.State == key.Press {
				p.stop(ws)
			}
			return
		}
		down := e.State == key.Press
		if e.Name == key.NameShift {
			p.shift = down
			return
		}
		p.shift = e.Modifiers.Contain(key.ModShift)
		if down && !p.held[e.Name] {
			switch e.Name {
			case key.NameSpace:
				p.jump = true
			case "F":
				p.flight = true
			case "V":
				p.firstPerson = !p.firstPerson
			}
		}
		if p.held != nil {
			p.held[e.Name] = down
		}
	case pointer.Event:
		switch e.Kind {
		case pointer.Press:
			p.dragging, p.last = true, e.Position
		case pointer.Release:
			p.dragging = e.Buttons != 0
			p.last = e.Position
		case pointer.Drag:
			d := e.Position.Sub(p.last)
			p.last = e.Position
			if p.dragging {
				p.lookX += d.X
				p.lookY += d.Y
			}
		}
	}
}

// step advances the human by the time since the last step and moves the
// camera with it. It reports whether the game mode needs the next frame.
func (p *playMode) step(now time.Time, ws *workspace) bool {
	if p.starting {
		return false // the preparation invalidates when done
	}
	if p.s == nil {
		return false
	}
	var dt float32
	if !p.stepped.IsZero() {
		dt = float32(now.Sub(p.stepped).Seconds())
	}
	p.stepped = now
	axis := func(pos, neg key.Name) float32 {
		var a float32
		if p.held[pos] {
			a++
		}
		if p.held[neg] {
			a--
		}
		return a
	}
	before := p.s.Feet()
	p.s.Step(play.Input{
		Seconds: dt,
		Forward: axis("W", "S"), Right: axis("D", "A"), Up: axis("E", "Q"),
		Fast: p.shift, Jump: p.jump, ToggleFlight: p.flight,
		LookX: p.lookX, LookY: p.lookY,
	})
	if p.flight {
		log.Printf("jogo: %s", p.s.Motion())
	}
	p.jump, p.flight, p.lookX, p.lookY = false, false, 0, 0
	clip := p.body.anim.Clip()
	if dt > 0 {
		// A frame with no time moves nothing: it must not read as "stopped".
		p.body.advance(min(dt, 0.1), p.s.Feet().Sub(before), p.s.Motion())
	}
	if c := p.body.anim.Clip(); c != clip {
		log.Printf("jogo: animação %s (%s)", clipName(c, p.bundle), p.s.Motion())
	}
	p.place(ws)
	return true
}

// place puts the viewport camera on the session's: behind the human in
// third person, at the eye in first person.
func (p *playMode) place(ws *workspace) {
	pos := p.s.Camera()
	if p.firstPerson {
		pos = p.s.Eye()
	}
	ws.cam.Position = scene.ToRender(pos.Sub(p.origin))
	ws.cam.Yaw, ws.cam.Pitch = p.s.View()
}

// bodyShown reports whether the human is drawn this frame: never in first
// person, nor when a wall pulled the third-person camera into the head.
func (p *playMode) bodyShown() bool {
	return p.s != nil && !p.firstPerson && p.s.BodyVisible()
}

// message is the game mode's line for the message card.
func (p *playMode) message(lang locale.Language) string {
	if p.starting {
		return locale.Text(lang, "spawn.play.preparing")
	}
	if p.firstPerson {
		return locale.Text(lang, "spawn.play.hud.first")
	}
	return locale.Text(lang, "spawn.play.hud.third")
}

// humanBody is the human's animation state: the clip chosen from the
// capsule's motion, and the yaw from its movement. Port of UE2-Studio's
// Avatar::advance (src/play_map/avatar.rs).
type humanBody struct {
	bundle *model.Bundle
	anim   *model.Animator
	yaw    float32
}

func newHumanBody(b *model.Bundle, yaw float32) *humanBody {
	return &humanBody{bundle: b, anim: model.NewAnimator(b), yaw: yaw}
}

// movingSpeed is the speed (units per second) above which the human counts
// as moving: it runs and turns to the movement.
const movingSpeed = 5

// advance picks the clip for motion and the feet's movement over seconds,
// turns the body to the movement, and plays the clip.
func (h *humanBody) advance(seconds float32, moved geom.Vec3, motion play.Motion) {
	horizontal := geom.Vec3{X: moved.X, Y: moved.Y}
	moving := seconds > 0 && horizontal.Length() > seconds*movingSpeed
	var clip int
	switch motion {
	case play.Jumping:
		clip = int(h.bundle.JumpClips[b2i(moving)])
		if c := h.anim.Clip(); c == int(h.bundle.JumpClips[0]) || c == int(h.bundle.JumpClips[1]) {
			clip = c
		}
	case play.Falling:
		clip = model.ClipFalling
	case play.Flying:
		clip = b2i(seconds > 0 && moved.Length() > seconds*movingSpeed)
	default:
		clip = b2i(moving)
	}
	if moving {
		h.yaw = float32(math.Atan2(float64(horizontal.Y), float64(horizontal.X)))
	}
	h.anim.Update(clip, seconds)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// clipName names clip c of the human for the log.
func clipName(c int, b *model.Bundle) string {
	switch {
	case c == model.ClipIdle:
		return "parado"
	case c == model.ClipRun:
		return "correndo"
	case c == model.ClipFalling:
		return "caindo"
	case b != nil && c == int(b.JumpClips[0]):
		return "pulo parado"
	case b != nil && c == int(b.JumpClips[1]):
		return "pulo correndo"
	}
	return "clipe"
}

// formatPoseOr is formatPose, or "-" without a map.
func formatPoseOr(cam *camera.Camera, w *scene.World) string {
	if w == nil {
		return "-"
	}
	return formatPose(cam, w)
}

// focusViewport gives the viewport the keyboard, so the game keys reach
// it right after the Jogar button took the click.
func focusViewport(gtx layout.Context, ws *workspace) {
	gtx.Execute(key.FocusCmd{Tag: &ws.shell.Viewport})
}

// draw queues the human on r for this frame, when shown. The GPU model
// belongs to the renderer it was made on: a new renderer makes it again.
func (p *playMode) draw(r *render.Renderer) error {
	if !p.bodyShown() {
		return nil
	}
	if p.owner != r {
		gpu, err := r.NewModel(p.bundle)
		if err != nil {
			return err
		}
		p.gpu, p.owner = gpu, r
	}
	p.gpu.Pose(p.body.anim)
	p.instance[0] = render.Instance{Pos: scene.ToServer(p.s.Feet()), Yaw: p.body.yaw}
	p.gpu.SetInstances(p.instance[:])
	r.DrawModel(p.gpu)
	return nil
}
