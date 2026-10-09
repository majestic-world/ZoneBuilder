package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zonebuilder/internal/coverage"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/project"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

func TestNativeDialogPresentationAtOpeningLanguage(t *testing.T) {
	for _, tc := range []struct {
		language locale.Language
		open, save, folder, description string
	}{
		{locale.PtBR, "Abrir projeto", "Salvar projeto", "Pasta do cliente Lineage II (a que contém Maps)", "Projeto do Zone Builder"},
		{locale.En, "Open project", "Save project", "Lineage II client folder (containing Maps)", "Zone Builder project"},
	} {
		if got := locale.Text(tc.language, "actions.project.open_dialog"); got != tc.open {
			t.Errorf("%s open title = %q", tc.language, got)
		}
		if got := locale.Text(tc.language, "actions.project.save_dialog"); got != tc.save {
			t.Errorf("%s save title = %q", tc.language, got)
		}
		if got := locale.Text(tc.language, "actions.map.folder_dialog"); got != tc.folder {
			t.Errorf("%s folder description = %q", tc.language, got)
		}
		if got := ui.ProjectFileFor(tc.language); got.Name != tc.description || got.Ext != project.Ext {
			t.Errorf("%s project file filter = %+v", tc.language, got)
		}
	}
}

func TestLanguageChoiceWhileEditingWithProblemsAndWarning(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config.json")
	sess := &session{cfgPath: config, cfg: project.Config{Language: locale.PtBR}}
	shell := ui.NewShell(ui.NewTheme(), "", "22_22")
	shell.Zone.Name.SetText("unfinished")
	e := newZoneEditor()
	id := e.doc.NewZoneID()
	if err := e.apply(zone.CreateZone{ID: id, Name: "[bad]", Type: zone.PeaceZone}); err != nil {
		t.Fatal(err)
	}
	if err := e.apply(zone.AddShape{Zone: id, Kind: zone.Rectangle, ZMin: 1200, ZMax: 1000,
		Points: []zone.Point{{X: 83000, Y: 147000}}}); err != nil {
		t.Fatal(err)
	}
	warnings := []floorWarning{{zone: id, shape: 0, Warning: coverage.Warning{Kind: coverage.AboveTop, Clearance: -1200, Share: .125}}}
	document := zonesJSON(t, e.doc)
	var first []ui.ProblemRow
	for _, language := range []locale.Language{locale.PtBR, locale.En, locale.PtBR} {
		if warning := sess.chooseLanguage(shell, language); warning != "" {
			t.Fatal(warning)
		}
		e.Language = shell.Language
		rows, changed := e.problemRows(warnings, false, shell.Language)
		if !changed || len(rows) < 2 || !rows[len(rows)-1].Warning {
			t.Fatalf("%s problem and warning rows: %+v (changed %v)", language, rows, changed)
		}
		if first == nil {
			first = rows
		} else {
			for i, row := range rows {
				if row.Zone != first[i].Zone || row.Warning != first[i].Warning {
					t.Fatalf("%s row %d changed target or order: %+v", language, i, row)
				}
				if language == locale.En && row.Message == first[i].Message {
					t.Errorf("%s row %d remained untranslated: %q", language, i, row.Message)
				}
				if language == locale.PtBR && row.Message != first[i].Message {
					t.Errorf("%s row %d did not return to Portuguese: %q", language, i, row.Message)
				}
			}
		}
		if zonesJSON(t, e.doc) != document || shell.Zone.Name.Text() != "unfinished" {
			t.Fatal("changing presentation altered document or unfinished edit")
		}
	}
	got, err := project.LoadConfig(config)
	if err != nil || got.Language != locale.PtBR {
		t.Fatalf("language preference after return = %+v, %v", got, err)
	}
}

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
	if !strings.Contains(warning, "Could not save the language preference; your choice applies only to this session.") || !strings.Contains(warning, "occupied") {
		t.Fatalf("warning does not include localized message and original error: %q", warning)
	}
}
