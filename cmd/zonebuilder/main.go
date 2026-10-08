// Command zonebuilder is the Zone Builder desktop app.
//
// Build with scripts/build.ps1, which also places ANGLE's libEGL.dll and
// libGLESv2.dll next to the executable; the app loads them at run time.
package main

import (
	"fmt"
	"image"
	"log"
	"os"
	"runtime"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/gpu"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"zonebuilder/internal/render"
	"zonebuilder/internal/render/egl"
	"zonebuilder/internal/ui"
)

func main() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("Zone Builder"), app.Size(unit.Dp(1280), unit.Dp(800)), app.CustomRenderer(true))
		if err := run(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

// gfx is everything bound to the window's EGL context.
type gfx struct {
	ctx      *egl.Context
	gio      gpu.GPU
	renderer *render.Renderer
}

// newGfx creates the context for the window's HWND (on the window thread, as
// Gio requires for window-bound calls) and makes it current on the calling
// thread, which then owns all GL calls.
func newGfx(w *app.Window, ve app.Win32ViewEvent) (*gfx, error) {
	var ctx *egl.Context
	var err error
	w.Run(func() { ctx, err = egl.NewContext(ve.HWND) })
	if err != nil {
		return nil, err
	}
	if err := ctx.MakeCurrent(); err != nil {
		ctx.Release()
		return nil, err
	}
	ctx.SetSwapInterval(1)
	g := &gfx{ctx: ctx}
	if g.renderer, err = render.New(ctx.SRGB); err != nil {
		g.release()
		return nil, err
	}
	if g.gio, err = gpu.New(gpu.OpenGL{ES: true, Shared: true}); err != nil {
		g.release()
		return nil, fmt.Errorf("gio gpu: %w", err)
	}
	i := g.renderer.Info
	log.Printf("gfx: %s", ctx.Describe())
	log.Printf("gfx: %s | %s", i.Version, i.Renderer)
	log.Printf("gfx: GL_EXT_clip_control=%t DXT1/3/5+sRGB=%t GL_EXT_texture_compression_s3tc=%t surface sRGB=%t",
		i.ClipControl, i.DXT, i.S3TC, ctx.SRGB)
	return g, nil
}

func (g *gfx) release() {
	if g == nil {
		return
	}
	if g.ctx.MakeCurrent() == nil {
		if g.gio != nil {
			g.gio.Release()
		}
		if g.renderer != nil {
			g.renderer.Release()
		}
	}
	g.ctx.Release()
}

func run(w *app.Window) error {
	// EGL binds the context to an OS thread: keep this goroutine on one.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	shell := &ui.Shell{Theme: th}

	var (
		ops       op.Ops
		g         *gfx
		view      app.Win32ViewEvent
		size      image.Point
		vpRect    image.Rectangle
		angle     float32
		paused    bool
		last      = time.Now()
		vpClicks  int
		lastClick image.Point
		btnClicks int
	)
	defer func() { g.release() }()

	for {
		switch e := w.Event().(type) {
		case app.Win32ViewEvent:
			g.release()
			g = nil
			view = e
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			if g == nil && view.Valid() && e.Size != (image.Point{}) {
				var err error
				if g, err = newGfx(w, view); err != nil {
					return err
				}
			}

			for {
				ev, ok := shell.Viewport.Update(gtx)
				if !ok {
					break
				}
				switch ev.Kind {
				case pointer.Press:
					vpClicks++
					lastClick = ev.Position.Round()
					log.Printf("viewport: press %v at (%d, %d)", ev.Buttons, lastClick.X, lastClick.Y)
				case pointer.Release:
					log.Printf("viewport: release at (%.0f, %.0f)", ev.Position.X, ev.Position.Y)
				case pointer.Scroll:
					log.Printf("viewport: scroll %.0f", ev.Scroll.Y)
				}
			}
			if shell.Pause.Clicked(gtx) {
				btnClicks++
				paused = !paused
				log.Printf("ui: button clicked (paused=%t)", paused)
			}

			now := time.Now()
			if !paused {
				angle += float32(now.Sub(last).Seconds())
			}
			last = now

			label := "Pausar rotação"
			if paused {
				label = "Retomar rotação"
			}
			rect := shell.Layout(gtx, label, panelLines(g, vpRect.Size(), vpClicks, lastClick, btnClicks))
			if e.Size != size || rect != vpRect {
				log.Printf("frame: window %dx%d, viewport %v (aspect %.3f)", e.Size.X, e.Size.Y, rect, aspect(rect))
				size, vpRect = e.Size, rect
			}
			if g == nil {
				e.Frame(gtx.Ops)
				continue
			}

			g.ctx.WaitClient() // lets ANGLE pick up a window resize
			if err := g.renderer.DrawViewport(rect, e.Size, angle); err != nil {
				return err
			}
			if err := g.gio.Frame(gtx.Ops, gpu.OpenGLRenderTarget{}, e.Size); err != nil {
				return fmt.Errorf("gio frame: %w", err)
			}
			if err := g.ctx.SwapBuffers(); err != nil {
				return err
			}
			if !paused {
				gtx.Execute(op.InvalidateCmd{})
			}
			e.Frame(gtx.Ops)
		}
	}
}

func aspect(r image.Rectangle) float64 {
	if r.Dy() == 0 {
		return 0
	}
	return float64(r.Dx()) / float64(r.Dy())
}

func panelLines(g *gfx, vp image.Point, vpClicks int, lastClick image.Point, btnClicks int) []string {
	lines := []string{"Zone Builder: spike M0"}
	if g == nil {
		return append(lines, "Sem contexto GL")
	}
	i := g.renderer.Info
	lines = append(lines,
		i.Renderer,
		i.Version,
		"Profundidade: Z reverso (GL_EXT_clip_control + Depth32F)",
		"Texturas: DXT1/3/5 nativas em sRGB",
		fmt.Sprintf("Viewport: %d×%d px", vp.X, vp.Y),
		fmt.Sprintf("Viewport: %s", count(vpClicks, "clique", "cliques")),
	)
	if vpClicks > 0 {
		lines = append(lines, fmt.Sprintf("Último clique no viewport: (%d, %d)", lastClick.X, lastClick.Y))
	}
	return append(lines, fmt.Sprintf("Botão: %s", count(btnClicks, "clique", "cliques")))
}

// count inflects a noun to n ("1 clique", "2 cliques").
func count(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}
