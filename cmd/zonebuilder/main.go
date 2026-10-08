// Command zonebuilder is the Zone Builder desktop app.
//
// Build with scripts/build.ps1, which also places ANGLE's libEGL.dll and
// libGLESv2.dll next to the executable; the app loads them at run time.
package main

import (
	"cmp"
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/gpu"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/project"
	"zonebuilder/internal/render"
	"zonebuilder/internal/render/egl"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
)

func main() {
	client := flag.String("client", "", "pasta do cliente (acima de Maps) que o campo traz preenchida; vazio usa a da configuração do usuário, depois ZB_CLIENT")
	tile := flag.String("tile", "", "tile que o campo traz preenchido; vazio usa o mapa mais recente, depois 22_22")
	out := flag.String("out", "", "pasta de saída do XML que o campo traz preenchida; vazio usa a da configuração do usuário")
	proj := flag.String("project", "", "projeto ("+project.Ext+") aberto ao iniciar")
	pose := flag.String("camera", "", `pose da câmera ao abrir um tile, "x,y,z,yaw,pitch": posição de mundo (coordenadas do servidor) e ângulos em radianos, no formato que o log "cena: câmera" imprime; vazio enquadra o mapa`)
	flag.Parse()
	sess := loadSession()
	fields := startFields{
		client: cmp.Or(*client, sess.cfg.Client, os.Getenv("ZB_CLIENT")),
		tile:   cmp.Or(*tile, firstOr(sess.cfg.RecentMaps, ""), "22_22"),
		out:    cmp.Or(*out, sess.cfg.Output),
	}
	var start *cameraPose
	if *pose != "" {
		p, err := parsePose(*pose)
		if err != nil {
			log.Fatalf("-camera: %v", err)
		}
		start = &p
	}
	go func() {
		w := new(app.Window)
		w.Option(app.Title("Zone Builder"), app.Size(unit.Dp(1280), unit.Dp(800)), app.CustomRenderer(true))
		if err := run(w, sess, fields, *proj, start); err != nil {
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
	log.Printf("gfx: GL_EXT_clip_control=%t DXT1/3/5+sRGB=%t GL_EXT_texture_compression_s3tc=%t anisotropic=%t surface sRGB=%t",
		i.ClipControl, i.DXT, i.S3TC, i.Anisotropic, ctx.SRGB)
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

// loaded is the outcome of a background scene.Load.
type loaded struct {
	root  string
	tiles []scene.Tile
	scene *scene.Scene
	err   error
	took  time.Duration
}

// startFields are what the client, tile and output fields hold on start.
type startFields struct{ client, tile, out string }

// firstOr is s[0], or def when s is empty.
func firstOr(s []string, def string) string {
	if len(s) == 0 {
		return def
	}
	return s[0]
}

// startLoad loads tiles of the client at root in the background; the
// outcome arrives on loads.
func startLoad(w *app.Window, loads chan<- loaded, root string, tiles []scene.Tile) {
	go func() {
		began := time.Now()
		s, err := scene.Load(root, tiles)
		loads <- loaded{root: root, tiles: tiles, scene: s, err: err, took: time.Since(began)}
		w.Invalidate()
	}()
}

func run(w *app.Window, sess *session, fields startFields, proj string, start *cameraPose) error {
	// EGL binds the context to an OS thread: keep this goroutine on one.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	shell := ui.NewShell(th, fields.client, fields.tile, fields.out)
	shell.Project.RecentMaps = sess.cfg.RecentMaps

	var (
		ops      op.Ops
		g        *gfx
		view     app.Win32ViewEvent
		size     image.Point
		vpRect   image.Rectangle
		fly      ui.FlyControls
		cam      = camera.ForBounds(geom.EmptyBox())
		current  *scene.Scene
		uploaded = true // nothing to upload yet
		loading  bool
		status   string
		loads    = make(chan loaded, 1)
		folders  = make(chan string, 1)
		outputs  = make(chan string, 1)
		probe    cursorProbe
		zones    = newZoneEditor()
		// tiles are the open map tiles, which the project file keeps.
		tiles []scene.Tile
		// zonesShown is the zones.version the renderer last got.
		zonesShown = -1
	)
	defer func() { g.release() }()
	if proj != "" {
		var load []scene.Tile
		status, load = sess.open(w, shell, zones, proj)
		if len(load) > 0 {
			tiles, loading = load, true
			startLoad(w, loads, shell.Client.Text(), load)
		}
	}

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
				uploaded, zonesShown = current == nil, -1
			}

			for {
				ev, ok := shell.Viewport.Update(gtx)
				if !ok {
					break
				}
				if msg, used := zones.viewportEvent(current, &cam, ev, shell.Viewport.Size()); used {
					if msg != "" {
						status = msg
					}
				} else {
					fly.Handle(ev, &cam)
				}
				switch e := ev.(type) {
				case pointer.Event:
					if probe.handle(e) && current != nil {
						probe.click, probe.clickHit = pickAt(current, &cam, e.Position, shell.Viewport.Size())
						logClick(probe.click, probe.clickHit)
						if msg := zones.click(current, &cam, e.Position, shell.Viewport.Size(), probe.click, probe.clickHit); msg != "" {
							status = msg
						}
					}
				case key.Event:
					if (e.Name == key.NameReturn || e.Name == key.NameEnter) && e.State == key.Press {
						if msg := zones.close(); msg != "" {
							status = msg
						}
					}
					if e.Name == key.NameEscape && e.State == key.Press {
						if msg := zones.escape(); msg != "" {
							status = msg
						}
					}
				}
			}
			if shell.Browse.Clicked(gtx) {
				start := shell.Client.Text()
				go func() {
					if p, ok := ui.PickFolder("Pasta do cliente Lineage II (a que contém Maps)", start); ok {
						folders <- p
						w.Invalidate()
					}
				}()
			}
			select {
			case p := <-folders:
				shell.Client.SetText(p)
			default:
			}
			if shell.Zone.BrowseOutput.Clicked(gtx) {
				start := shell.Zone.Output.Text()
				go func() {
					if p, ok := ui.PickFolder("Pasta de saída do XML de zonas", start); ok {
						outputs <- p
						w.Invalidate()
					}
				}()
			}
			select {
			case p := <-outputs:
				shell.Zone.Output.SetText(p)
				sess.outputUsed(p)
			default:
			}
			if shell.Zone.CreateRequested(gtx) {
				status = zones.create(shell.Zone.Name.Text(), shell.Zone.Type(), shell.Zone.Tools.Shape)
			}
			if t, ok := shell.Zone.Tools.Requested(gtx); ok {
				status = zones.arm(t, shell.Zone.Tools.Banned.Value)
			}
			if shell.Zone.Compile.Clicked(gtx) {
				status = zones.compile(shell.Zone.Output.Text())
				sess.outputUsed(shell.Zone.Output.Text())
			}
			if msg, load := sess.update(gtx, w, shell, zones, tiles, loading); msg != "" || len(load) > 0 {
				status = msg
				if len(load) > 0 {
					tiles, loading = load, true
					status = "Carregando " + strings.Join(tileNames(load), ", ") + "…"
					startLoad(w, loads, shell.Client.Text(), load)
				}
			}
			openTile := shell.OpenRequested(gtx)
			if m, ok := shell.Project.RecentMapClicked(gtx); ok {
				shell.Tile.SetText(m)
				openTile = true
			}
			if openTile && !loading {
				t, err := scene.ParseTile(shell.Tile.Text())
				if err != nil {
					status = err.Error()
				} else {
					loading, status = true, "Carregando "+t.Name()+"…"
					startLoad(w, loads, shell.Client.Text(), []scene.Tile{t})
				}
			}
			select {
			case r := <-loads:
				loading = false
				if r.err != nil {
					status = r.err.Error()
					log.Printf("cena: %s: %v", strings.Join(tileNames(r.tiles), ", "), r.err)
					break
				}
				current, uploaded, tiles = r.scene, false, r.tiles
				sess.mapOpened(r.root, r.tiles)
				shell.Project.RecentMaps = sess.cfg.RecentMaps
				cam = camera.ForBounds(renderBox(current, current.Framing))
				if start != nil {
					start.apply(&cam, current)
				}
				log.Printf("cena: câmera %s", formatPose(&cam, current))
				status = strings.Join(tileNames(r.tiles), ", ")
				logScene(r)
				probe.click = scene.Hit{}
				probe.clickHit = false
			default:
			}

			moving := fly.Step(&cam, gtx.Now)
			shell.Status = probe.status(current, &cam, shell.Viewport.Size())
			shell.Zone.Info = zones.info()
			if msg := zones.panel(gtx, &shell.Edit, current); msg != "" {
				status = msg
			}
			if zones.anchored && current != nil && probe.inside {
				zones.hoverAt(pickAt(current, &cam, probe.cursor, shell.Viewport.Size()))
			} else {
				zones.hoverAt(scene.Hit{}, false)
			}
			shell.Zone.Tools.Armed, shell.Zone.Tools.Active = zones.tool, zones.armed

			rect := shell.Layout(gtx, panelLines(g, status, current, &cam))
			if e.Size != size || rect != vpRect {
				log.Printf("frame: window %dx%d, viewport %v", e.Size.X, e.Size.Y, rect)
				size, vpRect = e.Size, rect
			}
			if g == nil {
				e.Frame(gtx.Ops)
				continue
			}
			if !uploaded {
				g.renderer.SetScene(current)
				uploaded = true
			}
			if zonesShown != zones.version {
				g.renderer.SetZones(zones.overlay())
				zonesShown = zones.version
			}

			g.ctx.WaitClient() // lets ANGLE pick up a window resize
			if err := g.renderer.DrawViewport(rect, e.Size, &cam); err != nil {
				return err
			}
			if err := g.gio.Frame(gtx.Ops, gpu.OpenGLRenderTarget{}, e.Size); err != nil {
				return fmt.Errorf("gio frame: %w", err)
			}
			if err := g.ctx.SwapBuffers(); err != nil {
				return err
			}
			if moving {
				gtx.Execute(op.InvalidateCmd{})
			}
			e.Frame(gtx.Ops)
		}
	}
}

