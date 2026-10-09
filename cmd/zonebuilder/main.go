// Command zonebuilder is the Zone Builder desktop app.
//
// Build with scripts/build.ps1, which also places ANGLE's libEGL.dll and
// libGLESv2.dll next to the executable; the app loads them at run time.
//
// The executable's icon (assets/icon/zonebuilder.ico, resource #1, which
// Gio puts on the window) is linked from rsrc_windows_amd64.syso; rebuild
// it with go generate after changing the icon.
package main

//go:generate go run github.com/tc-hib/go-winres@v0.3.3 make --arch amd64

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
	"gioui.org/gpu"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/unit"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/project"
	"zonebuilder/internal/render"
	"zonebuilder/internal/render/egl"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zonexml"
)

// version is the app version, set at build time from APP_VERSION in .env
// by scripts/build.ps1 (-ldflags "-X main.version=..."); a plain go build
// leaves it "dev".
var version = "dev"

// appTitle is the window title before a project is saved or opened.
func appTitle() string {
	return "Zone Builder v" + version + " - By Mk"
}

func main() {
	client := flag.String("client", "", "pasta do cliente (acima de Maps) que o campo traz preenchida; vazio usa a da configuração do usuário, depois ZB_CLIENT")
	tile := flag.String("tile", "", "tile que o campo traz preenchido; vazio usa o mapa mais recente, depois 22_22")
	proj := flag.String("project", "", "projeto ("+project.Ext+") aberto ao iniciar")
	pose := flag.String("camera", "", `pose da câmera ao abrir um tile, "x,y,z,yaw,pitch": posição de mundo (coordenadas do servidor) e ângulos em radianos, no formato que o log "cena: câmera" imprime; vazio enquadra o mapa`)
	fps := flag.Bool("fps", false, "mede a taxa de quadros: redesenha sem parar, sem vsync, e registra no log o tempo de quadro a cada 2 s")
	flag.Parse()
	sess := loadSession()
	fields := startFields{
		client: cmp.Or(*client, sess.cfg.Client, os.Getenv("ZB_CLIENT")),
		tile:   cmp.Or(*tile, firstOr(sess.cfg.RecentMaps, ""), "22_22"),
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
		w.Option(app.Title(appTitle()), app.Size(unit.Dp(1280), unit.Dp(800)), app.CustomRenderer(true))
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

// startFields are what the client and tile fields hold on start.
type startFields struct{ client, tile string }

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

	shell := ui.NewShell(ui.NewTheme(), fields.client, fields.tile)
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
		probe   cursorProbe
		zones   = newZoneEditor()
		cover   = newFloorCoverage(w)
		// zonesShown is the zones.version the renderer last got;
		// groundShown is what its ground marking was last built for;
		// groundMark was last built for groundBuilt.
		zonesShown  = -1
		groundShown = groundKey{version: -1}
		groundBuilt = groundKey{version: -1}
		groundMark  render.Ground
		// pins are the current shape's worst points (spec D4e), as last
		// laid out; pinsShown are the ones the renderer's overlay has.
		pins, pinsShown []worstPin
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
				zonesShown, groundShown = -1, groundKey{version: -1}
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
			if shell.Zone.CreateRequested(gtx) {
				status = zones.create(shell.Zone.Name.Text(), shell.Zone.Type(), shell.Zone.Tools.Shape)
			}
			if t, ok := shell.Zone.Tools.Requested(gtx); ok {
				status = zones.arm(t, shell.Zone.Tools.Banned.Value)
			}
			if shell.Zone.Tools.WholeTile.Clicked(gtx) {
				status = wholeTile(zones, tiles, &cam, shell.Viewport.Size(), shell.Zone.Tools.Banned.Value)
			}
			if shell.Zone.Compile.Clicked(gtx) {
				var files []zonexml.File
				status, files = zones.compile()
				if len(files) > 0 {
					shell.XML.Open(files)
				}
			}
			if name, ok := shell.XML.Copied(gtx); ok {
				status = name + " copiado para a área de transferência"
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
			if i, ok := shell.PinClicked(gtx); ok && i < len(pins) {
				status = goToPin(pins[i], tiles.world, &cam)
			}
			sel, selOK := zones.selectedZone()
			for _, req := range shell.Props.Update(gtx, sel, selOK) {
				if msg := shell.Props.Applied(req, zones.apply(req.Command)); msg != "" {
					status = msg
				}
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
				status = "" // the Tiles line names it
				probe.click = scene.Hit{}
				probe.clickHit = false
			}

			if shell.Meshes.Toggled(gtx) {
				status = "Static meshes visíveis"
				if shell.Meshes.On {
					status = "Static meshes ocultos: só terreno e BSP"
				}
			}
			if shell.Ground.Toggled(gtx) {
				status = "Grade do chão oculta"
				if shell.Ground.On {
					status = fmt.Sprintf("Chão: grade das células do terreno, linha forte a cada %d", render.GridMajor)
				}
			}
			moving := fly.Step(&cam, gtx.Now)
			if tiles.world != nil {
				// Hidden meshes are not picked either: a vertex never lands
				// on geometry the user cannot see.
				tiles.world.HideMeshes = shell.Meshes.On
				tiles.follow(worldPosition(tiles.world, cam.Position))
			}
			shell.Cursor, shell.Click = probe.status(tiles.world, &cam, shell.Viewport.Size())
			shell.Tiles, shell.Warnings = loadedTiles(tiles)
			shell.Zone.Info = zones.info()
			sel, _ = zones.selectedZone()
			shell.Zones.Rows, shell.Zones.Selected = zones.rows(), sel.ID
			if k := (groundKey{version: zones.version, zone: zones.zone, on: shell.Ground.On}); k != groundBuilt {
				groundMark = zones.ground(shell.Ground.On)
				shell.Zones.LeftOut = render.GroundLeftOut(groundMark)
				groundBuilt = k
			}
			if rows, ok := zones.problemRows(); ok {
				shell.Problems.Rows = rows
			}
			if msg := zones.panel(gtx, &shell.Edit, tiles.world); msg != "" {
				status = msg
			}
			shell.Edit.Coverage = cover.inspector(zones, tiles.world)
			if msg := zones.heightPanel(gtx, &shell.Height); msg != "" {
				status = msg
			}
			shell.Arrow = zones.layoutArrow(tiles.world, &cam, shell.Viewport.Size(), gtx.Dp(90))
			shell.EdgeLabels = nil
			if shell.Ground.On && tiles.world != nil {
				shell.EdgeLabels = zones.edgeLabels(tiles.world, &cam, shell.Viewport.Size())
			}
			pins = cover.pins(zones, tiles.world)
			shell.Pins = nil
			if tiles.world != nil {
				shell.Pins = pinLabels(pins, tiles.world, &cam, shell.Viewport.Size())
			}
			if zones.anchored && tiles.world != nil && probe.inside {
				h, ok := pickAt(tiles.world, &cam, probe.cursor, shell.Viewport.Size())
				zones.hoverAt(tiles.world, h, ok)
			} else {
				zones.hoverAt(nil, scene.Hit{}, false)
			}
			shell.Zone.Tools.Armed, shell.Zone.Tools.Active = zones.tool, zones.armed
			var renderer *render.Renderer
			if g != nil {
				renderer = g.renderer
			}
			shell.Loading, shell.Progress = tiles.progress(renderer)

			shell.Message = status
			rect := shell.Layout(gtx)
			if e.Size != size || rect != vpRect {
				log.Printf("frame: window %dx%d, viewport %v", e.Size.X, e.Size.Y, rect)
				size, vpRect = e.Size, rect
			}
			if g == nil {
				e.Frame(gtx.Ops)
				continue
			}
			g.renderer.SetMeshesHidden(shell.Meshes.On)
			uploading := tiles.sync(g.renderer, uploadBudget)
			if zonesShown != zones.version || !samePinShapes(pins, pinsShown) {
				g.renderer.SetZones(append(zones.overlay(), pinShapes(pins)...))
				zonesShown, pinsShown = zones.version, pins
			}
			if groundShown != groundBuilt {
				g.renderer.SetGround(groundMark)
				groundShown = groundBuilt
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
		tile.Name(), r.load.Round(time.Millisecond), r.prepare.Round(time.Millisecond), inflect.Count(len(s.Batches), "batch", "batches"))
	for _, t := range s.Terrains {
		ox, oy := t.Tile.Origin()
		textured := 0
		for _, b := range t.Layers {
			if s.Batches[b].Texture != nil {
				textured++
			}
		}
		log.Printf("cena: terreno %s: %d×%d amostras, %s, %s, faixa x [%.1f, %.1f] y [%.1f, %.1f] z [%.1f, %.1f]; início do tile (%.0f, %.0f); fallback MapX/MapY=%t",
			t.Tile.Name(), t.Width, t.Height, inflect.Count(len(s.Batches[t.Batch].Indices)/3, "triângulo", "triângulos"),
			inflect.Count(textured, "camada texturizada", "camadas texturizadas"),
			t.Bounds.Min.X, t.Bounds.Max.X, t.Bounds.Min.Y, t.Bounds.Max.Y, t.Bounds.Min.Z, t.Bounds.Max.Z,
			ox, oy, t.FallbackScale)
	}
	if n, tris := bspSummary(s); n > 0 {
		log.Printf("cena: BSP: %s, %s", inflect.Count(n, "superfície", "superfícies"), inflect.Count(tris, "triângulo", "triângulos"))
	}
	log.Printf("cena: %s", meshSummary(s))
	untextured := 0
	for _, b := range s.Batches {
		if b.Texture == nil {
			untextured += len(b.Indices) / 3
		}
	}
	log.Printf("cena: %s sem textura", inflect.Count(untextured, "triângulo", "triângulos"))
	for _, w := range s.Warnings {
		log.Printf("cena: aviso: %s", w)
	}
}

// loadedTiles names the open tiles and lists what failed to load in them.
// The counts and bounds of what loaded go to the log (logScene).
func loadedTiles(tiles *tiles) (names string, warnings []string) {
	w := tiles.world
	if w == nil {
		return "", nil
	}
	for _, s := range w.Scenes() {
		warnings = append(warnings, s.Warnings...)
	}
	return strings.Join(tiles.shown(), ", "), warnings
}

// meshSummary is the static mesh actor and triangle counts of s.
func meshSummary(s *scene.Scene) string {
	tris := 0
	for i := range s.Actors {
		tris += s.Actors[i].Triangles()
	}
	return fmt.Sprintf("Static meshes: %s, %s",
		inflect.Count(len(s.Actors), "ator", "atores"), inflect.Count(tris, "triângulo", "triângulos"))
}

// bspSummary is the BSP surface and triangle counts of s.
func bspSummary(s *scene.Scene) (surfaces, tris int) {
	for _, sf := range s.BSPSurfaces {
		tris += sf.Count / 3
	}
	return len(s.BSPSurfaces), tris
}
