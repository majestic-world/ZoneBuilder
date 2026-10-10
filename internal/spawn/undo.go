package spawn

import "reflect"

// history is the Undo and Redo stacks: the areas as they were before each
// undoable step, and after each undone one. Every command goes through
// Apply, which snapshots the areas (a deep clone, so commands may change
// them in place); undoing swaps the snapshot back. The ID counter is not
// part of a step: IDs are never reused.
type history struct {
	undo, redo [][]Area
}

// record applies c, keeping the areas from before it for Undo. On failure
// the snapshot is put back, so even a command that changed d before failing
// leaves it unchanged; a command that changes nothing records no step.
func (d *Document) record(c Command) error {
	before := cloneAreas(d.areas)
	if err := c.apply(d); err != nil {
		d.areas = before
		return err
	}
	if reflect.DeepEqual(before, d.areas) {
		return nil
	}
	d.history.undo = append(d.history.undo, before)
	d.history.redo = nil
	d.changed()
	return nil
}

// Undo reverts the last applied step and reports whether there was one.
func (d *Document) Undo() bool {
	h := &d.history
	if len(h.undo) == 0 {
		return false
	}
	h.redo = append(h.redo, d.areas)
	d.areas = h.undo[len(h.undo)-1]
	h.undo = h.undo[:len(h.undo)-1]
	d.changed()
	return true
}

// Redo reapplies the last undone step and reports whether there was one.
// Applying a command after Undo drops the steps left to redo.
func (d *Document) Redo() bool {
	h := &d.history
	if len(h.redo) == 0 {
		return false
	}
	h.undo = append(h.undo, d.areas)
	d.areas = h.redo[len(h.redo)-1]
	h.redo = h.redo[:len(h.redo)-1]
	d.changed()
	return true
}
