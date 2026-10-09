package zone_test

import (
	"strings"
	"testing"

	"zonebuilder/internal/zone"
)

// square is a closed polygon around Giran's town square.
var square = []zone.Point{{X: 83000, Y: 147600, Z: -3404}, {X: 83800, Y: 147600, Z: -3398}, {X: 83800, Y: 148300, Z: -3405}}

// The server's ZoneType.valueOf is case-sensitive and a wrong value makes
// the ZoneParser drop the rest of the file, so "Siege" can never become a
// zone's type, while "SIEGE" can and is written in that exact case.
func TestTypeOutsideTheServerEnumCaseCannotBeSet(t *testing.T) {
	d := zone.NewDocument()
	id := d.NewZoneID()
	polygonZone(t, d, id, "[zb_castle]", zone.PeaceZone, -3660, -3148, square...)

	if err := d.Apply(zone.SetType{Zone: id, Type: "Siege"}); err == nil {
		t.Fatal(`SetType "Siege" succeeded; want an error`)
	}
	if z, _ := d.Zone(id); z.Type != "peace_zone" {
		t.Fatalf("type after the rejected SetType = %q, want it unchanged (peace_zone)", z.Type)
	}

	// A SIEGE zone needs its residence to compile.
	apply(t, d, zone.SetType{Zone: id, Type: "SIEGE"}, zone.SetParam{Zone: id, Name: "residence", Value: "5"})
	files, err := d.Compile(d.ZoneIDs())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(files[0].Data), `<zone name="[zb_castle]" type="SIEGE" >`) {
		t.Errorf("compiled XML lacks type=\"SIEGE\":\n%s", files[0].Data)
	}
}

// Parameters compile as <set name="..." val="..." /> lines, before the
// shapes, in the order they were added: a re-set parameter keeping its
// place and a removed one gone.
func TestParamsCompileAsSetsInDocumentOrder(t *testing.T) {
	d := zone.NewDocument()
	id := d.NewZoneID()
	polygonZone(t, d, id, "[zb_level_gate]", zone.Dummy, -3660, -3148, square...)
	apply(t, d,
		zone.SetParam{Zone: id, Name: "playerMinLevel", Value: "20"},
		zone.SetParam{Zone: id, Name: "enabled", Value: "false"},
		zone.SetParam{Zone: id, Name: "residence", Value: "5"},
		zone.SetParam{Zone: id, Name: "playerMaxLevel", Value: "40"},
		zone.SetParam{Zone: id, Name: "playerLevelLimitBackLoc", Value: "83400 147943 -3404"},
		zone.SetParam{Zone: id, Name: "playerMinLevel", Value: "25"},
		zone.RemoveParam{Zone: id, Name: "residence"},
	)
	files, err := d.Compile(d.ZoneIDs())
	if err != nil {
		t.Fatal(err)
	}
	want := "\t<zone name=\"[zb_level_gate]\" type=\"dummy\" >\n" +
		"\t\t<set name=\"playerMinLevel\" val=\"25\" />\n" +
		"\t\t<set name=\"enabled\" val=\"false\" />\n" +
		"\t\t<set name=\"playerMaxLevel\" val=\"40\" />\n" +
		"\t\t<set name=\"playerLevelLimitBackLoc\" val=\"83400 147943 -3404\" />\n" +
		"\t\t<polygon>\n"
	if !strings.Contains(string(files[0].Data), want) {
		t.Errorf("compiled XML lacks\n%s\ngot:\n%s", want, files[0].Data)
	}
}
