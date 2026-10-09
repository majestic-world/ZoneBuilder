package locale_test

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"zonebuilder/internal/locale"
)

func TestBundledCatalogsCoverLiteralCodeUses(t *testing.T) {
	if err := locale.Verify(os.DirFS("../..")); err != nil {
		t.Fatal(err)
	}
}


func TestCatalogCheckerRejectsIncompleteAreasAndMessages(t *testing.T) {
	base := fstest.MapFS{
		"app/pt-BR.json": &fstest.MapFile{Data: []byte(`{"app.choice":"Escolha {name}","app.count":{"one":"{count} zona","other":"{count} zonas"}}`)},
		"app/en.json": &fstest.MapFile{Data: []byte(`{"app.choice":"Choose {name}","app.count":{"one":"{count} zone","other":"{count} zones"}}`)},
	}
	source := fstest.MapFS{"main.go": &fstest.MapFile{Data: []byte("package main\nimport \"zonebuilder/internal/locale\"\nfunc main(){ locale.Text(locale.En, \"app.choice\") }")}}
	if err := locale.Check(base, source); err != nil {
		t.Fatalf("valid pair: %v", err)
	}
	for _, tc := range []struct {
		name string
		path string
		data string
		want string
	}{
		{"missing English file", "app/en.json", "", "en.json"},
		{"missing key", "app/en.json", `{"app.count":{"one":"{count} zone","other":"{count} zones"}}`, "app.choice"},
		{"mismatched parameters", "app/en.json", `{"app.choice":"Choose {label}","app.count":{"one":"{count} zone","other":"{count} zones"}}`, "app.choice"},
		{"missing plural variant", "app/en.json", `{"app.choice":"Choose {name}","app.count":{"one":"{count} zone"}}`, "other"},
		{"different plural parameters", "app/en.json", `{"app.choice":"Choose {name}","app.count":{"one":"{count} zone","other":"{total} zones"}}`, "plural variants"},
		{"static instead of plural", "app/en.json", `{"app.choice":"Choose {name}","app.count":"{count} zones"}`, "app.count"},
		{"duplicate plural variant", "app/en.json", `{"app.choice":"Choose {name}","app.count":{"one":"{count} zone","one":"{count} zone","other":"{count} zones"}}`, "duplicate"},
		{"duplicate key", "app/en.json", `{"app.choice":"Choose {name}","app.choice":"Choose {name}","app.count":{"one":"{count} zone","other":"{count} zones"}}`, "duplicate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := fstest.MapFS{}
			for name, file := range base {
				files[name] = file
			}
			if tc.data == "" {
				delete(files, tc.path)
			} else {
				files[tc.path] = &fstest.MapFile{Data: []byte(tc.data)}
			}
			if err := locale.Check(files, source); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Check() = %v, want %q", err, tc.want)
			}
		})
	}
	files := fstest.MapFS{}
	for name, file := range base {
		files[name] = file
	}
	files["newarea/pt-BR.json"] = &fstest.MapFile{Data: []byte(`{"newarea.title":"Título"}`)}
	if err := locale.Check(files, source); err == nil || !strings.Contains(err.Error(), "newarea/en.json") {
		t.Errorf("new area without English pair = %v", err)
	}
	source["main.go"] = &fstest.MapFile{Data: []byte("package main\nimport \"zonebuilder/internal/locale\"\nfunc main(){ locale.Text(locale.En, \"app.missing\") }")}
	if err := locale.Check(base, source); err == nil || !strings.Contains(err.Error(), "app.missing") {
		t.Errorf("untranslated code use = %v", err)
	}
}

func TestCatalogCheckerRejectsDynamicKeysAndUntranslatedFutureArea(t *testing.T) {
	catalogs := fstest.MapFS{
		"app/pt-BR.json": {Data: []byte(`{"app.title":"Título"}`)},
		"app/en.json": {Data: []byte(`{"app.title":"Title"}`)},
	}
	for _, tc := range []struct {
		name, code, want string
	}{
		{"direct dynamic call", `locale.Text(locale.En, key)`, "dynamic key"},
		{"computed key", `locale.Text(locale.En, "app." + suffix)`, "dynamic key"},
		{"format helper", `locale.Format(locale.En, key, nil)`, "dynamic key"},
		{"plural helper", `locale.Plural(locale.En, key, 2, nil)`, "dynamic key"},
		{"message key", `locale.Message{Key: key}`, "dynamic key"},
		{"nested message key", `locale.Message{Key: "app.title", Parts: map[string]locale.Message{"hint": {Key: key}}}`, "dynamic key"},
		{"nested future message", `locale.Message{Key: "app.title", Parts: map[string]locale.Message{"hint": {Key: "future.hint"}}}`, "future.hint"},
		{"missing future key", `locale.Text(locale.En, "future.title")`, "future.title"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fstest.MapFS{"future/main.go": {Data: []byte("package future\nimport \"zonebuilder/internal/locale\"\nfunc show(key, suffix string) { _ = " + tc.code + " }")}}
			if err := locale.Check(catalogs, source); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Check() = %v, want %q", err, tc.want)
			}
		})
	}
}
