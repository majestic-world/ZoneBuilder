package unreal

import (
	"fmt"
	"math"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
)

// MeshActorClasses are the actor classes that place a StaticMesh, in the
// order UE2-Studio loads them (crates/map-engine/src/actor_classes.rs). An
// L2 Mover carries a plain StaticMesh and no brush. Brush is left out on
// purpose: its geometry is already baked into the Level's BSP Model.
var MeshActorClasses = [...]string{
	"StaticMeshActor",
	"MovableStaticMeshActor",
	"L2MovableStaticMeshActor",
	"Mover",
	"L2NMover",
}

// Actor is the placement part of a placed actor's properties. Port of
// UE2-Studio's Actor::from_properties (src/unreal/objects.rs).
type Actor struct {
	Location    geom.Vec3
	Rotation    l2pkg.Rotator
	DrawScale   float32
	DrawScale3D geom.Vec3
	PrePivot    geom.Vec3
	// StaticMesh is the object reference of the mesh in the map package's
	// index space, 0 when the actor names none.
	StaticMesh int32
	// Skins overrides the material of mesh section i when Skins[i] is not
	// 0: an object reference in the map package's index space.
	Skins []int32
	// Hidden and DeleteMe are bHidden and bDeleteMe: the client does not
	// draw such an actor.
	Hidden, DeleteMe bool
}

// ReadActor decodes the properties of export i (0-based) of p as an Actor.
func ReadActor(p *l2pkg.Package, i int) (*Actor, error) {
	r, err := p.ExportReader(i)
	if err != nil {
		return nil, err
	}
	props, err := l2pkg.ReadProperties(p, r, p.Exports[i].Flags)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", p.Exports[i].ClassName, p.Exports[i].ObjectName, err)
	}
	a := &Actor{DrawScale: 1, DrawScale3D: geom.Vec3{X: 1, Y: 1, Z: 1}}
	if v, ok := props.Vector("Location"); ok {
		a.Location = vec3(v)
	}
	a.Rotation, _ = props.Rotator("Rotation")
	if v, ok := props.Float("DrawScale"); ok {
		a.DrawScale = v
	}
	if v, ok := props.Vector("DrawScale3D"); ok {
		a.DrawScale3D = vec3(v)
	}
	if v, ok := props.Vector("PrePivot"); ok {
		a.PrePivot = vec3(v)
	}
	a.StaticMesh, _ = props.Index("StaticMesh")
	if b, ok := props.Bytes("Skins"); ok {
		sr := l2pkg.NewReader(b, 0)
		for sr.Pos < len(b) && sr.Err() == nil {
			a.Skins = append(a.Skins, sr.Index())
		}
		if err := sr.Err(); err != nil {
			return nil, fmt.Errorf("%s %s: Skins: %w", p.Exports[i].ClassName, p.Exports[i].ObjectName, err)
		}
	}
	a.Hidden = props.Bool("bHidden")
	a.DeleteMe = props.Bool("bDeleteMe")
	return a, nil
}

// Transform is the actor's local-to-world transform:
// position = Location - PrePivot, scale = DrawScale3D * DrawScale, and the
// rotation of rotatorQuat.
func (a *Actor) Transform() Transform {
	return Transform{
		Position: a.Location.Sub(a.PrePivot),
		Rotation: rotatorQuat(a.Rotation),
		Scale:    a.DrawScale3D.Scale(a.DrawScale),
	}
}

// rotatorQuat is the rotation of r as the Euler angles UE2-Studio feeds
// GLM's quaternion constructor (Rotator::vector in
// crates/package-engine/src/actor.rs).
func rotatorQuat(r l2pkg.Rotator) Quat {
	const toRadians = math.Pi / 32768
	return newGLMQuat(
		-float64(r.Roll)*toRadians,
		-float64(r.Pitch)*toRadians,
		float64(r.Yaw)*toRadians,
	)
}

// Transform places local mesh points in the world, all in Unreal's basis:
// world = Position + Rotation(p * Scale). Port of UE2-Studio's
// Transform::point (crates/package-engine/src/math.rs).
type Transform struct {
	Position geom.Vec3
	Rotation Quat
	Scale    geom.Vec3
}

// Point is the world position of local point p.
func (t Transform) Point(p geom.Vec3) geom.Vec3 {
	return t.Position.Add(t.Rotation.Rotate(p.Mul(t.Scale)))
}

// Quat is a unit quaternion.
type Quat struct{ X, Y, Z, W float32 }

// newGLMQuat is GLM's quat(vec3 euler) constructor.
func newGLMQuat(ex, ey, ez float64) Quat {
	cx, sx := math.Cos(ex/2), math.Sin(ex/2)
	cy, sy := math.Cos(ey/2), math.Sin(ey/2)
	cz, sz := math.Cos(ez/2), math.Sin(ez/2)
	return Quat{
		W: float32(cx*cy*cz + sx*sy*sz),
		X: float32(sx*cy*cz - cx*sy*sz),
		Y: float32(cx*sy*cz + sx*cy*sz),
		Z: float32(cx*cy*sz - sx*sy*cz),
	}
}

// Rotate applies q to v: v + 2w(q×v) + 2 q×(q×v).
func (q Quat) Rotate(v geom.Vec3) geom.Vec3 {
	u := geom.Vec3{X: q.X, Y: q.Y, Z: q.Z}
	c := u.Cross(v)
	return v.Add(c.Scale(2 * q.W)).Add(u.Cross(c).Scale(2))
}

func vec3(v [3]float32) geom.Vec3 { return geom.Vec3{X: v[0], Y: v[1], Z: v[2]} }
