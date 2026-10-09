package scene

import (
	"cmp"
	"math"
	"slices"

	"zonebuilder/internal/geom"
)

// waterPickSlack is how far past maxDist, in world units along the ray, a
// volume's entry still counts: a BSP water sheet lies on its volume's top,
// so a click on the sheet meets both at the same distance up to float
// error.
const waterPickSlack = 2

// waterTouchSlack is the float error, in world units, within which a ray
// grazing a volume's edge or sliding along one of its faces still meets it.
const waterTouchSlack = 0.01

// PickWater returns the water volume r enters first within maxDist (the
// distance of the scene hit behind the water; +Inf when the ray hits
// nothing), and how far along r it enters: 0 when r starts inside. Each
// volume is a convex polyhedron the ray is clipped against, plane by plane,
// so a click on the surface, on the bottom seen through the water or on a
// static mesh fountain finds the volume, and a click on the dry bank next
// to it does not (spec D3).
func (w *World) PickWater(r Ray, maxDist float32) (*WaterVolume, float32, bool) {
	if r.Dir.Dot(r.Dir) == 0 {
		return nil, 0, false
	}
	r.Dir = r.Dir.Normalize()
	var best *WaterVolume
	bestEntry := float64(maxDist) + waterPickSlack
	for _, v := range w.selectableWater() {
		if t, ok := v.entry(r); ok && t < bestEntry {
			best, bestEntry = v, t
		}
	}
	if best == nil {
		return nil, 0, false
	}
	return best, float32(bestEntry), true
}

// entry is the distance along r (unit Dir) at which it enters v, 0 when
// it starts inside; false when it misses v. Each face's plane cuts the
// ray's parameter range [enter, exit]: the ray enters through the faces
// it crosses inwards and leaves through the others.
func (v *WaterVolume) entry(r Ray) (float64, bool) {
	if len(v.Planes) == 0 {
		return 0, false
	}
	ox, oy, oz := float64(r.Origin.X), float64(r.Origin.Y), float64(r.Origin.Z)
	dx, dy, dz := float64(r.Dir.X), float64(r.Dir.Y), float64(r.Dir.Z)
	enter, exit := 0.0, math.Inf(1)
	for _, p := range v.Planes {
		nx, ny, nz := float64(p.Normal.X), float64(p.Normal.Y), float64(p.Normal.Z)
		// inside is how far the origin lies on the inner side of the plane.
		inside := float64(p.D) - (nx*ox + ny*oy + nz*oz)
		along := nx*dx + ny*dy + nz*dz
		if math.Abs(along) < 1e-9 {
			if inside < -waterTouchSlack {
				return 0, false // parallel, outside
			}
			continue
		}
		t := inside / along
		if along < 0 {
			enter = max(enter, t)
		} else {
			exit = min(exit, t)
		}
	}
	if enter > exit+waterTouchSlack {
		return 0, false
	}
	return enter, true
}

// waterBodySlack is how far apart, in world units, two volumes of one
// body may be: their XY gap and the difference of their tops (spec,
// "Corpo d'água").
const waterBodySlack = 1

// WaterBody is the body of water v belongs to: v and every selectable
// volume of the world joined to it through volumes that touch in XY (boxes
// at most 1 apart) with the same top (within 1), in the world's order. It
// crosses the tiles' scenes, as water does.
func (w *World) WaterBody(v *WaterVolume) []*WaterVolume {
	all := w.selectableWater()
	in := map[*WaterVolume]bool{v: true}
	for queue := []*WaterVolume{v}; len(queue) > 0; queue = queue[1:] {
		for _, o := range all {
			if !in[o] && sameBody(queue[0], o) {
				in[o] = true
				queue = append(queue, o)
			}
		}
	}
	var body []*WaterVolume
	for _, o := range all {
		if in[o] {
			body = append(body, o)
		}
	}
	if len(body) == 0 { // v is not in the world
		body = append(body, v)
	}
	return body
}

// selectableWater is every water volume of the world a click may select:
// all but the Unsupported ones.
func (w *World) selectableWater() []*WaterVolume {
	var all []*WaterVolume
	for _, s := range w.scenes {
		for i := range s.WaterVolumes {
			if v := &s.WaterVolumes[i]; v.Unsupported == "" {
				all = append(all, v)
			}
		}
	}
	return all
}

// WaterID identifies a water volume across loads: its tile and its export
// index in the tile's map package.
type WaterID struct {
	Tile   Tile
	Export int
}

// ID is v's identity.
func (v *WaterVolume) ID() WaterID { return WaterID{Tile: v.Tile, Export: v.Export} }

// WaterVolume is the world's volume id, false when no scene of the world
// has it (its tile is not loaded).
func (w *World) WaterVolume(id WaterID) (*WaterVolume, bool) {
	for _, s := range w.scenes {
		for i := range s.WaterVolumes {
			if v := &s.WaterVolumes[i]; v.ID() == id {
				return v, true
			}
		}
	}
	return nil, false
}

// Footprint is v's outline seen from above, laid at its top: the convex
// hull of its face vertices in XY, counter-clockwise, without collinear
// or repeated points. For a slanted volume it covers the volume with room
// to spare.
func (v *WaterVolume) Footprint() []geom.Vec3 {
	var pts []geom.Vec3
	for _, f := range v.Faces {
		pts = append(pts, f...)
	}
	if len(pts) == 0 {
		return nil
	}
	slices.SortFunc(pts, func(a, b geom.Vec3) int {
		if c := cmp.Compare(a.X, b.X); c != 0 {
			return c
		}
		return cmp.Compare(a.Y, b.Y)
	})
	// Andrew's monotone chain: the lower hull left to right, then the
	// upper hull back, each dropping points that do not turn left.
	hull := make([]geom.Vec3, 0, len(pts)+1)
	for pass := range 2 {
		start := len(hull)
		for _, p := range pts {
			for len(hull) >= start+2 && cross(hull[len(hull)-2], hull[len(hull)-1], p) <= 0 {
				hull = hull[:len(hull)-1]
			}
			hull = append(hull, p)
		}
		hull = hull[:len(hull)-1] // the chain's last point starts the other
		if pass == 0 {
			slices.Reverse(pts)
		}
	}
	top := v.Top()
	for k := range hull {
		hull[k].Z = top
	}
	return hull
}

// cross is the Z of (b-a)×(c-a): positive when a, b, c turn left in XY.
func cross(a, b, c geom.Vec3) float64 {
	abx, aby := float64(b.X)-float64(a.X), float64(b.Y)-float64(a.Y)
	acx, acy := float64(c.X)-float64(a.X), float64(c.Y)-float64(a.Y)
	return abx*acy - aby*acx
}

// sameBody reports whether a and b touch in XY and share their top.
func sameBody(a, b *WaterVolume) bool {
	ab, bb := a.Bounds, b.Bounds
	return math.Abs(float64(a.Top()-b.Top())) <= waterBodySlack &&
		ab.Min.X <= bb.Max.X+waterBodySlack && bb.Min.X <= ab.Max.X+waterBodySlack &&
		ab.Min.Y <= bb.Max.Y+waterBodySlack && bb.Min.Y <= ab.Max.Y+waterBodySlack
}
