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
		return fmt.Errorf("zona: %q não é um tipo de zona do servidor", c.Type)
	}
	z, err := d.zone(c.Zone)
	if err != nil {
		return err
	}
	z.Type = c.Type
	return nil
}

// SetParam sets a zone parameter, written as <set name val />. A parameter
// the zone already has keeps its place in the order and takes the new
// value; a new one goes last. Any name and value are taken, except the
// names the ZoneParser keeps its own data under.
type SetParam struct {
	Zone        ZoneID
	Name, Value string
}

// reservedParams are the names the ZoneParser stores its own values under
// in the same set as the <set> parameters (zone name and type, shapes,
// restart points): a <set> with one of them breaks the zone.
var reservedParams = []string{"name", "type", "territory", "restart_points", "PKrestart_points"}

func (c SetParam) apply(d *Document) error {
	switch {
	case c.Name == "":
		return fmt.Errorf("zona: o parâmetro precisa de um nome")
	case strings.TrimSpace(c.Name) != c.Name:
		return fmt.Errorf("zona: o nome de parâmetro %q começa ou termina com espaço", c.Name)
	case slices.Contains(reservedParams, c.Name):
		return fmt.Errorf("zona: %q é reservado pelo ZoneParser do servidor", c.Name)
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
		return fmt.Errorf("zona: %s não tem o parâmetro %q", z.Name, c.Name)
	}
	z.Params = slices.Delete(z.Params, i, i+1)
	return nil
}
