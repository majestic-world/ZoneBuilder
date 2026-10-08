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

// editTarget is the control an EditZone came from.
type editTarget struct {
	kind targetKind
	// known is the index into PropertiesPanel.known; name the free
	// parameter's name; value the text the edit tried.
	known       int
	name, value string
}

type targetKind int

const (
	typeTarget targetKind = iota
	knownTarget
	freeTarget
	addTarget
)

var (
	dimText   = color.NRGBA{R: 0x9A, G: 0x9E, B: 0xA6, A: 0xFF}
	errorText = color.NRGBA{R: 0xFF, G: 0x80, B: 0x70, A: 0xFF}
)

// PropertiesPanel edits the selected zone's type and parameters: the type
// out of the server's 23 values, every known ZoneTemplate parameter in a
// field of its kind showing the server's default while unset, and the free
// key/value parameters in document order.
type PropertiesPanel struct {
	shown bool
	zone  zone.ZoneID
	name  string
	typ   zone.Type

	prevType, nextType, listTypes widget.Clickable
	typeList                      bool
	typeItems                     []widget.Clickable

	known []knownField
	free  []*freeField

	newName, newValue widget.Editor
	addFree           widget.Clickable
	addErr            string
}

// knownField is the field of one known parameter. set and value are the
// zone's, as last loaded into the widgets.
type knownField struct {
	spec       zone.ParamSpec
	set        bool
	value      string
	editor     widget.Editor
	focused    bool
	prev, next widget.Clickable
	actions    []widget.Bool
	err        string
	// rejected is the text the last refused edit tried.
	rejected string
}

// freeField is the value field of one free parameter.
type freeField struct {
	name, value string
	editor      widget.Editor
	focused     bool
	remove      widget.Clickable
	err         string
}

func (p *PropertiesPanel) init() {
	p.known = make([]knownField, len(zone.KnownParams))
	p.typeItems = make([]widget.Clickable, len(zone.Types))
	for i, s := range zone.KnownParams {
		f := &p.known[i]
		f.spec = s
		f.editor.SingleLine, f.editor.Submit = true, true
		switch s.Kind {
		case zone.IntParam, zone.LongParam, zone.MessageParam:
			f.editor.Filter = "-+0123456789"
		case zone.DoubleParam:
			f.editor.Filter = "-+.0123456789eE"
		case zone.SkillParam:
			f.editor.Filter = "0123456789 ,;"
		case zone.ActionsParam:
			f.actions = make([]widget.Bool, len(s.Choices))
		}
	}
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
	case knownTarget:
		f := &p.known[t.known]
		f.err = ""
		if err != nil {
			f.err, f.rejected = "Recusado: "+kindText(f.spec), t.value
		}
	case freeTarget:
		for _, f := range p.free {
			if f.name == t.name {
				f.err = ""
				if err != nil {
					f.err = err.Error()
				}
			}
		}
	case addTarget:
		switch {
		case err == nil:
			p.addErr = ""
			p.newName.SetText("")
			p.newValue.SetText("")
		case isKnown(t.name):
			s, _ := zone.KnownParam(t.name)
			p.addErr = "Recusado: " + kindText(s)
		default:
			p.addErr = "Recusado: nome reservado pelo ZoneParser do servidor"
		}
	}
	if err != nil {
		return ""
	}
	return r.status
}

func isKnown(name string) bool {
	_, ok := zone.KnownParam(name)
	return ok
}