// renderBox converts a world box of s into the camera's rebased render
// space.
func renderBox(s *scene.Scene, b geom.Box) geom.Box {
	if b.Empty() {
		return b
	}
	return geom.Box{Min: scene.ToRender(b.Min.Sub(s.Origin)), Max: scene.ToRender(b.Max.Sub(s.Origin))}
}

// worldPosition is a rebased render-space point of s in world coordinates.
func worldPosition(s *scene.Scene, p geom.Vec3) geom.Vec3 {
	return scene.ToRender(p).Add(s.Origin)
}

func logScene(r loaded) {
	s := r.scene
	log.Printf("cena: %s carregado em %v: %s, origem de rebase %v",
		strings.Join(tileNames(r.tiles), ", "), r.took.Round(time.Millisecond), count(len(s.Batches), "batch", "batches"), s.Origin)
	for _, t := range s.Terrains {
		ox, oy := t.Tile.Origin()
		textured := 0
		for _, b := range t.Layers {
			if s.Batches[b].Texture != nil {
				textured++
			}
		}
		log.Printf("cena: terreno %s: %d×%d amostras, %s, %s, faixa x [%.1f, %.1f] y [%.1f, %.1f] z [%.1f, %.1f]; início do tile (%.0f, %.0f); fallback MapX/MapY=%t",
			t.Tile.Name(), t.Width, t.Height, count(len(s.Batches[t.Batch].Indices)/3, "triângulo", "triângulos"),
			count(textured, "camada texturizada", "camadas texturizadas"),
			t.Bounds.Min.X, t.Bounds.Max.X, t.Bounds.Min.Y, t.Bounds.Max.Y, t.Bounds.Min.Z, t.Bounds.Max.Z,
			ox, oy, t.FallbackScale)
	}
	if n := len(s.BSPSurfaces); n > 0 {
		log.Printf("cena: BSP: %s, %s", count(n, "superfície", "superfícies"), count(bspTriangles(s), "triângulo", "triângulos"))
	}
	log.Printf("cena: %s", meshSummary(s))
	untextured := 0
	for _, b := range s.Batches {
		if b.Texture == nil {
			untextured += len(b.Indices) / 3
		}
	}
	log.Printf("cena: %s sem textura", count(untextured, "triângulo", "triângulos"))
	for _, w := range s.Warnings {
		log.Printf("cena: aviso: %s", w)
	}
}

