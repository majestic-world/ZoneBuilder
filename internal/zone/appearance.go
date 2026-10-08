package zone

import (
	"fmt"
	"slices"
)

// Color is an sRGB colour: red, green, blue, 8 bits each.
type Color [3]uint8

// typeColors is each Type's viewport colour, in Types order: distinct hues
// so neighbouring zones of different types tell apart at a glance.
var typeColors = [...]Color{
	{0xE6, 0x19, 0x4B}, // SIEGE
	{0x91, 0x1E, 0xB4}, // RESIDENCE
	{0x80, 0x00, 0x00}, // HEADQUARTER
	{0x42, 0xD4, 0xF4}, // FISHING
	{0x43, 0x63, 0xD8}, // water
	{0xF5, 0x82, 0x31}, // battle_zone
	{0xF0, 0x32, 0xE6}, // damage
	{0xDC, 0xBE, 0xFF}, // instant_skill
	{0x3C, 0xB4, 0x4B}, // mother_tree
	{0xAA, 0xFF, 0xC3}, // peace_zone
	{0xBF, 0xEF, 0x45}, // poison
	{0x2F, 0x4F, 0x8F}, // ssq_zone
	{0x80, 0x80, 0x00}, // swamp
	{0xFA, 0xBE, 0xD4}, // no_escape
	{0xFF, 0xD8, 0xB1}, // no_landing
	{0x9A, 0x63, 0x24}, // no_restart
	{0x46, 0x99, 0x90}, // no_summon
	{0xA9, 0xA9, 0xA9}, // dummy
	{0xFF, 0xFA, 0xC8}, // offshore
	{0xFF, 0xE1, 0x19}, // epic
	{0xFF, 0x69, 0xB4}, // fun
	{0xFF, 0xFF, 0xFF}, // buff_store
	{0xC7, 0x15, 0x85}, // JUMPING
}

// Color is t's viewport colour; a Type outside the enum is grey.
func (t Type) Color() Color {
	if i := slices.Index(Types, t); i >= 0 {
		return typeColors[i]
	}
	return Color{0x80, 0x80, 0x80}
}

// DisplayColor is the colour z is drawn in: its own Color when set, else
// its type's.
func (z Zone) DisplayColor() Color {
	if z.Color != (Color{}) {
		return z.Color
	}
	return z.Type.Color()
}

// SetHidden hides or shows zones in the viewport: one zone, or every zone
// of a type at once (one edit to undo).
type SetHidden struct {
	Zones  []ZoneID
	Hidden bool
}

func (c SetHidden) apply(d *Document) error {
	for _, id := range c.Zones {
		if d.index(id) < 0 {
			return fmt.Errorf("zona: não há zona com ID %d", id)
		}
	}
	for _, id := range c.Zones {
		z, _ := d.zone(id)
		z.Hidden = c.Hidden
	}
	return nil
}

// SetColor sets a zone's own viewport colour; the zero Color goes back to
// its type's.
type SetColor struct {
	Zone  ZoneID
	Color Color
}

func (c SetColor) apply(d *Document) error {
	z, err := d.zone(c.Zone)
	if err != nil {
		return err
	}
	z.Color = c.Color
	return nil
}
