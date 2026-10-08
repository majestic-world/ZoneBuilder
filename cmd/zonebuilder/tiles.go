package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"time"

	"gioui.org/app"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/render"
	"zonebuilder/internal/scene"
)

// tileLoads is how many tiles load in the background at once.
const tileLoads = 3

// focusMargin is how far, in world units, the camera may stray past the
// focus tile's edge before the tile under it becomes the focus, so that
// flying back and forth over a border does not reload tiles each time.
const focusMargin = scene.TileSpan / 8

// tileState is where a tile is on its way to the screen.
type tileState uint8

const (
	// tileQueued waits for a free background load.
	tileQueued tileState = iota
	// tileLoading runs scene.Load and render.Prepare in the background.
	tileLoading
	// tileReady is prepared and waits for the renderer.
	tileReady
	// tileUploading is going to the GPU, a little per frame.
	tileUploading
	// tileShown is drawn and picked.
	tileShown
	// tileFailed did not load; it is not retried while it stays wanted.
	tileFailed
)

// tileEntry is one wanted tile.
type tileEntry struct {
	tile  scene.Tile
	state tileState
	scene *scene.Scene
	prep  *render.Prepared
}

// tileResult is the outcome of a background load (or, after the GPU was
// lost, of preparing an already loaded scene again).
type tileResult struct {
	entry *tileEntry
	scene *scene.Scene
	prep  *render.Prepared
	err   error
	// load and prepare are how long scene.Load and render.Prepare took;
	// load is 0 when only Prepare ran.
	load, prepare time.Duration
}

// tiles keeps the World's tiles: the tile the user opened (the focus) and,
// with neighbours, the tiles around it, up to 3×3. The focus follows the
// camera, so tiles load as it flies over them and the ones it left behind
// are dropped, from the GPU and from memory. Loading runs in the
// background; the GPU upload runs on the context thread, a little per
// frame (sync). Every method runs on the window's event loop.
type tiles struct {
	win  *app.Window
	root string
	// world is nil until the first open.
	world      *scene.World
	neighbours bool
	focus      scene.Tile
	// opened are the tiles of the last open, kept as they are without
	// neighbours.
	opened []scene.Tile
	// framed is set once the focus tile of the last open arrived and the
	// camera framed it; the focus follows the camera only after that.
	framed  bool
	entries map[scene.Tile]*tileEntry
	running int
	results chan tileResult
	// gone are scenes of dropped tiles the renderer may still hold.
	gone []*scene.Scene
	// loaded is set when a load ended since the memory was last returned.
	loaded bool
}

func newTiles(win *app.Window) *tiles {
	return &tiles{win: win, entries: map[scene.Tile]*tileEntry{}, results: make(chan tileResult, tileLoads)}
}

// open replaces the world with the given tiles of the client at root: the
// first one is the focus. With neighbours, the tiles around the focus come
// too and follow the camera; without, exactly the given tiles load. It
// fails, leaving the world as it was, when the client lacks the focus map.
func (ts *tiles) open(root string, list []scene.Tile, neighbours bool) error {
	if len(list) == 0 {
		return fmt.Errorf("nenhum tile para abrir")
	}
	focus := list[0]
	if !mapExists(root, focus) {
		return fmt.Errorf("o cliente não tem o mapa %s (Maps/%s.unr)", focus.Name(), focus.Name())
	}
	for _, e := range ts.entries {
		ts.drop(e, "outro mapa aberto")
	}
	x, y := focus.Origin()
	ts.root, ts.neighbours, ts.focus, ts.opened, ts.framed = root, neighbours, focus, list, false
	ts.world = scene.NewWorld(geom.Vec3{X: x + scene.TileSpan/2, Y: y + scene.TileSpan/2})
	log.Printf("cena: abrindo %s, vizinhos=%t, origem de rebase %v", focus.Name(), neighbours, ts.world.Origin)
	ts.want()
	return nil
}

