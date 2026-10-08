package main

import (
	"fmt"
	"strconv"
	"strings"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/scene"
)

// cameraPose is a camera placement independent of the scene's rebase
// origin: a world position (server coordinates, Unreal basis) and the
// camera's yaw and pitch. It lets a view be reopened exactly, e.g. to set it
// beside UE2-Studio's at the same pose.
type cameraPose struct {
	world      geom.Vec3
	yaw, pitch float32
}

// parsePose reads "x,y,z,yaw,pitch", the format formatPose writes.
func parsePose(s string) (cameraPose, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 5 {
		return cameraPose{}, fmt.Errorf("%q: use x,y,z,yaw,pitch", s)
	}
	var v [5]float32
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return cameraPose{}, fmt.Errorf("%q: %w", s, err)
		}
		v[i] = float32(f)
	}
	return cameraPose{world: geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, yaw: v[3], pitch: v[4]}, nil
}

// apply places cam, which lives in s's rebased render space, at the pose.
func (p *cameraPose) apply(cam *camera.Camera, s *scene.World) {
	cam.Position = scene.ToRender(p.world.Sub(s.Origin))
	cam.Yaw, cam.Pitch = p.yaw, p.pitch
}

// formatPose is cam's pose in s as parsePose reads it.
func formatPose(cam *camera.Camera, s *scene.World) string {
	w := worldPosition(s, cam.Position)
	return fmt.Sprintf("%g,%g,%g,%g,%g", w.X, w.Y, w.Z, cam.Yaw, cam.Pitch)
}
