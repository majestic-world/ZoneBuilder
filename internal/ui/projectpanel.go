package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"zonebuilder/internal/project"
)

// ProjectFile is the file type of the project dialogs.
var ProjectFile = FileType{Name: "Projeto do Zone Builder", Ext: project.Ext}

// ProjectPanel holds the side panel's project controls (open, save, save
// as, with the project's name above them) and the recent maps under the
// tile field. It only collects input; the window loop reads and writes the
// project file.
type ProjectPanel struct {
	OpenProject, Save, SaveAs widget.Clickable
	// Title is the line above the buttons (the project file, whether it
	// has unsaved changes).
	Title string
	// RecentMaps are the recent tiles, most recent first; a click on one
	// is reported by RecentMapClicked.
	RecentMaps []string
	recent     []widget.Clickable
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

func (s *Shell) projectControls() []layout.FlexChild {
	p := &s.Project
	return []layout.FlexChild{
		layout.Rigid(s.label(p.Title)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
				layout.Rigid(s.button(&p.OpenProject, "Abrir…")),
				layout.Rigid(s.button(&p.Save, "Salvar")),
				layout.Rigid(s.button(&p.SaveAs, "Salvar como…")),
			)
		}),
	}
}

// recentMapsPerRow is how many recent map buttons share a row.
const recentMapsPerRow = 3

func (s *Shell) recentMaps() []layout.FlexChild {
	p := &s.Project
	if len(p.RecentMaps) == 0 {
		return nil
	}
	for len(p.recent) < len(p.RecentMaps) {
		p.recent = append(p.recent, widget.Clickable{})
	}
	children := []layout.FlexChild{layout.Rigid(s.label("Mapas recentes"))}
	for row := 0; row < len(p.RecentMaps); row += recentMapsPerRow {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				cells := make([]layout.FlexChild, recentMapsPerRow)
				for k := range cells {
					i := row + k
					cells[k] = layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if i >= len(p.RecentMaps) {
							return layout.Dimensions{}
						}
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							b := material.Button(s.Theme, &p.recent[i], p.RecentMaps[i])
							b.TextSize = unit.Sp(12)
							b.Inset = layout.UniformInset(unit.Dp(6))
							return b.Layout(gtx)
						})
					})
				}
				return layout.Flex{}.Layout(gtx, cells...)
			})
		}))
	}
	return children
}
