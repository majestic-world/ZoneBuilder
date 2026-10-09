// Package water turns the map's water volumes into water zones (spec D5):
// one zone per server top, one polygon per volume. It has no GL and no Gio.
package water

import (
	"cmp"
	"math"
	"slices"

	"zonebuilder/internal/scene"
	"zonebuilder/internal/zone"
)

// ServerZOffset is what a volume's client Z gains to become the zone's
// server Z, for both zmin and zmax. It is the offset the Majestic datapack
// generated its water zones with, not the +32 of scene.ServerZOffset (ADR
// 0005).
const ServerZOffset = -30

// Plan is the water zone of one top of the volumes Compile was given.
type Plan struct {
	// Name is "[X_Y_<volume>]": the tile and name of the group's volume
	// first in natural order.
	Name string
	// Zone is the zone the plan creates, or the project's zone already
	// named Name when Existing.
	Zone zone.ZoneID
	// Existing is set when the project already has a zone named Name:
	// Commands is then empty and the project's version is the one to
	// compile, so the user's edits are never overwritten.
	Existing bool
	// Commands create the zone: a CreateZone, then one AddShape per volume,
	// in the order of Volumes. Apply them, with the other plans', in one
	// zone.Batch for a single undo step.
	Commands zone.Batch
	// Top is the server zmax every polygon of the zone shares.
	Top int
	// Volumes are the group's volumes in natural name order; polygon k of
	// the zone comes from Volumes[k].
	Volumes []scene.WaterVolume
	// Warnings are the group's D7 warnings. None blocks the compilation.
	Warnings []Warning
}

// WarningKind is one of the D7 warnings.
type WarningKind int

const (
	// Approximate: the volume has a slanted wall or top, so its prism
	// covers it with room to spare.
	Approximate WarningKind = iota + 1
	// Overlap: the volume crosses, in XY and Z, a live volume with another
	// top left out of the selection; where they cross, the server uses the
	// higher top of the two.
	Overlap
	// OutsideTile: the volume reaches past its own tile, so the zone enters
	// the neighbour tile.
	OutsideTile
)

// Warning is a D7 warning about one volume.
type Warning struct {
	Kind WarningKind
	// Volume is the volume warned about, "X_Y WaterVolumeN".
	Volume string
	// Other is the crossed volume of an Overlap, "" otherwise.
	Other string
}

// String is the warning for the status bar and the log, in pt-BR.
func (w Warning) String() string {
	switch w.Kind {
	case Approximate:
		return w.Volume + ": aproximada, o volume tem parede ou topo inclinado e a zona o cobre com sobra"
	case Overlap:
		return w.Volume + ": água sobreposta ao " + w.Other + ", de outro topo e fora da seleção; onde eles se cruzam o servidor usa o maior topo"
	case OutsideTile:
		return w.Volume + ": o volume passa do próprio tile, e a zona entra no tile vizinho"
	}
	return w.Volume + ": aviso desconhecido"
}

// prism is the server prism of a volume: its rounded convex XY footprint and
// its Z range, both in server coordinates.
type prism struct {
	ring       [][2]int
	zmin, zmax int
}

func prismOf(v *scene.WaterVolume) prism {
	var pts [][2]int
	for _, f := range v.Faces {
		for _, p := range f {
			pts = append(pts, [2]int{round(p.X), round(p.Y)})
		}
	}
	return prism{
		ring: hull(pts),
		zmin: round(v.Bottom()) + ServerZOffset,
		zmax: round(v.Top()) + ServerZOffset,
	}
}

func round(x float32) int { return int(math.Round(float64(x))) }

// Compile plans the water zones of the selected volumes. The volumes are
// grouped by their top in server Z, one zone per top, because the server
// takes a zone's highest zmax as the water surface; each volume becomes one
// polygon with its own zmin and zmax. live is every live volume of the
// loaded tiles (the selection may be among them); it is searched for the
// overlaps D7 warns about. A plan whose name doc already has reuses that
// zone and creates nothing; the others reserve their zone's ID in doc.
// Unsupported volumes are skipped. Plans come in natural name order.
func Compile(selected, live []scene.WaterVolume, doc *zone.Document) []Plan {
	type key struct {
		tile   scene.Tile
		export int
	}
	chosen := map[key]bool{}
	groups := map[int][]scene.WaterVolume{}
	for _, v := range selected {
		k := key{v.Tile, v.Export}
		if v.Unsupported != "" || chosen[k] {
			continue
		}
		chosen[k] = true
		top := round(v.Top()) + ServerZOffset
		groups[top] = append(groups[top], v)
	}
	var others []scene.WaterVolume
	var otherPrisms []prism
	for _, v := range live {
		if v.Unsupported == "" && !chosen[key{v.Tile, v.Export}] {
			others = append(others, v)
			otherPrisms = append(otherPrisms, prismOf(&v))
		}
	}

	var plans []Plan
	for top, vols := range groups {
		slices.SortFunc(vols, func(a, b scene.WaterVolume) int {
			return cmp.Or(naturalCompare(a.Name, b.Name), naturalCompare(a.Tile.Name(), b.Tile.Name()))
		})
		p := Plan{Name: "[" + label(&vols[0], "_") + "]", Top: top, Volumes: vols}
		var shapes []zone.AddShape
		for i := range vols {
			v := &vols[i]
			pr := prismOf(v)
			pts := make([]zone.Point, len(pr.ring))
			for k, q := range pr.ring {
				pts[k] = zone.Point{X: q[0], Y: q[1], Z: pr.zmax}
			}
			shapes = append(shapes, zone.AddShape{Kind: zone.Polygon, Points: pts, ZMin: pr.zmin, ZMax: pr.zmax})
			p.Warnings = append(p.Warnings, warnings(v, pr, others, otherPrisms)...)
		}
		if id, ok := zoneNamed(doc, p.Name); ok {
			p.Zone, p.Existing = id, true
		} else {
			p.Zone = doc.NewZoneID()
			p.Commands = zone.Batch{zone.CreateZone{ID: p.Zone, Name: p.Name, Type: zone.Water}}
			for _, s := range shapes {
				s.Zone = p.Zone
				p.Commands = append(p.Commands, s)
			}
		}
		plans = append(plans, p)
	}
	slices.SortFunc(plans, func(a, b Plan) int { return naturalCompare(a.Name, b.Name) })
	return plans
}

