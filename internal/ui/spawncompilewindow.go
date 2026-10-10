package ui

import (
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/icon"
)

// NPCIDsWindow asks for the NPC IDs used by one spawn compilation. The
// IDs belong to the compilation, not to the area's distribution inputs.
type NPCIDsWindow struct {
	Window          FloatWindow
	IDs             widget.Editor
	Invalid         bool
	compile, cancel widget.Clickable
	focus           bool
}

// Open asks again, retaining the last entered list for convenience.
func (p *NPCIDsWindow) Open() {
	p.IDs.SingleLine, p.IDs.Submit = true, true
	if p.Window.Width == 0 {
		p.Window.Width, p.Window.Height, p.Window.Left, p.Window.Top = 440, 260, 280, 160
	}
	p.Window.Closed, p.Window.Collapsed = false, false
	p.Invalid, p.focus = false, true
}

// Requested reports Compile or Enter; cancellation never submits a list.
// The caller keeps the window open on errors and closes it after success.
func (p *NPCIDsWindow) Requested(gtx layout.Context) (string, bool) {
	if p.Window.Closed {
		p.Discard(gtx)
		return "", false
	}
	if p.cancel.Clicked(gtx) {
		p.Window.Closed = true
		p.Discard(gtx)
		return "", false
	}
	if requested(gtx, &p.IDs, &p.compile) {
		return p.IDs.Text(), true
	}
	return "", false
}

// Discard rejects input without changing the text or replaying it later.
func (p *NPCIDsWindow) Discard(gtx layout.Context) {
	discardClicks(gtx, &p.compile, &p.cancel)
	discardEditorEvents(gtx, &p.IDs)
	p.focus = false
}

func (s *Shell) npcIDsWindow(gtx layout.Context) layout.Dimensions {
	p := &s.NPCIDs
	return p.Window.Layout(gtx, s, icon.Users, locale.Text(s.Language, "spawn.compile.ids.title"), func(gtx layout.Context) layout.Dimensions {
		if p.focus {
			gtx.Execute(key.FocusCmd{Tag: &p.IDs})
			p.focus = false
		}
		children := []layout.FlexChild{
			layout.Rigid(s.dimLabel(locale.Text(s.Language, "spawn.compile.ids.hint"))),
			layout.Rigid(s.fieldLabel(locale.Text(s.Language, "spawn.compile.ids.label"))),
			layout.Rigid(s.field(&p.IDs, "1 2 3 4", nil)),
		}
		if p.Invalid {
			children = append(children, layout.Rigid(s.dimLabel(locale.Text(s.Language, "spawn.compile.ids.invalid"))))
		}
		children = append(children, layout.Rigid(s.spaced(buttonRow(
			s.button(&p.compile, primaryButton, icon.CodeXML, locale.Text(s.Language, "spawn.compile.ids.submit")),
			s.button(&p.cancel, secondaryButton, nil, locale.Text(s.Language, "spawn.compile.ids.cancel")),
		))))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}
