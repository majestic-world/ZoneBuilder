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

	apply(t, d, zone.SetType{Zone: id, Type: "SIEGE"})
	files, err := d.Compile(d.ZoneIDs())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(files[0].Data), `<zone name="[zb_castle]" type="SIEGE" >`) {
		t.Errorf("compiled XML lacks type=\"SIEGE\":\n%s", files[0].Data)
	}
}

// Parameters compile as <set name="..." val="..." /> lines, before the
// shapes, in the order they were added: known and free ones mixed, a
// re-set parameter keeping its place and a removed one gone.
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

// A known ZoneTemplate parameter takes only values the server parses into
// its type; anything else (which the server would read as false, or throw
// on and drop the rest of the file) is refused and leaves the zone as it
// was. Free parameters take any value.
func TestKnownParamAcceptsOnlyValuesOfItsType(t *testing.T) {
	d := zone.NewDocument()
	id := d.NewZoneID()
	polygonZone(t, d, id, "[zb_params]", zone.Damage, -3660, -3148, square...)

	refused := []zone.SetParam{
		{Name: "enabled", Value: "yes"},           // parseBoolean: silently false
		{Name: "default", Value: "1"},             // same
		{Name: "damage_on_hp", Value: "30.5"},     // Integer.parseInt throws
		{Name: "damage_on_hp", Value: ""},         // same
		{Name: "skill_prob", Value: " 100"},       // same
		{Name: "restart_time", Value: "10min"},    // Long.parseLong throws
		{Name: "move_bonus", Value: "fast"},       // Double.parseDouble throws
		{Name: "target", Value: "PC"},             // ZoneTarget.valueOf is case-sensitive
		{Name: "affect_race", Value: "kamael"},    // not in Race
		{Name: "skill_name", Value: "4150"},       // needs "id level"
		{Name: "message_no", Value: "999999"},     // SystemMsg.valueOf throws
		{Name: "blocked_actions", Value: "jump"},  // never checked by the server
		{Name: "entering_message_no", Value: "x"}, // Integer.parseInt throws
	}
	for _, c := range refused {
		c.Zone = id
		if err := d.Apply(c); err == nil {
			t.Errorf("SetParam %s=%q succeeded; want an error", c.Name, c.Value)
		}
	}
	if z, _ := d.Zone(id); len(z.Params) != 0 {
		t.Fatalf("refused values changed the zone: %v", z.Params)
	}

	accepted := []zone.SetParam{
		{Name: "enabled", Value: "false"},
		{Name: "default", Value: "true"},
		{Name: "damage_on_hp", Value: "30"},
		{Name: "skill_prob", Value: "-5"},
		{Name: "restart_time", Value: "600"},
		{Name: "move_bonus", Value: "-80"},
		{Name: "hp_regen_bonus", Value: "1.5"},
		{Name: "target", Value: "only_pc"},
		{Name: "affect_race", Value: "darkelf"},
		{Name: "skill_name", Value: "4150;1"},
		{Name: "message_no", Value: "686"},
		{Name: "entering_message_no", Value: "-1"},
		{Name: "blocked_actions", Value: "open_private_store;open_private_workshop"},
		{Name: "restart_allowed_time", Value: "10min"},
		{Name: "playerMinLevel", Value: "anything goes"},
	}
	for _, c := range accepted {
		c.Zone = id
		if err := d.Apply(c); err != nil {
			t.Errorf("SetParam %s=%q: %v", c.Name, c.Value, err)
		}
	}
}
