package spawn

import (
	"encoding/json"
	"fmt"
)

// savedDocument is a Document as the project file stores it. The areas go
// through encoding/json as they are, so every exported field of Area (and
// of what it reaches) is saved and loaded with no change here.
type savedDocument struct {
	LastID AreaID
	Areas  []Area
}

// MarshalJSON encodes the areas, incomplete ones included, and the last
// reserved ID, so a loaded document never hands out a saved area's ID again.
func (d *Document) MarshalJSON() ([]byte, error) {
	return json.Marshal(savedDocument{LastID: d.lastID, Areas: d.areas})
}

// UnmarshalJSON replaces d with a document MarshalJSON encoded. It rejects
// areas that no command could have made (an ID used twice or not positive,
// a warning that is not a distribution warning); d is unchanged on error.
// The loaded document starts a fresh edit history.
func (d *Document) UnmarshalJSON(data []byte) error {
	var s savedDocument
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	loaded := NewDocument()
	loaded.lastID = s.LastID
	seen := make(map[AreaID]bool, len(s.Areas))
	for _, a := range s.Areas {
		switch {
		case a.ID <= 0:
			return fmt.Errorf("spawn: %s tem o ID inválido %d", a.Name, a.ID)
		case seen[a.ID]:
			return fmt.Errorf("spawn: o ID %d é usado por mais de uma área", a.ID)
		}
		for _, w := range a.Warnings {
			if !w.Rule.warning() {
				return fmt.Errorf("spawn: %s: a regra %d não é um aviso de distribuição", a.Name, w.Rule)
			}
		}
		seen[a.ID] = true
		loaded.lastID = max(loaded.lastID, a.ID)
	}
	loaded.areas = s.Areas
	*d = *loaded
	return nil
}
