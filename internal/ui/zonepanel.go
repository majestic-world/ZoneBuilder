package ui

import (
	"slices"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"zonebuilder/internal/zone"
)

// ZonePanel holds the side panel's zone controls: the name and type of the
// next zone and the button that creates it and starts its polygon, then the
// XML output folder and the Compile button. It only collects input; the
// window loop turns the requests into zone.Document commands.
type ZonePanel struct {
	Name widget.Editor
	// TypeIndex is the chosen type in zone.Types; PrevType and NextType
	// step through the list.
	TypeIndex          int
	PrevType, NextType widget.Clickable
	Create             widget.Clickable
	// Output is the folder the XML is compiled into; BrowseOutput opens
	// the folder picker for it.
	Output       widget.Editor
	BrowseOutput widget.Clickable
	Compile      widget.Clickable
	// Info lines are shown under the controls (tool hints, the zones,
	// where the XML went).
	Info []string
}

func (p *ZonePanel) init(output string) {
	p.Name.SingleLine = true
	p.Output.SingleLine = true
	p.Output.SetText(output)
	p.TypeIndex = slices.Index(zone.Types, zone.PeaceZone)
}

// Type is the chosen zone type.
func (p *ZonePanel) Type() zone.Type { return zone.Types[p.TypeIndex] }

// CreateRequested reports a click on the create button since the last call;
// it also applies clicks on the type arrows.
func (p *ZonePanel) CreateRequested(gtx layout.Context) bool {
	n := len(zone.Types)
	for p.PrevType.Clicked(gtx) {
		p.TypeIndex = (p.TypeIndex + n - 1) % n
	}
	for p.NextType.Clicked(gtx) {
		p.TypeIndex = (p.TypeIndex + 1) % n
	}
	return p.Create.Clicked(gtx)
}

func (s *Shell) zonePanel() []layout.FlexChild {
	p := &s.Zone
	children := []layout.FlexChild{
		layout.Rigid(s.label("Nova zona: nome")),
		layout.Rigid(s.field(&p.Name, "[nome_da_zona]")),
		layout.Rigid(s.label("Tipo")),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(material.Button(s.Theme, &p.PrevType, "<").Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							lbl := material.Body1(s.Theme, string(p.Type()))
							lbl.Color = panelText
							return lbl.Layout(gtx)
						})
					}),
					layout.Rigid(material.Button(s.Theme, &p.NextType, ">").Layout),
				)
			})
		}),
		layout.Rigid(s.button(&p.Create, "Criar zona e desenhar polígono")),
		layout.Rigid(s.label("Pasta de saída do XML")),
		layout.Rigid(s.field(&p.Output, "pasta onde o XML é gravado")),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
				layout.Rigid(s.button(&p.BrowseOutput, "Procurar…")),
				layout.Rigid(s.button(&p.Compile, "Compilar")),
			)
		}),
	}
	for _, l := range p.Info {
		children = append(children, layout.Rigid(s.label(l)))
	}
	return children
}
