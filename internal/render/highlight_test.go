package render

import (
	"reflect"
	"testing"

	"zonebuilder/internal/scene"
)

func TestSectionRangesPerBatch(t *testing.T) {
	target := scene.ActorKey{Export: 7}
	other := scene.ActorKey{Export: 8}
	actors := []scene.MeshActor{
		{Export: 8, Sections: []scene.MeshActorSection{{Batch: 0, First: 0, Count: 3}}},
		{Export: 7, Sections: []scene.MeshActorSection{
			{Batch: 1, First: 9, Count: 6},
			{Batch: 0, First: 3, Count: 3},
			{Batch: 1, First: 3, Count: 3},
			{Batch: -1, First: 0, Count: 0},
		}},
	}
	got := sectionRanges(actors, func(k scene.ActorKey) bool { return k == target })
	want := map[int][]indexRange{0: {{3, 3}}, 1: {{3, 3}, {9, 6}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ranges of the target = %v, want %v", got, want)
	}
	if got := sectionRanges(actors, func(k scene.ActorKey) bool { return k == other }); !reflect.DeepEqual(got, map[int][]indexRange{0: {{0, 3}}}) {
		t.Fatalf("ranges of the other actor = %v", got)
	}
	if got := sectionRanges(actors, func(scene.ActorKey) bool { return false }); len(got) != 0 {
		t.Fatalf("ranges with no actor wanted = %v, want none", got)
	}
}

func TestGatherRangesKeepsOnlyTheRanges(t *testing.T) {
	src := []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	got := gatherRanges(src, []indexRange{{3, 3}, {9, 3}})
	if want := []uint32{3, 4, 5, 9, 10, 11}; !reflect.DeepEqual(got, want) {
		t.Fatalf("gathered = %v, want %v", got, want)
	}
}
