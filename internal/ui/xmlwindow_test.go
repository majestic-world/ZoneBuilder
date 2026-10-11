package ui

import "testing"

// Copiar conteúdo copies the lines inside <list>, indentation and all, and
// none of the header, the <list> line or the </list> line, for both the
// spawn XML (spaces) and the zone XML (tabs). Catches a cut that keeps the
// declaration or the closing tag, or one that eats the first or last line
// of the content.
func TestListBodyKeepsOnlyTheLinesInsideList(t *testing.T) {
	for _, c := range []struct{ name, xml, want string }{
		{"spawn",
			"<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<!DOCTYPE list SYSTEM \"spawn.dtd\">\n<list>\n   <spawn name=\"[area_1_0]\">\n      <npc id=\"21365\" />\n   </spawn>\n</list>\n",
			"   <spawn name=\"[area_1_0]\">\n      <npc id=\"21365\" />\n   </spawn>\n"},
		{"zone",
			"<?xml version='1.0' encoding='utf-8'?>\r\n<!DOCTYPE list SYSTEM \"zone.dtd\">\r\n<list>\r\n\t<zone name=\"[nave]\" type=\"peace_zone\" >\r\n\t</zone>\r\n</list>\r\n",
			"\t<zone name=\"[nave]\" type=\"peace_zone\" >\r\n\t</zone>\r\n"},
		{"empty list", "<list>\n</list>\n", ""},
		{"no list", "<spawn/>\n", "<spawn/>\n"},
	} {
		if got := listBody(c.xml); got != c.want {
			t.Errorf("%s: listBody = %q, want %q", c.name, got, c.want)
		}
	}
}
