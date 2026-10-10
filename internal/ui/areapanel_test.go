package ui

import (
	"image"
	"strconv"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"

	"zonebuilder/internal/spawn"
)

type areaGenerationObservation struct {
	params     spawn.Params
	regenerate bool
}

// The router delivers real editor and button events to AreaPanel.Update.
// Field requests are applied to a real document in their returned order;
// generation observes that document at the instant its request is reached.
type areaPanelInput struct {
	t           *testing.T
	shell       *Shell
	doc         *spawn.Document
	area        spawn.AreaID
	router      input.Router
	ops         op.Ops
	generations []areaGenerationObservation
}

func newAreaPanelInput(t *testing.T) *areaPanelInput {
	t.Helper()
	h := &areaPanelInput{t: t, shell: NewShell(NewTheme(), "", "22_22"), doc: spawn.NewDocument()}
	h.area = h.doc.NewAreaID()
	if err := h.doc.Apply(spawn.CreateArea{ID: h.area, Name: "area", Params: spawn.Params{Count: 50, Radius: 20, Clearance: 32}}); err != nil {
		t.Fatal(err)
	}
	h.frame(false)
	return h
}

func (h *areaPanelInput) context() layout.Context {
	return layout.Context{Ops: &h.ops, Source: h.router.Source(), Constraints: layout.Exact(image.Pt(500, 400))}
}

func (h *areaPanelInput) frame(discard bool) []any {
	h.t.Helper()
	gtx := h.context()
	p := &h.shell.Spawn
	var requests []any
	if discard {
		p.Discard(gtx)
	} else {
		a, ok := h.doc.Area(h.area)
		requests = p.Update(gtx, a, ok)
		for _, request := range requests {
			switch r := request.(type) {
			case SetAreaField:
				a, _ := h.doc.Area(r.Area)
				if r.Field == AreaName {
					if err := h.doc.Apply(spawn.Rename{Area: r.Area, Name: strings.TrimSpace(r.Text)}); err != nil {
						h.t.Fatal(err)
					}
					continue
				}
				n, err := strconv.Atoi(strings.TrimSpace(r.Text))
				if err != nil {
					// Integer errors and generation veto belong to the cmd
					// consumer, not this panel's request-order regression.
					continue
				}
				params := a.Params
				switch r.Field {
				case AreaCount:
					params.Count = n
				case AreaRadius:
					params.Radius = n
				case AreaClearance:
					params.Clearance = n
				}
				if err := h.doc.Apply(spawn.SetParams{Area: r.Area, Params: params}); err != nil {
					h.t.Fatal(err)
				}
			case GenerateArea:
				a, _ := h.doc.Area(r.Area)
				h.generations = append(h.generations, areaGenerationObservation{params: a.Params, regenerate: r.Regenerate})
			}
		}
	}
	if discard {
		gtx = gtx.Disabled()
	}
	for i := range p.fields {
		fieldContext := gtx
		fieldContext.Constraints = layout.Exact(image.Pt(220, 48))
		offset := op.Offset(image.Pt(0, i*60)).Push(gtx.Ops)
		h.shell.field(&p.fields[i].editor, "", nil)(fieldContext)
		offset.Pop()
	}
	for i, button := range []layout.Widget{
		h.shell.button(&p.Generate, primaryButton, nil, "Gerar"),
		h.shell.button(&p.Regenerate, secondaryButton, nil, "Regerar"),
	} {
		buttonContext := gtx
		buttonContext.Constraints = layout.Exact(image.Pt(120, 40))
		offset := op.Offset(image.Pt(i*140, 300)).Push(gtx.Ops)
		button(buttonContext)
		offset.Pop()
	}
	h.router.Frame(&h.ops)
	h.ops.Reset()
	return requests
}

func (h *areaPanelInput) edit(field AreaField, text string) {
	h.t.Helper()
	e := &h.shell.Spawn.fields[field].editor
	gtx := h.context()
	gtx.Execute(key.FocusCmd{Tag: e})
	h.frame(false)
	h.frame(false)
	h.router.Queue(key.EditEvent{Range: key.Range{Start: 0, End: len([]rune(e.Text()))}, Text: text})
	if requests := h.frame(false); len(requests) != 0 {
		h.t.Fatalf("typing committed before an action: %#v", requests)
	}
	source := h.router.Source()
	if !source.Focused(e) || e.Text() != text {
		h.t.Fatalf("focused edit not routed: text = %q, focused = %v", e.Text(), source.Focused(e))
	}
	start, end := e.Selection()
	h.frame(false)
	if a, b := e.Selection(); e.Text() != text || a != start || b != end {
		h.t.Fatalf("pending text or caret changed: %q, %d:%d -> %d:%d", e.Text(), start, end, a, b)
	}
}

