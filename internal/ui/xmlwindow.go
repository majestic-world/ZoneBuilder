package ui

import (
	"image/color"
	"io"
	"strings"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/fonts"
	"zonebuilder/internal/ui/icon"
	"zonebuilder/internal/zonexml"
)

// codeSurface is the well the XML sits in.
var codeSurface = color.NRGBA{R: 0x0B, G: 0x0B, B: 0x0E, A: 0xFF}

// XMLWindow shows the last compilation in a window floating over the
// viewport: each file's suggested name, its XML in a read-only field the
// text can be selected from, a button that copies the whole file and one
// that copies only what lies inside its <list>.
type XMLWindow struct {
	Window FloatWindow
	files  []xmlFile
	copied string
	kind   XMLKind
}

type xmlFile struct {
	name   string
	text   string
	editor widget.Editor
	copy   widget.Clickable
	body   widget.Clickable
}

// XMLKind is what a compilation produced, which tells the server
// directory its files go to.
type XMLKind int

const (
	// ZoneXML files go to data/zone/.
	ZoneXML XMLKind = iota
	// SpawnXML files go to data/spawn/.
	SpawnXML
)

// Open shows files of kind in the window, opening it if it was closed.
func (x *XMLWindow) Open(files []zonexml.File, kind XMLKind) {
	x.kind = kind
	x.files = make([]xmlFile, len(files))
	for i, f := range files {
		x.files[i].name, x.files[i].text = f.Name, string(f.Data)
		x.files[i].editor.ReadOnly = true
		// The font draws no tab glyph: show the indentation as spaces.
		// Copiar copies text, tabs and all.
		x.files[i].editor.SetText(strings.ReplaceAll(x.files[i].text, "\t", "    "))
	}
	x.Window.Open(560, 520)
}

// Copied reports the file whose Copiar or Copiar conteúdo button was
// clicked since the last call, and whether it was Copiar conteúdo; the
// whole XML, or its listBody, is put on the clipboard.
func (x *XMLWindow) Copied(gtx layout.Context) (name string, body, ok bool) {
	for i := range x.files {
		f := &x.files[i]
		for f.copy.Clicked(gtx) {
			gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(f.text))})
			name, body, ok = f.name, false, true
		}
		for f.body.Clicked(gtx) {
			gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(listBody(f.text)))})
			name, body, ok = f.name, true, true
		}
	}
	return name, body, ok
}

// listBody is the lines of xml between the line that opens <list> and the
// one that closes it, as they are, indentation included: what goes into
// an existing file's list. Without a <list>, it is the whole of xml.
func listBody(xml string) string {
	open := strings.Index(xml, "<list>")
	end := strings.LastIndex(xml, "</list>")
	if open < 0 || end < open {
		return xml
	}
	start := strings.IndexByte(xml[open:], '\n')
	if start < 0 || open+start+1 > end {
		return ""
	}
	start += open + 1
	return xml[start : strings.LastIndexByte(xml[:end], '\n')+1]
}

// xmlWindow lays the window out over the viewport, while it holds files.
func (s *Shell) xmlWindow(gtx layout.Context) layout.Dimensions {
	x := &s.XML
	if len(x.files) == 0 {
		return layout.Dimensions{}
	}
	hint := locale.Text(s.Language, "ui.xml.hint")
	if x.kind == SpawnXML {
		hint = locale.Text(s.Language, "ui.xml.hint_spawn")
	}
	return x.Window.Layout(gtx, s, icon.CodeXML, locale.Text(s.Language, "ui.xml.title"), func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(s.dimLabel(hint)),
		}
		for i := range x.files {
			f := &x.files[i]
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return icon.FileCode.Layout(gtx, iconSize, dimText) }),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Flexed(1, s.text(f.name, bodySize, font.Medium, textColor, 1)),
							layout.Rigid(s.button(&f.copy, secondaryButton, icon.Copy, locale.Text(s.Language, "ui.xml.copy"))),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(s.button(&f.body, secondaryButton, icon.Copy, locale.Text(s.Language, "ui.xml.copy_body"))),
						)
					})
				}),
				layout.Rigid(s.code(&f.editor)),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// code is a read-only, selectable block of monospace text in a dark well.
func (s *Shell) code(e *widget.Editor) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			call, content := measure(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				ed := material.Editor(s.Theme, e, "")
				ed.Color = textColor
				ed.SelectionColor = accentSoft
				ed.Font = font.Font{Typeface: fonts.Mono}
				ed.TextSize = smallSize
				return layout.UniformInset(unit.Dp(12)).Layout(gtx, ed.Layout)
			})
			fillRRect(gtx, content, controlRadius, codeSurface, hairline)
			call.Add(gtx.Ops)
			return layout.Dimensions{Size: content}
		})
	}
}
