package zone

import (
	"fmt"
	"slices"
	"strings"
)

// SetType changes a zone's type. Only the 23 server values, in their exact
// case, are accepted.
type SetType struct {
	Zone ZoneID
	Type Type
}

func (c SetType) apply(d *Document) error {
	if !c.Type.Valid() {
		return fmt.Errorf("zone: %q is not a server zone type", c.Type)
	}
	z, err := d.zone(c.Zone)
	if err != nil {
		return err
	}
	z.Type = c.Type
	return nil
}

// SetParam sets a zone parameter. A parameter the zone already has keeps
// its place in the order and takes the new value; a new one goes last.
// A known parameter (KnownParams) takes only values of its kind; any other
// name is a free parameter and takes any value, except the names the
// ZoneParser keeps its own data under.
type SetParam struct {
	Zone        ZoneID
	Name, Value string
}

func (c SetParam) apply(d *Document) error {
	switch {
	case c.Name == "":
		return fmt.Errorf("zone: a parameter needs a name")
	case strings.TrimSpace(c.Name) != c.Name:
		return fmt.Errorf("zone: parameter name %q starts or ends with a space", c.Name)
	case slices.Contains(reservedParams, c.Name):
		return fmt.Errorf("zone: %q is reserved by the server's ZoneParser", c.Name)
	}
	if spec, ok := KnownParam(c.Name); ok {
		if err := spec.Check(c.Value); err != nil {
			return err
		}
	}
	z, err := d.zone(c.Zone)
	if err != nil {
		return err
	}
	if i := slices.IndexFunc(z.Params, func(p Param) bool { return p.Name == c.Name }); i >= 0 {
		z.Params[i].Value = c.Value
		return nil
	}
	z.Params = append(z.Params, Param{Name: c.Name, Value: c.Value})
	return nil
}

// RemoveParam removes a zone parameter; the zone must have it.
type RemoveParam struct {
	Zone ZoneID
	Name string
}

func (c RemoveParam) apply(d *Document) error {
	z, err := d.zone(c.Zone)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(z.Params, func(p Param) bool { return p.Name == c.Name })
	if i < 0 {
		return fmt.Errorf("zone: %s has no parameter %q", z.Name, c.Name)
	}
	z.Params = slices.Delete(z.Params, i, i+1)
	return nil
}
