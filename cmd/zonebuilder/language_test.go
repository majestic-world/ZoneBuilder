package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/project"
	"zonebuilder/internal/ui"
)

func TestSessionLanguageChoicePersistsWithoutTouchingEditingState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	sess := &session{cfgPath: path, cfg: project.Config{Language: locale.PtBR, Project: "existing.zbproj"}}
	shell := ui.NewShell(ui.NewTheme(), "client", "22_22")
	shell.Project.Unsaved = false
	shell.Tile.SetText("in progress")
	for _, language := range []locale.Language{locale.En, locale.PtBR} {
		if warning := sess.chooseLanguage(shell, language); warning != "" { t.Fatal(warning) }
		got, err := project.LoadConfig(path)
		if err != nil { t.Fatal(err) }
		if got.Language != language || shell.Language != language || sess.cfg.Language != language {
			t.Fatalf("language not applied and saved: config %q, shell %q, session %q", got.Language, shell.Language, sess.cfg.Language)
		}
		if shell.Tile.Text() != "in progress" || shell.Project.Unsaved { t.Fatal("language choice altered editing state") }
		if got.Project != "existing.zbproj" { t.Fatalf("language choice altered project path: %q", got.Project) }
	}
}

func TestSessionKeepsChoiceAndDisplaysLocalizedFailure(t *testing.T) {
	occupied := filepath.Join(t.TempDir(), "occupied")
	if err := os.WriteFile(occupied, []byte("keep"), 0o600); err != nil { t.Fatal(err) }
	sess := &session{cfgPath: filepath.Join(occupied, "config.json"), cfg: project.Config{Language: locale.PtBR}}
	shell := ui.NewShell(ui.NewTheme(), "", "")
	warning := sess.chooseLanguage(shell, locale.En)
	if sess.cfg.Language != locale.En || shell.Language != locale.En { t.Fatal("failed save reverted session language") }
	if !strings.Contains(warning, locale.Text(locale.En, "app.preference.unsaved")) || !strings.Contains(warning, "occupied") {
		t.Fatalf("warning does not include localized message and original error: %q", warning)
	}
}
