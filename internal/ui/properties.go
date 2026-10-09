package ui

import (
	"fmt"
	"image/color"
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"zonebuilder/internal/zone"
)

// EditZone is a properties panel request: apply Command to the zone the
// panel shows. The window loop applies it and hands the outcome back to
// PropertiesPanel.Applied; the panel never changes a zone itself.
type EditZone struct {
	Command zone.Command
	// status is the status line on success; target is what the outcome
	// lands on.
	status string
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

var (
	dimText   = color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA6, A: 0xFF}
	errorText = color.NRGBA{R: 0xFF, G: 0x80, B: 0x70, A: 0xFF}
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
	addErr            string
}

// paramField is the value field of one parameter; value is the zone's, as
// last loaded into the editor.
type paramField struct {
	name, value string
	editor      widget.Editor
	focused     bool
	remove      widget.Clickable
	err         string
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
func (p *PropertiesPanel) Applied(r EditZone, err error) string {
	t := r.target
	switch t.kind {
	case typeTarget:
		if err != nil {
			return err.Error()
		}
	case paramTarget:
		for _, f := range p.params {
			if f.name == t.name {
				f.err = ""
				if err != nil {
					f.err = err.Error()
				}
			}
		}
	case addTarget:
		p.addErr = ""
		if err != nil {
			p.addErr = err.Error()
			break
		}
		p.newName.SetText("")
		p.newValue.SetText("")
	}
	if err != nil {
		return ""
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
		p.addErr = ""
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
			f.value, f.err = prm.Value, ""
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
			status:  fmt.Sprintf("%s: tipo %s", p.name, t),
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
				status:  fmt.Sprintf("%s: %s removido", p.name, f.name),
				target:  t,
			})
		}
		if committed(gtx, &f.editor, &f.focused) && f.editor.Text() != f.value {
			v := f.editor.Text()
			reqs = append(reqs, EditZone{
				Command: zone.SetParam{Zone: p.zone, Name: f.name, Value: v},
				status:  fmt.Sprintf("%s: %s = %s", p.name, f.name, v),
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
			p.addErr = "Digite o nome do parâmetro"
		case slices.ContainsFunc(p.params, func(f *paramField) bool { return f.name == name }):
			p.addErr = name + " já está definido"
		default:
			reqs = append(reqs, EditZone{
				Command: zone.SetParam{Zone: p.zone, Name: name, Value: v},
				status:  fmt.Sprintf("%s: %s = %s", p.name, name, v),
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

func (s *Shell) propertiesPanel() []layout.FlexChild {
	p := &s.Props
	children := []layout.FlexChild{layout.Rigid(s.heading("Propriedades da zona"))}
	if !p.shown {
		return append(children, layout.Rigid(s.label("Nenhuma zona selecionada")))
	}
	children = append(children,
		layout.Rigid(s.label("Zona "+p.name)),
		layout.Rigid(s.label("Tipo")),
		layout.Rigid(s.stepper(&p.prevType, &p.nextType, &p.listTypes, string(p.typ), true)),
	)
	if p.typeList {
		for i, t := range zone.Types {
			children = append(children, layout.Rigid(s.listItem(&p.typeItems[i], string(t), t == p.typ)))
		}
	}

	children = append(children,
		layout.Rigid(s.heading("Parâmetros")),
		layout.Rigid(s.dimLabel("Só os que o servidor exige: residence (SIEGE, HEADQUARTER), distribution_id e fishing_place_type (FISHING)")),
	)
	for _, f := range p.params {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, s.label(f.name)),
					layout.Rigid(s.button(&f.remove, "Remover")),
				)
			}),
			layout.Rigid(s.field(&f.editor, "valor")),
		)
		if f.err != "" {
			children = append(children, layout.Rigid(s.errorLabel(f.err)))
		}
	}
	children = append(children,
		layout.Rigid(s.field(&p.newName, "nome (ex.: residence)")),
		layout.Rigid(s.field(&p.newValue, "valor")),
		layout.Rigid(s.button(&p.add, "Adicionar parâmetro")),
	)
	if p.addErr != "" {
		children = append(children, layout.Rigid(s.errorLabel(p.addErr)))
	}
	return children
}

// stepper is "< text >": prev and next step through a closed list. With
// toggle set, clicking text clicks it. Unset values (bright false) are
// dimmed.
func (s *Shell) stepper(prev, next, toggle *widget.Clickable, text string, bright bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(material.Button(s.Theme, prev, "<").Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body1(s.Theme, text)
					lbl.Color = panelText
					if !bright {
						lbl.Color = dimText
					}
					w := func(gtx layout.Context) layout.Dimensions { return layout.Center.Layout(gtx, lbl.Layout) }
					if toggle == nil {
						return w(gtx)
					}
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return toggle.Layout(gtx, w)
				}),
				layout.Rigid(material.Button(s.Theme, next, ">").Layout),
			)
		})
	}
}

// listItem is one clickable row of a closed list; the current one is
// highlighted.
func (s *Shell) listItem(c *widget.Clickable, text string, current bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(s.Theme, text)
				lbl.Color = dimText
				if current {
					lbl.Color = s.Theme.ContrastBg
				}
				return lbl.Layout(gtx)
			})
		})
	}
}

func (s *Shell) checkBox(b *widget.Bool, text string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		cb := material.CheckBox(s.Theme, b, text)
		cb.Color, cb.IconColor = panelText, panelText
		cb.Size = unit.Dp(20)
		return cb.Layout(gtx)
	}
}

func (s *Shell) heading(text string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lbl := material.Subtitle1(s.Theme, text)
		lbl.Color = panelText
		lbl.Font.Weight = font.Bold
		return layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(6)}.Layout(gtx, lbl.Layout)
	}
}

func (s *Shell) dimLabel(text string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lbl := material.Body2(s.Theme, text)
		lbl.Color = dimText
		return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, lbl.Layout)
	}
}

func (s *Shell) errorLabel(text string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lbl := material.Body2(s.Theme, text)
		lbl.Color = errorText
		return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, lbl.Layout)
	}
}