func (h *areaPanelInput) clickGeneration(regenerate bool) {
	x := float32(20)
	if regenerate {
		x += 140
	}
	h.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x, 320)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(x, 320)},
	)
}

func TestAreaPanelGenerationAppliesFocusedEditsFirst(t *testing.T) {
	for _, regenerate := range []bool{false, true} {
		for _, field := range []AreaField{AreaCount, AreaRadius, AreaClearance} {
			t.Run(strconv.Itoa(int(field))+"/regenerate="+strconv.FormatBool(regenerate), func(t *testing.T) {
				h := newAreaPanelInput(t)
				h.edit(field, "80")
				h.clickGeneration(regenerate)
				h.frame(false)
				if len(h.generations) != 1 {
					t.Fatalf("generation clicks routed = %d, want 1", len(h.generations))
				}
				want := spawn.Params{Count: 50, Radius: 20, Clearance: 32}
				switch field {
				case AreaCount:
					want.Count = 80
				case AreaRadius:
					want.Radius = 80
				case AreaClearance:
					want.Clearance = 80
				}
				got := h.generations[0]
				if got.params != want || got.regenerate != regenerate {
					t.Fatalf("generation observed %+v, want params %+v and regenerate %v", got, want, regenerate)
				}
			})
		}
	}
}

func TestAreaPanelGenerationDrainsSameFrameEdit(t *testing.T) {
	h := newAreaPanelInput(t)
	h.edit(AreaCount, "80")
	h.router.Queue(key.EditEvent{Range: key.Range{Start: 0, End: 2}, Text: "90"})
	h.clickGeneration(false)
	h.frame(false)
	if len(h.generations) != 1 || h.generations[0].params.Count != 90 {
		t.Fatalf("generation did not consume the latest routed edit: %+v", h.generations)
	}
}

func TestAreaPanelGenerationReportsInvalidPendingIntegerFirst(t *testing.T) {
	for _, regenerate := range []bool{false, true} {
		t.Run(strconv.FormatBool(regenerate), func(t *testing.T) {
			h := newAreaPanelInput(t)
			h.edit(AreaCount, "invalid")
			// Repeat the action: an invalid document edit must not make
			// the still-pending buffer disappear on the next attempt.
			for attempt := range 2 {
				h.clickGeneration(regenerate)
				requests := h.frame(false)
				if len(requests) != 2 {
					t.Fatalf("attempt %d: requests = %#v, want pending edit then generation", attempt, requests)
				}
				edit, ok := requests[0].(SetAreaField)
				if !ok || edit.Area != h.area || edit.Field != AreaCount || edit.Text != "invalid" {
					t.Fatalf("attempt %d: first request = %#v, want invalid pending count", attempt, requests[0])
				}
				generation, ok := requests[1].(GenerateArea)
				if !ok || generation.Area != h.area || generation.Regenerate != regenerate {
					t.Fatalf("attempt %d: second request = %#v, want generation", attempt, requests[1])
				}
			}
		})
	}
}

func TestAreaPanelEnterAndBlurStillApplyEdits(t *testing.T) {
	for _, submit := range []bool{false, true} {
		t.Run("submit="+strconv.FormatBool(submit), func(t *testing.T) {
			h := newAreaPanelInput(t)
			h.edit(AreaCount, "80")
			if submit {
				h.router.Queue(key.Event{Name: key.NameReturn, State: key.Press})
			} else {
				gtx := h.context()
				gtx.Execute(key.FocusCmd{})
			}
			h.frame(false)
			a, _ := h.doc.Area(h.area)
			if a.Params.Count != 80 || len(h.generations) != 0 {
				t.Fatalf("commit result: count %d, generation requests %d", a.Params.Count, len(h.generations))
			}
		})
	}
}

func TestAreaPanelDiscardDoesNotReplayGeneration(t *testing.T) {
	h := newAreaPanelInput(t)
	h.edit(AreaCount, "80")
	h.clickGeneration(false)
	h.frame(true)
	h.frame(false)
	a, _ := h.doc.Area(h.area)
	if a.Params.Count != 50 || len(h.generations) != 0 {
		t.Fatalf("discarded input replayed: count %d, generation requests %d", a.Params.Count, len(h.generations))
	}
	if text := h.shell.Spawn.fields[AreaCount].editor.Text(); text != "80" {
		t.Fatalf("discard changed pending text to %q", text)
	}
}
