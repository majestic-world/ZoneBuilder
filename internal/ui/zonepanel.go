package ui

import (
	"slices"

	"gioui.org/layout"
	"gioui.org/widget"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/icon"
	"zonebuilder/internal/zone"
)

// ZonePanel holds the zone controls: in the inspector, the name and type
// of the next zone and the button that creates it and arms the chosen
// shape tool; and the tool dock. It only collects input; the window loop
// turns the requests into zone.Document commands.
type ZonePanel struct {
	Name widget.Editor
	// TypeIndex is the chosen type in zone.Types; PrevType and NextType
	// step through the list.
	TypeIndex          int
	PrevType, NextType widget.Clickable
	Create             widget.Clickable
	// Tools are the viewport tool buttons.
	Tools ToolPanel
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
	return []layout.FlexChild{
		layout.Rigid(s.section(false, icon.Plus, locale.Text(s.Language, "ui.zone.new"), "")),
		layout.Rigid(s.fieldLabel(locale.Text(s.Language, "ui.zone.name"))),
		layout.Rigid(s.field(&p.Name, locale.Text(s.Language, "ui.zone.name_hint"), nil)),
		layout.Rigid(s.fieldLabel(locale.Text(s.Language, "ui.zone.type"))),
		layout.Rigid(s.stepper(&p.PrevType, &p.NextType, nil, string(p.Type()))),
		layout.Rigid(s.fullButton(&p.Create, limeButton, icon.Pencil, locale.Text(s.Language, "ui.zone.create"))),
	}
}
