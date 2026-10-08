package zone

import (
	"encoding/json"
	"fmt"
)

// savedDocument is a Document as the project file stores it. The zones go
// through encoding/json as they are, so every exported field of Zone and
// Shape (and of what they reach) is saved and loaded with no change here.
type savedDocument struct {
	LastID ZoneID
	Zones  []Zone
}

// MarshalJSON encodes the zones, incomplete ones included, and the last
// reserved ID, so a loaded document never hands out a saved zone's ID again.
func (d *Document) MarshalJSON() ([]byte, error) {
	return json.Marshal(savedDocument{LastID: d.lastID, Zones: d.zones})
}

// UnmarshalJSON replaces d with a document MarshalJSON encoded. It rejects
// zones that no command could have made (an ID used twice or not positive,
// a type outside the server enum, an unknown shape kind); d is unchanged on
// error. The loaded document starts a fresh edit history.
func (d *Document) UnmarshalJSON(data []byte) error {
	var s savedDocument
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	loaded := NewDocument()
	loaded.lastID = s.LastID
	seen := make(map[ZoneID]bool, len(s.Zones))
	for _, z := range s.Zones {
		switch {
		case z.ID <= 0:
			return fmt.Errorf("zona: %s tem o ID inválido %d", z.Name, z.ID)
		case seen[z.ID]:
			return fmt.Errorf("zona: o ID %d é usado por mais de uma zona", z.ID)
		case !z.Type.Valid():
			return fmt.Errorf("zona: %s: %q não é um tipo de zona do servidor", z.Name, z.Type)
		}
		for i, sh := range z.Shapes {
			if !sh.Kind.Valid() {
				return fmt.Errorf("zona: %s: a forma %d tem o tipo desconhecido %d", z.Name, i, sh.Kind)
			}
		}
		seen[z.ID] = true
		loaded.lastID = max(loaded.lastID, z.ID)
	}
	loaded.zones = s.Zones
	*d = *loaded
	return nil
}
