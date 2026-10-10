package spawn

import (
	"fmt"
	"slices"

	"zonebuilder/internal/zone"
)

// Document holds a project's spawn areas. It changes only through Apply.
type Document struct {
	areas   []Area
	lastID  AreaID
	history history
	// problems caches Problems while checked is set; every change of the
	// areas clears it (changed).
	problems []Problem
	checked  bool
}

// NewDocument returns an empty document.
func NewDocument() *Document { return &Document{} }

// NewAreaID reserves an ID no area of d has used, for a CreateArea or a
// DuplicateArea.
func (d *Document) NewAreaID() AreaID {
	d.lastID++
	return d.lastID
}

// Areas returns the areas in creation order. The slice and everything it
// reaches belong to d: read them, never modify them.
func (d *Document) Areas() []Area { return d.areas }

// Area returns the area with id.
func (d *Document) Area(id AreaID) (Area, bool) {
	i := d.index(id)
	if i < 0 {
		return Area{}, false
	}
	return d.areas[i], true
}

func (d *Document) index(id AreaID) int {
	return slices.IndexFunc(d.areas, func(a Area) bool { return a.ID == id })
}

func (d *Document) area(id AreaID) (*Area, error) {
	i := d.index(id)
	if i < 0 {
		return nil, fmt.Errorf("spawn: não há área com ID %d", id)
	}
	return &d.areas[i], nil
}

// Command is one edit of a Document.
type Command interface {
	apply(d *Document) error
}

// Apply runs c on d and records the step for Undo. A command that fails
// leaves d unchanged and records nothing.
func (d *Document) Apply(c Command) error { return d.record(c) }

// Batch runs its commands in order as one step: one Undo reverts them
// all, and when one fails none of them is applied.
type Batch []Command

func (b Batch) apply(d *Document) error {
	for _, c := range b {
		if err := c.apply(d); err != nil {
			return err
		}
	}
	return nil
}

// CreateArea adds an area. ID comes from Document.NewAreaID. A rectangle
// or a circle comes as the polygon of its outline; a polygon drawn click by
// click may start with fewer vertices and grow with InsertVertex.
type CreateArea struct {
	ID         AreaID
	Name       string
	Outline    []Vertex
	ZMin, ZMax int
	Params     Params
}

func (c CreateArea) apply(d *Document) error {
	if err := d.reserved(c.ID); err != nil {
		return err
	}
	d.areas = append(d.areas, Area{
		ID: c.ID, Name: c.Name, Outline: slices.Clone(c.Outline),
		ZMin: c.ZMin, ZMax: c.ZMax, Params: c.Params,
	})
	return nil
}

// reserved fails unless id came from NewAreaID and no area has it.
func (d *Document) reserved(id AreaID) error {
	if id <= 0 || id > d.lastID {
		return fmt.Errorf("spawn: o ID %d não foi reservado com NewAreaID", id)
	}
	if d.index(id) >= 0 {
		return fmt.Errorf("spawn: o ID %d já está em uso", id)
	}
	return nil
}

// DeleteArea removes an area. Its ID is never handed out again.
type DeleteArea struct {
	Area AreaID
}

func (c DeleteArea) apply(d *Document) error {
	i := d.index(c.Area)
	if i < 0 {
		return fmt.Errorf("spawn: não há área com ID %d", c.Area)
	}
	d.areas = append(d.areas[:i:i], d.areas[i+1:]...)
	return nil
}

// DuplicateArea adds a deep copy of an area (outline, params, points and
// the fingerprint of its last distribution) under ID, which comes from
// Document.NewAreaID, named after the original with a numeric suffix no
// area of the document has: "giran" becomes "giran_2" (zone.CopyName).
type DuplicateArea struct {
	Area AreaID
	ID   AreaID
}

func (c DuplicateArea) apply(d *Document) error {
	src, err := d.area(c.Area)
	if err != nil {
		return err
	}
	if err := d.reserved(c.ID); err != nil {
		return err
	}
	a := src.clone()
	a.ID = c.ID
	taken := make(map[string]bool, len(d.areas))
	for _, other := range d.areas {
		taken[other.Name] = true
	}
	a.Name = zone.CopyName(src.Name, taken)
	d.areas = append(d.areas, a)
	return nil
}

// Rename sets an area's name. Names are not checked here: an empty or
// repeated name is a Problem.
type Rename struct {
	Area AreaID
	Name string
}

func (c Rename) apply(d *Document) error {
	a, err := d.area(c.Area)
	if err != nil {
		return err
	}
	a.Name = c.Name
	return nil
}

// SetParams replaces an area's Params. Values are not checked here: the
// ones the server rejects are Problems.
type SetParams struct {
	Area   AreaID
	Params Params
}

func (c SetParams) apply(d *Document) error {
	a, err := d.area(c.Area)
	if err != nil {
		return err
	}
	a.Params = c.Params
	return nil
}

// SetZRange sets the Z range an area spans.
type SetZRange struct {
	Area       AreaID
	ZMin, ZMax int
}

func (c SetZRange) apply(d *Document) error {
	a, err := d.area(c.Area)
	if err != nil {
		return err
	}
	a.ZMin, a.ZMax = c.ZMin, c.ZMax
	return nil
}

