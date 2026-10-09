package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"zonebuilder/internal/project"
	"zonebuilder/internal/ui/icon"
)

// ProjectFile is the file type of the project dialogs.
var ProjectFile = FileType{Name: "Projeto do Zone Builder", Ext: project.Ext}

// ProjectPanel holds the project menu, opened from the project's name in
// the command bar (open, save, save as), and the recent maps in the map
// section. It only collects input; the window loop reads and writes the
// project file.
type ProjectPanel struct {
	OpenProject, Save, SaveAs widget.Clickable
	// Name is the project file's name ("sem nome" before it has one);
	// Unsaved marks changes since it was last saved or opened.
	Name    string
	Unsaved bool
	// RecentMaps are the recent tiles, most recent first; a click on one
	// is reported by RecentMapClicked.
	RecentMaps []string
	recent     []widget.Clickable

	menu     widget.Clickable
	menuOpen bool
	// menuAt is where the menu drops down, below the command bar.
	menuAt   image.Point
	dismiss  pointerSink
	sink     pointerSink
	menuSink pointerSink
}

// Requests reports the menu items clicked since the last call, and closes
// the menu after one.
func (p *ProjectPanel) Requests(gtx layout.Context) (open, save, saveAs bool) {
	open, save, saveAs = p.OpenProject.Clicked(gtx), p.Save.Clicked(gtx), p.SaveAs.Clicked(gtx)
	if open || save || saveAs {
		p.menuOpen = false
	}
	return open, save, saveAs
}

// RecentMapClicked returns the recent map clicked since the last call.
func (p *ProjectPanel) RecentMapClicked(gtx layout.Context) (string, bool) {
	for i := range p.recent {
		if p.recent[i].Clicked(gtx) && i < len(p.RecentMaps) {
			return p.RecentMaps[i], true
		}
	}
	return "", false
}

// commandBar is the card at the top: the project menu, undo and redo, the
// static mesh switch and Compilar XML.
func (s *Shell) commandBar(gtx layout.Context) layout.Dimensions {
	p := &s.Project
	if p.menu.Clicked(gtx) {
		p.menuOpen = !p.menuOpen
	}
	divider := func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Dp(1), gtx.Dp(20))
			paint.FillShape(gtx.Ops, hairline, clip.Rect{Max: size}.Op())
			return layout.Dimensions{Size: size}
		})
	}
	meshes := icon.Eye
	if s.Meshes.Hidden {
		meshes = icon.EyeOff
	}
	return card(gtx, &p.sink, layout.UniformInset(unit.Dp(6)), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(s.projectButton),
			layout.Rigid(divider),
			layout.Rigid(s.iconToggle(&s.Edit.Undo, icon.Undo2, false)),
			layout.Rigid(s.iconToggle(&s.Edit.Redo, icon.Redo2, false)),
			layout.Rigid(divider),
			layout.Rigid(s.toggleButton(&s.Meshes.button, meshes, "Static meshes", s.Meshes.Hidden)),
			layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
			layout.Rigid(s.button(&s.Zone.Compile, primaryButton, icon.CodeXML, "Compilar XML")),
		)
	})
}

// projectButton is the project's name, with a dot for unsaved changes,
// opening the project menu.
func (s *Shell) projectButton(gtx layout.Context) layout.Dimensions {
	p := &s.Project
	c := &p.menu
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg, _, _ := ghostButton.colors(c.Hovered() || p.menuOpen, c.Pressed())
		call, content := measure(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(10), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return icon.Folder.Layout(gtx, iconSize, dimText) }),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(220))
						return s.text(p.Name, bodySize, font.Medium, textColor, 1)(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !p.Unsaved {
							return layout.Dimensions{}
						}
						return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							n := gtx.Dp(6)
							paint.FillShape(gtx.Ops, lime, clip.Ellipse{Max: image.Pt(n, n)}.Op(gtx.Ops))
							return layout.Dimensions{Size: image.Pt(n, n)}
						})
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return icon.ChevronDown.Layout(gtx, 14, dimText) }),
				)
			})
		})
		size := image.Pt(content.X, gtx.Dp(controlHeight))
		fillRRect(gtx, size, controlRadius, bg, color.NRGBA{})
		place(gtx, image.Pt(0, (size.Y-content.Y)/2), call)
		pointerCursor(gtx, size)
		return layout.Dimensions{Size: size}
	})
}

