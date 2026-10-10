// Package spawnxml writes spawn points in the XML the Java server's
// SpawnParser loads (majestic-java SpawnParser.java; data/spawn/*.xml in the
// datapack): the header, DOCTYPE and <list> root, space indentation, LF line
// ends, and one <spawn> per area with fixed positions and count="1", so each
// monster is born and reborn on the point the user approved. It never
// writes <mesh> (the server would draw a new point on every respawn),
// event_name, period_of_day, respawn_cron or ai_params.
package spawnxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// Ext ends every compiled file name.
const Ext = ".xml"

// Area is the spawn points of one area, written in this order.
type Area struct {
	// Name makes the area's <spawn name="[Name_0]">.
	Name string
	// NPCIDs is nonempty for areas with points; IDs are assigned cyclically.
	NPCIDs []int
	Points []Point
}

// Point is a pos="X Y Z Heading" in server coordinates.
type Point struct{ X, Y, Z, Heading int }

// File is one compiled XML file.
type File struct {
	// Name is the base name, ending in Ext.
	Name string
	Data []byte
}

// FileName is name with Ext appended unless it already ends in it
// (case-insensitively), trimmed of surrounding spaces.
func FileName(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasSuffix(strings.ToLower(name), Ext) {
		return name
	}
	return name + Ext
}

// Compile writes areas, in the given order, as one file named
// FileName(name). It is a pure function of its input, so the same input
// always gives the same bytes.
func Compile(name string, areas []Area) File {
	var b bytes.Buffer
	b.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
	b.WriteString("<!DOCTYPE list SYSTEM \"spawn.dtd\">\n")
	b.WriteString("<list>\n")
	for _, a := range areas {
		fmt.Fprintf(&b, "   <spawn name=\"%s\">\n", attr("["+a.Name+"_0]"))
		for i, p := range a.Points {
			fmt.Fprintf(&b, "      <npc id=\"%d\" count=\"1\" respawn=\"60\" pos=\"%d %d %d %d\" />\n", a.NPCIDs[i%len(a.NPCIDs)], p.X, p.Y, p.Z, Heading(p.Heading))
		}
		b.WriteString("   </spawn>\n")
	}
	b.WriteString("</list>\n")
	return File{Name: FileName(name), Data: b.Bytes()}
}

// Heading is h in 0..65535 (65536 = 360°) and never 0: the server ignores
// a 0 heading, so 0 becomes 1, the nearest heading it keeps.
func Heading(h int) int {
	h %= 65536
	if h < 0 {
		h += 65536
	}
	if h == 0 {
		return 1
	}
	return h
}

// attr escapes s for a double-quoted attribute value.
func attr(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}
