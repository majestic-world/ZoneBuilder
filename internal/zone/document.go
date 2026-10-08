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
	// problems caches Problems while checked is set; every change of the
	// zones clears it (changed).
	problems []Problem
	checked  bool
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
		return fmt.Errorf("zona: o ID %d não foi reservado com NewZoneID", c.ID)
	}
	if d.index(c.ID) >= 0 {
		return fmt.Errorf("zona: o ID %d já está em uso", c.ID)
	}
	if !c.Type.Valid() {
		return fmt.Errorf("zona: %q não é um tipo de zona do servidor", c.Type)
	}
	d.zones = append(d.zones, Zone{ID: c.ID, Name: c.Name, Type: c.Type})
	return nil
}

// AddShape appends a shape to a zone; it is the zone's shape number
// len(Shapes) before the command. The zero value of the optional fields
// starts an empty included polygon whose vertices come with AddVertex; a
// shape drawn in one go (a rectangle's 2 corners, a circle's polygon)
// comes whole, with its points and Z range.
type AddShape struct {
	Zone       ZoneID
	Kind       ShapeKind
	Banned     bool
	Points     []Point
	ZMin, ZMax int
}

func (c AddShape) apply(d *Document) error {
	z, err := d.zone(c.Zone)
	if err != nil {
		return err
	}
	if !c.Kind.Valid() {
		return fmt.Errorf("zona: tipo de forma %d desconhecido", c.Kind)
	}
	z.Shapes = append(z.Shapes, Shape{
		Kind: c.Kind, Banned: c.Banned, Points: slices.Clone(c.Points), ZMin: c.ZMin, ZMax: c.ZMax,
	})
	return nil
}

// AddVertex appends a vertex to a polygon; a rectangle is always its 2
// corners, so appending to one fails.
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
	if s.Kind == Rectangle {
		return fmt.Errorf("zona: a forma %d é um retângulo; seus 2 cantos são fixos", c.Shape)
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
		return nil, fmt.Errorf("zona: não há zona com ID %d", id)
	}
	return &d.zones[i], nil
}

func (d *Document) shape(id ZoneID, shape int) (*Shape, error) {
	z, err := d.zone(id)
	if err != nil {
		return nil, err
	}
	if shape < 0 || shape >= len(z.Shapes) {
		return nil, fmt.Errorf("zona: %s não tem a forma %d", z.Name, shape)
	}
	return &z.Shapes[shape], nil
}

// Compile turns the zones in selection into the server's XML: one file per
// type, zones in name order, every shape coords carrying the shape's Z
// range, exclusions as banned_polygon, restart points as x y z. Writing the
// files is up to the caller (zonexml.Write). While a selected zone has a
// problem nothing is compiled: the error is a *BlockedError.
func (d *Document) Compile(selection []ZoneID) ([]zonexml.File, error) {
	var blocked []Problem
	for _, id := range selection {
		if _, err := d.zone(id); err != nil {
			return nil, err
		}
		blocked = append(blocked, d.ZoneProblems(id)...)
	}
	if len(blocked) > 0 {
		return nil, &BlockedError{Problems: blocked}
	}
	zones := make([]zonexml.Zone, 0, len(selection))
	for _, id := range selection {
		z, err := d.zone(id)
		if err != nil {
			return nil, err
		}
		x := zonexml.Zone{
			Name:            z.Name,
			Type:            string(z.Type),
			RestartPoints:   compilePoints(z.RestartPoints),
			PKRestartPoints: compilePoints(z.PKRestartPoints),
		}
		for _, p := range z.Params {
			x.Params = append(x.Params, zonexml.Param{Name: p.Name, Value: p.Value})
		}
		for _, s := range z.Shapes {
			x.Shapes = append(x.Shapes, compileShape(s))
		}
		zones = append(zones, x)
	}
	return zonexml.Compile(zones), nil
}
