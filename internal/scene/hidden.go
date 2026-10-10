package scene

import (
	"cmp"
	"slices"
)

// ActorKey names a placed static mesh actor across loads of its tile: the
// tile and the actor's export index in the map package.
type ActorKey struct {
	Tile   Tile
	Export int
}

// HiddenActors is a set of static mesh actors hidden one by one, with the
// name each had when hidden. A set is never changed once made: With and
// Without return a new one, so the pointer names the contents and a cache
// can key on it. The nil set is empty.
type HiddenActors struct {
	names map[ActorKey]string
}

// Has reports whether k is in the set.
func (h *HiddenActors) Has(k ActorKey) bool {
	if h == nil {
		return false
	}
	_, ok := h.names[k]
	return ok
}

// Len is the number of actors in the set.
func (h *HiddenActors) Len() int {
	if h == nil {
		return 0
	}
	return len(h.names)
}

// Name is the name k had when it was hidden.
func (h *HiddenActors) Name(k ActorKey) string {
	if h == nil {
		return ""
	}
	return h.names[k]
}

// With is the set plus k, named name.
func (h *HiddenActors) With(k ActorKey, name string) *HiddenActors {
	out := &HiddenActors{names: make(map[ActorKey]string, h.Len()+1)}
	if h != nil {
		for key, n := range h.names {
			out.names[key] = n
		}
	}
	out.names[k] = name
	return out
}

// Without is the set minus k; nil once empty.
func (h *HiddenActors) Without(k ActorKey) *HiddenActors {
	if !h.Has(k) {
		return h
	}
	if h.Len() == 1 {
		return nil
	}
	out := &HiddenActors{names: make(map[ActorKey]string, h.Len()-1)}
	for key, n := range h.names {
		if key != k {
			out.names[key] = n
		}
	}
	return out
}

// Keys are the actors of the set by tile, then name, then export.
func (h *HiddenActors) Keys() []ActorKey {
	if h == nil {
		return nil
	}
	keys := make([]ActorKey, 0, len(h.names))
	for k := range h.names {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b ActorKey) int {
		return cmp.Or(
			cmp.Compare(a.Tile.X, b.Tile.X),
			cmp.Compare(a.Tile.Y, b.Tile.Y),
			boolCompare(a.Tile.Classic, b.Tile.Classic),
			cmp.Compare(h.names[a], h.names[b]),
			cmp.Compare(a.Export, b.Export),
		)
	})
	return keys
}

func boolCompare(a, b bool) int {
	switch {
	case a == b:
		return 0
	case b:
		return -1
	}
	return 1
}

// meshFilter is which static mesh actors Pick and Geometry leave out: all
// of them, or the hidden ones.
type meshFilter struct {
	all    bool
	hidden *HiddenActors
}

// skips reports whether set is left out.
func (f meshFilter) skips(set *triangleSet) bool {
	return set.Surface == SurfaceMesh && (f.all || f.hidden.Has(set.Actor))
}
