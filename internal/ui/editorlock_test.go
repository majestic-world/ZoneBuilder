package ui

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
)

// Real Gio input routing, not a mock of Update: an already focused editor
// can receive a late IME edit after play starts. Its visible buffer, caret
// and pending submissions must remain unchanged across lock and resume.
func TestLockedEditorsRejectInputAndDoNotReplayAfterResume(t *testing.T) {
	for _, target := range []string{"spawn count", "spawn XML name", "height", "shape range", "zone name", "zone search", "zone rename", "property name", "property value"} {
		t.Run(target, func(t *testing.T) {
			s := NewShell(NewTheme(), "", "22_22")
			var e *widget.Editor
			switch target {
			case "spawn count":
				e = &s.Spawn.fields[AreaCount].editor
			case "spawn XML name":
				e = &s.Spawn.XMLName
			case "height":
				e = &s.Height.Height
			case "shape range":
				e = &s.Edit.ZRange
			case "zone name":
				e = &s.Zone.Name
			case "zone search":
				e = &s.Zones.Search
			case "zone rename":
				e = &s.Zones.NewName
			case "property name":
				e = &s.Props.newName
			case "property value":
				e = &s.Props.newValue
			}
			e.SetText("50")
			var router input.Router
			var ops op.Ops
			gtx := layout.Context{Ops: &ops, Source: router.Source(), Constraints: layout.Exact(image.Pt(200, 60))}
			frame := func() {
				s.editorControls(s.field(e, "", nil))(gtx)
				router.Frame(&ops)
				ops.Reset()
			}
			frame()
			gtx.Execute(key.FocusCmd{Tag: e})
			frame()
			frame()
			start, end := e.Selection()
			s.EditorLocked = true
			router.Queue(
				pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(20, 20)},
				pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(20, 20)},
				key.Event{Name: "A", Modifiers: key.ModShortcut, State: key.Press},
				key.EditEvent{Range: key.Range{Start: 0, End: 2}, Text: "99"},
				key.Event{Name: key.NameReturn, State: key.Press},
			)
			s.DiscardEditorInput(gtx)
			s.Spawn.Discard(gtx)
			s.Zones.Discard(gtx)
			s.Props.Discard(gtx)
			frame()
			if e.Text() != "50" {
				t.Fatalf("locked field changed to %q", e.Text())
			}
			if a, b := e.Selection(); a != start || b != end {
				t.Fatalf("locked caret changed: %d:%d -> %d:%d", start, end, a, b)
			}
			// The Esc frame remains disabled, then the next frame resumes editing.
			frame()
			s.EditorLocked = false
			for {
				ev, ok := e.Update(gtx)
				if !ok {
					break
				}
				if _, submit := ev.(widget.SubmitEvent); submit {
					t.Fatal("locked submission replayed on resume")
				}
			}
			frame()
			if e.Text() != "50" {
				t.Fatalf("locked edit replayed after resume: %q", e.Text())
			}
			// A real edit after resume still works; the lock must not permanently
			// make the field read-only or restore text over legitimate input.
			gtx.Execute(key.FocusCmd{Tag: e})
			frame()
			router.Queue(key.EditEvent{Range: key.Range{Start: 0, End: 2}, Text: "51"})
			frame()
			if e.Text() != "51" {
				t.Fatalf("editing did not resume: %q", e.Text())
			}
		})
	}
}
