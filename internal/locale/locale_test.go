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
		{"", locale.PtBR, "Não foi possível salvar a preferência de idioma; a escolha vale apenas nesta sessão."},
		{"pt-BR", locale.PtBR, "Não foi possível salvar a preferência de idioma; a escolha vale apenas nesta sessão."},
		{"en", locale.En, "Could not save the language preference; your choice applies only to this session."},
		{"fr", locale.PtBR, "Não foi possível salvar a preferência de idioma; a escolha vale apenas nesta sessão."},
	} {
		language := locale.Normalize(tc.preference)
		if language != tc.want || locale.Text(language, "app.preference.unsaved") != tc.message {
			t.Errorf("preference %q: language %q, message %q", tc.preference, language, locale.Text(language, "app.preference.unsaved"))
		}
	}
}

func TestNamedArgumentsAndPlural(t *testing.T) {
	for _, tc := range []struct {
		language locale.Language
		action   string
		detail   string
		failure  string
		counts   [3]string
	}{
		{locale.PtBR, "abrir mapa", "arquivo ausente", "Falha ao abrir mapa: arquivo ausente", [3]string{"0 zonas selecionadas", "1 zona selecionada", "2 zonas selecionadas"}},
		{locale.En, "open map", "missing file", "Could not open map: missing file", [3]string{"0 zones selected", "1 zone selected", "2 zones selected"}},
	} {
		if got := locale.Format(tc.language, "app.action.failed", map[string]string{"action": tc.action, "detail": tc.detail}); got != tc.failure {
			t.Errorf("%s: failure = %q, want %q", tc.language, got, tc.failure)
		}
		for n, want := range tc.counts {
			if got := locale.Plural(tc.language, "app.status.zone_count", n, nil); got != want {
				t.Errorf("%s: count %d = %q, want %q", tc.language, n, got, want)
			}
		}
	}
}

func TestRetainedMessageRendersAfterLanguageChange(t *testing.T) {
	status := locale.Message{Key: "app.action.save_failed", Args: map[string]string{"detail": "disk full"}}
	if got := status.Render(locale.En); got != "Could not save: disk full" {
		t.Errorf("English status = %q", got)
	}
	if got := status.Render(locale.PtBR); got != "Não foi possível salvar: disk full" {
		t.Errorf("Portuguese status = %q", got)
	}
	count := locale.Message{Key: "app.status.zone_count", Plural: true, Count: 0}
	if got := count.Render(locale.En); got != "0 zones selected" {
		t.Errorf("zero-count English status = %q", got)
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
