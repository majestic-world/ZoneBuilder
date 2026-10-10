package main

import (
	"math"
	"slices"
	"time"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/model"
	"zonebuilder/internal/render"
	"zonebuilder/internal/spawn"
)

// previewClip is the preview monster's Wait, the only clip it plays.
const previewClip = 0

// previewStep caps the time one frame advances the animation, so the
// first frame after a pause does not jump.
const previewStep = 100 * time.Millisecond

// headingYaw is the yaw of an L2 heading, in radians about Z from +X: an
// Unreal rotator's unit, 65536 per turn (spec D7, [INFERENCE] until the
// in-game check).
func headingYaw(h int) float32 {
	return float32(float64(h) * (2 * math.Pi / 65536))
}

// monsterPreview draws the embedded preview monster (model.Monster) on
// spawn points: feet on the point, turned to its heading, every copy in
// the one pose of its Wait clip at the frame's time.
type monsterPreview struct {
	bundle *model.Bundle
	anim   *model.Animator
	last   time.Time
	// gpu is the monster on owner, the renderer it was made for.
	gpu   *render.Model
	owner *render.Renderer
	// points are the points gpu's instances were made from.
	points    []spawn.Point
	instances []render.Instance
}

// advance plays Wait up to now, the frame's time.
func (p *monsterPreview) advance(now time.Time) error {
	if p.anim == nil {
		b, err := model.Decode(model.Monster)
		if err != nil {
			return err
		}
		p.bundle, p.anim = b, model.NewAnimator(b)
		p.last = now
	}
	step := min(max(now.Sub(p.last), 0), previewStep)
	p.anim.Update(previewClip, float32(step.Seconds()))
	p.last = now
	return nil
}

// draw queues the monster on points for r's next frame, in the pose of
// the last advance. It makes the monster on r first: on the first call
// and after a new GPU context.
func (p *monsterPreview) draw(r *render.Renderer, points []spawn.Point) error {
	if p.anim == nil {
		return nil
	}
	if p.owner != r {
		gpu, err := r.NewModel(p.bundle)
		if err != nil {
			return err
		}
		p.gpu, p.owner, p.points = gpu, r, nil
		p.gpu.SetInstances(nil)
	}
	if !slices.Equal(p.points, points) {
		p.points = append(p.points[:0], points...)
		p.instances = p.instances[:0]
		for _, pt := range points {
			p.instances = append(p.instances, render.Instance{
				Pos: geom.Vec3{X: float32(pt.X), Y: float32(pt.Y), Z: float32(pt.Z)},
				Yaw: headingYaw(pt.Heading),
			})
		}
		p.gpu.SetInstances(p.instances)
	}
	p.gpu.Pose(p.anim)
	r.DrawModel(p.gpu)
	return nil
}
