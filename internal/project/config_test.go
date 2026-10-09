package project_test

import (
	"os"
	"path/filepath"
	"testing"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/project"
)

func TestLanguagePreferenceDefaultsAndRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	for _, tc := range []struct {
		name, data string
		want locale.Language
	}{
		{"missing", "", locale.PtBR},
		{"old config", `{"Client":"existing"}`, locale.PtBR},
		{"invalid language", `{"Language":"fr"}`, locale.PtBR},
		{"saved language", `{"Language":"en"}`, locale.En},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.data != "" {
				if err := os.WriteFile(path, []byte(tc.data), 0o600); err != nil { t.Fatal(err) }
			} else {
				_ = os.Remove(path)
			}
			cfg, err := project.LoadConfig(path)
			if err != nil { t.Fatal(err) }
			if cfg.Language != tc.want { t.Fatalf("language %q, want %q", cfg.Language, tc.want) }
			cfg.Language = locale.En
			if err := cfg.Save(path); err != nil { t.Fatal(err) }
			reopened, err := project.LoadConfig(path)
			if err != nil { t.Fatal(err) }
			if reopened.Language != locale.En { t.Fatalf("reopened language %q, want en", reopened.Language) }
		})
	}
}

func TestConfigSaveReportsWriteFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "occupied")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil { t.Fatal(err) }
	if err := (project.Config{Language: locale.En}).Save(filepath.Join(path, "config.json")); err == nil {
		t.Fatal("saving beneath a file should fail")
	}
}
