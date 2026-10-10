package spawn

// CloneArea is the deep copy the undo history snapshots with, for tests
// that keep states of the document.
func CloneArea(a Area) Area { return a.clone() }