// load brings the fields up to date with z. A field whose zone value did
// not change since the last load keeps what is being typed in it, unless
// fresh (another zone) is set.
func (p *PropertiesPanel) load(z zone.Zone, fresh bool) {
	if fresh {
		p.free = nil
		p.typeList = false
		p.addErr = ""
		p.newName.SetText("")
		p.newValue.SetText("")
	}
	p.shown, p.zone, p.name, p.typ = true, z.ID, z.Name, z.Type
	for i := range p.known {
		f := &p.known[i]
		v, set := paramValue(z.Params, f.spec.Name)
		if !fresh && set == f.set && v == f.value {
			continue
		}
		f.set, f.value, f.err = set, v, ""
		f.editor.SetText(v)
		if f.actions != nil {
			names := listItems(v)
			for k, a := range f.spec.Choices {
				f.actions[k].Value = slices.Contains(names, a)
			}
		}
	}
	var free []*freeField
	for _, prm := range z.Params {
		if _, known := zone.KnownParam(prm.Name); known {
			continue
		}
		i := slices.IndexFunc(p.free, func(f *freeField) bool { return f.name == prm.Name })
		if i < 0 {
			f := &freeField{name: prm.Name, value: prm.Value}
			f.editor.SingleLine, f.editor.Submit = true, true
			f.editor.SetText(prm.Value)
			free = append(free, f)
			continue
		}
		f := p.free[i]
		if f.value != prm.Value {
			f.value, f.err = prm.Value, ""
			f.editor.SetText(prm.Value)
		}
		free = append(free, f)
	}
	p.free = free
}

func paramValue(params []zone.Param, name string) (string, bool) {
	i := slices.IndexFunc(params, func(p zone.Param) bool { return p.Name == name })
	if i < 0 {
		return "", false
	}
	return params[i].Value, true
}

// listItems splits a skill or action list value the way the server does.
func listItems(v string) []string {
	return strings.FieldsFunc(v, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
	})
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

	// set asks for name=value from target t, or for the parameter's
	// removal when value is nil.
	set := func(t editTarget, name string, value *string) {
		r := EditZone{
			Command: zone.RemoveParam{Zone: p.zone, Name: name},
			status:  fmt.Sprintf("%s: %s removido", p.name, name),
			target:  t,
		}
		if value != nil {
			r.Command = zone.SetParam{Zone: p.zone, Name: name, Value: *value}
			r.status = fmt.Sprintf("%s: %s = %s", p.name, name, *value)
			r.target.value = *value
		}
		reqs = append(reqs, r)
	}

	for i := range p.known {
		f := &p.known[i]
		s := f.spec
		t := editTarget{kind: knownTarget, known: i}
		switch {
		case s.Kind == zone.BoolParam || s.Kind == zone.ChoiceParam:
			// The options are the default (unset) and then each choice.
			at := 0
			if f.set {
				at = slices.Index(s.Choices, f.value) + 1
			}
			step := 0
			for f.prev.Clicked(gtx) {
				step--
			}
			for f.next.Clicked(gtx) {
				step++
			}
			if step == 0 {
				continue
			}
			n := len(s.Choices) + 1
			at = ((at+step)%n + n) % n
			if at == 0 {
				set(t, s.Name, nil)
			} else {
				set(t, s.Name, &s.Choices[at-1])
			}
		case s.Kind == zone.ActionsParam:
			changed := false
			for k := range f.actions {
				if f.actions[k].Update(gtx) {
					changed = true
				}
			}
			if !changed {
				continue
			}
			// Keep the order of the actions already listed; new ones go last.
			var names []string
			for _, a := range listItems(f.value) {
				if k := slices.Index(s.Choices, a); k >= 0 && f.actions[k].Value && !slices.Contains(names, a) {
					names = append(names, a)
				}
			}
			for k, a := range s.Choices {
				if f.actions[k].Value && !slices.Contains(names, a) {
					names = append(names, a)
				}
			}
			if len(names) == 0 {
				if f.set {
					set(t, s.Name, nil)
				}
				continue
			}
			v := strings.Join(names, ";")
			set(t, s.Name, &v)
		default:
			if !committed(gtx, &f.editor, &f.focused) {
				continue
			}
			v := f.editor.Text()
			if s.Kind != zone.TextParam {
				v = strings.TrimSpace(v)
			}
			switch {
			case v == "" && f.set:
				set(t, s.Name, nil)
			case v == "" || f.set && v == f.value:
				f.err = ""
			case f.err != "" && v == f.rejected:
				// Already refused; leaving the field does not retry it.
			default:
				set(t, s.Name, &v)
			}
		}
	}

	for _, f := range p.free {
		t := editTarget{kind: freeTarget, name: f.name}
		for f.remove.Clicked(gtx) {
			set(t, f.name, nil)
		}
		if committed(gtx, &f.editor, &f.focused) && f.editor.Text() != f.value {
			v := f.editor.Text()
			set(t, f.name, &v)
		}
	}

	add := p.addFree.Clicked(gtx)
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
		if name == "" {
			p.addErr = "Digite o nome do parâmetro"
		} else if _, dup := paramValue(p.params(), name); dup {
			p.addErr = name + " já está definido"
		} else {
			set(editTarget{kind: addTarget, name: name}, name, &v)
		}
	}
	return reqs
}

