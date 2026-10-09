package ui

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"

	"zonebuilder/internal/locale"
)

func TestLanguageSwitchKeepsEditorsAndViewport(t *testing.T) {
	s := NewShell(NewTheme(), "client", "22_22")
	s.Tile.SetText("23_24")
	s.Zone.Name.SetText("unfinished zone")
	s.Project.Unsaved = true
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
	}
}
