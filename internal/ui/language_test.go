package ui

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/zonexml"
)

func TestLanguageSwitchKeepsEditorsAndViewport(t *testing.T) {
	s := NewShell(NewTheme(), "client", "22_22")
	s.Tile.SetText("23_24")
	s.Zone.Name.SetText("unfinished zone")
	s.Project.Unsaved = true
	s.Height.Zone = "zone floor"
	s.XML.Open([]zonexml.File{{Name: "peace_zone.xml", Data: []byte("<zone id=\"42\"/>")}}, ZoneXML)
	s.Language = locale.PtBR
	for _, language := range []locale.Language{locale.PtBR, locale.En, locale.PtBR} {
		s.Language = language
		var ops op.Ops
		gtx := layout.Context{Ops: &ops, Constraints: layout.Exact(image.Pt(1280, 800))}
		gtx.Constraints.Min = image.Point{}
		selector := s.languageSelector(gtx).Size
		if selector.X > 170 || selector.Y > 50 || selector.X < 100 || selector.Y < 32 {
			t.Fatalf("language selector does not fit the compact top card: %v", selector)
		}
		if got := s.Layout(gtx); got != (image.Rectangle{Max: image.Pt(1280, 800)}) {
			t.Fatalf("viewport after %q: %v", language, got)
		}
		if s.Tile.Text() != "23_24" || s.Zone.Name.Text() != "unfinished zone" || !s.Project.Unsaved {
			t.Fatalf("editing state changed after selecting %q", language)
		}
		if s.Language != language { t.Fatalf("language = %q, want %q", s.Language, language) }
		if s.Height.Window.Closed || s.XML.Window.Closed {
			t.Fatalf("floating windows changed after selecting %q", language)
		}
		if len(s.XML.files) != 1 || s.XML.files[0].text != "<zone id=\"42\"/>" {
			t.Fatalf("XML changed after selecting %q", language)
		}
	}
}

func TestPanelPresentationSwitchesWithoutChangingZoneData(t *testing.T) {
	s := NewShell(NewTheme(), "", "")
	s.Zones.NewName.SetText("Minha zona")
	s.Zones.Search.SetText("search in progress")
	s.Zones.Rows = []ZoneRow{{Name: "Minha zona", Problems: 2}}
	s.Problems.Rows = []ProblemRow{{Zone: "Minha zona", Message: "warning", Warning: true}}
	for _, tc := range []struct {
		language locale.Language
		zoneNote, problems string
	}{
		{locale.PtBR, "1 zona", "1 aviso"},
		{locale.En, "1 zone", "1 warning"},
		{locale.PtBR, "1 zona", "1 aviso"},
	} {
		s.Language = tc.language
		if got := zoneListNote(s.Language, len(s.Zones.Rows), 1); got != tc.zoneNote {
			t.Errorf("zone header in %s: %q, want %q", tc.language, got, tc.zoneNote)
		}
		if got := problemNote(s.Language, 0, len(s.Problems.Rows)); got != tc.problems {
			t.Errorf("problem header in %s: %q, want %q", tc.language, got, tc.problems)
		}
		if s.Zones.NewName.Text() != "Minha zona" || s.Zones.Search.Text() != "search in progress" || s.Zones.Rows[0].Name != "Minha zona" {
			t.Fatalf("zone data or focused editor content changed after selecting %q", tc.language)
		}
	}
}

func TestProjectDialogDescriptionFollowsLanguage(t *testing.T) {
	if got := ProjectFileFor(locale.PtBR).Name; got != "Projeto do Zone Builder" {
		t.Errorf("Portuguese file type: %q", got)
	}
	if got := ProjectFileFor(locale.En).Name; got != "Zone Builder project" {
		t.Errorf("English file type: %q", got)
	}
}

func TestPropertyFeedbackRendersInCurrentLanguage(t *testing.T) {
	p := PropertiesPanel{}
	message := p.Applied(EditZone{status: locale.Message{
		Key: "ui.properties.type_changed",
		Args: map[string]string{"name": "Minha zona", "type": "peace_zone"},
	}}, nil)
	if got := message.Render(locale.PtBR); got != "Minha zona: tipo peace_zone" {
		t.Errorf("Portuguese feedback: %q", got)
	}
	if got := message.Render(locale.En); got != "Minha zona: type peace_zone" {
		t.Errorf("English feedback: %q", got)
	}
}
