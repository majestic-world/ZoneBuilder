package zone

import (
	"fmt"

	"zonebuilder/internal/inflect"
)

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
		return fmt.Errorf("zona: a forma %d não tem o vértice %d", c.Shape, c.Index)
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
		return fmt.Errorf("zona: a forma %d é um retângulo; seus 2 cantos são fixos", c.Shape)
	}
	if c.Index < 0 || c.Index > len(s.Points) {
		return fmt.Errorf("zona: não é possível inserir o vértice %d numa forma de %s", c.Index, inflect.Count(len(s.Points), "vértice", "vértices"))
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
		return fmt.Errorf("zona: a forma %d é um retângulo; seus 2 cantos são fixos", c.Shape)
	}
	if c.Index < 0 || c.Index >= len(s.Points) {
		return fmt.Errorf("zona: a forma %d não tem o vértice %d", c.Shape, c.Index)
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

// ShiftZoneZ raises (DZ > 0) or lowers a whole zone: every shape, its
// exclusions included, moves its vertices and Z range by DZ, so the
// exclusions keep cutting the same part of the zone. Restart points stay
// where they were clicked: they are spots on the ground, not part of the
// zone's volume.
type ShiftZoneZ struct {
	Zone ZoneID
	DZ   int
}

func (c ShiftZoneZ) apply(d *Document) error {
	z, err := d.zone(c.Zone)
	if err != nil {
		return err
	}
	for i := range z.Shapes {
		s := &z.Shapes[i]
		pts := make([]Point, len(s.Points))
		for j, p := range s.Points {
			pts[j] = Point{X: p.X, Y: p.Y, Z: p.Z + c.DZ}
		}
		s.Points = pts
		s.ZMin += c.DZ
		s.ZMax += c.DZ
	}
	return nil
}

// SetZoneHeight makes every shape of a zone Height tall: each keeps its
// floor (ZMin) and its top becomes ZMin + Height.
type SetZoneHeight struct {
	Zone   ZoneID
	Height int
}

func (c SetZoneHeight) apply(d *Document) error {
	if c.Height < 0 {
		return fmt.Errorf("zona: altura %d negativa", c.Height)
	}
	z, err := d.zone(c.Zone)
	if err != nil {
		return err
	}
	for i := range z.Shapes {
		z.Shapes[i].ZMax = z.Shapes[i].ZMin + c.Height
	}
	return nil
}
