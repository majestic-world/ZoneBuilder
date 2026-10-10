package spawn_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"strconv"
	"strings"
	"testing"

	"zonebuilder/internal/spawn"
)

// spawnList is the parsed compiled XML.
type spawnList struct {
	Spawns []struct {
		Name string `xml:"name,attr"`
		Mesh []struct{} `xml:"mesh"`
		NPC  []struct {
			ID          string     `xml:"id,attr"`
			Count       string     `xml:"count,attr"`
			Respawn     string     `xml:"respawn,attr"`
			RespawnRand *string    `xml:"respawn_rand,attr"`
			Pos         string     `xml:"pos,attr"`
			Attrs       []xml.Attr `xml:",any,attr"`
		} `xml:"npc"`
	} `xml:"spawn"`
}

// A document of 2 areas, created in reverse ID order, one with
// respawn_rand 0 and a point added by hand at heading 0, the other with
// respawn_rand 15 and a name to escape, compiles to 1 <spawn> with
// count="1" per point, areas in ID order, no heading 0, respawn_rand only on
// the second, no <mesh> nor other attributes, and the same bytes twice.
// Catches a mesh or a count > 1 emitted (the server would draw new points
// on respawn), a heading the server ignores, an unstable order (the diff on
// the server repository would churn) and an unescaped name (the server
// would lose the whole file).
func TestCompiledXMLHasOneFixedSpawnPerPoint(t *testing.T) {
	d := spawn.NewDocument()
	first, second := d.NewAreaID(), d.NewAreaID()
	for _, a := range []struct {
		id   spawn.AreaID
		name string
		rand int
	}{{second, "Den & <Evil>", 15}, {first, "farm", 0}} {
		params := spawn.DefaultParams(24)
		params.NPCID, params.Count, params.RespawnRand = 20001, 2, a.rand
		apply(t, d, spawn.CreateArea{ID: a.id, Name: a.name, Outline: square, ZMin: -3600, ZMax: -3200, Params: params})
		area, _ := d.Area(a.id)
		apply(t, d, spawn.SetPoints{
			Area: a.id, Seed: 7, Fingerprint: area.Fingerprint(7),
			Points: []spawn.Point{{X: 83100, Y: 147700, Z: -3404, Heading: 1200}, {X: 83300, Y: 147900, Z: -3398, Heading: 40000}},
		})
	}
	apply(t, d, spawn.AddPoint{Area: first, Point: spawn.Point{X: 83200, Y: 147800, Z: -3400, Heading: 0}})

	f, err := d.Compile("giran")
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "giran.xml" {
		t.Errorf("Name = %q, want giran.xml", f.Name)
	}
	if !bytes.HasPrefix(f.Data, []byte("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<!DOCTYPE list SYSTEM \"spawn.dtd\">\n<list>\n")) {
		t.Errorf("header:\n%s", f.Data)
	}
	if bytes.Contains(f.Data, []byte("<mesh")) || bytes.Contains(f.Data, []byte("\r")) {
		t.Errorf("mesh or CR in:\n%s", f.Data)
	}
	var list spawnList
	if err := xml.Unmarshal(f.Data, &list); err != nil {
		t.Fatalf("not well-formed: %v\n%s", err, f.Data)
	}
	wantNames := []string{"[farm_0]", "[farm_1]", "[farm_2]", "[Den & <Evil>_0]", "[Den & <Evil>_1]"}
	if len(list.Spawns) != len(wantNames) {
		t.Fatalf("%d spawns, want %d:\n%s", len(list.Spawns), len(wantNames), f.Data)
	}
	for i, s := range list.Spawns {
		if s.Name != wantNames[i] {
			t.Errorf("spawn %d name = %q, want %q", i, s.Name, wantNames[i])
		}
		if len(s.Mesh) != 0 || len(s.NPC) != 1 {
			t.Fatalf("spawn %d: %d mesh, %d npc", i, len(s.Mesh), len(s.NPC))
		}
		n := s.NPC[0]
		if n.ID != "20001" || n.Count != "1" || n.Respawn != "60" {
			t.Errorf("spawn %d npc id=%q count=%q respawn=%q", i, n.ID, n.Count, n.Respawn)
		}
		if len(n.Attrs) != 0 {
			t.Errorf("spawn %d has extra attributes %v", i, n.Attrs)
		}
		wantRand := i >= 3
		if (n.RespawnRand != nil) != wantRand || (wantRand && *n.RespawnRand != "15") {
			t.Errorf("spawn %d respawn_rand = %v, want present only on the second area", i, n.RespawnRand)
		}
		pos := strings.Fields(n.Pos)
		if len(pos) != 4 {
			t.Fatalf("spawn %d pos = %q", i, n.Pos)
		}
		if h, _ := strconv.Atoi(pos[3]); h == 0 {
			t.Errorf("spawn %d heading 0", i)
		}
	}

	again, err := d.Compile("giran")
	if err != nil || !bytes.Equal(again.Data, f.Data) {
		t.Errorf("second compile differs (err %v):\n%s\n---\n%s", err, f.Data, again.Data)
	}
}

// A stale area or one with a blocking problem makes Compile return a
// *BlockedError with no file, while an area with only distribution
// warnings compiles. Catches an XML of a distribution the user never saw,
// a file the server would drop whole, or warnings stopping a valid farm.
func TestCompileBlocksOnProblemsButNotOnWarnings(t *testing.T) {
	blocked := map[string]func(d *spawn.Document, id spawn.AreaID){
		"stale": func(d *spawn.Document, id spawn.AreaID) {
			apply(t, d, spawn.MoveArea{Area: id, DX: 16})
		},
		"respawn_rand above respawn": func(d *spawn.Document, id spawn.AreaID) {
			a, _ := d.Area(id)
			p := a.Params
			p.RespawnRand = p.Respawn + 1
			apply(t, d, spawn.SetParams{Area: id, Params: p})
		},
	}
	for name, change := range blocked {
		t.Run(name, func(t *testing.T) {
			d, id := generated(t, "farm")
			change(d, id)
			f, err := d.Compile("giran")
			var be *spawn.BlockedError
			if !errors.As(err, &be) || len(be.Problems) == 0 {
				t.Fatalf("err = %v, want *BlockedError", err)
			}
			if f.Data != nil {
				t.Errorf("blocked compile produced:\n%s", f.Data)
			}
		})
	}

	d := spawn.NewDocument()
	id := d.NewAreaID()
	params := spawn.DefaultParams(24)
	params.NPCID, params.Count = 20001, 5
	apply(t, d, spawn.CreateArea{ID: id, Name: "farm", Outline: square, ZMin: -3600, ZMax: -3200, Params: params})
	a, _ := d.Area(id)
	apply(t, d, spawn.SetPoints{
		Area: id, Seed: 7, Fingerprint: a.Fingerprint(7),
		Points:   []spawn.Point{{X: 83100, Y: 147700, Z: -3404, Heading: 1200}},
		Warnings: []spawn.Warning{{Rule: spawn.FitsOnly, Placed: 1, Requested: 5}},
	})
	if len(d.Problems()) == 0 {
		t.Fatal("the FitsOnly warning is not reported")
	}
	if _, err := d.Compile("giran"); err != nil {
		t.Errorf("warning blocked compiling: %v", err)
	}
}
