package ui

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"zonebuilder/internal/ui/icon"
)

// The router transforms queued pointer events using the last rendered frame.
// Exercise Layout itself so batching, moving hit regions and pointer capture
// have the same timing as the compilation, height and XML windows.
type floatWindowFixture struct {
	window FloatWindow
	shell  *Shell
	router input.Router
	ops    op.Ops
	area   image.Point
	dims   layout.Dimensions
}

func newFloatWindowFixture() *floatWindowFixture {
	f := &floatWindowFixture{
		window: FloatWindow{Left: 100, Top: 80, Width: 320, Height: 220},
		shell:  NewShell(NewTheme(), "", "22_22"),
		area:   image.Pt(1000, 800),
	}
	f.frame()
	f.frame()
	return f
}

func (f *floatWindowFixture) frame(events ...pointer.Event) {
	for _, ev := range events {
		f.router.Queue(ev)
	}
	gtx := layout.Context{
		Ops:         &f.ops,
		Source:      f.router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(f.area),
	}
	f.dims = f.window.Layout(gtx, f.shell, icon.CodeXML, "Compile spawn XML", func(layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(100, 20)}
	})
	f.router.Frame(&f.ops)
	f.ops.Reset()
}

func floatWindowPointer(kind pointer.Kind, x, y float32) pointer.Event {
	buttons := pointer.ButtonPrimary
	if kind == pointer.Release || kind == pointer.Cancel {
		buttons = 0
	}
	return pointer.Event{
		Kind:      kind,
		Source:    pointer.Mouse,
		PointerID: 1,
		Buttons:   buttons,
		Position:  f32.Pt(x, y),
	}
}

func (f *floatWindowFixture) wantGeometry(t *testing.T, position, size image.Point) {
	t.Helper()
	if f.window.pos != position || f.dims.Size != size {
		t.Fatalf("window geometry = position %v, size %v; want position %v, size %v", f.window.pos, f.dims.Size, position, size)
	}
}

func TestFloatWindowDragTracksCursorAcrossFrames(t *testing.T) {
	f := newFloatWindowFixture()
	size := image.Pt(320, 220)
	f.frame(floatWindowPointer(pointer.Press, 260, 100))
	f.wantGeometry(t, image.Pt(100, 80), size)
	// Establish capture before batching events; the second batch leaves the
	// previous title-bar rectangle while the button remains pressed.
	f.frame(floatWindowPointer(pointer.Move, 280, 110))
	f.wantGeometry(t, image.Pt(120, 90), size)
	f.frame()
	f.frame(
		floatWindowPointer(pointer.Move, 310, 130),
		floatWindowPointer(pointer.Move, 340, 150),
		floatWindowPointer(pointer.Move, 370, 165),
	)
	f.wantGeometry(t, image.Pt(210, 145), size)
	for range 5 {
		f.frame(floatWindowPointer(pointer.Move, 370, 165))
		f.wantGeometry(t, image.Pt(210, 145), size)
		f.frame()
		f.wantGeometry(t, image.Pt(210, 145), size)
	}
	f.frame(floatWindowPointer(pointer.Move, 405, 190))
	f.wantGeometry(t, image.Pt(245, 170), size)
	f.frame(floatWindowPointer(pointer.Release, 405, 190))
	f.frame()
	f.frame(floatWindowPointer(pointer.Move, 50, 50))
	f.wantGeometry(t, image.Pt(245, 170), size)

	// Regrab the newly rendered title bar at a different local offset. The
	// previous gesture's press position must not affect the next drag.
	f.frame(floatWindowPointer(pointer.Press, 385, 190))
	f.frame(
		floatWindowPointer(pointer.Move, 400, 200),
		floatWindowPointer(pointer.Move, 420, 215),
	)
	f.frame()
	f.wantGeometry(t, image.Pt(280, 195), size)
	f.frame(floatWindowPointer(pointer.Release, 420, 215))
	f.frame()
	f.wantGeometry(t, image.Pt(280, 195), size)
}

