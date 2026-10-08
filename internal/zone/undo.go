package zone

import "reflect"

// history is the Undo and Redo stacks: the zones as they were before each
// undoable step, and after each undone one. Every command goes through
// Apply, which snapshots the zones (a deep clone, so commands may change
// them in place); undoing swaps the snapshot back. Commands need nothing of
// their own to be undoable, and any model field reachable from Zone is
// covered. The ID counter is not part of a step: IDs are never reused.
type history struct {
	undo, redo [][]Zone
}

// record applies c, keeping the zones from before it for Undo. On failure
// the snapshot is put back, so even a command that changed d before failing
// leaves it unchanged; a command that changes nothing records no step.
func (d *Document) record(c Command) error {
	before := clone(d.zones)
	if err := c.apply(d); err != nil {
		d.zones = before
		return err
	}
	if reflect.DeepEqual(before, d.zones) {
		return nil
	}
	d.history.undo = append(d.history.undo, before)
	d.history.redo = nil
	return nil
}

// Undo reverts the last applied step and reports whether there was one.
func (d *Document) Undo() bool {
	h := &d.history
	if len(h.undo) == 0 {
		return false
	}
	h.redo = append(h.redo, d.zones)
	d.zones = h.undo[len(h.undo)-1]
	h.undo = h.undo[:len(h.undo)-1]
	return true
}

// Redo reapplies the last undone step and reports whether there was one.
// Applying a command after Undo drops the steps left to redo.
func (d *Document) Redo() bool {
	h := &d.history
	if len(h.redo) == 0 {
		return false
	}
	h.undo = append(h.undo, d.zones)
	d.zones = h.redo[len(h.redo)-1]
	h.redo = h.redo[:len(h.redo)-1]
	return true
}

// CanUndo reports whether Undo has a step to revert.
func (d *Document) CanUndo() bool { return len(d.history.undo) > 0 }

// CanRedo reports whether Redo has a step to reapply.
func (d *Document) CanRedo() bool { return len(d.history.redo) > 0 }