// want brings the entries in line with the tiles wanted around the focus:
// new ones are queued, the ones no longer wanted dropped.
func (ts *tiles) want() {
	wanted := map[scene.Tile]bool{}
	if ts.neighbours {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if t, ok := resolveTile(ts.root, ts.focus.X+dx, ts.focus.Y+dy, ts.focus.Classic); ok {
					wanted[t] = true
				}
			}
		}
	} else {
		for _, t := range ts.opened {
			wanted[t] = true
		}
	}
	for t, e := range ts.entries {
		if !wanted[t] {
			ts.drop(e, "longe da câmera")
		}
	}
	for t := range wanted {
		if ts.entries[t] == nil {
			ts.entries[t] = &tileEntry{tile: t}
		}
	}
	ts.pump()
}

// drop forgets e's tile: out of the world now, off the GPU and out of
// memory on the next sync. A load still running for it is discarded when
// it ends.
func (ts *tiles) drop(e *tileEntry, why string) {
	delete(ts.entries, e.tile)
	if e.scene == nil {
		return
	}
	ts.world.Remove(e.scene)
	ts.gone = append(ts.gone, e.scene)
	log.Printf("cena: %s descarregado (%s)", e.tile.Name(), why)
}

// pump starts background loads, nearest to the focus first, while fewer
// than tileLoads run.
func (ts *tiles) pump() {
	for ts.running < tileLoads {
		var next *tileEntry
		for _, e := range ts.entries {
			if e.state == tileQueued && (next == nil || ts.nearer(e.tile, next.tile)) {
				next = e
			}
		}
		if next == nil {
			return
		}
		next.state = tileLoading
		ts.start(next, nil)
	}
}

// nearer orders tiles by distance to the focus, then by name.
func (ts *tiles) nearer(a, b scene.Tile) bool {
	da, db := ts.distance(a), ts.distance(b)
	if da != db {
		return da < db
	}
	return a.Name() < b.Name()
}

// distance is how many tiles t lies from the focus (Chebyshev).
func (ts *tiles) distance(t scene.Tile) int {
	return max(abs(t.X-ts.focus.X), abs(t.Y-ts.focus.Y))
}

// start loads e's tile in the background, or only prepares s again when
// it is already loaded.
func (ts *tiles) start(e *tileEntry, s *scene.Scene) {
	ts.running++
	root := ts.root
	go func() {
		r := tileResult{entry: e, scene: s}
		if r.scene == nil {
			began := time.Now()
			r.scene, r.err = scene.Load(root, []scene.Tile{e.tile})
			r.load = time.Since(began)
		}
		if r.err == nil {
			began := time.Now()
			r.prep = render.Prepare(r.scene)
			r.prepare = time.Since(began)
		}
		ts.results <- r
		ts.win.Invalidate()
	}()
}

// receive takes the finished background loads and returns those still
// wanted, successful or not; a successful one is prepared and waits for
// sync.
func (ts *tiles) receive() []tileResult {
	var got []tileResult
	for {
		select {
		case r := <-ts.results:
			ts.running--
			ts.loaded = true
			if ts.entries[r.entry.tile] != r.entry {
				log.Printf("cena: %s carregado, mas já não é preciso", r.entry.tile.Name())
				continue
			}
			if r.err != nil {
				r.entry.state = tileFailed
			} else {
				r.entry.scene, r.entry.prep, r.entry.state = r.scene, r.prep, tileReady
			}
			got = append(got, r)
		default:
			ts.pump()
			return got
		}
	}
}

// sync hands the GPU work to r: the dropped scenes leave it, the prepared
// ones join its upload queue, and about budget is spent freeing and
// uploading. A tile whose upload completes enters the world, where Pick
// sees it. Once tiles were dropped, or every wanted tile is shown after
// loads, the memory they leave behind (a load reads whole packages) goes
// back to the OS. It reports whether GPU work remains for later frames.
func (ts *tiles) sync(r *render.Renderer, budget time.Duration) bool {
	if ts.world != nil {
		r.SetOrigin(ts.world.Origin)
	}
	for _, s := range ts.gone {
		r.RemoveScene(s)
	}
	free := len(ts.gone) > 0
	ts.gone = nil
	for _, e := range ts.entries {
		if e.state == tileReady {
			r.AddScene(e.prep)
			e.prep, e.state = nil, tileUploading
		}
	}
	done, more := r.Upload(budget)
	for _, s := range done {
		for _, e := range ts.entries {
			if e.scene == s {
				e.state = tileShown
				ts.world.Add(s)
				log.Printf("cena: %s na GPU", e.tile.Name())
			}
		}
	}
	settled := true
	for _, e := range ts.entries {
		settled = settled && (e.state == tileShown || e.state == tileFailed)
	}
	if settled && ts.loaded {
		ts.loaded, free = false, true
	}
	if free {
		go debug.FreeOSMemory() // a forced collection: off this thread
	}
	return more
}

