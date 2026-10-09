package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"zonebuilder/internal/locale"
)

// MaxRecentMaps is how many recent maps the config keeps.
const MaxRecentMaps = 8

// Config is the user's preferences, kept outside any project: what the app
// pre-fills on start.
type Config struct {
	// Client is the last client folder a map was opened from.
	Client string
	// RecentMaps are the tiles opened last, by scene.Tile name, most
	// recent first.
	RecentMaps []string
	// Project is the last project file saved or opened; the project file
	// dialogs start in its folder.
	Project string
	// Language is the interface language; it never belongs to a project.
	Language locale.Language
}

// ConfigPath is the config file: ZoneBuilder\config.json under the user's
// config folder (%AppData% on Windows).
func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("configuração: %w", err)
	}
	return filepath.Join(dir, "ZoneBuilder", "config.json"), nil
}

// LoadConfig reads the config at path; a missing file is an empty config.
func LoadConfig(path string) (Config, error) {
	c := Config{Language: locale.PtBR}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, fmt.Errorf("configuração: %w", err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("configuração: %s: %w", path, err)
	}
	c.Language = locale.Normalize(string(c.Language))
	return c, nil
}

// Save writes c to path, creating its folder.
func (c Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		return fmt.Errorf("configuração: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("configuração: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("configuração: %w", err)
	}
	return nil
}

// AddRecentMap puts tile first in RecentMaps (once, ignoring case) and
// drops the oldest beyond MaxRecentMaps.
func (c *Config) AddRecentMap(tile string) {
	c.RecentMaps = slices.DeleteFunc(c.RecentMaps, func(m string) bool { return strings.EqualFold(m, tile) })
	c.RecentMaps = slices.Insert(c.RecentMaps, 0, tile)
	if len(c.RecentMaps) > MaxRecentMaps {
		c.RecentMaps = c.RecentMaps[:MaxRecentMaps]
	}
}