// label is "X_Y<sep>WaterVolumeN" for volume v.
func label(v *scene.WaterVolume, sep string) string { return v.Tile.Name() + sep + v.Name }

func zoneNamed(doc *zone.Document, name string) (zone.ZoneID, bool) {
	for _, z := range doc.Zones() {
		if z.Name == name {
			return z.ID, true
		}
	}
	return 0, false
}

// warnings are volume v's D7 warnings; pr is its prism, others the live
// volumes outside the selection and otherPrisms their prisms.
func warnings(v *scene.WaterVolume, pr prism, others []scene.WaterVolume, otherPrisms []prism) []Warning {
	var ws []Warning
	name := label(v, " ")
	if !v.Exact {
		ws = append(ws, Warning{Kind: Approximate, Volume: name})
	}
	for i, o := range otherPrisms {
		if o.zmax != pr.zmax && crosses(pr, o) {
			ws = append(ws, Warning{Kind: Overlap, Volume: name, Other: label(&others[i], " ")})
		}
	}
	x0, y0 := v.Tile.Origin()
	minX, minY := round(x0), round(y0)
	maxX, maxY := minX+scene.TileSpan, minY+scene.TileSpan
	for _, q := range pr.ring {
		if q[0] < minX || q[0] > maxX || q[1] < minY || q[1] > maxY {
			ws = append(ws, Warning{Kind: OutsideTile, Volume: name})
			break
		}
	}
	return ws
}

// crosses reports whether prisms a and b share a volume: their Z ranges
// overlap and their footprints share an area. Prisms that only touch, on a
// face, an edge or a point, do not cross.
func crosses(a, b prism) bool {
	if a.zmax <= b.zmin || b.zmax <= a.zmin {
		return false
	}
	// Separating axis test on the edge normals of both convex rings; integer
	// coordinates keep the projections exact.
	for _, r := range [2][][2]int{a.ring, b.ring} {
		for i := range r {
			p, q := r[i], r[(i+1)%len(r)]
			nx, ny := int64(q[1]-p[1]), int64(p[0]-q[0])
			amin, amax := project(a.ring, nx, ny)
			bmin, bmax := project(b.ring, nx, ny)
			if amax <= bmin || bmax <= amin {
				return false
			}
		}
	}
	return len(a.ring) >= 3 && len(b.ring) >= 3
}

func project(r [][2]int, nx, ny int64) (lo, hi int64) {
	lo, hi = math.MaxInt64, math.MinInt64
	for _, p := range r {
		d := int64(p[0])*nx + int64(p[1])*ny
		lo, hi = min(lo, d), max(hi, d)
	}
	return lo, hi
}

// hull is the convex hull of pts, counter-clockwise from the lowest X (then
// lowest Y), without repeated or collinear points (Andrew's monotone chain).
func hull(pts [][2]int) [][2]int {
	pts = slices.Clone(pts)
	slices.SortFunc(pts, func(a, b [2]int) int { return cmp.Or(cmp.Compare(a[0], b[0]), cmp.Compare(a[1], b[1])) })
	pts = slices.Compact(pts)
	if len(pts) < 3 {
		return pts
	}
	cross := func(o, a, b [2]int) int64 {
		return int64(a[0]-o[0])*int64(b[1]-o[1]) - int64(a[1]-o[1])*int64(b[0]-o[0])
	}
	h := make([][2]int, 0, 2*len(pts))
	for _, p := range pts {
		for len(h) >= 2 && cross(h[len(h)-2], h[len(h)-1], p) <= 0 {
			h = h[:len(h)-1]
		}
		h = append(h, p)
	}
	lower := len(h) + 1
	for i := len(pts) - 2; i >= 0; i-- {
		p := pts[i]
		for len(h) >= lower && cross(h[len(h)-2], h[len(h)-1], p) <= 0 {
			h = h[:len(h)-1]
		}
		h = append(h, p)
	}
	return h[:len(h)-1]
}

// naturalCompare orders strings with their digit runs compared as numbers:
// "WaterVolume2" before "WaterVolume10".
func naturalCompare(a, b string) int {
	for a != "" && b != "" {
		da, db := digits(a), digits(b)
		if da > 0 && db > 0 {
			na, nb := trimZeros(a[:da]), trimZeros(b[:db])
			if c := cmp.Or(cmp.Compare(len(na), len(nb)), cmp.Compare(na, nb)); c != 0 {
				return c
			}
			a, b = a[da:], b[db:]
			continue
		}
		if a[0] != b[0] {
			return cmp.Compare(a[0], b[0])
		}
		a, b = a[1:], b[1:]
	}
	return cmp.Compare(len(a), len(b))
}

func digits(s string) int {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	return n
}

func trimZeros(s string) string {
	for len(s) > 1 && s[0] == '0' {
		s = s[1:]
	}
	return s
}
