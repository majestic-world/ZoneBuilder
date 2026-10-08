package zone

import (
	"fmt"
	"slices"

	"zonebuilder/internal/zonexml"
)

// Document holds a project's zones. It changes only through Apply.
type Document struct {
	zones   []Zone
	lastID  ZoneID
	history history
}

// NewDocument returns an empty document.
func NewDocument() *Document { return &Document{} }

// NewZoneID reserves an ID no zone of d has used, for a CreateZone.
func (d *Document) NewZoneID() ZoneID {
	d.lastID++
	return d.lastID
}

// Zones returns the zones in creation order. The slice and everything it
// reaches belong to d: read them, never modify them.
func (d *Document) Zones() []Zone { return d.zones }

// Zone returns the zone with id.
func (d *Document) Zone(id ZoneID) (Zone, bool) {
	i := d.index(id)
	if i < 0 {
		return Zone{}, false
	}
	return d.zones[i], true
}

// ZoneIDs returns the ID of every zone, in creation order.
func (d *Document) ZoneIDs() []ZoneID {
	ids := make([]ZoneID, len(d.zones))
	for i, z := range d.zones {
		ids[i] = z.ID
	}
	return ids
}

func (d *Document) index(id ZoneID) int {
	return slices.IndexFunc(d.zones, func(z Zone) bool { return z.ID == id })
}

// Command is one edit of a Document.
type Command interface {
	apply(d *Document) error
}

// Apply runs c on d and records the step for Undo. A command that fails
// leaves d unchanged and records nothing.
func (d *Document) Apply(c Command) error { return d.record(c) }

// CreateZone adds an empty zone. ID comes from Document.NewZoneID.
type CreateZone struct {
	ID   ZoneID
	Name string
	Type Type
}

func (c CreateZone) apply(d *Document) error {
	if c.ID <= 0 || c.ID > d.lastID {
		return fmt.Errorf("zone: ID %d was not reserved with NewZoneID", c.ID)
	}
	if d.index(c.ID) >= 0 {
		return fmt.Errorf("zone: ID %d is already in use", c.ID)
	}
	if !c.Type.Valid() {
		return fmt.Errorf("zone: %q is not a server zone type", c.Type)
	}
	d.zones = append(d.zones, Zone{ID: c.ID, Name: c.Name, Type: c.Type})
	return nil
}

// AddShape appends an empty polygon to a zone; it is the zone's shape
// number len(Shapes) before the command.
type AddShape struct {
	Zone ZoneID
}

func (c AddShape) apply(d *Document) error {
	z, err := d.zone(c.Zone)
	if err != nil {
		return err
	}
	z.Shapes = append(z.Shapes, Shape{})
	return nil
}

// AddVertex appends a vertex to a shape.
type AddVertex struct {
	Zone  ZoneID
	Shape int
	Point Point
}

func (c AddVertex) apply(d *Document) error {
	s, err := d.shape(c.Zone, c.Shape)
	if err != nil {
		return err
	}
	s.Points = append(s.Points, c.Point)
	return nil
}

// SetZRange sets the Z range a shape spans.
type SetZRange struct {
	Zone       ZoneID
	Shape      int
	ZMin, ZMax int
}

func (c SetZRange) apply(d *Document) error {
	s, err := d.shape(c.Zone, c.Shape)
	if err != nil {
		return err
	}
	s.ZMin, s.ZMax = c.ZMin, c.ZMax
	return nil
}

func (d *Document) zone(id ZoneID) (*Zone, error) {
	i := d.index(id)
	if i < 0 {
		return nil, fmt.Errorf("zone: no zone with ID %d", id)
	}
	return &d.zones[i], nil
}

func (d *Document) shape(id ZoneID, shape int) (*Shape, error) {
	z, err := d.zone(id)
	if err != nil {
		return nil, err
	}
	if shape < 0 || shape >= len(z.Shapes) {
		return nil, fmt.Errorf("zone: %s has no shape %d", z.Name, shape)
	}
	return &z.Shapes[shape], nil
}

// Compile turns the zones in selection into the server's XML: one file per
// type, zones in name order, every polygon coords carrying the shape's Z
// range. Writing the files is up to the caller (zonexml.Write).
func (d *Document) Compile(selection []ZoneID) ([]zonexml.File, error) {
	zones := make([]zonexml.Zone, 0, len(selection))
	for _, id := range selection {
		z, err := d.zone(id)
		if err != nil {
			return nil, err
		}
		x := zonexml.Zone{Name: z.Name, Type: string(z.Type)}
		for _, p := range z.Params {
			x.Params = append(x.Params, zonexml.Param{Name: p.Name, Value: p.Value})
		}
		for _, s := range z.Shapes {
			p := zonexml.Polygon{Points: make([][2]int, len(s.Points)), ZMin: s.ZMin, ZMax: s.ZMax}
			for i, pt := range s.Points {
				p.Points[i] = [2]int{pt.X, pt.Y}
			}
			x.Polygons = append(x.Polygons, p)
		}
		zones = append(zones, x)
	}
	return zonexml.Compile(zones), nil
}
