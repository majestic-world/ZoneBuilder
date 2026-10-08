// Package zonexml writes zones in the XML the Java server's ZoneParser loads
// (majestic-java ZoneParser.java; data/zone/*.xml in the datapack): the
// header, DOCTYPE and <list> root, tab indentation, LF line ends, and
// polygon coords of 4 integers that all carry the polygon's Z range, since
// the parser applies the last coords' range to the whole polygon and spans
// the full world height when a coords has fewer than 4 numbers.
package zonexml

import (
	"bytes"
	"cmp"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
)

// FilePrefix starts every file name Compile produces, so the output never
// overwrites the datapack's own zone files (named after the type alone).
const FilePrefix = "zonebuilder_"

// Zone is one <zone> element.
type Zone struct {
	Name string
	// Type is the ZoneType value, written as is.
	Type   string
	Shapes []Shape
	// RestartPoints and PKRestartPoints are x y z, written as one
	// <restart_point> and one <PKrestart_point> block (the parser keeps
	// only the last block of each).
	RestartPoints, PKRestartPoints [][3]int
}

// Shape is one <polygon>, <rectangle> or, when Banned, <banned_polygon>:
// its points (x, y) and the Z range every coords carries. A rectangle's
// points are its 2 opposite corners. A banned rectangle is not a valid
// Shape: give its 4 corners as a banned polygon.
type Shape struct {
	Rectangle, Banned bool
	Points            [][2]int
	ZMin, ZMax        int
}

// File is one compiled XML file.
type File struct {
	// Name is the base name, FilePrefix + type + ".xml".
	Name string
	Data []byte
}

// Compile writes zones as one file per type, files in type order and zones
// in name order inside each file (zones sharing type and name keep their
// order in zones). It is a pure function of zones, so the same input always
// gives the same bytes.
func Compile(zones []Zone) []File {
	sorted := slices.Clone(zones)
	slices.SortStableFunc(sorted, func(a, b Zone) int {
		return cmp.Or(cmp.Compare(a.Type, b.Type), cmp.Compare(a.Name, b.Name))
	})
	var files []File
	for len(sorted) > 0 {
		n := 1
		for n < len(sorted) && sorted[n].Type == sorted[0].Type {
			n++
		}
		files = append(files, File{Name: FilePrefix + sorted[0].Type + ".xml", Data: encode(sorted[:n])})
		sorted = sorted[n:]
	}
	return files
}

func encode(zones []Zone) []byte {
	var b bytes.Buffer
	b.WriteString("<?xml version='1.0' encoding='utf-8'?>\n")
	b.WriteString("<!DOCTYPE list SYSTEM \"zone.dtd\">\n")
	b.WriteString("<list>\n")
	for _, z := range zones {
		fmt.Fprintf(&b, "\t<zone name=\"%s\" type=\"%s\" >\n", attr(z.Name), attr(z.Type))
		for _, s := range z.Shapes {
			elem := "polygon"
			if s.Rectangle {
				elem = "rectangle"
			}
			if s.Banned {
				elem = "banned_" + elem
			}
			fmt.Fprintf(&b, "\t\t<%s>\n", elem)
			for _, pt := range s.Points {
				coords(&b, pt[0], pt[1], s.ZMin, s.ZMax)
			}
			fmt.Fprintf(&b, "\t\t</%s>\n", elem)
		}
		restartPoints(&b, "restart_point", z.RestartPoints)
		restartPoints(&b, "PKrestart_point", z.PKRestartPoints)
		b.WriteString("\t</zone>\n")
	}
	b.WriteString("</list>\n")
	return b.Bytes()
}

// coords writes one <coords> line of the numbers v.
func coords(b *bytes.Buffer, v ...int) {
	b.WriteString("\t\t\t<coords loc=\"")
	for i, n := range v {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strconv.Itoa(n))
	}
	b.WriteString("\" />\n")
}

// restartPoints writes pts as one elem block (the element name is
// case-sensitive in the parser), or nothing when there are none.
func restartPoints(b *bytes.Buffer, elem string, pts [][3]int) {
	if len(pts) == 0 {
		return
	}
	fmt.Fprintf(b, "\t\t<%s>\n", elem)
	for _, p := range pts {
		coords(b, p[0], p[1], p[2])
	}
	fmt.Fprintf(b, "\t\t</%s>\n", elem)
}

// attr escapes s for a double-quoted attribute value.
func attr(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// Write writes files into dir and returns their paths.
func Write(dir string, files []File) ([]string, error) {
	paths := make([]string, 0, len(files))
	for _, f := range files {
		p := filepath.Join(dir, f.Name)
		if err := os.WriteFile(p, f.Data, 0o644); err != nil {
			return paths, err
		}
		paths = append(paths, p)
	}
	return paths, nil
}
