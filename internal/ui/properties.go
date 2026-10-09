package ui

import (
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/icon"
	"zonebuilder/internal/zone"
)

// EditZone is a properties panel request: apply Command to the zone the
// panel shows. The window loop applies it and hands the outcome back to
// PropertiesPanel.Applied; the panel never changes a zone itself.
type EditZone struct {
	Command zone.Command
	// status is the status line on success; target is what the outcome
	// lands on.
	status locale.Message
	target editTarget
}

// editTarget is the control an EditZone came from: name is the
// parameter's name.
type editTarget struct {
	kind targetKind
	name string
}

type targetKind int

const (
	typeTarget targetKind = iota
	paramTarget
	addTarget
)

// PropertiesPanel edits the selected zone's type, out of the server's 23
// values, and its <set> parameters as key/value pairs in document order,
// for the ones the server needs (residence, distribution_id, ...).
type PropertiesPanel struct {
	shown bool
	zone  zone.ZoneID
	name  string
	typ   zone.Type

	prevType, nextType, listTypes widget.Clickable
	typeList                      bool
	typeItems                     []widget.Clickable

	params []*paramField

	newName, newValue widget.Editor
	add               widget.Clickable
	addErr            locale.Message
}

// paramField is the value field of one parameter; value is the zone's, as
// last loaded into the editor.
type paramField struct {
	name, value string
	editor      widget.Editor
	focused     bool
	remove      widget.Clickable
	err         locale.Message
}

func (p *PropertiesPanel) init() {
	p.typeItems = make([]widget.Clickable, len(zone.Types))
	p.newName.SingleLine, p.newName.Submit = true, true
	p.newValue.SingleLine, p.newValue.Submit = true, true
}

// Update loads z, the selected zone (ok false: none), into the fields and
// returns the edits the panel's input asks for. Call it once per frame
// before Layout, and hand each request's outcome to Applied.
func (p *PropertiesPanel) Update(gtx layout.Context, z zone.Zone, ok bool) []EditZone {
	if !ok {
		p.shown = false
		return nil
	}
	p.load(z, !p.shown || z.ID != p.zone)
	return p.input(gtx)
}

// Applied takes the outcome err of applying r: it sets the error of the
// control r came from, clears the new parameter fields once one is added,
// and returns the status line ("" to keep the current one).
func (p *PropertiesPanel) Applied(r EditZone, err error) locale.Message {
	t := r.target
	switch t.kind {
	case typeTarget:
		if err != nil {
			return locale.Message{Key: "ui.properties.type_error", Args: map[string]string{"detail": err.Error()}}
		}
	case paramTarget:
		for _, f := range p.params {
			if f.name == t.name {
				f.err = locale.Message{}
				if err != nil {
					f.err = locale.Message{Key: "ui.properties.param_error", Args: map[string]string{"detail": err.Error()}}
				}
			}
		}
	case addTarget:
		p.addErr = locale.Message{}
		if err != nil {
			p.addErr = locale.Message{Key: "ui.properties.param_error", Args: map[string]string{"detail": err.Error()}}
			break
		}
		p.newName.SetText("")
		p.newValue.SetText("")
	}
	if err != nil {
		return locale.Message{}
	}
	return r.status
}

// load brings the fields up to date with z. A field whose zone value did
// not change since the last load keeps what is being typed in it, unless
// fresh (another zone) is set.
func (p *PropertiesPanel) load(z zone.Zone, fresh bool) {
	if fresh {
		p.params = nil
		p.typeList = false
		p.addErr = locale.Message{}
		p.newName.SetText("")
		p.newValue.SetText("")
	}
	p.shown, p.zone, p.name, p.typ = true, z.ID, z.Name, z.Type
	var params []*paramField
	for _, prm := range z.Params {
		i := slices.IndexFunc(p.params, func(f *paramField) bool { return f.name == prm.Name })
		if i < 0 {
			f := &paramField{name: prm.Name, value: prm.Value}
			f.editor.SingleLine, f.editor.Submit = true, true
			f.editor.SetText(prm.Value)
			params = append(params, f)
			continue
		}
		f := p.params[i]
		if f.value != prm.Value {
			f.value, f.err = prm.Value, locale.Message{}
			f.editor.SetText(prm.Value)
		}
		params = append(params, f)
	}
	p.params = params
}