// params is every parameter the fields hold, known ones first.
func (p *PropertiesPanel) params() []zone.Param {
	var ps []zone.Param
	for _, f := range p.known {
		if f.set {
			ps = append(ps, zone.Param{Name: f.spec.Name, Value: f.value})
		}
	}
	for _, f := range p.free {
		ps = append(ps, zone.Param{Name: f.name, Value: f.value})
	}
	return ps
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

// kindText describes, in the panel's language, the values a known
// parameter takes.
func kindText(s zone.ParamSpec) string {
	switch s.Kind {
	case zone.BoolParam, zone.ChoiceParam:
		return strings.Join(s.Choices, " | ")
	case zone.IntParam:
		return "inteiro (32 bits)"
	case zone.LongParam:
		return "inteiro"
	case zone.DoubleParam:
		return "número decimal (ex.: -80 ou 1.5)"
	case zone.MessageParam:
		return "id de SystemMsg, ou -1 para nenhuma"
	case zone.SkillParam:
		return `skill "id;nível" (ex.: 4150;1)`
	case zone.ActionsParam:
		return "ações bloqueadas"
	}
	return "texto"
}

// defaultText is the hint an unset known parameter shows.
func defaultText(s zone.ParamSpec) string {
	if s.Default == "" {
		return "padrão: nenhum"
	}
	return "padrão: " + s.Default
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

	children = append(children, layout.Rigid(s.heading("Parâmetros conhecidos")))
	for i := range p.known {
		f := &p.known[i]
		sp := f.spec
		children = append(children, layout.Rigid(s.label(sp.Name+" ("+kindName(sp.Kind)+")")))
		switch sp.Kind {
		case zone.BoolParam, zone.ChoiceParam:
			text := "padrão (" + sp.Default + ")"
			if f.set {
				text = f.value
			}
			children = append(children, layout.Rigid(s.stepper(&f.prev, &f.next, nil, text, f.set)))
		case zone.ActionsParam:
			if !f.set {
				children = append(children, layout.Rigid(s.dimLabel(defaultText(sp))))
			}
			for k, a := range sp.Choices {
				children = append(children, layout.Rigid(s.checkBox(&f.actions[k], a)))
			}
		default:
			children = append(children, layout.Rigid(s.field(&f.editor, defaultText(sp))))
		}
		if f.err != "" {
			children = append(children, layout.Rigid(s.errorLabel(f.err)))
		}
	}

	children = append(children, layout.Rigid(s.heading("Parâmetros livres")))
	if len(p.free) == 0 {
		children = append(children, layout.Rigid(s.dimLabel("Nenhum")))
	}
	for _, f := range p.free {
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
		layout.Rigid(s.label("Novo parâmetro livre")),
		layout.Rigid(s.field(&p.newName, "nome (ex.: residence, playerMinLevel)")),
		layout.Rigid(s.field(&p.newValue, "valor")),
		layout.Rigid(s.button(&p.addFree, "Adicionar parâmetro")),
	)
	if p.addErr != "" {
		children = append(children, layout.Rigid(s.errorLabel(p.addErr)))
	}
	return children
}

// kindName is the short type label next to a known parameter's name.
func kindName(k zone.ParamKind) string {
	switch k {
	case zone.BoolParam:
		return "booleano"
	case zone.IntParam, zone.LongParam:
		return "inteiro"
	case zone.DoubleParam:
		return "decimal"
	case zone.ChoiceParam:
		return "lista"
	case zone.MessageParam:
		return "SystemMsg"
	case zone.SkillParam:
		return "skill id;nível"
	case zone.ActionsParam:
		return "ações"
	}
	return "texto"
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
