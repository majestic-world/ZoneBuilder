package zone

import "fmt"

// MoveVertex puts a shape's vertex Index at Point.
type MoveVertex struct {
	Zone  ZoneID
	Shape int
	Index int
	Point Point
}

func (c MoveVertex) apply(d *Document) error {
	s, err := d.shape(c.Zone, c.Shape)
	if err != nil {
		return err
	}
	if c.Index < 0 || c.Index >= len(s.Points) {
		return fmt.Errorf("zone: shape %d has no vertex %d", c.Shape, c.Index)
	}
	s.Points[c.Index] = c.Point
	return nil
}

// InsertVertex inserts Point into a shape so it becomes vertex Index: 0
// puts it first, len(Points) appends it, and an index in between puts it on
// the edge from vertex Index-1 to the old vertex Index. A rectangle is
// always its 2 corners: inserting into it fails.
type InsertVertex struct {
	Zone  ZoneID
	Shape int
	Index int
	Point Point
}

func (c InsertVertex) apply(d *Document) error {
	s, err := d.shape(c.Zone, c.Shape)
	if err != nil {
		return err
	}
	if s.Kind == Rectangle {
		return fmt.Errorf("zone: shape %d is a rectangle; its 2 corners are fixed", c.Shape)
	}
	if c.Index < 0 || c.Index > len(s.Points) {
		return fmt.Errorf("zone: cannot insert vertex %d into a shape of %d", c.Index, len(s.Points))
	}
	pts := make([]Point, 0, len(s.Points)+1)
	pts = append(append(append(pts, s.Points[:c.Index]...), c.Point), s.Points[c.Index:]...)
	s.Points = pts
	return nil
}

// RemoveVertex deletes a shape's vertex Index. A rectangle is always its 2
// corners: removing one fails.
type RemoveVertex struct {
	Zone  ZoneID
	Shape int
	Index int
}

func (c RemoveVertex) apply(d *Document) error {
	s, err := d.shape(c.Zone, c.Shape)
	if err != nil {
		return err
	}
	if s.Kind == Rectangle {
		return fmt.Errorf("zone: shape %d is a rectangle; its 2 corners are fixed", c.Shape)
	}
	if c.Index < 0 || c.Index >= len(s.Points) {
		return fmt.Errorf("zone: shape %d has no vertex %d", c.Shape, c.Index)
	}
	pts := make([]Point, 0, len(s.Points)-1)
	s.Points = append(append(pts, s.Points[:c.Index]...), s.Points[c.Index+1:]...)
	return nil
}

// MoveShape translates a whole shape: every vertex by DX DY DZ, and its Z
// range by DZ, so the prism keeps its height around the moved outline.
type MoveShape struct {
	Zone       ZoneID
	Shape      int
	DX, DY, DZ int
}

func (c MoveShape) apply(d *Document) error {
	s, err := d.shape(c.Zone, c.Shape)
	if err != nil {
		return err
	}
	pts := make([]Point, len(s.Points))
	for i, p := range s.Points {
		pts[i] = Point{X: p.X + c.DX, Y: p.Y + c.DY, Z: p.Z + c.DZ}
	}
	s.Points = pts
	s.ZMin += c.DZ
	s.ZMax += c.DZ
	return nil
}
