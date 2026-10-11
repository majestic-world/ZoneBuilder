package ui

import (
	"image"
	"slices"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// waterWindowFixture lays out the zone mode's water window alone, open,
// driving it like the window loop: Update before Layout, or, while the
// editor is locked, the shell's input discard.
type waterWindowFixture struct {
	shell  *Shell
	router input.Router
	ops    op.Ops
}

// waterRows are 2 tiles' volumes, the second row unsupported.
func waterRows() []WaterRow {
	return []WaterRow{
		{Tile: "22_22", Label: "22_22 · WaterVolume2", Detail: "topo -3780 (servidor -3810) · exata"},
		{Tile: "22_22", Label: "22_22 · WaterVolume10", Detail: "não suportado: cisalhamento", Disabled: true},
		{Tile: "22_24", Label: "22_24 · WaterVolume0", Detail: "topo -3780 (servidor -3810) · exata", Checked: true},
		{Tile: "22_24", Label: "22_24 · WaterVolume1", Detail: "topo -3746 (servidor -3776) · aproximada"},
	}
}

func newWaterWindowFixture(rows []WaterRow) *waterWindowFixture {
	f := &waterWindowFixture{shell: NewShell(NewTheme(), "", "22_22")}
	f.shell.Mode = ModeZones
	f.shell.Water.Rows = rows
	f.shell.Water.Open()
	f.frame()
	f.frame()
	return f
}

// frame queues events, then runs one frame and returns the requests the
// window reported in it.
func (f *waterWindowFixture) frame(events ...pointer.Event) []any {
	for _, ev := range events {
		f.router.Queue(ev)
	}
	gtx := layout.Context{
		Ops:         &f.ops,
		Source:      f.router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(1000, 800)),
	}
	var reqs []any
	if f.shell.EditorLocked {
		f.shell.DiscardEditorInput(gtx)
	} else {
		reqs = f.shell.Water.Update(gtx)
	}
	f.shell.editorControls(f.shell.waterWindow)(gtx)
	f.router.Frame(&f.ops)
	f.ops.Reset()
	return reqs
}

// click presses and releases at (x, y) in one frame.
func (f *waterWindowFixture) click(x, y float32) []any {
	return f.frame(floatWindowPointer(pointer.Press, x, y), floatWindowPointer(pointer.Release, x, y))
}

// sweep clicks down the middle of the window, under the title bar, a
// click a frame, and returns the rows the clicks toggled, in the order
// they first did, and the height of the first click on each.
func (f *waterWindowFixture) sweep(t *testing.T) (rows []int, at map[int]float32) {
	t.Helper()
	w := &f.shell.Water.Window
	x := float32(w.pos.X + w.size.X/2)
	at = map[int]float32{}
	for y := w.pos.Y + int(windowTitle) + 1; y < w.pos.Y+w.size.Y-int(windowGrip); y += 2 {
		got := f.click(x, float32(y))
		toggles := 0
		for _, req := range got {
			if r, ok := req.(ToggleWater); ok {
				toggles++
				if _, seen := at[r.Row]; !seen {
					at[r.Row] = float32(y)
					rows = append(rows, r.Row)
				}
			}
		}
		if toggles > 1 {
			t.Fatalf("one click at y %d toggled %d rows: %v", y, toggles, got)
		}
	}
	return rows, at
}

// Clicking down the whole window toggles each enabled row by its own
// index, top to bottom, and never the unsupported one. Catches a toggle
// reported for the wrong row (the app would select another volume) or
// for a disabled row (an unsupported volume would be selected and
// compiled).
func TestWaterWindowTogglesTheClickedRowAndNeverADisabledOne(t *testing.T) {
	f := newWaterWindowFixture(waterRows())
	if rows, _ := f.sweep(t); !slices.Equal(rows, []int{0, 2, 3}) {
		t.Fatalf("clicks down the window toggled rows %v, want [0 2 3]", rows)
	}
}

// While the game mode locks the editor, a click on a row is rejected and
// does not come back as a toggle once the editor resumes. Catches a
// selection change replayed from a click made while playing.
func TestLockedWaterWindowRejectsClicksAndDoesNotReplayAfterResume(t *testing.T) {
	f := newWaterWindowFixture(waterRows())
	_, at := f.sweep(t)
	y, ok := at[3]
	if !ok {
		t.Fatal("no click toggled row 3")
	}
	x := float32(f.shell.Water.Window.pos.X + f.shell.Water.Window.size.X/2)
	f.shell.EditorLocked = true
	f.click(x, y)
	f.frame()
	f.shell.EditorLocked = false
	if reqs := f.frame(); len(reqs) != 0 {
		t.Fatalf("locked click replayed after resume: %v", reqs)
	}
	if reqs := f.click(x, y); !slices.Contains(reqs, any(ToggleWater{Row: 3})) {
		t.Fatalf("click after resume reported %v, want row 3 toggled", reqs)
	}
}

// compileAt clicks down the middle of the window, a click a frame, and
// returns the height of the first click that reported Compilar zona de
// água, and whether one did.
func (f *waterWindowFixture) compileAt() (y float32, ok bool) {
	w := &f.shell.Water.Window
	x := float32(w.pos.X + w.size.X/2)
	for y := w.pos.Y + int(windowTitle) + 1; y < w.pos.Y+w.size.Y-int(windowGrip); y += 2 {
		if slices.Contains(f.click(x, float32(y)), any(CompileWater{})) {
			return float32(y), true
		}
	}
	return 0, false
}

// Compilar zona de água is reported with volumes checked and no polygon
// open, and never with no volume checked or with a polygon open, even for
// a click landed while it was still enabled. Catches an empty compile, or
// a compile under an open polygon, reaching the zone editor.
func TestWaterWindowCompilesOnlyWithACheckedVolumeAndNoOpenPolygon(t *testing.T) {
	f := newWaterWindowFixture(waterRows())
	y, ok := f.compileAt()
	if !ok {
		t.Fatal("no click down the window reported Compilar with a volume checked")
	}
	x := float32(f.shell.Water.Window.pos.X + f.shell.Water.Window.size.X/2)
	f.shell.Water.Busy = true
	if reqs := f.click(x, y); slices.Contains(reqs, any(CompileWater{})) {
		t.Fatal("a click landed before the polygon opened reported Compilar")
	}
	if _, ok := f.compileAt(); ok {
		t.Fatal("Compilar was reported with a polygon open")
	}

	rows := waterRows()
	for i := range rows {
		rows[i].Checked = false
	}
	if _, ok := newWaterWindowFixture(rows).compileAt(); ok {
		t.Fatal("Compilar was reported with no volume checked")
	}
}

// While the game mode locks the editor, a click on Compilar zona de água
// is rejected and does not come back once the editor resumes. Catches a
// water compile replayed from a click made while playing.
func TestLockedWaterWindowRejectsCompileAndDoesNotReplayAfterResume(t *testing.T) {
	f := newWaterWindowFixture(waterRows())
	y, ok := f.compileAt()
	if !ok {
		t.Fatal("no click down the window reported Compilar")
	}
	x := float32(f.shell.Water.Window.pos.X + f.shell.Water.Window.size.X/2)
	f.shell.EditorLocked = true
	f.click(x, y)
	f.frame()
	f.shell.EditorLocked = false
	if reqs := f.frame(); len(reqs) != 0 {
		t.Fatalf("locked click replayed after resume: %v", reqs)
	}
	if reqs := f.click(x, y); !slices.Contains(reqs, any(CompileWater{})) {
		t.Fatalf("click after resume reported %v, want Compilar", reqs)
	}
}
