package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"gioui.org/app"
	"gioui.org/layout"

	"zonebuilder/internal/inflect"
	"zonebuilder/internal/project"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// session is the window's project file and the user config: which file
// Save writes, whether the zones changed since, and the folders and maps
// the app pre-fills on its next start.
type session struct {
	cfgPath string
	cfg     project.Config
	// path is the project file; "" until the first save or open.
	path string
	// saved is the zoneEditor.version at the last save or open.
	saved int
	// picks delivers the file chosen in a project dialog.
	picks chan projectPick
}

// projectPick is a file chosen in the save (save set) or open dialog.
type projectPick struct {
	path string
	save bool
}

// loadSession reads the user config; without one the app starts empty.
func loadSession() *session {
	s := &session{picks: make(chan projectPick, 1)}
	var err error
	if s.cfgPath, err = project.ConfigPath(); err == nil {
		s.cfg, err = project.LoadConfig(s.cfgPath)
	}
	if err != nil {
		log.Printf("configuração: %v", err)
	}
	return s
}

func (s *session) saveConfig() {
	if s.cfgPath == "" {
		return
	}
	if err := s.cfg.Save(s.cfgPath); err != nil {
		log.Printf("configuração: %v", err)
	}
}

// mapOpened records that tiles opened from client.
func (s *session) mapOpened(client string, tiles []scene.Tile) {
	s.cfg.Client = client
	for i := len(tiles) - 1; i >= 0; i-- {
		s.cfg.AddRecentMap(tiles[i].Name())
	}
	s.saveConfig()
}

// update handles the project buttons: the dialogs run off the event loop
// and the file chosen is saved or opened on a later frame. Opening is
// refused while busy (a map is loading). It returns the status line ("" for
// no news) and, after opening a project, its tiles to load.
func (s *session) update(gtx layout.Context, w *app.Window, shell *ui.Shell, zones *zoneEditor, tiles []scene.Tile, busy bool) (status string, load []scene.Tile) {
	p := &shell.Project
	open, save, saveAs := p.Requests(gtx)
	if save {
		if s.path != "" {
			status = s.save(w, shell, zones, tiles, s.path)
		} else {
			s.pick(w, true)
		}
	}
	if saveAs {
		s.pick(w, true)
	}
	if open {
		s.pick(w, false)
	}
	select {
	case pk := <-s.picks:
		switch {
		case pk.save:
			status = s.save(w, shell, zones, tiles, pk.path)
		case busy:
			status = "Espere o mapa terminar de carregar para abrir um projeto"
		default:
			status, load = s.open(w, shell, zones, pk.path)
		}
	default:
	}
	p.Name, p.Unsaved = s.name(), zones.version != s.saved
	return status, load
}

// pick shows the save or open dialog, starting at the current project, else
// the last one.
func (s *session) pick(w *app.Window, save bool) {
	start := s.path
	if start == "" {
		start = s.cfg.Project
	}
	go func() {
		var path string
		var ok bool
		if save {
			path, ok = ui.PickSaveFile("Salvar projeto", start, ui.ProjectFile)
		} else {
			path, ok = ui.PickOpenFile("Abrir projeto", start, ui.ProjectFile)
		}
		if ok {
			s.picks <- projectPick{path: path, save: save}
			w.Invalidate()
		}
	}()
}

// save writes the window's project to path, which becomes the project file.
// It returns the status line.
func (s *session) save(w *app.Window, shell *ui.Shell, zones *zoneEditor, tiles []scene.Tile, path string) string {
	p := project.Project{
		Client:   strings.TrimSpace(shell.Client.Text()),
		Tiles:    tileNames(tiles),
		Document: zones.doc,
	}
	if err := project.Save(path, p); err != nil {
		log.Print(err)
		return err.Error()
	}
	s.path, s.saved = path, zones.version
	s.cfg.Project = path
	s.saveConfig()
	w.Option(app.Title(windowTitle(path)))
	log.Printf("projeto: salvo em %s: %s, tiles %q", path, inflect.Count(len(p.Document.Zones()), "zona", "zonas"), p.Tiles)
	return "Projeto salvo em " + path
}

// open reads the project file at path into the window, which then edits
// its zones; path becomes the project file. It returns the status line and
// the project's tiles to load.
func (s *session) open(w *app.Window, shell *ui.Shell, zones *zoneEditor, path string) (string, []scene.Tile) {
	p, err := project.Load(path)
	var tiles []scene.Tile
	if err == nil {
		tiles, err = parseTiles(p.Tiles)
	}
	if err != nil {
		log.Print(err)
		return err.Error(), nil
	}
	shell.Client.SetText(p.Client)
	if len(tiles) > 0 {
		shell.Tile.SetText(tiles[0].Name())
	}
	zones.replace(p.Document)
	shell.Zones.Reset()
	s.path, s.saved = path, zones.version
	s.cfg.Project = path
	s.saveConfig()
	w.Option(app.Title(windowTitle(path)))
	log.Printf("projeto: aberto %s: %s, tiles %q", path, inflect.Count(len(p.Document.Zones()), "zona", "zonas"), p.Tiles)
	return "Projeto aberto: " + path, tiles
}

// name is the project file's name, "Projeto sem nome" before it has one.
func (s *session) name() string {
	if s.path == "" {
		return "Projeto sem nome"
	}
	return filepath.Base(s.path)
}

// windowTitle is the app title followed by the project file at path.
func windowTitle(path string) string {
	return appTitle() + " - " + filepath.Base(path)
}

// tileNames are the names of tiles, for the project file.
func tileNames(tiles []scene.Tile) []string {
	names := make([]string, len(tiles))
	for i, t := range tiles {
		names[i] = t.Name()
	}
	return names
}

// parseTiles reads a project's tile names.
func parseTiles(names []string) ([]scene.Tile, error) {
	tiles := make([]scene.Tile, len(names))
	for i, n := range names {
		t, err := scene.ParseTile(n)
		if err != nil {
			return nil, fmt.Errorf("projeto: tile %q: %w", n, err)
		}
		tiles[i] = t
	}
	return tiles, nil
}

// replace makes doc the edited document: the zones of an opened project.
// Tool, selection and drag start over; the Z margin setting stays. The last
// zone is selected, unless a polygon was left with fewer than 3 vertices:
// then its zone is, with the polygon tool armed on it, so the user carries
// on drawing it.
func (e *zoneEditor) replace(doc *zone.Document) {
	margin := e.margin
	*e = zoneEditor{doc: doc, version: e.version + 1, editState: newEditState()}
	e.margin = margin
	for _, z := range doc.Zones() {
		e.zone = z.ID
		for i, s := range z.Shapes {
			if s.Kind == zone.Polygon && len(s.Points) < 3 {
				e.zone, e.shape, e.banned = z.ID, i, s.Banned
				e.tool, e.armed, e.drawing = ui.ToolPolygon, true, true
				return
			}
		}
	}
}
