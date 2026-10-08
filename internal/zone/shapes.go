package zone

import (
	"fmt"
	"math"

	"zonebuilder/internal/zonexml"
)

// ShapeKind is what a Shape's points describe.
type ShapeKind int

const (
	// Polygon is a list of vertices, compiled as <polygon>.
	Polygon ShapeKind = iota
	// Rectangle is 2 opposite corners, compiled as <rectangle>.
	Rectangle
)

// Valid reports whether k is Polygon or Rectangle.
func (k ShapeKind) Valid() bool { return k == Polygon || k == Rectangle }

// CircleSides is how many vertices CirclePoints gives a circle.
const CircleSides = 32

// CirclePoints is the polygon of n vertices inscribed in the circle of
// radius r (server units) around center, counterclockwise from +X, every
// vertex at center's Z. The server's own <circle> tests its bounding square
// (Circle.isInside always passes), so a drawn circle reaches the Document
// as this polygon instead. Rounding to whole units can merge neighbours of
// a tiny circle; those are dropped, so fewer than n may come back.
func CirclePoints(center Point, r, n int) []Point {
	pts := make([]Point, 0, n)
	for i := range n {
		a := 2 * math.Pi * float64(i) / float64(n)
		p := Point{
			X: center.X + int(math.Round(float64(r)*math.Cos(a))),
			Y: center.Y + int(math.Round(float64(r)*math.Sin(a))),
			Z: center.Z,
		}
		if len(pts) > 0 && (pts[len(pts)-1] == p || (i == n-1 && pts[0] == p)) {
			continue
		}
		pts = append(pts, p)
	}
	return pts
}

// RectangleCorners is the outline of the rectangle with opposite corners a
// and b: a, then counterclockwise or clockwise around, each corner at the Z
// of the given corner it shares an X with.
func RectangleCorners(a, b Point) [4]Point {
	return [4]Point{a, {X: b.X, Y: a.Y, Z: b.Z}, b, {X: a.X, Y: b.Y, Z: a.Z}}
}

// AddRestartPoint appends a point to a zone's restart points, or to its
// player-killer restart points when PK is set.
type AddRestartPoint struct {
	Zone  ZoneID
	PK    bool
	Point Point
}

func (c AddRestartPoint) apply(d *Document) error {
	z, err := d.zone(c.Zone)
	if err != nil {
		return err
	}
	pts := z.restartPoints(c.PK)
	*pts = append(*pts, c.Point)
	return nil
}

// RemoveRestartPoint removes restart point Index of a zone, from its
// player-killer restart points when PK is set.
type RemoveRestartPoint struct {
	Zone  ZoneID
	PK    bool
	Index int
}

func (c RemoveRestartPoint) apply(d *Document) error {
	z, err := d.zone(c.Zone)
	if err != nil {
		return err
	}
	pts := z.restartPoints(c.PK)
	if c.Index < 0 || c.Index >= len(*pts) {
		return fmt.Errorf("zone: %s has no restart point %d", z.Name, c.Index)
	}
	*pts = append((*pts)[:c.Index:c.Index], (*pts)[c.Index+1:]...)
	return nil
}

func (z *Zone) restartPoints(pk bool) *[]Point {
	if pk {
		return &z.PKRestartPoints
	}
	return &z.RestartPoints
}

// compileShape is s for the XML. An exclusion is always a banned_polygon,
// so a banned rectangle goes out as its 4 corners.
func compileShape(s Shape) zonexml.Shape {
	pts := s.Points
	rect := s.Kind == Rectangle
	if rect && s.Banned && len(pts) == 2 {
		c := RectangleCorners(pts[0], pts[1])
		pts, rect = c[:], false
	}
	x := zonexml.Shape{Rectangle: rect, Banned: s.Banned, Points: make([][2]int, len(pts)), ZMin: s.ZMin, ZMax: s.ZMax}
	for i, p := range pts {
		x.Points[i] = [2]int{p.X, p.Y}
	}
	return x
}

// compilePoints is pts as x y z triples, nil for none.
func compilePoints(pts []Point) [][3]int {
	if len(pts) == 0 {
		return nil
	}
	out := make([][3]int, len(pts))
	for i, p := range pts {
		out[i] = [3]int{p.X, p.Y, p.Z}
	}
	return out
}
