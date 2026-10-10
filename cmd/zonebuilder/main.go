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
	"zonebuilder/internal/locale"
	"zonebuilder/internal/project"
	"zonebuilder/internal/render"
	"zonebuilder/internal/render/egl"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
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
	modeArg := flag.String("mode", "", "abre direto num modo, sem passar pela tela inicial: "+modeFlagZones+" (Construir zonas) ou "+modeFlagPopulate+" (Popular zona); vazio abre a tela inicial, mesmo com -project")
	flag.Parse()
	startMode, err := parseMode(*modeArg)
	if err != nil {
		log.Fatalf("-mode: %v", err)
	}
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
		if err := run(w, sess, fields, *proj, start, startMode, *fps); err != nil {
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

// run is the window's event loop, opening on startMode (ui.ModeHome: the
// home screen). measure redraws without pause or vsync and logs the frame
// times (the -fps flag).
func run(w *app.Window, sess *session, fields startFields, proj string, start *cameraPose, startMode ui.Mode, measure bool) error {
	// EGL binds the context to an OS thread: keep this goroutine on one.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	shell := ui.NewShell(ui.NewTheme(), fields.client, fields.tile)
	shell.Language = sess.cfg.Language
	shell.Project.RecentMaps = sess.cfg.RecentMaps

	var (
		ops     op.Ops
		g       *gfx
		view    app.Win32ViewEvent
		size    image.Point
		vpRect  image.Rectangle
		fly     ui.FlyControls
		frames  frameLog
		folders = make(chan string, 1)
		ws      = &workspace{shell: shell, tiles: newTiles(w), cam: camera.ForBounds(geom.EmptyBox())}
		modes   = newModes(w, startMode)
		// zones and spawns are the zone and spawn area editors, which the
		// project file saves and opens whatever the active mode.
		zones  = modes.zones.zones
		spawns = modes.populate.spawns
	)
	zones.Language = shell.Language
	log.Printf("modo: %s", modeName(modes.active))
	defer func() { g.release() }()
	if proj != "" {
		var load []scene.Tile
		ws.status, load = sess.open(w, shell, zones, spawns, proj)
		if len(load) > 0 {
			ws.status = openTiles(ws.tiles, shell, load)
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
				ws.tiles.lostGPU()
				modes.forget()
			}

			// Viewport events and keys go to the active mode only; what
			// it leaves moves the camera.
			m := modes.current()
			for {
				ev, ok := shell.Viewport.Update(gtx)
				if !ok {
					break
				}
				if !m.viewportEvent(ws, ev) {
					fly.Handle(ev, &ws.cam)
				}
				switch e := ev.(type) {
				case pointer.Event:
					button := ws.probe.handle(e)
					if button == 0 || ws.world() == nil {
						break
					}
					if button == pointer.ButtonPrimary {
						ws.probe.click, ws.probe.clickHit = pickAt(ws.world(), &ws.cam, e.Position, ws.viewport())
						logClick(ws.probe.click, ws.probe.clickHit)
					}
					m.click(ws, e, button)
				case key.Event:
					if e.Name == key.NameEscape && e.State == key.Press && shell.CloseMenus() {
						break
					}
					m.key(ws, e)
				}
			}
			if next, ok := shell.ModeRequested(gtx); ok && next != modes.active {
				modes.set(next)
				m = modes.current()
				// Keys held when the mode changed send no release to it.
				fly = ui.FlyControls{}
				if world := ws.world(); world != nil {
					log.Printf("modo: câmera %s", formatPose(&ws.cam, world))
				}
			}
			if shell.Browse.Clicked(gtx) {
				lang := shell.Language
				start := shell.Client.Text()
				go func() {
					if p, ok := ui.PickFolder(locale.Text(lang, "actions.map.folder_dialog"), start); ok {
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
			if shell.Undo.Clicked(gtx) {
				m.undo(ws)
			}
			if shell.Redo.Clicked(gtx) {
				m.redo(ws)
			}
			if shell.Compile.Clicked(gtx) {
				m.compile(ws)
			}
			if text, ok := shell.GoToRequested(gtx); ok {
				ws.status = action(goTo(text, ws.world(), &ws.cam))
			}
			m.update(gtx, ws)
			if name, ok := shell.XML.Copied(gtx); ok {
				ws.status = actionArgs(locale.Message{Key: "actions.xml.copied"}, map[string]string{"name": name})
			}
			if msg, load := sess.update(gtx, w, shell, zones, spawns, ws.tiles.openTiles(), ws.tiles.opening()); msg.render(shell.Language) != "" || len(load) > 0 {
				ws.status = msg
				if len(load) > 0 {
					ws.status = openTiles(ws.tiles, shell, load)
				}
			}
			openTile := shell.OpenRequested(gtx)
			if recent, ok := shell.Project.RecentMapClicked(gtx); ok {
				shell.Tile.SetText(recent)
				openTile = true
			}
			if openTile && !ws.tiles.opening() {
				t, err := scene.ParseTile(shell.Tile.Text())
				if err != nil {
					ws.status = actionError(locale.Message{Key: "actions.error.invalid_tile"}, err, nil)
				} else {
					ws.status = openTiles(ws.tiles, shell, []scene.Tile{t})
				}
			}
			for _, r := range ws.tiles.receive() {
				t := r.entry.tile
				if r.err != nil {
					ws.status = actionError(locale.Message{Key: "actions.error.load_tile"}, r.err, map[string]string{"tile": t.Name()})
					log.Printf("cena: %s: %v", t.Name(), r.err)
					continue
				}
				logScene(t, r)
				if t != ws.tiles.focus || ws.tiles.framed {
					continue
				}
				// The opened tile arrived: frame it, as opening one map
				// always did.
				ws.tiles.framed = true
				sess.mapOpened(ws.tiles.root, []scene.Tile{t})
				shell.Project.RecentMaps = sess.cfg.RecentMaps
				ws.cam = camera.ForBounds(renderBox(ws.tiles.world, r.scene.Framing))
				if start != nil {
					start.apply(&ws.cam, ws.tiles.world)
				}
				log.Printf("cena: câmera %s", formatPose(&ws.cam, ws.tiles.world))
				ws.status = actionStatus{} // the Tiles line names it
				ws.probe.click = scene.Hit{}
				ws.probe.clickHit = false
			}

			if shell.Meshes.Toggled(gtx) {
				ws.status = action(locale.Message{Key: "actions.map.meshes_visible"})
				if shell.Meshes.On {
					ws.status = action(locale.Message{Key: "actions.map.meshes_hidden"})
				}
			}
			if shell.Ground.Toggled(gtx) {
				ws.status = action(locale.Message{Key: "actions.map.grid_hidden"})
				if shell.Ground.On {
					ws.status = actionArgs(locale.Message{Key: "actions.map.grid_visible"}, map[string]string{"step": fmt.Sprint(render.GridMajor)})
				}
			}
			moving := fly.Step(&ws.cam, gtx.Now)
			if world := ws.world(); world != nil {
				// Hidden meshes are not picked either: a vertex never lands
				// on geometry the user cannot see.
				world.HideMeshes = shell.Meshes.On
				ws.tiles.follow(worldPosition(world, ws.cam.Position))
			}
			if lang, ok := shell.LanguageRequested(gtx); ok {
				warning := sess.chooseLanguage(shell, lang)
				zones.Language = shell.Language
				if warning != "" {
					ws.status = sess.languageWarning
				} else if ws.status.problemClick {
					ws.status.raw = zones.reformatProblemClick(shell.Language)
				}
				// Render the same state in the selected language next frame.
				shell.Project.Name = sess.name(shell.Language)
				gtx.Execute(op.InvalidateCmd{})
			}
			m.present(gtx, ws)
			shell.Cursor, shell.Click = ws.probe.status(ws.world(), &ws.cam, ws.viewport(), shell.Language)
			shell.Tiles, shell.Warnings = loadedTiles(ws.tiles, shell.Language)
			var renderer *render.Renderer
			if g != nil {
				renderer = g.renderer
			}
			shell.Loading, shell.Progress = ws.tiles.progress(renderer, shell.Language)

			shell.Mode = modes.active
			shell.Message = ws.status.render(shell.Language)
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
			uploading := ws.tiles.sync(g.renderer, uploadBudget)
			m.sync(g.renderer, ws)

			g.ctx.WaitClient() // lets ANGLE pick up a window resize
			if err := g.renderer.DrawViewport(rect, e.Size, &ws.cam); err != nil {
				return err
			}
			if err := g.gio.Frame(gtx.Ops, gpu.OpenGLRenderTarget{}, e.Size); err != nil {
				return fmt.Errorf("gio frame: %w", err)
			}
			if err := g.ctx.SwapBuffers(); err != nil {
				return err
			}
			if measure {
				frames.frame(gtx.Now, g.renderer.Stats(), ws.tiles.shown())
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

// openTiles starts loading the selected tiles and retains the status identity.
func openTiles(tiles *tiles, shell *ui.Shell, list []scene.Tile) actionStatus {
	if err := tiles.open(strings.TrimSpace(shell.Client.Text()), list, shell.Neighbours.Value); err != nil {
		log.Printf("cena: %v", err)
		return actionError(locale.Message{Key: "actions.error.open_map"}, err, nil)
	}
	if shell.Neighbours.Value {
		return actionArgs(locale.Message{Key: "actions.map.loading_neighbours"}, map[string]string{"tile": list[0].Name()})
	}
	return actionArgs(locale.Message{Key: "actions.map.loading_tiles"}, map[string]string{"tiles": strings.Join(tileNames(list), ", ")})
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
	for _, v := range s.WaterVolumes {
		if v.Unsupported != "" {
			log.Printf("cena: água: %s %s não suportado: %s", v.Tile.Name(), v.Name, v.Unsupported)
		}
	}
}

// loadedTiles presents names and raw scene warnings with localized context.
func loadedTiles(tiles *tiles, lang locale.Language) (names string, warnings []string) {
	w := tiles.world
	if w == nil {
		return "", nil
	}
	for _, s := range w.Scenes() {
		for _, warning := range s.Warnings {
			warnings = append(warnings, locale.Format(lang, "actions.map.warning", map[string]string{"detail": warning}))
		}
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
