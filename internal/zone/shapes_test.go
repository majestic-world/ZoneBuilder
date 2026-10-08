package zone_test

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"testing"

	"zonebuilder/internal/zone"
)

// compileOne compiles every zone of d and returns the single file's text.
func compileOne(t *testing.T, d *zone.Document) string {
	t.Helper()
	files, err := d.Compile(d.ZoneIDs())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	return string(files[0].Data)
}

// A rectangle compiles as <rectangle> with its 2 corners, each coords of 4
// numbers carrying the shape's Z range. Catches a rectangle written as a
// 4-vertex polygon, with 3-number coords (the ZoneParser then throws and
// drops the rest of the file) or with fewer than 2 coords (the parser's
// iterator.next() throws).
func TestRectangleCompilesAsRectangleWith2Corners(t *testing.T) {
	d := zone.NewDocument()
	id := d.NewZoneID()
	apply(t, d,
		zone.CreateZone{ID: id, Name: "[zb_rect]", Type: zone.PeaceZone},
		zone.AddShape{Zone: id, Kind: zone.Rectangle, ZMin: -3700, ZMax: -3100, Points: []zone.Point{
			{X: 82000, Y: 148000, Z: -3467}, {X: 82800, Y: 148700, Z: -3404},
		}},
	)
	got := compileOne(t, d)
	want := "\t\t<rectangle>\n" +
		"\t\t\t<coords loc=\"82000 148000 -3700 -3100\" />\n" +
		"\t\t\t<coords loc=\"82800 148700 -3700 -3100\" />\n" +
		"\t\t</rectangle>\n"
	if !strings.Contains(got, want) {
		t.Errorf("no rectangle with 2 corners in:\n%s", got)
	}
	if strings.Contains(got, "<polygon>") {
		t.Errorf("the rectangle was also written as a polygon:\n%s", got)
	}
}

// A circle reaches the Document as a polygon and compiles as one: every
// coords lies on the drawn circle, and no file mentions circle. Catches a
// <circle> in the output, which the server tests as its bounding square
// (Circle.isInside always passes), and a polygon off the drawn radius.
func TestCircleCompilesAsPolygonOnTheCircle(t *testing.T) {
	d := zone.NewDocument()
	id := d.NewZoneID()
	center := zone.Point{X: 81000, Y: 148000, Z: -3467}
	const r = 300
	apply(t, d,
		zone.CreateZone{ID: id, Name: "[zb_round]", Type: zone.Dummy},
		zone.AddShape{Zone: id, Points: zone.CirclePoints(center, r, zone.CircleSides), ZMin: -3723, ZMax: -3211},
	)
	got := compileOne(t, d)
	if strings.Contains(strings.ToLower(got), "circle") {
		t.Errorf("the output mentions circle:\n%s", got)
	}
	if strings.Count(got, "<polygon>") != 1 {
		t.Fatalf("want 1 polygon:\n%s", got)
	}
	locs := coordsLoc.FindAllStringSubmatch(got, -1)
	if len(locs) < 16 {
		t.Fatalf("got %d coords, want a polygon fine enough to pass for a circle:\n%s", len(locs), got)
	}
	for _, m := range locs {
		var x, y, zmin, zmax int
		if _, err := fmt.Sscanf(m[1], "%d %d %d %d", &x, &y, &zmin, &zmax); err != nil {
			t.Fatalf("coords %q: %v", m[1], err)
		}
		if dist := math.Hypot(float64(x-center.X), float64(y-center.Y)); math.Abs(dist-r) > 1 {
			t.Errorf("coords %q is %.1f from the center, want %d", m[1], dist, r)
		}
		if zmin != -3723 || zmax != -3211 {
			t.Errorf("coords %q does not carry the Z range -3723 -3211", m[1])
		}
	}
}

var shapeBlock = regexp.MustCompile(`(?s)<(polygon|rectangle|banned_\w+)>(.*?)</`)

// shapeBlocks lists the shape elements of xml in order with their coords,
// e.g. "polygon 1 2 -10 10; 3 4 -10 10".
func shapeBlocks(xml string) []string {
	var out []string
	for _, m := range shapeBlock.FindAllStringSubmatch(xml, -1) {
		var locs []string
		for _, c := range coordsLoc.FindAllStringSubmatch(m[2], -1) {
			locs = append(locs, c[1])
		}
		out = append(out, m[1]+" "+strings.Join(locs, "; "))
	}
	return out
}

