package main

import (
	"strings"
	"testing"

	"zonebuilder/internal/coverage"
	"zonebuilder/internal/locale"
)

func TestCoveragePresentationChangesLanguageWithoutChangingReport(t *testing.T) {
	r := coverage.Report{
		Measured: true,
		GroundMin: coverage.Spot{X: 1234, Y: 2345, Z: -3456},
		GroundMax: coverage.Spot{X: 2468, Y: 3456, Z: 4567},
		FloorClearance: -1234, TopClearance: 2345,
		Inside: 1234, Above: 245, Below: 123, Excluded: 45, Other: 55, NoGround: 298,
		Layers: 2,
	}
	pt := coverageText(locale.PtBR, r, false)
	en := coverageText(locale.En, r, false)
	for _, expected := range []string{"Chão mais baixo: 1234 2345 -3456", "Folga do piso: −1.234 (fura)", "2 camadas", "Dentro da faixa: 1.234 u² (61,7%)", "Sem chão: 298 u² (14,9%)"} {
		if !strings.Contains(pt, expected) { t.Errorf("pt-BR lacks %q: %s", expected, pt) }
	}
	for _, expected := range []string{"Lowest ground: 1234 2345 -3456", "Floor clearance: −1,234 (pierces)", "2 layers", "Inside range: 1,234 u² (61.7%)", "No ground: 298 u² (14.9%)"} {
		if !strings.Contains(en, expected) { t.Errorf("en lacks %q: %s", expected, en) }
	}
	if r.Inside != 1234 || r.FloorClearance != -1234 || r.Layers != 2 { t.Fatalf("presentation changed report: %+v", r) }
}

func TestGroundLayerCountUsesSingularOnlyForOne(t *testing.T) {
	for _, tc := range []struct {
		lang locale.Language
		zero, one, two string
	}{
		{locale.PtBR, "Chão em 0 camadas", "Chão em 1 camada", "Chão em 2 camadas"},
		{locale.En, "Ground in 0 layers", "Ground in 1 layer", "Ground in 2 layers"},
	} {
		for count, want := range []string{tc.zero, tc.one, tc.two} {
			got := coverageText(tc.lang, coverage.Report{Measured: true, Layers: count}, false)
			if !strings.Contains(got, want) {
				t.Errorf("%s %d layers: %q lacks %q", tc.lang, count, got, want)
			}
		}
	}
}

func TestCoverageNoGroundAndLayerPluralInBothLanguages(t *testing.T) {
	r := coverage.Report{NoGround: 1234, Other: 766}
	for _, tc := range []struct{ lang locale.Language; none, one, many, percent string }{
		{locale.PtBR, "Todo o chão está excluído ou em outras camadas", "Outra camada: 1 (z 1.234)", "Outras camadas: 2 (z 1.234; z 2.345 … 3.456)", "Sem chão: 1.234 u² (61,7%)"},
		{locale.En, "All ground is excluded or in other layers", "Other layer: 1 (z 1,234)", "Other layers: 2 (z 1,234; z 2,345 … 3,456)", "No ground: 1,234 u² (61.7%)"},
	} {
		shown := coverageText(tc.lang, r, false)
		if !strings.Contains(shown, tc.none) || !strings.Contains(shown, tc.percent) { t.Errorf("%s no-ground presentation: %s", tc.lang, shown) }
		one := othersText(tc.lang, []coverage.Layer{{Low: 1234, High: 1234}})
		many := othersText(tc.lang, []coverage.Layer{{Low: 1234, High: 1234}, {Low: 2345, High: 3456}})
		if one != tc.one || many != tc.many { t.Errorf("%s other layers: %q; %q", tc.lang, one, many) }
	}
}

func TestHeightSummaryAndRulerLabelsFollowLanguage(t *testing.T) {
	r := coverage.Report{Measured: true, GroundMin: coverage.Spot{Z: -1234}, GroundMax: coverage.Spot{Z: 2345}, FloorClearance: -3456, TopClearance: 1234, Inside: 1234, Above: 766}
	pt := heightSummary(locale.PtBR, r, false)
	en := heightSummary(locale.En, r, false)
	if !strings.Contains(pt, "Chão sob a zona: −1.234 … 2.345") || !strings.Contains(pt, "Cobertura 61,7% · acima 38,3%") { t.Fatalf("pt-BR height: %s", pt) }
	if !strings.Contains(en, "Ground under zone: −1,234 … 2,345") || !strings.Contains(en, "Coverage 61.7% · above 38.3%") { t.Fatalf("en height: %s", en) }
	marks := rulerMarks(locale.En, r)
	if len(marks) != 2 || !marks[1].Alert {
		t.Fatalf("English ruler marks or warning state: %+v", marks)
	}
	if !strings.Contains(marks[0].Text, "2,345") || !strings.Contains(marks[0].Text, "1,234") ||
		!strings.Contains(marks[1].Text, "−1,234") || !strings.Contains(marks[1].Text, "−3,456") {
		t.Fatalf("ruler lost localized ground and clearance values: %+v", marks)
	}
}

func TestCoverageTitlesChangeWhileMeasurementsRemainAvailable(t *testing.T) {
	r := coverage.Report{Measured: true}
	if got := coverageText(locale.En, r, true); !strings.HasPrefix(got, "Ground coverage: measuring…") {
		t.Errorf("measuring shape: %q", got)
	}
	if got := heightSummary(locale.En, r, true); !strings.HasPrefix(got, "Zone coverage (sum of shapes): measuring…") {
		t.Errorf("measuring zone: %q", got)
	}
}
