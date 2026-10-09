package locale_test

import (
	"strings"
	"testing"

	"zonebuilder/internal/locale"
)

func TestEffectiveLanguageAndMessages(t *testing.T) {
	for _, tc := range []struct {
		preference string
		want       locale.Language
		message    string
	}{
		{"", locale.PtBR, "Não foi possível salvar a preferência de idioma; a escolha vale apenas nesta sessão. disco cheio"},
		{"pt-BR", locale.PtBR, "Não foi possível salvar a preferência de idioma; a escolha vale apenas nesta sessão. disco cheio"},
		{"en", locale.En, "Could not save the language preference; your choice applies only to this session. disco cheio"},
		{"fr", locale.PtBR, "Não foi possível salvar a preferência de idioma; a escolha vale apenas nesta sessão. disco cheio"},
	} {
		language := locale.Normalize(tc.preference)
		args := map[string]string{"detail": "disco cheio"}
		if language != tc.want || locale.Format(language, "actions.preference.unsaved", args) != tc.message {
			t.Errorf("preference %q: language %q, message %q", tc.preference, language, locale.Format(language, "actions.preference.unsaved", args))
		}
	}
}

func TestNamedArgumentsAndPlural(t *testing.T) {
	for _, tc := range []struct {
		language locale.Language
		detail   string
		failure  string
		counts   [3]string
	}{
		{locale.PtBR, "arquivo ausente", "Não foi possível abrir o mapa: arquivo ausente", [3]string{"0 zonas selecionadas de 2 para compilar", "1 zona selecionada de 2 para compilar", "2 zonas selecionadas de 2 para compilar"}},
		{locale.En, "missing file", "Could not open map: missing file", [3]string{"0 zones selected out of 2 for compilation", "1 zone selected out of 2 for compilation", "2 zones selected out of 2 for compilation"}},
	} {
		if got := locale.Format(tc.language, "actions.error.open_map", map[string]string{"detail": tc.detail}); got != tc.failure {
			t.Errorf("%s: failure = %q, want %q", tc.language, got, tc.failure)
		}
		for n, want := range tc.counts {
			if got := locale.Plural(tc.language, "editor.compile.selection", n, map[string]string{"total": "2"}); got != want {
				t.Errorf("%s: count %d = %q, want %q", tc.language, n, got, want)
			}
		}
	}
}

func TestRetainedMessageRendersAfterLanguageChange(t *testing.T) {
	status := locale.Message{Key: "actions.error.open_map", Args: map[string]string{"detail": "disk full"}}
	if got := status.Render(locale.En); got != "Could not open map: disk full" {
		t.Errorf("English status = %q", got)
	}
	if got := status.Render(locale.PtBR); got != "Não foi possível abrir o mapa: disk full" {
		t.Errorf("Portuguese status = %q", got)
	}
	count := locale.Message{Key: "editor.compile.selection", Plural: true, Count: 0, Args: map[string]string{"total": "2"}}
	if got := count.Render(locale.En); got != "0 zones selected out of 2 for compilation" {
		t.Errorf("zero-count English status = %q", got)
	}
}

func TestRetainedMessageRendersNestedMessagesInChosenLanguage(t *testing.T) {
	status := locale.Message{Key: "editor.create.done", Args: map[string]string{"name": "area"}, Parts: map[string]locale.Message{
		"hint": {Key: "editor.hint.polygon_first"},
	}}
	if got := status.Render(locale.En); got != "Zone area created. Polygon: click a surface for the first vertex; Esc cancels" {
		t.Errorf("English status = %q", got)
	}
	if got := status.Render(locale.PtBR); !strings.Contains(got, "Zona area criada.") || !strings.Contains(got, "Polígono:") {
		t.Errorf("Portuguese status = %q", got)
	}
}

func TestCircleCompletionUsesSingularOnlyForOneVertex(t *testing.T) {
	for _, tc := range []struct {
		lang locale.Language
		count int
		want string
	}{
		{locale.PtBR, 0, "0 vértices"}, {locale.PtBR, 1, "1 vértice"}, {locale.PtBR, 2, "2 vértices"},
		{locale.En, 0, "0 vertices"}, {locale.En, 1, "1 vertex"}, {locale.En, 2, "2 vertices"},
	} {
		for _, key := range []string{"editor.circle.done", "editor.circle.exclusion_done"} {
			args := map[string]string{"name": "area", "radius": "10", "vertices": locale.Number(tc.lang, float64(tc.count), 0), "min": "0", "max": "20", "source": "ground"}
			if got := locale.Plural(tc.lang, key, tc.count, args); !strings.Contains(got, tc.want) {
				t.Errorf("%s %s %d: %q lacks %q", tc.lang, key, tc.count, got, tc.want)
			}
		}
	}
}

func TestPresentationNumbers(t *testing.T) {
	for _, tc := range []struct {
		language             locale.Language
		negative, coverage   string
	}{
		{locale.PtBR, "-12.345,60", "87,5%"},
		{locale.En, "-12,345.60", "87.5%"},
	} {
		if got := locale.Number(tc.language, -12345.6, 2); got != tc.negative {
			t.Errorf("%s: negative clearance = %q, want %q", tc.language, got, tc.negative)
		}
		if got := locale.Percent(tc.language, .875, 1); got != tc.coverage {
			t.Errorf("%s: coverage = %q, want %q", tc.language, got, tc.coverage)
		}
	}
}

func TestUnknownMessageNeverFallsBackToPortuguese(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil || !strings.Contains(recovered.(string), "app.missing") {
			t.Errorf("missing key panic = %v", recovered)
		}
	}()
	locale.Text(locale.En, "app.missing")
}
