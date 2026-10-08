package zone_test

import (
	"errors"
	"testing"

	"zonebuilder/internal/zone"
)

// problemsOf is the rules zone id breaks, with the shape and vertex each
// one points at.
func problemsOf(d *zone.Document, id zone.ZoneID) []zone.Problem {
	var out []zone.Problem
	for _, p := range d.Problems() {
		if p.Zone == id {
			out = append(out, p)
		}
	}
	return out
}

func hasProblem(ps []zone.Problem, r zone.Rule, shape, vertex int) bool {
	for _, p := range ps {
		if p.Rule == r && p.Shape == shape && p.Vertex == vertex {
			return true
		}
	}
	return false
}

// A bow-tie whose crossing is edge 0→1 against edge 2→3 is a problem. The
// server's Polygon.validate starts at edge 1 and loads this zone with no
// error; a validator that copied its loop would too.
func TestBowTieCrossingEdge0IsAProblem(t *testing.T) {
	d := zone.NewDocument()
	id := d.NewZoneID()
	polygonZone(t, d, id, "[zb_bowtie]", zone.PeaceZone, -3600, -3200,
		zone.Point{X: 83000, Y: 147000}, zone.Point{X: 83100, Y: 147100},
		zone.Point{X: 83100, Y: 147000}, zone.Point{X: 83000, Y: 147100},
	)
	ps := problemsOf(d, id)
	if !hasProblem(ps, zone.SelfIntersection, 0, 0) {
		t.Fatalf("no self-intersection at shape 0, edge 0; problems: %+v", ps)
	}
	if len(ps) != 1 {
		t.Errorf("want only the crossing, got %+v", ps)
	}
}

// Two consecutive vertices at the same x y are a problem, the last and the
// first included: the server accepts the zero-length edge silently.
func TestRepeatedConsecutiveVertexIsAProblem(t *testing.T) {
	a, b, c := zone.Point{X: 83000, Y: 147000, Z: -3400}, zone.Point{X: 83400, Y: 147000}, zone.Point{X: 83400, Y: 147400}
	for _, tc := range []struct {
		name   string
		pts    []zone.Point
		vertex int // the repeated one
	}{
		{"middle", []zone.Point{a, b, b, c}, 2},
		{"last to first", []zone.Point{a, b, c, {X: a.X, Y: a.Y, Z: -3300}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := zone.NewDocument()
			id := d.NewZoneID()
			polygonZone(t, d, id, "[zb_repeat]", zone.PeaceZone, -3600, -3200, tc.pts...)
			ps := problemsOf(d, id)
			if !hasProblem(ps, zone.RepeatedVertex, 0, tc.vertex) {
				t.Fatalf("no repeated vertex %d; problems: %+v", tc.vertex, ps)
			}
			if len(ps) != 1 {
				t.Errorf("the repeat alone must be reported, got %+v", ps)
			}
		})
	}
}

// A zone whose only shape is an exclusion has no included shape: the
// server logs "Empty territory" and the zone breaks at runtime.
func TestZoneWithoutIncludedShapeIsAProblem(t *testing.T) {
	d := zone.NewDocument()
	id := d.NewZoneID()
	apply(t, d,
		zone.CreateZone{ID: id, Name: "[zb_only_banned]", Type: zone.PeaceZone},
		zone.AddShape{Zone: id, Kind: zone.Rectangle, Banned: true, ZMin: -3600, ZMax: -3200,
			Points: []zone.Point{{X: 83000, Y: 147000}, {X: 83400, Y: 147400}}},
	)
	if ps := problemsOf(d, id); !hasProblem(ps, zone.NoIncludedShape, -1, -1) {
		t.Fatalf("no NoIncludedShape problem; problems: %+v", ps)
	}
}

// Two zones with one name are each a problem: the server keeps only the
// last one it reads, across files too.
func TestDuplicateNameIsAProblemOnEachZone(t *testing.T) {
	d := zone.NewDocument()
	square := []zone.Point{{X: 83000, Y: 147000}, {X: 83400, Y: 147000}, {X: 83400, Y: 147400}}
	first, second, other := d.NewZoneID(), d.NewZoneID(), d.NewZoneID()
	polygonZone(t, d, first, "[zb_same]", zone.PeaceZone, -3600, -3200, square...)
	polygonZone(t, d, second, "[zb_same]", zone.Water, -3600, -3200, square...)
	polygonZone(t, d, other, "[zb_other]", zone.PeaceZone, -3600, -3200, square...)
	for _, id := range []zone.ZoneID{first, second} {
		if ps := problemsOf(d, id); !hasProblem(ps, zone.DuplicateName, -1, -1) {
			t.Errorf("zone %d: no DuplicateName problem; problems: %+v", id, ps)
		}
	}
	if ps := problemsOf(d, other); len(ps) != 0 {
		t.Errorf("zone with a unique name has problems: %+v", ps)
	}
}

// With an invalid zone in the selection, Compile produces no file at all,
// not even the valid zones' (so the caller has nothing to write); left out
// of the selection, the invalid zone no longer blocks.
func TestCompileWithAnInvalidSelectedZoneProducesNoFile(t *testing.T) {
	d := zone.NewDocument()
	good, bad := d.NewZoneID(), d.NewZoneID()
	polygonZone(t, d, good, "[zb_good]", zone.PeaceZone, -3600, -3200,
		zone.Point{X: 83000, Y: 147000}, zone.Point{X: 83400, Y: 147000}, zone.Point{X: 83400, Y: 147400})
	polygonZone(t, d, bad, "[zb_fishing]", zone.Fishing, -3600, -3200,
		zone.Point{X: 84000, Y: 147000}, zone.Point{X: 84400, Y: 147000}, zone.Point{X: 84400, Y: 147400})

	files, err := d.Compile([]zone.ZoneID{good, bad})
	var blocked *zone.BlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("Compile with FISHING lacking its params: err %v, want *BlockedError", err)
	}
	if len(files) != 0 {
		t.Errorf("blocked Compile returned %d files", len(files))
	}
	for _, p := range blocked.Problems {
		if p.Zone != bad {
			t.Errorf("problem of a zone that is fine: %+v", p)
		}
	}

	files, err = d.Compile([]zone.ZoneID{good})
	if err != nil || len(files) != 1 {
		t.Fatalf("Compile of the valid zone alone: %d files, err %v", len(files), err)
	}
}