func TestFloatWindowResizeTracksCursorAcrossFrames(t *testing.T) {
	f := newFloatWindowFixture()
	position := image.Pt(100, 80)
	// The press is 8 px from the bottom-right corner of the window.
	f.frame(floatWindowPointer(pointer.Press, 412, 292))
	f.frame(floatWindowPointer(pointer.Move, 432, 302))
	f.wantGeometry(t, position, image.Pt(340, 230))
	f.frame()
	f.frame(
		floatWindowPointer(pointer.Move, 452, 322),
		floatWindowPointer(pointer.Move, 482, 342),
		floatWindowPointer(pointer.Move, 512, 362),
	)
	f.wantGeometry(t, position, image.Pt(420, 290))
	for range 5 {
		f.frame(floatWindowPointer(pointer.Move, 512, 362))
		f.wantGeometry(t, position, image.Pt(420, 290))
		f.frame()
		f.wantGeometry(t, position, image.Pt(420, 290))
	}
	f.frame(floatWindowPointer(pointer.Move, 472, 332))
	f.wantGeometry(t, position, image.Pt(380, 260))
	f.frame(floatWindowPointer(pointer.Release, 472, 332))
	f.frame()

	f.frame(floatWindowPointer(pointer.Press, 472, 332))
	f.frame(
		floatWindowPointer(pointer.Move, 482, 342),
		floatWindowPointer(pointer.Move, 497, 357),
	)
	f.frame()
	f.wantGeometry(t, position, image.Pt(405, 285))
	f.frame(floatWindowPointer(pointer.Release, 497, 357))
	f.frame()
	f.wantGeometry(t, position, image.Pt(405, 285))
}

func TestFloatWindowDragAndResizeRespectBoundsAndCapture(t *testing.T) {
	t.Run("drag", func(t *testing.T) {
		f := newFloatWindowFixture()
		size := image.Pt(320, 220)
		f.frame(floatWindowPointer(pointer.Press, 260, 100))
		f.frame(floatWindowPointer(pointer.Move, 1400, 1000))
		f.frame()
		f.wantGeometry(t, image.Pt(680, 580), size)
		for range 3 {
			f.frame(floatWindowPointer(pointer.Move, 1400, 1000))
			f.wantGeometry(t, image.Pt(680, 580), size)
		}
		// Returning from beyond the viewport restores the original cursor
		// anchor instead of accumulating the displacement rejected by clamp.
		f.frame(floatWindowPointer(pointer.Move, 800, 520))
		f.wantGeometry(t, image.Pt(640, 500), size)
		f.frame(floatWindowPointer(pointer.Move, -100, -100))
		f.wantGeometry(t, image.Pt(0, 0), size)
		f.frame(floatWindowPointer(pointer.Move, 260, 100))
		f.wantGeometry(t, image.Pt(100, 80), size)
		f.frame(floatWindowPointer(pointer.Release, 260, 100))
		f.frame()
	})

	t.Run("resize", func(t *testing.T) {
		f := newFloatWindowFixture()
		f.frame(floatWindowPointer(pointer.Press, 412, 292))
		f.frame(floatWindowPointer(pointer.Move, 200, 100))
		f.frame()
		f.wantGeometry(t, image.Pt(100, 80), image.Pt(220, 140))
		for range 3 {
			f.frame(floatWindowPointer(pointer.Move, 200, 100))
			f.wantGeometry(t, image.Pt(100, 80), image.Pt(220, 140))
		}
		f.frame(floatWindowPointer(pointer.Move, 432, 302))
		f.wantGeometry(t, image.Pt(100, 80), image.Pt(340, 230))
		f.frame(floatWindowPointer(pointer.Release, 432, 302))
		f.frame()
		// A smaller viewport clamps both dimensions and the rendered origin.
		f.area = image.Pt(300, 200)
		f.frame()
		f.wantGeometry(t, image.Pt(0, 0), image.Pt(300, 200))
	})
}

func TestFloatWindowCollapseAndCloseControlsRemainClickable(t *testing.T) {
	f := newFloatWindowFixture()
	f.frame(
		floatWindowPointer(pointer.Press, 122, 101),
		floatWindowPointer(pointer.Release, 122, 101),
	)
	f.frame()
	if !f.window.Collapsed {
		t.Fatal("collapse control did not collapse the window")
	}
	f.wantGeometry(t, image.Pt(100, 80), image.Pt(320, 42))
	f.frame(floatWindowPointer(pointer.Press, 260, 100))
	f.frame(floatWindowPointer(pointer.Move, 280, 110))
	f.frame(floatWindowPointer(pointer.Release, 280, 110))
	f.frame()
	f.wantGeometry(t, image.Pt(120, 90), image.Pt(320, 42))
	f.frame(
		floatWindowPointer(pointer.Press, 142, 111),
		floatWindowPointer(pointer.Release, 142, 111),
	)
	f.frame()
	if f.window.Collapsed {
		t.Fatal("collapse control did not expand the moved window")
	}
	f.wantGeometry(t, image.Pt(120, 90), image.Pt(320, 220))
	f.frame(
		floatWindowPointer(pointer.Press, 418, 111),
		floatWindowPointer(pointer.Release, 418, 111),
	)
	f.frame()
	if !f.window.Closed || f.dims.Size != (image.Point{}) {
		t.Fatalf("close control left the window visible: closed %v, size %v", f.window.Closed, f.dims.Size)
	}
}