// SetHidden shows or hides areas in the viewport.
type SetHidden struct {
	Areas  []AreaID
	Hidden bool
}

func (c SetHidden) apply(d *Document) error {
	for _, id := range c.Areas {
		a, err := d.area(id)
		if err != nil {
			return err
		}
		a.Hidden = c.Hidden
	}
	return nil
}

// MoveVertex puts an outline vertex Index at Point.
type MoveVertex struct {
	Area  AreaID
	Index int
	Point Vertex
}

func (c MoveVertex) apply(d *Document) error {
	a, err := d.area(c.Area)
	if err != nil {
		return err
	}
	if c.Index < 0 || c.Index >= len(a.Outline) {
		return fmt.Errorf("spawn: %s não tem o vértice %d", a.Name, c.Index)
	}
	a.Outline[c.Index] = c.Point
	return nil
}

// InsertVertex inserts Point into an outline so it becomes vertex Index: 0
// puts it first, len(Outline) appends it, and an index in between puts it
// on the edge from vertex Index-1 to the old vertex Index.
type InsertVertex struct {
	Area  AreaID
	Index int
	Point Vertex
}

func (c InsertVertex) apply(d *Document) error {
	a, err := d.area(c.Area)
	if err != nil {
		return err
	}
	if c.Index < 0 || c.Index > len(a.Outline) {
		return fmt.Errorf("spawn: não é possível inserir o vértice %d no contorno de %s", c.Index, a.Name)
	}
	a.Outline = slices.Insert(a.Outline, c.Index, c.Point)
	return nil
}

// RemoveVertex deletes an outline vertex Index.
type RemoveVertex struct {
	Area  AreaID
	Index int
}

func (c RemoveVertex) apply(d *Document) error {
	a, err := d.area(c.Area)
	if err != nil {
		return err
	}
	if c.Index < 0 || c.Index >= len(a.Outline) {
		return fmt.Errorf("spawn: %s não tem o vértice %d", a.Name, c.Index)
	}
	a.Outline = slices.Delete(a.Outline, c.Index, c.Index+1)
	return nil
}

// MoveArea translates an area's outline by DX DY and its Z range by DZ.
// The points stay where they are: the area becomes stale and the next
// distribution places them in the moved outline.
type MoveArea struct {
	Area       AreaID
	DX, DY, DZ int
}

func (c MoveArea) apply(d *Document) error {
	a, err := d.area(c.Area)
	if err != nil {
		return err
	}
	for i, v := range a.Outline {
		a.Outline[i] = Vertex{X: v.X + c.DX, Y: v.Y + c.DY}
	}
	a.ZMin += c.DZ
	a.ZMax += c.DZ
	return nil
}

// SetPoints is the result of a distribution: it replaces an area's points,
// seed and warnings and records Fingerprint (Area.Fingerprint of the inputs
// the distribution read, with Seed) as the inputs of its points, in one
// undoable step.
type SetPoints struct {
	Area        AreaID
	Seed        uint64
	Fingerprint Fingerprint
	Points      []Point
	Warnings    []Warning
}

func (c SetPoints) apply(d *Document) error {
	a, err := d.area(c.Area)
	if err != nil {
		return err
	}
	if c.Fingerprint == "" {
		return fmt.Errorf("spawn: os pontos de %s vieram sem a impressão das entradas", a.Name)
	}
	for _, w := range c.Warnings {
		if !w.Rule.warning() {
			return fmt.Errorf("spawn: a regra %d não é um aviso de distribuição", w.Rule)
		}
	}
	a.Seed, a.Generated = c.Seed, c.Fingerprint
	a.Points = slices.Clone(c.Points)
	a.Warnings = slices.Clone(c.Warnings)
	return nil
}

// MovePoint puts an area's point Index at Point. Its inputs stay the same:
// a moved point never makes the area stale.
type MovePoint struct {
	Area  AreaID
	Index int
	Point Point
}

func (c MovePoint) apply(d *Document) error {
	a, err := d.area(c.Area)
	if err != nil {
		return err
	}
	if c.Index < 0 || c.Index >= len(a.Points) {
		return fmt.Errorf("spawn: %s não tem o ponto %d", a.Name, c.Index)
	}
	a.Points[c.Index] = c.Point
	return nil
}

// RemovePoint deletes an area's point Index.
type RemovePoint struct {
	Area  AreaID
	Index int
}

func (c RemovePoint) apply(d *Document) error {
	a, err := d.area(c.Area)
	if err != nil {
		return err
	}
	if c.Index < 0 || c.Index >= len(a.Points) {
		return fmt.Errorf("spawn: %s não tem o ponto %d", a.Name, c.Index)
	}
	a.Points = slices.Delete(a.Points, c.Index, c.Index+1)
	return nil
}

// AddPoint appends a point to an area.
type AddPoint struct {
	Area  AreaID
	Point Point
}

func (c AddPoint) apply(d *Document) error {
	a, err := d.area(c.Area)
	if err != nil {
		return err
	}
	a.Points = append(a.Points, c.Point)
	return nil
}