// lostGPU starts over the GPU side after the renderer was recreated: every
// tile that had reached it is prepared again and leaves the world until it
// is back on the GPU.
func (ts *tiles) lostGPU() {
	ts.gone = nil
	for _, e := range ts.entries {
		if e.state == tileUploading || e.state == tileShown {
			ts.world.Remove(e.scene)
			e.state = tileLoading
			ts.start(e, e.scene)
		}
	}
}

// follow moves the focus to the tile under the camera, at world position
// p, once the camera has gone focusMargin past the focus tile; only with
// neighbours and after the camera framed the opened tile.
func (ts *tiles) follow(p geom.Vec3) {
	if !ts.neighbours || !ts.framed {
		return
	}
	x0, y0 := ts.focus.Origin()
	if p.X >= x0-focusMargin && p.X < x0+scene.TileSpan+focusMargin &&
		p.Y >= y0-focusMargin && p.Y < y0+scene.TileSpan+focusMargin {
		return
	}
	t := scene.Tile{
		X:       int(math.Floor(float64(p.X/scene.TileSpan))) + 20,
		Y:       int(math.Floor(float64(p.Y/scene.TileSpan))) + 18,
		Classic: ts.focus.Classic,
	}
	log.Printf("cena: câmera sobre %d_%d: tiles em volta dele", t.X, t.Y)
	ts.focus = t
	ts.want()
}

// opening reports whether the focus tile of the last open is still on its
// way (not arrived, not failed).
func (ts *tiles) opening() bool {
	e := ts.entries[ts.focus]
	return !ts.framed && e != nil && e.state != tileFailed
}

// progress is the loading line and bar for the panel: "" once every
// wanted tile is shown (or failed). r may be nil (no GPU yet).
func (ts *tiles) progress(r *render.Renderer) (string, float32) {
	total, shown := 0, 0
	var done float32
	for _, e := range ts.entries {
		switch e.state {
		case tileFailed:
			continue
		case tileReady:
			done += 0.7
		case tileUploading:
			f := float32(0)
			if r != nil {
				f, _ = r.UploadProgress(e.scene)
			}
			done += 0.7 + 0.3*f
		case tileShown:
			done++
			shown++
		}
		total++
	}
	if shown == total {
		return "", 0
	}
	return fmt.Sprintf("Carregando tiles: %d de %d prontos", shown, total), done / float32(total)
}

// openTiles are the tiles a project records: the focus first, then the
// others loaded or loading, by name.
func (ts *tiles) openTiles() []scene.Tile {
	if ts.world == nil {
		return nil
	}
	var rest []scene.Tile
	for t, e := range ts.entries {
		if t != ts.focus && e.state != tileFailed {
			rest = append(rest, t)
		}
	}
	slices.SortFunc(rest, func(a, b scene.Tile) int {
		if a.Name() < b.Name() {
			return -1
		}
		if a.Name() > b.Name() {
			return 1
		}
		return 0
	})
	return append([]scene.Tile{ts.focus}, rest...)
}

// shown are the tiles in the world, by name.
func (ts *tiles) shown() []string {
	var names []string
	for t, e := range ts.entries {
		if e.state == tileShown {
			names = append(names, t.Name())
		}
	}
	slices.Sort(names)
	return names
}

// resolveTile is the map of tile (x, y) to open, the Classic variant first
// when classic: the other variant stands in when the client lacks it; false
// when it has neither.
func resolveTile(root string, x, y int, classic bool) (scene.Tile, bool) {
	for _, c := range []bool{classic, !classic} {
		if t := (scene.Tile{X: x, Y: y, Classic: c}); mapExists(root, t) {
			return t, true
		}
	}
	return scene.Tile{}, false
}

// mapExists reports whether the client at root has t's map.
func mapExists(root string, t scene.Tile) bool {
	fi, err := os.Stat(filepath.Join(root, "Maps", t.Name()+".unr"))
	return err == nil && fi.Mode().IsRegular()
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