// toggleButton is a ghost button with an icon and text that shows on as a
// violet tint.
func (s *Shell) toggleButton(c *widget.Clickable, ic *icon.Icon, txt string, on bool) layout.Widget {
	if !on {
		return s.button(c, ghostButton, ic, txt)
	}
	return func(gtx layout.Context) layout.Dimensions {
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			call, content := measure(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ic.Layout(gtx, iconSize, accentText) }),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Rigid(s.text(txt, bodySize, font.Medium, accentText, 1)),
					)
				})
			})
			size := image.Pt(content.X, gtx.Dp(controlHeight))
			fillRRect(gtx, size, controlRadius, accentSoft, color.NRGBA{})
			place(gtx, image.Pt(0, (size.Y-content.Y)/2), call)
			pointerCursor(gtx, size)
			return layout.Dimensions{Size: size}
		})
	}
}

// projectMenu drops the project menu down under the command bar while it
// is open; a press anywhere else closes it.
func (s *Shell) projectMenu(gtx layout.Context) {
	p := &s.Project
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &p.dismiss, Kinds: pointer.Press})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok && e.Kind == pointer.Press {
			p.menuOpen = false
		}
	}
	if !p.menuOpen {
		return
	}
	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, &p.dismiss)
	area.Pop()

	item := func(c *widget.Clickable, ic *icon.Icon, txt string) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := image.Pt(gtx.Dp(220), gtx.Dp(34))
				if c.Hovered() {
					fillRRect(gtx, size, controlRadius, controlHover, color.NRGBA{})
				}
				at(gtx, image.Pt(gtx.Dp(10), (size.Y-gtx.Dp(iconSize))/2), func(gtx layout.Context) layout.Dimensions {
					return ic.Layout(gtx, iconSize, dimText)
				})
				call, t := measure(gtx, s.text(txt, bodySize, font.Normal, textColor, 1))
				place(gtx, image.Pt(gtx.Dp(36), (size.Y-t.Y)/2), call)
				pointerCursor(gtx, size)
				return layout.Dimensions{Size: size}
			})
		})
	}
	at(gtx, p.menuAt, func(gtx layout.Context) layout.Dimensions {
		return card(gtx, &p.menuSink, layout.UniformInset(unit.Dp(6)), func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				item(&p.OpenProject, icon.FolderOpen, "Abrir projeto…"),
				item(&p.Save, icon.Save, "Salvar"),
				item(&p.SaveAs, icon.SaveAll, "Salvar como…"),
			)
		})
	})
}

// recentMaps are the recent map chips, wrapping onto as many rows as they
// need; the one in the tile field is violet.
func (s *Shell) recentMaps() []layout.FlexChild {
	p := &s.Project
	if len(p.RecentMaps) == 0 {
		return nil
	}
	for len(p.recent) < len(p.RecentMaps) {
		p.recent = append(p.recent, widget.Clickable{})
	}
	current := s.Tile.Text()
	return []layout.FlexChild{
		layout.Rigid(s.fieldLabel("Mapas recentes")),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gap := gtx.Dp(6)
			x, y, rowH := 0, 0, 0
			for i, name := range p.RecentMaps {
				call, size := measure(gtx, s.chip(&p.recent[i], name, name == current))
				if x > 0 && x+size.X > gtx.Constraints.Max.X {
					x, y = 0, y+rowH+gap
					rowH = 0
				}
				place(gtx, image.Pt(x, y), call)
				x += size.X + gap
				rowH = max(rowH, size.Y)
			}
			return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, y+rowH+gtx.Dp(12))}
		}),
	}
}