func panelLines(g *gfx, status string, s *scene.Scene, cam *camera.Camera) []string {
	var lines []string
	if status != "" {
		lines = append(lines, status)
	}
	if s != nil {
		for _, t := range s.Terrains {
			b := t.Bounds
			lines = append(lines,
				fmt.Sprintf("Terreno: %s", count(len(s.Batches[t.Batch].Indices)/3, "triângulo", "triângulos")),
				fmt.Sprintf("x %.0f … %.0f", b.Min.X, b.Max.X),
				fmt.Sprintf("y %.0f … %.0f", b.Min.Y, b.Max.Y),
				fmt.Sprintf("z %.0f … %.0f", b.Min.Z, b.Max.Z),
			)
			if t.FallbackScale {
				lines = append(lines, "TerrainScale quebrado: posição por MapX/MapY")
			}
		}
		if len(s.Terrains) == 0 {
			lines = append(lines, "O mapa não tem terreno")
		}
		if n := len(s.BSPSurfaces); n > 0 {
			lines = append(lines, fmt.Sprintf("BSP: %s, %s", count(n, "superfície", "superfícies"), count(bspTriangles(s), "triângulo", "triângulos")))
		}
		lines = append(lines, meshSummary(s))
		lines = append(lines, s.Warnings...)
		p := worldPosition(s, cam.Position)
		lines = append(lines, fmt.Sprintf("Câmera: %.0f %.0f %.0f", p.X, p.Y, p.Z))
	}
	lines = append(lines,
		"WASD move, Q/E desce/sobe, Shift acelera,",
		"arrastar olha, Shift+arrastar sobe, roda aproxima",
	)
	if g != nil {
		lines = append(lines, g.renderer.Info.Renderer)
	}
	return lines
}

// meshSummary is the static mesh actor and triangle counts of s.
func meshSummary(s *scene.Scene) string {
	tris := 0
	for i := range s.Actors {
		tris += s.Actors[i].Triangles()
	}
	return fmt.Sprintf("Static meshes: %s, %s",
		count(len(s.Actors), "ator", "atores"), count(tris, "triângulo", "triângulos"))
}

func bspTriangles(s *scene.Scene) int {
	n := 0
	for _, sf := range s.BSPSurfaces {
		n += sf.Count / 3
	}
	return n
}

// count inflects a noun to n ("1 triângulo", "2 triângulos").
func count(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}
