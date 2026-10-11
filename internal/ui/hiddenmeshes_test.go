package ui

import (
	"fmt"
	"image"
	"slices"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// hiddenMeshesInput drives the hidden meshes list through Gio's router, as
// the window loop does: requests are read before the frame is laid out,
// from the clicks routed against the previous frame.
type hiddenMeshesInput struct {
	shell  *Shell
	router input.Router
	ops    op.Ops
	area   image.Point
	layout func(gtx layout.Context)
}

func newHiddenMeshesInput(rows int, area image.Point, lay func(s *Shell, gtx layout.Context)) *hiddenMeshesInput {
	h := &hiddenMeshesInput{shell: NewShell(NewTheme(), "", "22_22"), area: area}
	for i := range rows {
		h.shell.HiddenMeshes.Rows = append(h.shell.HiddenMeshes.Rows, fmt.Sprintf("22_22 mesh %d", i))
	}
	h.layout = func(gtx layout.Context) { lay(h.shell, gtx) }
	return h
}

// frame runs one frame and returns what it reported: "show i" for a
// Mostrar, "show all" for Mostrar todos.
func (h *hiddenMeshesInput) frame(events ...pointer.Event) []string {
	for _, ev := range events {
		h.router.Queue(ev)
	}
	gtx := layout.Context{
		Ops:         &h.ops,
		Source:      h.router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(h.area),
	}
	if h.shell.EditorLocked {
		h.shell.DiscardEditorInput(gtx)
	}
	var reported []string
	if i, ok := h.shell.HiddenMeshes.ShowRequested(gtx); ok {
		reported = append(reported, fmt.Sprintf("show %d", i))
	}
	if h.shell.HiddenMeshes.ShowAllRequested(gtx) {
		reported = append(reported, "show all")
	}
	h.layout(gtx)
	h.router.Frame(&h.ops)
	h.ops.Reset()
	return reported
}

// sweep clicks down the column at x, from y0 to y1 every 4 px, and
// returns the reports in order, a report repeated by consecutive clicks on
// the same control kept once.
func (h *hiddenMeshesInput) sweep(x, y0, y1 int) []string {
	var reported []string
	for y := y0; y < y1; y += 4 {
		for _, r := range h.frame(
			floatWindowPointer(pointer.Press, float32(x), float32(y)),
			floatWindowPointer(pointer.Release, float32(x), float32(y)),
		) {
			if len(reported) == 0 || reported[len(reported)-1] != r {
				reported = append(reported, r)
			}
		}
	}
	return reported
}

func layoutHiddenMeshesPanel(s *Shell, gtx layout.Context) {
	layout.Flex{Axis: layout.Vertical}.Layout(gtx, s.hiddenMeshes()...)
}

func layoutHiddenMeshesWindow(s *Shell, gtx layout.Context) {
	s.editorControls(s.hiddenMeshesWindow)(gtx)
}

func shows(n int) []string {
	var want []string
	for i := range n {
		want = append(want, fmt.Sprintf("show %d", i))
	}
	return want
}

// The panel draws at most 5 rows: clicking down the column of Mostrar
// buttons reaches rows 0 to 4 and then Mostrar todos, never row 5 or
// later; Ver todos os 12 opens the window. With 5 hidden meshes there is
// no link. Catches an uncapped list and a link drawn without overflow.
func TestHiddenMeshesPanelDrawsAtMostFiveRowsAndLinksOnlyBeyondFive(t *testing.T) {
	for _, c := range []struct {
		rows int
		link bool
	}{{12, true}, {6, true}, {5, false}, {2, false}} {
		t.Run(fmt.Sprint(c.rows), func(t *testing.T) {
			h := newHiddenMeshesInput(c.rows, image.Pt(300, 700), layoutHiddenMeshesPanel)
			h.frame()
			want := append(shows(min(c.rows, 5)), "show all")
			if got := h.sweep(270, 0, 700); !slices.Equal(got, want) {
				t.Fatalf("panel clicks reported %v, want %v", got, want)
			}
			if opened := !h.shell.HiddenMeshes.Window.Closed; opened != c.link {
				t.Fatalf("Ver todos opened the window = %v, want %v", opened, c.link)
			}
		})
	}
}

// openHiddenMeshesWindow opens the window over 12 hidden meshes, tall
// enough to show every row, and returns the input with the x of the
// Mostrar column and the y span of the window's body.
func openHiddenMeshesWindow(t *testing.T) (h *hiddenMeshesInput, x, y0, y1 int) {
	t.Helper()
	h = newHiddenMeshesInput(12, image.Pt(900, 900), layoutHiddenMeshesWindow)
	w := &h.shell.HiddenMeshes.Window
	if !w.Closed {
		t.Fatal("hidden meshes window open before Ver todos")
	}
	w.Width, w.Height = 400, 800
	h.shell.HiddenMeshes.Open()
	h.frame()
	if w.Closed {
		t.Fatal("hidden meshes window closed after Open")
	}
	if want := image.Pt(250, 50); w.pos != want {
		t.Fatalf("window opened at %v, want centred at %v", w.pos, want)
	}
	return h, 250 + 400 - 50, 50 + int(windowTitle), 50 + 800 - int(windowGrip)
}

// Mostrar on row i of the window reports i, rows 5 and later included,
// and its Mostrar todos is reported. Catches a window that reuses the
// panel's 5 clickables or offsets its indices.
func TestHiddenMeshesWindowReportsEveryRowAndShowAll(t *testing.T) {
	h, x, y0, y1 := openHiddenMeshesWindow(t)
	want := append(shows(12), "show all")
	if got := h.sweep(x, y0, y1); !slices.Equal(got, want) {
		t.Fatalf("window clicks reported %v, want %v", got, want)
	}
}

// With no hidden mesh left the window stays open with the hint instead of
// rows: clicking down it reports nothing.
func TestHiddenMeshesWindowEmptyReportsNothing(t *testing.T) {
	h, x, y0, y1 := openHiddenMeshesWindow(t)
	h.shell.HiddenMeshes.Rows = nil
	h.frame()
	if got := h.sweep(x, y0, y1); len(got) != 0 {
		t.Fatalf("empty window reported %v", got)
	}
	if h.shell.HiddenMeshes.Window.Closed {
		t.Fatal("empty window closed itself")
	}
}

// Esc in the viewport closes the window, as it does the menus.
func TestCloseMenusClosesHiddenMeshesWindow(t *testing.T) {
	for _, mode := range []Mode{ModeZones, ModePopulate} {
		s := NewShell(NewTheme(), "", "22_22")
		s.Mode = mode
		if s.CloseMenus() {
			t.Fatalf("mode %v: CloseMenus reported a closed window with none open", mode)
		}
		s.HiddenMeshes.Open()
		if !s.CloseMenus() || !s.HiddenMeshes.Window.Closed {
			t.Fatalf("mode %v: CloseMenus left the hidden meshes window open", mode)
		}
	}
}