// input turns the clicks and edits since the last frame into requests.
func (p *PropertiesPanel) input(gtx layout.Context) []EditZone {
	var reqs []EditZone
	setType := func(t zone.Type) {
		reqs = append(reqs, EditZone{
			Command: zone.SetType{Zone: p.zone, Type: t},
			status:  locale.Message{Key: "ui.properties.type_changed", Args: map[string]string{"name": p.name, "type": string(t)}},
		})
	}
	n := len(zone.Types)
	cur := slices.Index(zone.Types, p.typ)
	for p.prevType.Clicked(gtx) {
		setType(zone.Types[(cur+n-1)%n])
	}
	for p.nextType.Clicked(gtx) {
		setType(zone.Types[(cur+1)%n])
	}
	for p.listTypes.Clicked(gtx) {
		p.typeList = !p.typeList
	}
	for i := range p.typeItems {
		for p.typeItems[i].Clicked(gtx) {
			setType(zone.Types[i])
			p.typeList = false
		}
	}

	for _, f := range p.params {
		t := editTarget{kind: paramTarget, name: f.name}
		for f.remove.Clicked(gtx) {
			reqs = append(reqs, EditZone{
				Command: zone.RemoveParam{Zone: p.zone, Name: f.name},
				status:  locale.Message{Key: "ui.properties.param_removed", Args: map[string]string{"name": p.name, "param": f.name}},
				target:  t,
			})
		}
		if committed(gtx, &f.editor, &f.focused) && f.editor.Text() != f.value {
			v := f.editor.Text()
			reqs = append(reqs, EditZone{
				Command: zone.SetParam{Zone: p.zone, Name: f.name, Value: v},
				status:  locale.Message{Key: "ui.properties.param_set", Args: map[string]string{"name": p.name, "param": f.name, "value": v}},
				target:  t,
			})
		}
	}

	add := p.add.Clicked(gtx)
	for _, e := range []*widget.Editor{&p.newName, &p.newValue} {
		for {
			ev, ok := e.Update(gtx)
			if !ok {
				break
			}
			if _, ok := ev.(widget.SubmitEvent); ok {
				add = true
			}
		}
	}
	if add {
		name, v := strings.TrimSpace(p.newName.Text()), p.newValue.Text()
		switch {
		case name == "":
			p.addErr = locale.Message{Key: "ui.properties.required"}
		case slices.ContainsFunc(p.params, func(f *paramField) bool { return f.name == name }):
			p.addErr = locale.Message{Key: "ui.properties.duplicate", Args: map[string]string{"name": name}}
		default:
			reqs = append(reqs, EditZone{
				Command: zone.SetParam{Zone: p.zone, Name: name, Value: v},
				status:  locale.Message{Key: "ui.properties.param_set", Args: map[string]string{"name": p.name, "param": name, "value": v}},
				target:  editTarget{kind: addTarget, name: name},
			})
		}
	}
	return reqs
}

// committed drains e's events and reports whether its text is to be
// applied: Enter was pressed in it, or it lost the focus.
func committed(gtx layout.Context, e *widget.Editor, focused *bool) bool {
	commit := false
	for {
		ev, ok := e.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			commit = true
		}
	}
	now := gtx.Source.Focused(e)
	if *focused && !now {
		commit = true
	}
	*focused = now
	return commit
}

// propertiesPanel is the selected zone's type, with the whole list
// unfolding under it, and its parameters, each removable, then the fields
// that add one.
func (s *Shell) propertiesPanel() []layout.FlexChild {
	p := &s.Props
	if !p.shown {
		return nil
	}
	children := []layout.FlexChild{
		layout.Rigid(s.fieldLabel(locale.Text(s.Language, "ui.zone.type"))),
		layout.Rigid(s.stepper(&p.prevType, &p.nextType, &p.listTypes, string(p.typ))),
	}
	if p.typeList {
		for i, t := range zone.Types {
			children = append(children, layout.Rigid(s.listItem(&p.typeItems[i], string(t), t == p.typ)))
		}
		children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout))
	}

	children = append(children,
		layout.Rigid(s.fieldLabel(locale.Text(s.Language, "ui.properties.parameters"))),
		layout.Rigid(s.dimLabel(locale.Text(s.Language, "ui.properties.description"))),
	)
	for _, f := range p.params {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, s.text(f.name, smallSize, font.Medium, textColor, 1)),
						layout.Rigid(s.iconToggle(&f.remove, icon.Trash2, false)),
					)
				})
			}),
			layout.Rigid(s.field(&f.editor, locale.Text(s.Language, "ui.properties.value"), nil)),
		)
		if f.err.Key != "" {
			children = append(children, layout.Rigid(s.errorLabel(f.err.Render(s.Language))))
		}
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Flexed(1, s.field(&p.newName, locale.Text(s.Language, "ui.properties.name_hint"), nil)),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Flexed(1, s.field(&p.newValue, locale.Text(s.Language, "ui.properties.value"), nil)),
			)
		}),
		layout.Rigid(s.spaced(s.fullButton(&p.add, secondaryButton, icon.Plus, locale.Text(s.Language, "ui.properties.add")))),
	)
	if p.addErr.Key != "" {
		children = append(children, layout.Rigid(s.errorLabel(p.addErr.Render(s.Language))))
	}
	return children
}
