package ui

import (
	"slices"

	"gioui.org/layout"
	"gioui.org/widget"

	"zonebuilder/internal/zone"
)

// ZonePanel holds the side panel's zone controls: the name and type of the
// next zone and the button that creates it and arms the chosen shape tool,
// the tool buttons, then the Compile button, which opens the compiled XML
// in the XML window. It only collects input; the window loop turns the
// requests into zone.Document commands.
type ZonePanel struct {
	Name widget.Editor
	// TypeIndex is the chosen type in zone.Types; PrevType and NextType
	// step through the list.
	TypeIndex          int
	PrevType, NextType widget.Clickable
	Create             widget.Clickable
	// Tools are the viewport tool buttons.
	Tools   ToolPanel
	Compile widget.Clickable
	// Info lines are shown under the controls: the armed tool's hint.
	Info []string
}

func (p *ZonePanel) init() {
	p.Name.SingleLine = true
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
		layout.Rigid(s.stepper(&p.PrevType, &p.NextType, nil, string(p.Type()), true)),
		layout.Rigid(s.button(&p.Create, "Criar zona e desenhar")),
	}
	children = append(children, s.toolPanel()...)
	children = append(children, layout.Rigid(s.button(&p.Compile, "Compilar XML")))
	for _, l := range p.Info {
		children = append(children, layout.Rigid(s.label(l)))
	}
	return children
}
