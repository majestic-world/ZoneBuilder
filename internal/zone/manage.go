package zone

import (
	"fmt"
	"strconv"
	"strings"
)

// Rename sets a zone's name. Names are not checked for uniqueness here: two
// zones may share one while being edited, and the duplicate is a Problem.
type Rename struct {
	Zone ZoneID
	Name string
}

func (c Rename) apply(d *Document) error {
	z, err := d.zone(c.Zone)
	if err != nil {
		return err
	}
	z.Name = c.Name
	return nil
}

// DeleteZone removes a zone. Its ID is never handed out again.
type DeleteZone struct {
	Zone ZoneID
}

func (c DeleteZone) apply(d *Document) error {
	i := d.index(c.Zone)
	if i < 0 {
		return fmt.Errorf("zona: não há zona com ID %d", c.Zone)
	}
	d.zones = append(d.zones[:i:i], d.zones[i+1:]...)
	return nil
}

// DuplicateZone adds a deep copy of a zone (shapes and everything else)
// under ID, which comes from Document.NewZoneID, named after the original
// with a numeric suffix no zone of the document has: "[giran]" becomes
// "[giran_2]", and copying "[giran_2]" gives "[giran_3]".
type DuplicateZone struct {
	Zone ZoneID
	ID   ZoneID
}

func (c DuplicateZone) apply(d *Document) error {
	src, err := d.zone(c.Zone)
	if err != nil {
		return err
	}
	if c.ID <= 0 || c.ID > d.lastID {
		return fmt.Errorf("zona: o ID %d não foi reservado com NewZoneID", c.ID)
	}
	if d.index(c.ID) >= 0 {
		return fmt.Errorf("zona: o ID %d já está em uso", c.ID)
	}
	z := src.clone()
	z.ID = c.ID
	taken := make(map[string]bool, len(d.zones))
	for _, other := range d.zones {
		taken[other.Name] = true
	}
	z.Name = CopyName(src.Name, taken)
	d.zones = append(d.zones, z)
	return nil
}

// CopyName is name with the first "_N" suffix (N ≥ 2, counting on from a
// suffix name already has) that is not in taken. A name in brackets keeps
// them outermost, the datapack's "[name]" style. The spawn areas name
// their copies the same way.
func CopyName(name string, taken map[string]bool) string {
	open, close := "", ""
	base := name
	if strings.HasPrefix(base, "[") && strings.HasSuffix(base, "]") && len(base) >= 2 {
		open, close, base = "[", "]", base[1:len(base)-1]
	}
	n := 2
	if i := strings.LastIndexByte(base, '_'); i >= 0 {
		if k, err := strconv.Atoi(base[i+1:]); err == nil && k >= 1 && base[i+1] != '+' && base[i+1] != '-' {
			base, n = base[:i], k+1
		}
	}
	for ; ; n++ {
		if candidate := fmt.Sprintf("%s%s_%d%s", open, base, n, close); !taken[candidate] {
			return candidate
		}
	}
}
