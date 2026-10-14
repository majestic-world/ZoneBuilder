package camera_test

import (
	"testing"

	"zonebuilder/internal/camera"
)

// Look drags the view FPS-style on both axes: dragging right turns right
// (yaw grows, toward the +Z side of a camera facing +X) and dragging up
// looks up (pitch grows). Catches a flipped sign on either axis, the
// "camera moves the wrong way" report.
func TestLookDragDirections(t *testing.T) {
	c := camera.Camera{}
	c.Look(100, 0)
	if c.Yaw <= 0 {
		t.Errorf("dragging right gave yaw %v, want a right turn (yaw > 0)", c.Yaw)
	}
	c = camera.Camera{}
	c.Look(0, -100)
	if c.Pitch <= 0 {
		t.Errorf("dragging up gave pitch %v, want looking up (pitch > 0)", c.Pitch)
	}
}

// The pitch stops short of straight up and straight down, where the view
// basis degenerates.
func TestLookPitchStopsShortOfVertical(t *testing.T) {
	c := camera.Camera{}
	c.Look(0, -1e6)
	up := c.Pitch
	c.Look(0, 1e12)
	if up >= 1.53 || c.Pitch <= -1.53 {
		t.Errorf("pitch limits %v and %v, want inside ±1.53", up, c.Pitch)
	}
}
