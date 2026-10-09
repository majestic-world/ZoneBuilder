// Package project reads and writes the Zone Builder project file (JSON,
// extension Ext) and the user config kept apart from it.
//
// A project holds the work of a session: the client folder, the XML output
// folder, the open map tiles and the zone Document, incomplete zones
// included. The user config (config.go) holds what the app pre-fills on
// start: the last client and output folders and the recent maps.
package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"zonebuilder/internal/zone"
)

// Ext is the project file extension.
const Ext = ".zbproj"

// Version is the project file format the app writes; Load refuses newer
// ones.
const Version = 1

// Project is one project file.
type Project struct {
	// Client is the client folder (the folder above Maps).
	Client string
	// Tiles are the open map tiles, by scene.Tile name ("22_22",
	// "22_22_Classic"), in opening order.
	Tiles []string
	// Document holds the zones.
	Document *zone.Document
}

// file is the JSON object a project file holds.
type file struct {
	Version int
	Project
}

// Save writes p to path. The file is replaced only once the new contents
// are fully written, so a failed save keeps the previous one.
func Save(path string, p Project) error {
	if p.Document == nil {
		p.Document = zone.NewDocument()
	}
	data, err := json.MarshalIndent(file{Version: Version, Project: p}, "", "\t")
	if err != nil {
		return fmt.Errorf("projeto: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("projeto: %w", err)
	}
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("projeto: %w", err)
	}
	return nil
}

// Load reads the project file at path.
func Load(path string) (Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Project{}, fmt.Errorf("projeto: %w", err)
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return Project{}, fmt.Errorf("projeto: %s: %w", filepath.Base(path), err)
	}
	switch {
	case f.Version == 0:
		return Project{}, fmt.Errorf("projeto: %s não é um projeto do Zone Builder", filepath.Base(path))
	case f.Version > Version:
		return Project{}, fmt.Errorf("projeto: %s usa o formato %d; esta versão do app lê até o %d", filepath.Base(path), f.Version, Version)
	}
	if f.Document == nil {
		f.Document = zone.NewDocument()
	}
	return f.Project, nil
}
