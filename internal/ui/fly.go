package ui

import (
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"zonebuilder/internal/camera"
)

// FlyControls turns viewport input into fly-camera motion with UE2-Studio's
// bindings: W/S forward/back along the view, A/D strafe, E/Q up/down, Shift
// for the fast gear, drag to look, Shift+drag or a left+right drag to lift,
// and the wheel to fly forward/back.
type FlyControls struct {
	held     map[key.Name]bool
	shift    bool
	dragging bool
	last     f32.Point
	// stepped is when Step last moved the camera, zero when it did not.
	stepped time.Time
}

// Handle applies one viewport event (from Viewport.Update) to cam.
func (f *FlyControls) Handle(ev event.Event, cam *camera.Camera) {
	if f.held == nil {
		f.held = map[key.Name]bool{}
	}
	switch e := ev.(type) {
	case key.FocusEvent:
		if !e.Focus {
			// Releases are not delivered once focus is gone.
			clear(f.held)
			f.shift = false
		}
	case key.Event:
		down := e.State == key.Press
		if e.Name == key.NameShift {
			f.shift = down
		} else {
			f.held[e.Name] = down
			f.shift = e.Modifiers.Contain(key.ModShift)
		}
	case pointer.Event:
		f.shift = e.Modifiers.Contain(key.ModShift)
		switch e.Kind {
		case pointer.Press:
			f.dragging, f.last = true, e.Position
		case pointer.Release:
			f.dragging = e.Buttons != 0
			f.last = e.Position
		case pointer.Drag:
			d := e.Position.Sub(f.last)
			f.last = e.Position
			if !f.dragging {
				break
			}
			both := pointer.ButtonPrimary | pointer.ButtonSecondary
			if f.shift || e.Buttons&both == both {
				cam.Lift(d.Y)
			} else {
				cam.Look(d.X, d.Y)
			}
		case pointer.Scroll:
			// Gio reports -120 per wheel notch away from the user, on X
			// instead of Y while Shift is held.
			notches := -(e.Scroll.X + e.Scroll.Y) / 120
			cam.Fly(notches*camera.WheelStep, 0, 0, false)
		}
	}
}

// Step flies cam with the held keys for the time since the previous moving
// Step (none on the first one, so time spent idle is not flown), and
// reports whether any fly key is held, so the caller keeps redrawing.
func (f *FlyControls) Step(cam *camera.Camera, now time.Time) bool {
	axis := func(pos, neg key.Name) float32 {
		var a float32
		if f.held[pos] {
			a++
		}
		if f.held[neg] {
			a--
		}
		return a
	}
	moving := f.held["W"] || f.held["S"] || f.held["A"] || f.held["D"] || f.held["E"] || f.held["Q"]
	if !moving {
		f.stepped = time.Time{}
		return false
	}
	if !f.stepped.IsZero() {
		dt := float32(now.Sub(f.stepped).Seconds())
		cam.Fly(axis("W", "S")*dt, axis("D", "A")*dt, axis("E", "Q")*dt, f.shift)
	}
	f.stepped = now
	return true
}
