package spawn_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"reflect"
	"testing"

	"zonebuilder/internal/spawn"
)

type spawnList struct {
	Spawns []struct {
		Name string     `xml:"name,attr"`
		Mesh []struct{} `xml:"mesh"`
		NPC  []struct {
			ID      string     `xml:"id,attr"`
			Count   string     `xml:"count,attr"`
			Respawn string     `xml:"respawn,attr"`
			Pos     string     `xml:"pos,attr"`
			Attrs   []xml.Attr `xml:",any,attr"`
		} `xml:"npc"`
	} `xml:"spawn"`
}

func TestCompiledXMLGroupsAndBalancesNPCsPerArea(t *testing.T) {
	d := spawn.NewDocument()
	first, second := d.NewAreaID(), d.NewAreaID()
	for _, a := range []struct {
		id   spawn.AreaID
		name string
	}{{second, "Den & <Evil>"}, {first, "farm"}} {
		params := spawn.DefaultParams(24)
		params.Count = 2
		apply(t, d, spawn.CreateArea{ID: a.id, Name: a.name, Outline: square, ZMin: -3600, ZMax: -3200, Params: params})
		area, _ := d.Area(a.id)
		apply(t, d, spawn.SetPoints{Area: a.id, Seed: 7, Fingerprint: area.Fingerprint(7), Points: []spawn.Point{{X: 83100, Y: 147700, Z: -3404, Heading: 1200}, {X: 83300, Y: 147900, Z: -3398, Heading: 40000}}})
	}
	apply(t, d, spawn.AddPoint{Area: first, Point: spawn.Point{X: 83200, Y: 147800, Z: -3400, Heading: 0}}, spawn.SetHidden{Areas: []spawn.AreaID{second}, Hidden: true})
	f, err := d.Compile(" giran.XML ", []int{20001, 20002})
	if err != nil {
		t.Fatal(err)
	}
	const want = `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE list SYSTEM "spawn.dtd">
<list>
   <spawn name="[farm_0]">
      <npc id="20001" count="1" respawn="60" pos="83100 147700 -3404 1200" />
      <npc id="20002" count="1" respawn="60" pos="83300 147900 -3398 40000" />
      <npc id="20001" count="1" respawn="60" pos="83200 147800 -3400 1" />
   </spawn>
   <spawn name="[Den &amp; &lt;Evil&gt;_0]">
      <npc id="20001" count="1" respawn="60" pos="83100 147700 -3404 1200" />
      <npc id="20002" count="1" respawn="60" pos="83300 147900 -3398 40000" />
   </spawn>
</list>
`
	if f.Name != "giran.XML" || string(f.Data) != want {
		t.Errorf("compiled %q:\n%s\nwant:\n%s", f.Name, f.Data, want)
	}
	var list spawnList
	if err := xml.Unmarshal(f.Data, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Spawns) != 2 || list.Spawns[1].Name != "[Den & <Evil>_0]" {
		t.Fatalf("spawns = %+v", list.Spawns)
	}
	for _, s := range list.Spawns {
		if len(s.Mesh) != 0 {
			t.Errorf("unexpected mesh: %+v", s)
		}
		for _, n := range s.NPC {
			if len(n.Attrs) != 0 {
				t.Errorf("unexpected NPC attributes: %v", n.Attrs)
			}
		}
	}
	again, err := d.Compile(" giran.XML ", []int{20001, 20002})
	if err != nil || !bytes.Equal(again.Data, f.Data) {
		t.Errorf("second compile differs: %v", err)
	}
}

func TestCompileBlocksOnProblemsButNotOnWarnings(t *testing.T) {
	for name, change := range map[string]func(*spawn.Document, spawn.AreaID){
		"stale":      func(d *spawn.Document, id spawn.AreaID) { apply(t, d, spawn.MoveArea{Area: id, DX: 16}) },
		"empty name": func(d *spawn.Document, id spawn.AreaID) { apply(t, d, spawn.Rename{Area: id, Name: " "}) },
	} {
		t.Run(name, func(t *testing.T) {
			d, id := generated(t, "farm")
			change(d, id)
			f, err := d.Compile("giran", []int{20001})
			var blocked *spawn.BlockedError
			if !errors.As(err, &blocked) || len(blocked.Problems) == 0 || f.Data != nil {
				t.Fatalf("file=%+v err=%v", f, err)
			}
		})
	}
	d, id := generated(t, "farm")
	a, _ := d.Area(id)
	apply(t, d, spawn.SetPoints{Area: id, Seed: 7, Fingerprint: a.Fingerprint(7), Points: a.Points[:1], Warnings: []spawn.Warning{{Rule: spawn.FitsOnly, Placed: 1, Requested: 2}}})
	if _, err := d.Compile("giran", []int{20001}); err != nil {
		t.Errorf("warning blocked compiling: %v", err)
	}
}

func TestParseNPCIDsPreservesUniqueInputOrder(t *testing.T) {
	got, err := spawn.ParseNPCIDs(" 20002\t20001\n20002 20003 ")
	if err != nil || !reflect.DeepEqual(got, []int{20002, 20001, 20003}) {
		t.Fatalf("IDs=%v err=%v", got, err)
	}
}

func TestInvalidNPCIDsProduceNoXML(t *testing.T) {
	for _, text := range []string{"", " \t\n", "0", "-1", "20001 nope", "20001,20002", "1.5", "999999999999999999999999999999"} {
		t.Run(text, func(t *testing.T) {
			if ids, err := spawn.ParseNPCIDs(text); err == nil || ids != nil {
				t.Fatalf("IDs=%v err=%v", ids, err)
			}
		})
	}
	d, _ := generated(t, "farm")
	for _, ids := range [][]int{nil, {}, {0}, {-1}, {20001, 0}, {1, 1, 2}} {
		if f, err := d.Compile("giran", ids); err == nil || f.Data != nil {
			t.Errorf("IDs=%v file=%+v err=%v", ids, f, err)
		}
	}
}

func TestCompileBalancesThreeIDsWithRemainderAndNormalizesHeadings(t *testing.T) {
	d, id := generated(t, "farm")
	a, _ := d.Area(id)
	headings := []int{-1, 65536, 65537, 0, 65535, -65536, 1200}
	points := make([]spawn.Point, len(headings))
	for i, h := range headings {
		points[i] = spawn.Point{X: 83100, Y: 147700, Z: -3404, Heading: h}
	}
	apply(t, d, spawn.SetPoints{Area: id, Seed: 7, Fingerprint: a.Fingerprint(7), Points: points})
	f, err := d.Compile("farm", []int{9, 7, 8})
	if err != nil {
		t.Fatal(err)
	}
	var list spawnList
	if err := xml.Unmarshal(f.Data, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Spawns) != 1 || len(list.Spawns[0].NPC) != 7 {
		t.Fatalf("spawns = %+v", list.Spawns)
	}
	wantIDs := []string{"9", "7", "8", "9", "7", "8", "9"}
	wantPos := []string{
		"83100 147700 -3404 65535", "83100 147700 -3404 1",
		"83100 147700 -3404 1", "83100 147700 -3404 1",
		"83100 147700 -3404 65535", "83100 147700 -3404 1",
		"83100 147700 -3404 1200",
	}
	for i, n := range list.Spawns[0].NPC {
		if n.ID != wantIDs[i] || n.Pos != wantPos[i] {
			t.Errorf("NPC %d = %+v, want ID %s pos %s", i, n, wantIDs[i], wantPos[i])
		}
	}
}
