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

	"zonebuilder/internal/ui/fonts"
	"zonebuilder/internal/ui/icon"
	"zonebuilder/internal/zonexml"
)

// codeSurface is the well the XML sits in.
var codeSurface = color.NRGBA{R: 0x0B, G: 0x0B, B: 0x0E, A: 0xFF}

// XMLWindow shows the last compilation in a window floating over the
// viewport: each file's suggested name, its XML in a read-only field the
// text can be selected from, and a button that copies the whole file.
type XMLWindow struct {
	Window FloatWindow
	files  []xmlFile
	copied string
}

type xmlFile struct {
	name   string
	text   string
	editor widget.Editor
	copy   widget.Clickable
}

// Open shows files in the window, opening it if it was closed.
func (x *XMLWindow) Open(files []zonexml.File) {
	x.files = make([]xmlFile, len(files))
	for i, f := range files {
		x.files[i].name, x.files[i].text = f.Name, string(f.Data)
		x.files[i].editor.ReadOnly = true
		// The font draws no tab glyph: show the indentation as spaces.
		// Copiar copies text, tabs and all.
		x.files[i].editor.SetText(strings.ReplaceAll(x.files[i].text, "\t", "    "))
	}
	if x.Window.Width == 0 {
		x.Window.Width, x.Window.Height, x.Window.Left, x.Window.Top = 560, 520, 420, 84
	}
	x.Window.Closed, x.Window.Collapsed = false, false
}

// Copied reports the file whose Copiar button was clicked since the last
// call; its XML is put on the clipboard.
func (x *XMLWindow) Copied(gtx layout.Context) (string, bool) {
	name, ok := "", false
	for i := range x.files {
		f := &x.files[i]
		for f.copy.Clicked(gtx) {
			gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(f.text))})
			name, ok = f.name, true
		}
	}
	return name, ok
}

// xmlWindow lays the window out over the viewport, while it holds files.
func (s *Shell) xmlWindow(gtx layout.Context) layout.Dimensions {
	x := &s.XML
	if len(x.files) == 0 {
		return layout.Dimensions{}
	}
	return x.Window.Layout(gtx, s, icon.CodeXML, "XML compilado", func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(s.dimLabel("Cole cada arquivo em data/zone/ do servidor, com o nome indicado.")),
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
							layout.Rigid(s.button(&f.copy, secondaryButton, icon.Copy, "Copiar")),
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