// A zone with 2 included polygons and an exclusion compiles each included
// one as its own <polygon> and the exclusion as <banned_polygon>. Catches
// the exclusion written as an included shape (the area it should cut out
// would join the zone) or the shapes merged.
func TestExclusionCompilesAsBannedPolygon(t *testing.T) {
	d := zone.NewDocument()
	id := d.NewZoneID()
	apply(t, d,
		zone.CreateZone{ID: id, Name: "[zb_market]", Type: zone.PeaceZone},
		zone.AddShape{Zone: id, ZMin: -3700, ZMax: -3100, Points: []zone.Point{
			{X: 83000, Y: 147600, Z: -3404}, {X: 83800, Y: 147600, Z: -3398}, {X: 83800, Y: 148300, Z: -3405},
		}},
		zone.AddShape{Zone: id, ZMin: -3800, ZMax: -3200, Points: []zone.Point{
			{X: 81000, Y: 145000, Z: -3533}, {X: 81500, Y: 145000, Z: -3533}, {X: 81500, Y: 145500, Z: -3533}, {X: 81000, Y: 145500, Z: -3533},
		}},
		zone.AddShape{Zone: id, Banned: true, ZMin: -3600, ZMax: -3300, Points: []zone.Point{
			{X: 83500, Y: 147700, Z: -3404}, {X: 83700, Y: 147700, Z: -3404}, {X: 83700, Y: 147900, Z: -3404},
		}},
	)
	got := shapeBlocks(compileOne(t, d))
	want := []string{
		"polygon 83000 147600 -3700 -3100; 83800 147600 -3700 -3100; 83800 148300 -3700 -3100",
		"polygon 81000 145000 -3800 -3200; 81500 145000 -3800 -3200; 81500 145500 -3800 -3200; 81000 145500 -3800 -3200",
		"banned_polygon 83500 147700 -3600 -3300; 83700 147700 -3600 -3300; 83700 147900 -3600 -3300",
	}
	if !slices.Equal(got, want) {
		t.Errorf("shapes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// An exclusion drawn with the rectangle tool still compiles as a
// banned_polygon, through its 4 corners. Catches a banned_rectangle in the
// output, the element the spec rules out for exclusions.
func TestBannedRectangleCompilesAsBannedPolygonOf4Corners(t *testing.T) {
	d := zone.NewDocument()
	id := d.NewZoneID()
	apply(t, d,
		zone.CreateZone{ID: id, Name: "[zb_market]", Type: zone.PeaceZone},
		zone.AddShape{Zone: id, Kind: zone.Rectangle, ZMin: -3700, ZMax: -3100, Points: []zone.Point{
			{X: 82000, Y: 148000, Z: -3467}, {X: 82800, Y: 148700, Z: -3404},
		}},
		zone.AddShape{Zone: id, Kind: zone.Rectangle, Banned: true, ZMin: -3600, ZMax: -3300, Points: []zone.Point{
			{X: 82100, Y: 148100, Z: -3467}, {X: 82300, Y: 148400, Z: -3404},
		}},
	)
	got := shapeBlocks(compileOne(t, d))
	want := []string{
		"rectangle 82000 148000 -3700 -3100; 82800 148700 -3700 -3100",
		"banned_polygon 82100 148100 -3600 -3300; 82300 148100 -3600 -3300; 82300 148400 -3600 -3300; 82100 148400 -3600 -3300",
	}
	if !slices.Equal(got, want) {
		t.Errorf("shapes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Restart points compile as one <restart_point> and one <PKrestart_point>
// block (the parser keeps only the last block of each) whose coords are
// the clicked x y z. Catches the PK element in the wrong case (the parser
// matches it case-sensitively and would skip it) and coords without Z
// (Location.parseLoc throws and the rest of the file is lost).
func TestRestartPointsCompileWithClickedXYZ(t *testing.T) {
	d := zone.NewDocument()
	id := d.NewZoneID()
	polygonZone(t, d, id, "[zb_arena]", zone.BattleZone, -3700, -3100,
		zone.Point{X: 83000, Y: 147600, Z: -3404}, zone.Point{X: 83800, Y: 147600, Z: -3398}, zone.Point{X: 83800, Y: 148300, Z: -3405})
	apply(t, d,
		zone.AddRestartPoint{Zone: id, Point: zone.Point{X: 83400, Y: 147943, Z: -3404}},
		zone.AddRestartPoint{Zone: id, PK: true, Point: zone.Point{X: 82277, Y: 148598, Z: -3467}},
		zone.AddRestartPoint{Zone: id, Point: zone.Point{X: 83332, Y: 149160, Z: -3405}},
	)
	got := compileOne(t, d)
	want := "\t\t<restart_point>\n" +
		"\t\t\t<coords loc=\"83400 147943 -3404\" />\n" +
		"\t\t\t<coords loc=\"83332 149160 -3405\" />\n" +
		"\t\t</restart_point>\n" +
		"\t\t<PKrestart_point>\n" +
		"\t\t\t<coords loc=\"82277 148598 -3467\" />\n" +
		"\t\t</PKrestart_point>\n"
	if !strings.Contains(got, want) {
		t.Errorf("no restart points as clicked in:\n%s", got)
	}
}
