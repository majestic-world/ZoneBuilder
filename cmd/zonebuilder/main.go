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
	fps := flag.Bool("fps", false, "mede a taxa de quadros: redesenha sem parar, sem vsync, e registra no log o tempo de quadro a cada 2 s")
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
		if err := run(w, sess, fields, *proj, start, *fps); err != nil {
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
// thread, which then owns all GL calls. vsync waits for the display on
// every swap.
func newGfx(w *app.Window, ve app.Win32ViewEvent, vsync bool) (*gfx, error) {
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
	ctx.SetSwapInterval(boolInt(vsync))
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

// uploadBudget is how long a frame may spend putting tiles on the GPU.
const uploadBudget = 6 * time.Millisecond

// startFields are what the client, tile and output fields hold on start.
type startFields struct{ client, tile, out string }

// firstOr is s[0], or def when s is empty.
func firstOr(s []string, def string) string {
	if len(s) == 0 {
		return def
	}
	return s[0]
}

// run is the window's event loop. measure redraws without pause or vsync
// and logs the frame times (the -fps flag).
func run(w *app.Window, sess *session, fields startFields, proj string, start *cameraPose, measure bool) error {
	// EGL binds the context to an OS thread: keep this goroutine on one.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	shell := ui.NewShell(th, fields.client, fields.tile, fields.out)
	shell.Project.RecentMaps = sess.cfg.RecentMaps

	var (
		ops     op.Ops
		g       *gfx
		view    app.Win32ViewEvent
		size    image.Point
		vpRect  image.Rectangle
		fly     ui.FlyControls
		cam     = camera.ForBounds(geom.EmptyBox())
		tiles   = newTiles(w)
		frames  frameLog
		status  string
		folders = make(chan string, 1)
		outputs = make(chan string, 1)
		probe   cursorProbe
		zones   = newZoneEditor()
		// zonesShown is the zones.version the renderer last got.
		zonesShown = -1
	)
	defer func() { g.release() }()
	if proj != "" {
		var load []scene.Tile
		status, load = sess.open(w, shell, zones, proj)
		if len(load) > 0 {
			status = openTiles(tiles, shell, load)
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
				if g, err = newGfx(w, view, !measure); err != nil {
					return err
				}
				tiles.lostGPU()
				zonesShown = -1
			}

			for {
				ev, ok := shell.Viewport.Update(gtx)
				if !ok {
					break
				}
				if msg, used := zones.viewportEvent(tiles.world, &cam, ev, shell.Viewport.Size()); used {
					if msg != "" {
						status = msg
					}
				} else {
					fly.Handle(ev, &cam)
				}
				switch e := ev.(type) {
				case pointer.Event:
					if probe.handle(e) && tiles.world != nil {
						probe.click, probe.clickHit = pickAt(tiles.world, &cam, e.Position, shell.Viewport.Size())
						logClick(probe.click, probe.clickHit)
						if msg := zones.click(tiles.world, &cam, e.Position, shell.Viewport.Size(), probe.click, probe.clickHit); msg != "" {
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
			for _, req := range shell.Zones.Update(gtx) {
				if msg := zones.listRequest(req, tiles.world, &cam); msg != "" {
					status = msg
				}
			}
			if i, ok := shell.Problems.Clicked(gtx); ok {
				if msg := zones.goToProblem(i, tiles.world, &cam); msg != "" {
					status = msg
				}
			}
			if msg := shell.Props.Update(gtx, zones); msg != "" {
				status = msg
			}
			if msg, load := sess.update(gtx, w, shell, zones, tiles.openTiles(), tiles.opening()); msg != "" || len(load) > 0 {
				status = msg
				if len(load) > 0 {
					status = openTiles(tiles, shell, load)
				}
			}
			openTile := shell.OpenRequested(gtx)
			if m, ok := shell.Project.RecentMapClicked(gtx); ok {
				shell.Tile.SetText(m)
				openTile = true
			}
			if openTile && !tiles.opening() {
				t, err := scene.ParseTile(shell.Tile.Text())
				if err != nil {
					status = err.Error()
				} else {
					status = openTiles(tiles, shell, []scene.Tile{t})
				}
			}
			for _, r := range tiles.receive() {
				t := r.entry.tile
				if r.err != nil {
					status = fmt.Sprintf("%s: %v", t.Name(), r.err)
					log.Printf("cena: %s: %v", t.Name(), r.err)
					continue
				}
				logScene(t, r)
				if t != tiles.focus || tiles.framed {
					continue
				}
				// The opened tile arrived: frame it, as opening one map
				// always did.
				tiles.framed = true
				sess.mapOpened(tiles.root, []scene.Tile{t})
				shell.Project.RecentMaps = sess.cfg.RecentMaps
				cam = camera.ForBounds(renderBox(tiles.world, r.scene.Framing))
				if start != nil {
					start.apply(&cam, tiles.world)
				}
				log.Printf("cena: câmera %s", formatPose(&cam, tiles.world))
				status = t.Name()
				probe.click = scene.Hit{}
				probe.clickHit = false
			}

			moving := fly.Step(&cam, gtx.Now)
			if tiles.world != nil {
				tiles.follow(worldPosition(tiles.world, cam.Position))
			}
			shell.Status = probe.status(tiles.world, &cam, shell.Viewport.Size())
			shell.Zone.Info = zones.info()
			shell.Zones.Rows, shell.Zones.Selected = zones.rows(), zones.selectedZone()
			if rows, ok := zones.problemRows(); ok {
				shell.Problems.Rows = rows
			}
			if msg := zones.panel(gtx, &shell.Edit, tiles.world); msg != "" {
				status = msg
			}
			if zones.anchored && tiles.world != nil && probe.inside {
				zones.hoverAt(pickAt(tiles.world, &cam, probe.cursor, shell.Viewport.Size()))
			} else {
				zones.hoverAt(scene.Hit{}, false)
			}
			shell.Zone.Tools.Armed, shell.Zone.Tools.Active = zones.tool, zones.armed
			var renderer *render.Renderer
			if g != nil {
				renderer = g.renderer
			}
			shell.Loading, shell.Progress = tiles.progress(renderer)

			rect := shell.Layout(gtx, panelLines(g, status, tiles, &cam))
			if e.Size != size || rect != vpRect {
				log.Printf("frame: window %dx%d, viewport %v", e.Size.X, e.Size.Y, rect)
				size, vpRect = e.Size, rect
			}
			if g == nil {
				e.Frame(gtx.Ops)
				continue
			}
			uploading := tiles.sync(g.renderer, uploadBudget)
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
			if measure {
				frames.frame(gtx.Now, g.renderer.Stats(), tiles.shown())
			}
			if moving || uploading || measure {
				gtx.Execute(op.InvalidateCmd{})
			}
			e.Frame(gtx.Ops)
		}
	}
}

// renderBox converts a world box into the camera's rebased render space.
func renderBox(w *scene.World, b geom.Box) geom.Box {
	if b.Empty() {
		return b
	}
	return geom.Box{Min: scene.ToRender(b.Min.Sub(w.Origin)), Max: scene.ToRender(b.Max.Sub(w.Origin))}
}

// worldPosition is a rebased render-space point of w in world coordinates.
func worldPosition(w *scene.World, p geom.Vec3) geom.Vec3 {
	return scene.ToRender(p).Add(w.Origin)
}

// openTiles opens list in tiles, with the neighbours when the panel asks
// for them, from the client folder of the panel; it returns the status
// line.
func openTiles(tiles *tiles, shell *ui.Shell, list []scene.Tile) string {
	if err := tiles.open(strings.TrimSpace(shell.Client.Text()), list, shell.Neighbours.Value); err != nil {
		log.Printf("cena: %v", err)
		return err.Error()
	}
	if shell.Neighbours.Value {
		return "Carregando " + list[0].Name() + " e os vizinhos…"
	}
	return "Carregando " + strings.Join(tileNames(list), ", ") + "…"
}

// logScene logs what loading tile brought.
func logScene(tile scene.Tile, r tileResult) {
	s := r.scene
	log.Printf("cena: %s carregado em %v (preparo para a GPU %v): %s",
		tile.Name(), r.load.Round(time.Millisecond), r.prepare.Round(time.Millisecond), count(len(s.Batches), "batch", "batches"))
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
	if n, tris := bspSummary(s); n > 0 {
		log.Printf("cena: BSP: %s, %s", count(n, "superfície", "superfícies"), count(tris, "triângulo", "triângulos"))
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

// panelLines are the side panel's info lines: the status, then what the
// world's tiles hold, all tiles together.
func panelLines(g *gfx, status string, tiles *tiles, cam *camera.Camera) []string {
	var lines []string
	if status != "" {
		lines = append(lines, status)
	}
	if w := tiles.world; w != nil {
		scenes := w.Scenes()
		if names := tiles.shown(); len(names) > 0 {
			lines = append(lines, "Tiles: "+strings.Join(names, ", "))
		}
		terrain, bounds, terrains := 0, geom.EmptyBox(), 0
		for _, s := range scenes {
			for _, t := range s.Terrains {
				terrain += len(s.Batches[t.Batch].Indices) / 3
				bounds.Union(t.Bounds)
				terrains++
				if t.FallbackScale {
					lines = append(lines, t.Tile.Name()+": TerrainScale quebrado, posição por MapX/MapY")
				}
			}
		}
		if terrains > 0 {
			lines = append(lines,
				fmt.Sprintf("Terreno: %s", count(terrain, "triângulo", "triângulos")),
				fmt.Sprintf("x %.0f … %.0f", bounds.Min.X, bounds.Max.X),
				fmt.Sprintf("y %.0f … %.0f", bounds.Min.Y, bounds.Max.Y),
				fmt.Sprintf("z %.0f … %.0f", bounds.Min.Z, bounds.Max.Z),
			)
		} else if len(scenes) > 0 {
			lines = append(lines, "Nenhum tile aberto tem terreno")
		}
		if n, tris := bspSummary(scenes...); n > 0 {
			lines = append(lines, fmt.Sprintf("BSP: %s, %s", count(n, "superfície", "superfícies"), count(tris, "triângulo", "triângulos")))
		}
		if len(scenes) > 0 {
			lines = append(lines, meshSummary(scenes...))
		}
		for _, s := range scenes {
			lines = append(lines, s.Warnings...)
		}
		p := worldPosition(w, cam.Position)
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

// meshSummary is the static mesh actor and triangle counts of scenes.
func meshSummary(scenes ...*scene.Scene) string {
	actors, tris := 0, 0
	for _, s := range scenes {
		actors += len(s.Actors)
		for i := range s.Actors {
			tris += s.Actors[i].Triangles()
		}
	}
	return fmt.Sprintf("Static meshes: %s, %s",
		count(actors, "ator", "atores"), count(tris, "triângulo", "triângulos"))
}

// bspSummary is the BSP surface and triangle counts of scenes.
func bspSummary(scenes ...*scene.Scene) (surfaces, tris int) {
	for _, s := range scenes {
		surfaces += len(s.BSPSurfaces)
		for _, sf := range s.BSPSurfaces {
			tris += sf.Count / 3
		}
	}
	return surfaces, tris
}

// count inflects a noun to n ("1 triângulo", "2 triângulos").
func count(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}
