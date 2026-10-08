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
	Type     string
	Polygons []Polygon
}

// Polygon is one <polygon>: its vertices (x, y) and the Z range every
// coords carries.
type Polygon struct {
	Points     [][2]int
	ZMin, ZMax int
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
		for _, p := range z.Polygons {
			b.WriteString("\t\t<polygon>\n")
			for _, pt := range p.Points {
				b.WriteString("\t\t\t<coords loc=\"")
				b.WriteString(strconv.Itoa(pt[0]))
				b.WriteByte(' ')
				b.WriteString(strconv.Itoa(pt[1]))
				b.WriteByte(' ')
				b.WriteString(strconv.Itoa(p.ZMin))
				b.WriteByte(' ')
				b.WriteString(strconv.Itoa(p.ZMax))
				b.WriteString("\" />\n")
			}
			b.WriteString("\t\t</polygon>\n")
		}
		b.WriteString("\t</zone>\n")
	}
	b.WriteString("</list>\n")
	return b.Bytes()
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
