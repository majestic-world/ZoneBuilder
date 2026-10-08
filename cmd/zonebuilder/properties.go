package main

import (
	"log"

	"zonebuilder/internal/zone"
)

// SelectedZone is the zone the properties panel edits: the one being drawn
// or last created.
func (e *zoneEditor) SelectedZone() (zone.Zone, bool) {
	if e.zone == 0 {
		return zone.Zone{}, false
	}
	return e.doc.Zone(e.zone)
}

// Edit applies a command from the properties panel.
func (e *zoneEditor) Edit(c zone.Command) error {
	if err := e.doc.Apply(c); err != nil {
		log.Printf("zona: %v", err)
		return err
	}
	e.version++
	log.Printf("zona: %#v", c)
	return nil
}
