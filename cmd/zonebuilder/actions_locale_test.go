package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/water"
)

func TestActionStatusRendersAgainWithoutRepeatingAction(t *testing.T) {
	status := actionArgs(locale.Message{Key: "actions.project.saved"}, map[string]string{"path": `C:\maps\giran.zbproj`})
	if got := status.render(locale.PtBR); got != `Projeto salvo em C:\maps\giran.zbproj` {
		t.Fatalf("Portuguese status = %q", got)
	}
	if got := status.render(locale.En); got != `Project saved to C:\maps\giran.zbproj` {
		t.Fatalf("English status = %q", got)
	}
	if got := status.render(locale.PtBR); got != `Projeto salvo em C:\maps\giran.zbproj` {
		t.Fatalf("restored status = %q", got)
	}
	failed := actionError(locale.Message{Key: "actions.error.load_tile"}, errors.New("access denied: map.unr"), map[string]string{"tile": "22_24"})
	if got := failed.render(locale.En); !strings.Contains(got, "Could not load tile 22_24") || !strings.Contains(got, "access denied: map.unr") {
		t.Fatalf("localized failure lost technical detail: %q", got)
	}
}

func TestWaterCompilationStatusChangesLanguageWithoutChangingXMLOrDocument(t *testing.T) {
	e := newZoneEditor()
	v := waterBox(t, 1, "WaterVolume1", 0, 0, 100, 100, -200, 0)
	selected := []scene.WaterVolume{v}
	status, files := e.compileWater(selected, selected)
	if len(files) != 1 {
		t.Fatalf("compiled %d files, want 1", len(files))
	}
	beforeXML := bytes.Clone(files[0].Data)
	beforeDocument := zonesJSON(t, e.doc)
	if got := status.render(locale.PtBR); !strings.Contains(got, "Criada 1 zona de água") || !strings.Contains(got, "polígono") {
		t.Fatalf("Portuguese water result = %q", got)
	}
	if got := status.render(locale.En); !strings.Contains(got, "Created 1 water zone") || !strings.Contains(got, "polygon") {
		t.Fatalf("English water result = %q", got)
	}
	if !bytes.Equal(files[0].Data, beforeXML) || zonesJSON(t, e.doc) != beforeDocument {
		t.Fatal("changing language modified compiled XML or the document")
	}
}

func TestWaterSelectionPresentationFormatsCountsAndTopsInEachLanguage(t *testing.T) {
	status := actionStatus{water: &waterSummary{kind: waterSelectionStatus, volumes: 2, tops: []int{-3780, -3850}, exact: false}}
	if got := status.render(locale.PtBR); !strings.Contains(got, "2 volumes") || !strings.Contains(got, "topos") || !strings.Contains(got, "aproximada") {
		t.Fatalf("Portuguese water selection = %q", got)
	}
	if got := status.render(locale.En); !strings.Contains(got, "2 volumes") || !strings.Contains(got, "tops") || !strings.Contains(got, "approximate") {
		t.Fatalf("English water selection = %q", got)
	}
}

func TestLoadingProgressChangesLanguageWithoutAdvancingTileLoad(t *testing.T) {
	shown := scene.Tile{X: 22, Y: 24}
	waiting := scene.Tile{X: 22, Y: 25}
	ts := &tiles{entries: map[scene.Tile]*tileEntry{
		shown: {tile: shown, state: tileShown},
		waiting: {tile: waiting, state: tileReady},
	}}
	pt, before := ts.progress(nil, locale.PtBR)
	en, after := ts.progress(nil, locale.En)
	if pt != "Carregando: 1 de 2 tiles prontos" || en != "Loading: 1 of 2 tiles ready" {
		t.Fatalf("progress stayed untranslated: pt %q, en %q", pt, en)
	}
	if before != after || ts.entries[shown].state != tileShown || ts.entries[waiting].state != tileReady {
		t.Fatal("presenting progress altered the loading state")
	}
}

func TestWaterWarningsRetainVolumeIdentityAcrossLanguageChoice(t *testing.T) {
	status := waterAction(locale.Message{Key: "actions.water.compiled"}, &waterSummary{
		kind: waterResult,
		existing: []string{"[giran]"},
		warnings: []water.Warning{{Kind: water.Approximate, Volume: "22_24 WaterVolume1"}},
	})
	if got := status.render(locale.PtBR); !strings.Contains(got, "Aproximada, parede ou topo inclinado: 22_24 WaterVolume1") {
		t.Fatalf("Portuguese warning = %q", got)
	}
	if got := status.render(locale.En); !strings.Contains(got, "Approximate, slanted wall or top: 22_24 WaterVolume1") {
		t.Fatalf("English warning = %q", got)
	}
}
