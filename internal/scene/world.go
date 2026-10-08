package scene

import (
	"math"
	"slices"

	"zonebuilder/internal/geom"
)

// World is a set of scenes open together, typically one per map tile so
// that each tile can be loaded and dropped on its own while the camera
// moves, all sharing one rebase origin. It is what the app picks against.
type World struct {
	// Origin is the rebase origin of the whole world, used instead of each
	// scene's own Origin: the renderer subtracts it from every vertex and
	// the camera's render space has it at zero.
	Origin geom.Vec3
	scenes []*Scene
}

// NewWorld returns an empty world rebased on origin.
func NewWorld(origin geom.Vec3) *World {
	return &World{Origin: origin}
}

// Add puts s in the world.
func (w *World) Add(s *Scene) {
	w.scenes = append(w.scenes, s)
}

// Remove takes s out of the world, if it is there.
func (w *World) Remove(s *Scene) {
	w.scenes = slices.DeleteFunc(w.scenes, func(o *Scene) bool { return o == s })
}

// Scenes are the scenes in the world, in the order they were added. The
// slice is the world's: read it, don't keep it across Add or Remove.
func (w *World) Scenes() []*Scene { return w.scenes }

// Pick returns the nearest point where r meets any scene of the world (see
// Scene.Pick), or false when it meets nothing.
func (w *World) Pick(r Ray) (Hit, bool) {
	best, found := Hit{Distance: float32(math.Inf(1))}, false
	for _, s := range w.scenes {
		if h, ok := s.Pick(r); ok && h.Distance < best.Distance {
			best, found = h, true
		}
	}
	return best, found
}
