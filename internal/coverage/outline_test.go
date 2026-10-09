package coverage_test

import (
	"testing"

	"zonebuilder/internal/coverage"
)

// An L-shaped outline 200 wide, whose notch is the square [100, 200]²,
// has none of its area in a box over the notch and beyond, though the
// box crosses its bounding box, and 5000 in a box over the end of its
// lower arm. Catches a check of "part of the outline lies off the loaded
// tiles" made on the outline's bounding box, which sends a concave
// polygon or a circle near a tile's edge to the vertex-based range
// (spec D5), and a clip of the concave outline whose bridging edges add
// area.
func TestAreaInBoxFollowsAConcaveOutline(t *testing.T) {
	l := coverage.Outline{{0, 0}, {200, 0}, {200, 100}, {100, 100}, {100, 200}, {0, 200}}
	if a := l.Area(); a != 30000 {
		t.Errorf("area %v, want 30000", a)
	}
	if a := l.AreaIn(coverage.Point{X: 100, Y: 100}, coverage.Point{X: 300, Y: 300}); a != 0 {
		t.Errorf("area in the box over the notch %v, want 0", a)
	}
	if a := l.AreaIn(coverage.Point{X: 150, Y: -50}, coverage.Point{X: 300, Y: 300}); a != 5000 {
		t.Errorf("area in the box over the lower arm's end %v, want 5000", a)
	}
}
