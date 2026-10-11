package zone

import (
	"fmt"
	"math"
)

// Frame is a box's footprint: its center, its width along its own X axis,
// its length along its own Y axis, and Yaw, the degrees its X axis turns
// counterclockwise from the world's +X, in [0, 360) to a tenth of a degree
// (NormalYaw). Server units.
type Frame struct {
	CX, CY        int
	Width, Length int
	Yaw           float64
}

// Box is a box shape: its footprint and its Z range. A box with Yaw 0 is a
// Rectangle shape; a turned one is a Polygon of its 4 corners that keeps
// its Frame (Shape.Frame), so it can still be edited as a box.
type Box struct {
	Frame
	ZMin, ZMax int
}

// NormalYaw is yaw in degrees rounded to a tenth and brought into [0, 360).
func NormalYaw(yaw float64) float64 {
	r := math.Mod(math.Round(yaw*10)/10, 360)
	if r < 0 {
		r += 360
	}
	if r == 0 || r >= 360 { // also -0, and 359.95 rounding up
		return 0
	}
	return r
}

// BoxOf is shape s as a box: a Rectangle (its 2 corners in either order,
// Yaw 0) or a Polygon of 4 corners with a Frame; ok is false for any other
// shape.
func BoxOf(s Shape) (Box, bool) {
	switch {
	case s.Kind == Rectangle && len(s.Points) == 2:
		a, b := s.Points[0], s.Points[1]
		minX, minY := min(a.X, b.X), min(a.Y, b.Y)
		w, l := max(a.X, b.X)-minX, max(a.Y, b.Y)-minY
		return Box{Frame: Frame{CX: minX + w/2, CY: minY + l/2, Width: w, Length: l}, ZMin: s.ZMin, ZMax: s.ZMax}, true
	case s.Kind == Polygon && s.Frame != nil && len(s.Points) == 4:
		return Box{Frame: *s.Frame, ZMin: s.ZMin, ZMax: s.ZMax}, true
	}
	return Box{}, false
}

// BoxAt is the box with Yaw 0 centred on cx cy, width along X, length
// along Y, its floor at zmin and height tall.
func BoxAt(cx, cy, width, length, zmin, height int) Box {
	return Box{Frame: Frame{CX: cx, CY: cy, Width: width, Length: length}, ZMin: zmin, ZMax: zmin + height}
}

// Size is the box's width, length and height.
func (b Box) Size() (width, length, height int) {
	return b.Width, b.Length, b.ZMax - b.ZMin
}

// halves are the box's extents from its center along its own X and Y: an
// odd size puts the extra unit on the + side, so with Yaw 0 the corners
// are whole and the center is the min corner plus half the size.
func (b Box) halves() (loX, hiX, loY, hiY int) {
	return -(b.Width / 2), b.Width - b.Width/2, -(b.Length / 2), b.Length - b.Length/2
}

// Corners are the box's 4 corners, counterclockwise from its own (-X, -Y)
// corner, all at ZMin, rounded to whole units.
func (b Box) Corners() []Point {
	loX, hiX, loY, hiY := b.halves()
	local := [4][2]int{{loX, loY}, {hiX, loY}, {hiX, hiY}, {loX, hiY}}
	out := make([]Point, 4)
	for i, l := range local {
		out[i] = b.At(float64(l[0]), float64(l[1]), b.ZMin)
	}
	return out
}

// At is the point x y along the box's own axes from its center, at z.
func (b Box) At(x, y float64, z int) Point {
	if b.Yaw == 0 {
		return Point{X: b.CX + int(math.Round(x)), Y: b.CY + int(math.Round(y)), Z: z}
	}
	c, s := b.Axes()
	return Point{
		X: b.CX + int(math.Round(x*c-y*s)),
		Y: b.CY + int(math.Round(x*s+y*c)),
		Z: z,
	}
}

// Axes is the box's own X axis as a world direction (cos, sin of Yaw); its
// Y axis is (-sin, cos).
func (b Box) Axes() (cos, sin float64) {
	r := b.Yaw * math.Pi / 180
	return math.Cos(r), math.Sin(r)
}

// SetBox makes shape Shape of Zone, a box (see BoxOf), the box Box: a
// Rectangle of its min and max corners when its yaw is 0, else a Polygon of
// its 4 corners with its Frame. A shape that is no box, a box with no width
// or length, or one with a negative height fails.
type SetBox struct {
	Zone  ZoneID
	Shape int
	Box   Box
}

func (c SetBox) apply(d *Document) error {
	s, err := d.shape(c.Zone, c.Shape)
	if err != nil {
		return err
	}
	if _, ok := BoxOf(*s); !ok {
		return fmt.Errorf("zona: a forma %d não é uma caixa", c.Shape)
	}
	b := c.Box
	w, l, h := b.Size()
	if w <= 0 || l <= 0 {
		return fmt.Errorf("zona: caixa sem largura ou comprimento (%d × %d)", w, l)
	}
	if h < 0 {
		return fmt.Errorf("zona: caixa com altura %d negativa", h)
	}
	b.Yaw = NormalYaw(b.Yaw)
	corners := b.Corners()
	if b.Yaw == 0 {
		s.Kind, s.Points, s.Frame = Rectangle, []Point{corners[0], corners[2]}, nil
	} else {
		f := b.Frame
		s.Kind, s.Points, s.Frame = Polygon, corners, &f
	}
	s.ZMin, s.ZMax = b.ZMin, b.ZMax
	return nil
}

// moved is f shifted by dx dy, nil for nil. Frames are never changed in
// place: undo snapshots share nothing (clone), and neither do commands.
func (f *Frame) moved(dx, dy int) *Frame {
	if f == nil {
		return nil
	}
	m := *f
	m.CX += dx
	m.CY += dy
	return &m
}
