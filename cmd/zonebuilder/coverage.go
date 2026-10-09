package main

import (
	"hash/maphash"
	"image/color"
	"log"
	"math"
	"slices"
	"strings"
	"time"

	"gioui.org/app"

	"zonebuilder/internal/coverage"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/locale"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// profileBudget is how long measuring a shape's floor profile may take off
// the event loop (spec D2: a whole tile); longer ones are logged.
const profileBudget = 100 * time.Millisecond

// shapeRef names one shape of one zone.
type shapeRef struct {
	zone  zone.ZoneID
	shape int
}

// coverageKey is what a shape's floor profile is measured from: the
// shape's outline as shown, the world's tiles and whether static meshes
// are hidden. Any change of it measures the profile again.
type coverageKey struct {
	shape shapeRef
	// outline hashes the shape's outline (outlineKey); bans the outlines
	// of the zone's other banned shapes, whose Z ranges are applied when
	// classifying, so they are not in the key.
	outline, bans uint64
	world         *scene.World
	// scenes hashes the world's scenes, in order (scenesKey).
	scenes     uint64
	hideMeshes bool
}

// keySeed seeds the hashes of coverageKey.
var keySeed = maphash.MakeSeed()

// zoneBans is the zone's banned shapes other than the shape at index
// self, as shown (with any drag in progress), and the key of their
// outlines.
func zoneBans(e *zoneEditor, z zone.Zone, self int) ([]coverage.Ban, uint64) {
	var bans []coverage.Ban
	var key maphash.Hash
	key.SetSeed(keySeed)
	for i, s := range z.Shapes {
		if !s.Banned || i == self {
			continue
		}
		pts := outline(s.Kind, e.shownPoints(z.ID, i, s.Points))
		if len(pts) < 3 {
			continue
		}
		zmin, zmax := e.shownZRange(z.ID, i, s)
		bans = append(bans, coverage.Ban{Outline: coverageOutline(pts), ZMin: float64(zmin), ZMax: float64(zmax)})
		maphash.WriteComparable(&key, outlineKey(pts))
	}
	return bans, key.Sum64()
}

// sameBanRanges reports whether bans a and b, of the same outlines, have
// the same Z ranges.
func sameBanRanges(a, b []coverage.Ban) bool {
	return slices.EqualFunc(a, b, func(x, y coverage.Ban) bool { return x.ZMin == y.ZMin && x.ZMax == y.ZMax })
}

// profileResult is a profile measured in the background, started when
// the store's generation was since.
type profileResult struct {
	key     coverageKey
	profile *coverage.Profile
	hist    coverage.Histogram
	since   uint64
}

// floorCoverage keeps the floor profiles of every zone's shapes, measured
// lazily as they are asked for: in the background, one at a time, each
// for its latest key; classified by each shape's Z range on the event
// loop. Every method runs on the event loop.
type floorCoverage struct {
	win     *app.Window
	results chan profileResult
	running bool
	shapes  map[shapeRef]*shapeCoverage
	// gen counts the profiles stored, so a background profile started
	// before its slot was filled on the spot is dropped (receive).
	gen uint64
	// received counts the profiles received, so the floor warnings are
	// judged again when one comes in; warned are those warnings, judged
	// for warnedAt.
	received int
	warned   []floorWarning
	warnedAt warningsKey
}

// shapeCoverage is the latest profile of one shape and its report.
type shapeCoverage struct {
	// done is the key profile was measured for; gen is the store's
	// generation when it was stored.
	done    coverageKey
	gen     uint64
	profile *coverage.Profile
	// hist is profile's floor area by Z, for the height window's ruler.
	hist coverage.Histogram
	// report is profile classified by [zmin, zmax] with the bans' Z
	// ranges of bans.
	report     coverage.Report
	zmin, zmax int
	bans       []coverage.Ban
	classified *coverage.Profile
	// ground is the floor line along the walls of lined's outline.
	ground []geom.Vec3
	lined  *coverage.Profile
}

func newFloorCoverage(win *app.Window) *floorCoverage {
	return &floorCoverage{win: win, results: make(chan profileResult, 1), shapes: map[shapeRef]*shapeCoverage{}}
}

// current is the floor coverage of e's current shape over w: its report
// and whether it is still being measured, for a profile measured for an
// earlier outline or scene of the same shape. ok is false when there is no
// closed shape or no world, or when the shape's first profile is not in
// yet: a report of another shape is never given.
func (c *floorCoverage) current(e *zoneEditor, w *scene.World) (r coverage.Report, measuring, ok bool) {
	c.receive(e)
	z, sh, found := e.currentShape()
	if !found || e.drawing || w == nil {
		return coverage.Report{}, false, false
	}
	return c.shape(e, w, z.ID, e.shape, sh)
}

// zone is the floor coverage of the selected zone over w: the sum of its
// included shapes' reports (coverage.Sum) and whether any of them is
// still being measured. ok is false when the zone has no closed included
// shape or no world, or while any of them has no profile yet.
func (c *floorCoverage) zone(e *zoneEditor, w *scene.World) (r coverage.Report, measuring, ok bool) {
	c.receive(e)
	z, found := e.doc.Zone(e.zone)
	if !found || e.drawing || w == nil {
		return coverage.Report{}, false, false
	}
	var rs []coverage.Report
	ok = true
	for i, sh := range z.Shapes {
		if sh.Banned {
			continue
		}
		r, m, shapeOK := c.shape(e, w, z.ID, i, sh)
		if !shapeOK && !m {
			continue // an open polygon: no outline yet
		}
		measuring = measuring || m
		ok = ok && shapeOK
		rs = append(rs, r)
	}
	if len(rs) == 0 || !ok {
		return coverage.Report{}, measuring, false
	}
	return coverage.Sum(rs...), measuring, true
}

// shape is the floor coverage of shape i of zone id over w, as current
// gives it, measuring it in the background when its profile is stale and
// nothing else is being measured.
func (c *floorCoverage) shape(e *zoneEditor, w *scene.World, id zone.ZoneID, i int, sh zone.Shape) (r coverage.Report, measuring, ok bool) {
	pts := outline(sh.Kind, e.shownPoints(id, i, sh.Points))
	if len(pts) < 3 {
		return coverage.Report{}, false, false
	}
	ref := shapeRef{id, i}
	key, bans := newCoverageKey(e, ref, pts, w)
	s := c.shapes[ref]
	if (s == nil || key != s.done) && !c.running {
		c.start(key, pts, bans, w)
	}
	if s == nil || s.done.world != w {
		return coverage.Report{}, true, false
	}
	zmin, zmax := e.shownZRange(id, i, sh)
	if s.classified != s.profile || zmin != s.zmin || zmax != s.zmax || !sameBanRanges(bans, s.bans) {
		s.report = s.profile.WithBanRanges(bans).Classify(float64(zmin), float64(zmax))
		s.classified, s.zmin, s.zmax, s.bans = s.profile, zmin, zmax, bans
	}
	return s.report, key != s.done, true
}

// newCoverageKey is the key of shape ref's profile for outline pts over w,
// and the zone's other banned shapes it is measured with.
func newCoverageKey(e *zoneEditor, ref shapeRef, pts []zone.Point, w *scene.World) (coverageKey, []coverage.Ban) {
	z, _ := e.doc.Zone(ref.zone)
	bans, bansKey := zoneBans(e, z, ref.shape)
	return coverageKey{shape: ref, outline: outlineKey(pts), bans: bansKey, world: w, scenes: scenesKey(w), hideMeshes: w.HideMeshes}, bans
}

// profile is the floor profile of shape ref for outline pts over w: the
// one kept when it is up to date; else, when now, measured on the spot
// and kept, so a click that fits a Z range to the floor or closes a shape
// never reads a stale outline's profile (spec D5); else nil, and measured
// in the background once nothing else is. A shape being added is kept as
// the index it gets once added, so it needs no measuring again. nil for an
// outline of fewer than 3 points.
func (c *floorCoverage) profile(e *zoneEditor, w *scene.World, ref shapeRef, pts []zone.Point, now bool) *coverage.Profile {
	c.receive(e)
	if len(pts) < 3 {
		return nil
	}
	key, bans := newCoverageKey(e, ref, pts, w)
	if s := c.shapes[ref]; s != nil && s.done == key {
		return s.profile
	}
	if !now {
		if !c.running {
			c.start(key, pts, bans, w)
		}
		return nil
	}
	began := time.Now()
	p := coverage.Measure(w, coverageOutline(pts), bans)
	log.Printf("cobertura: perfil do shape %d medido na hora em %v", ref.shape+1, time.Since(began).Round(time.Millisecond))
	c.store(key, p, p.Histogram())
	return p
}

// store keeps profile p, measured for key, as its shape's.
func (c *floorCoverage) store(key coverageKey, p *coverage.Profile, h coverage.Histogram) {
	c.gen++
	c.shapes[key.shape] = &shapeCoverage{done: key, gen: c.gen, profile: p, hist: h}
}

// coverageOutline is pts on the X/Y plane.
func coverageOutline(pts []zone.Point) coverage.Outline {
	o := make(coverage.Outline, len(pts))
	for i, p := range pts {
		o[i] = coverage.Point{X: float64(p.X), Y: float64(p.Y)}
	}
	return o
}

// start measures key's profile in the background, on a world of the same
// scenes: the event loop may add tiles to w or drop them meanwhile, and
// the scenes themselves are only read.
func (c *floorCoverage) start(key coverageKey, pts []zone.Point, bans []coverage.Ban, w *scene.World) {
	snap := scene.NewWorld(w.Origin)
	for _, s := range w.Scenes() {
		snap.Add(s)
	}
	snap.HideMeshes = w.HideMeshes
	o := coverageOutline(pts)
	since := c.gen
	c.running = true
	go func() {
		began := time.Now()
		p := coverage.Measure(snap, o, bans)
		h := p.Histogram()
		if d := time.Since(began); d > profileBudget {
			log.Printf("cobertura: perfil do chão medido em %v, acima do orçamento de %v", d.Round(time.Millisecond), profileBudget)
		}
		c.results <- profileResult{key, p, h, since}
		c.win.Invalidate()
	}()
}

// receive takes the profile measured in the background, if it is in,
// unless its shape's profile was stored after it started (measured on the
// spot, for a newer outline), and then forgets the profiles of shapes no
// longer in e's document. The slot one past a zone's last shape stays: it
// holds the profile of the shape being placed (floorCoverage.suggest).
func (c *floorCoverage) receive(e *zoneEditor) {
	select {
	case r := <-c.results:
		c.running = false
		c.received++
		if s := c.shapes[r.key.shape]; s == nil || s.gen <= r.since {
			c.store(r.key, r.profile, r.hist)
		}
	default:
		return
	}
	for ref := range c.shapes {
		if z, ok := e.doc.Zone(ref.zone); !ok || ref.shape > len(z.Shapes) {
			delete(c.shapes, ref)
		}
	}
}

// groundLine is the floor line along the walls of e's current shape over
// w (spec D4d), as segments in server coordinates, and the profile it
// comes from. Both are nil while the shape's outline or scene is being
// measured, so the line leaves while the outline is dragged and comes
// back with the new measure.
func (c *floorCoverage) groundLine(e *zoneEditor, w *scene.World) ([]geom.Vec3, *coverage.Profile) {
	r, measuring, ok := c.current(e, w)
	if !ok || measuring {
		return nil, nil
	}
	s := c.shapes[shapeRef{e.zone, e.shape}]
	if s.lined != s.profile {
		s.ground = nil
		for _, sp := range r.Edges {
			for _, g := range sp {
				s.ground = append(s.ground,
					geom.Vec3{X: float32(g.From.X), Y: float32(g.From.Y), Z: float32(g.From.Z)},
					geom.Vec3{X: float32(g.To.X), Y: float32(g.To.Y), Z: float32(g.To.Z)})
			}
		}
		s.lined = s.profile
	}
	return s.ground, s.profile
}

// inspector presents the current shape's measured ground without changing its profile.
func (c *floorCoverage) inspector(e *zoneEditor, w *scene.World, lang locale.Language) string {
	r, measuring, ok := c.current(e, w)
	if !ok {
		if measuring {
			return locale.Text(lang, "coverage.shape.measuring")
		}
		return ""
	}
	text := coverageText(lang, r, measuring)
	if len(r.Others) > 0 {
		text += "\n" + othersText(lang, r.Others)
	}
	return text
}

// othersShown is the most left-out layers the inspector lists by Z.
const othersShown = 4

// othersText presents the excluded layers by their Z spans.
func othersText(lang locale.Language, others []coverage.Layer) string {
	zs := make([]string, 0, min(len(others), othersShown)+1)
	for _, l := range others[:min(len(others), othersShown)] {
		lo, hi := int(math.Floor(l.Low)), int(math.Ceil(l.High))
		if hi == lo {
			zs = append(zs, locale.Format(lang, "coverage.layer.point", map[string]string{"z": measureNumber(lang, lo)}))
		} else {
			zs = append(zs, locale.Format(lang, "coverage.layer.span", map[string]string{"low": measureNumber(lang, lo), "high": measureNumber(lang, hi)}))
		}
	}
	if len(others) > othersShown {
		zs = append(zs, "…")
	}
	return locale.Plural(lang, "coverage.other_layers", len(others), map[string]string{"layers": strings.Join(zs, "; ")})
}

// rulerRows is how many histogram bars the height window's ruler draws
// over its Z span.
const rulerRows = 48

// heightWindow presents the selected zone's report and ruler from stored measurements.
func (c *floorCoverage) heightWindow(e *zoneEditor, w *scene.World, p *ui.HeightPanel, lang locale.Language) {
	p.Coverage, p.Ruler = "", ui.Ruler{}
	r, measuring, ok := c.zone(e, w)
	if !ok {
		if measuring {
			p.Coverage = locale.Text(lang, "coverage.zone.measuring")
		}
		return
	}
	p.Coverage = heightSummary(lang, r, measuring)
	if r.Measured {
		p.Ruler = c.ruler(e, w, r, lang)
	}
}

func heightSummary(lang locale.Language, r coverage.Report, measuring bool) string {
	title := locale.Text(lang, "coverage.zone.title")
	if measuring {
		title = locale.Text(lang, "coverage.zone.title_measuring")
	}
	lines := []string{title}
	if r.Measured {
		lines = append(lines,
			locale.Format(lang, "coverage.zone.range", map[string]string{
				"low": measureNumber(lang, roundF(r.GroundMin.Z)), "high": measureNumber(lang, roundF(r.GroundMax.Z)),
			}),
			locale.Format(lang, "coverage.zone.clearances", map[string]string{
				"floor": clearance(lang, r.FloorClearance), "top": clearance(lang, r.TopClearance),
			}),
		)
	} else {
		lines = append(lines, noFloor(lang, r, true))
	}
	total := r.Total()
	lines = append(lines, locale.Format(lang, "coverage.zone.shares", map[string]string{
		"coverage": share(lang, r.Coverage()), "above": percent(lang, r.Above, total),
		"below": percent(lang, r.Below, total), "missing": percent(lang, r.NoGround, total),
	}))
	return strings.Join(lines, "\n")
}

// clearance marks ground that pierces a Z boundary while preserving the measured value.
func clearance(lang locale.Language, v float64) string {
	if n := roundF(v); n < 0 {
		return locale.Format(lang, "coverage.clearance.pierces", map[string]string{"value": measureNumber(lang, n)})
	}
	return measureNumber(lang, roundF(v))
}

// ruler is the height window's ruler for e's selected zone over w, whose
// summed report is r: Z from the lowest of the range and the floor to the
// highest, padded; the range bar from the lowest zmin to the highest zmax;
// each row's floor by state, every shape's histogram split by its own
// range; and marks at the lowest and highest floor with their clearances.
// Only the split depends on the range, so a Z drag redraws it every frame.
func (c *floorCoverage) ruler(e *zoneEditor, w *scene.World, r coverage.Report, lang locale.Language) ui.Ruler {
	z, _ := e.doc.Zone(e.zone)
	type part struct {
		hist       coverage.Histogram
		zmin, zmax float64
	}
	var parts []part
	zmin, zmax := math.Inf(1), math.Inf(-1)
	for i, sh := range z.Shapes {
		s := c.shapes[shapeRef{z.ID, i}]
		if sh.Banned || s == nil || s.done.world != w {
			continue
		}
		lo, hi := e.shownZRange(z.ID, i, sh)
		parts = append(parts, part{s.hist, float64(lo), float64(hi)})
		zmin, zmax = min(zmin, float64(lo)), max(zmax, float64(hi))
	}
	if len(parts) == 0 {
		return ui.Ruler{}
	}
	lo, hi := min(zmin, r.GroundMin.Z), max(zmax, r.GroundMax.Z)
	pad := max(32, 0.06*(hi-lo))
	zc := z.DisplayColor()
	u := ui.Ruler{Lo: lo - pad, Hi: hi + pad, ZMin: zmin, ZMax: zmax, Color: color.NRGBA{R: zc[0], G: zc[1], B: zc[2], A: 0xFF}}
	u.Bars = make([]ui.RulerBar, rulerRows)
	step := (u.Hi - u.Lo) / rulerRows
	for i := range u.Bars {
		b := &u.Bars[i]
		z0 := u.Lo + float64(i)*step
		for _, p := range parts {
			in, above, below := p.hist.Split(z0, z0+step, p.zmin, p.zmax)
			b.Inside, b.Above, b.Below = b.Inside+in, b.Above+above, b.Below+below
		}
	}
	u.Marks = rulerMarks(lang, r)
	return u
}

func rulerMarks(lang locale.Language, r coverage.Report) []ui.RulerMark {
	return []ui.RulerMark{
		{Z: r.GroundMax.Z, Text: locale.Format(lang, "coverage.ruler.top", map[string]string{
			"z": measureNumber(lang, roundF(r.GroundMax.Z)), "clearance": clearance(lang, r.TopClearance),
		}), Alert: roundF(r.TopClearance) < 0},
		{Z: r.GroundMin.Z, Text: locale.Format(lang, "coverage.ruler.floor", map[string]string{
			"z": measureNumber(lang, roundF(r.GroundMin.Z)), "clearance": clearance(lang, r.FloorClearance),
		}), Alert: roundF(r.FloorClearance) < 0},
	}
}

// coverageText presents a shape's report with localized measures and literal coordinates.
func coverageText(lang locale.Language, r coverage.Report, measuring bool) string {
	title := locale.Text(lang, "coverage.shape.title")
	if measuring {
		title = locale.Text(lang, "coverage.shape.title_measuring")
	}
	lines := []string{title}
	if r.Measured {
		lo, hi := r.GroundMin, r.GroundMax
		lines = append(lines,
			locale.Format(lang, "coverage.shape.low", map[string]string{
				"x": intArg(roundF(lo.X)), "y": intArg(roundF(lo.Y)), "z": intArg(roundF(lo.Z)),
			}),
			locale.Format(lang, "coverage.shape.high", map[string]string{
				"x": intArg(roundF(hi.X)), "y": intArg(roundF(hi.Y)), "z": intArg(roundF(hi.Z)),
			}),
			locale.Format(lang, "coverage.shape.clearances", map[string]string{
				"floor": clearance(lang, r.FloorClearance), "top": clearance(lang, r.TopClearance),
			}),
			locale.Plural(lang, "coverage.shape.layers", r.Layers, nil),
		)
	} else {
		lines = append(lines, noFloor(lang, r, false))
	}
	total := r.Total()
	areaArgs := func(area float64) map[string]string {
		return map[string]string{"area": measureNumber(lang, roundF(area)), "share": percent(lang, area, total)}
	}
	lines = append(lines,
		locale.Format(lang, "coverage.area.inside", areaArgs(r.Inside)),
		locale.Format(lang, "coverage.area.above", areaArgs(r.Above)),
		locale.Format(lang, "coverage.area.below", areaArgs(r.Below)),
		locale.Format(lang, "coverage.area.excluded", areaArgs(r.Excluded)),
		locale.Format(lang, "coverage.area.other", areaArgs(r.Other)),
		locale.Format(lang, "coverage.area.missing", areaArgs(r.NoGround)),
	)
	return strings.Join(lines, "\n")
}

func noFloor(lang locale.Language, r coverage.Report, zoneSummary bool) string {
	if r.Excluded > 0 || r.Other > 0 {
		return locale.Text(lang, "coverage.no_included_ground")
	}
	if zoneSummary {
		return locale.Text(lang, "coverage.zone.none")
	}
	return locale.Text(lang, "coverage.shape.none")
}

func percent(lang locale.Language, part, whole float64) string {
	if whole <= 0 {
		return locale.Percent(lang, 0, 1)
	}
	return share(lang, part/whole)
}

func share(lang locale.Language, fraction float64) string {
	return locale.Percent(lang, fraction, 1)
}

// measureNumber is presentation-only; XML and numeric inputs remain untouched.
func measureNumber(lang locale.Language, value int) string {
	if value < 0 {
		return "−" + locale.Number(lang, float64(-value), 0)
	}
	return locale.Number(lang, float64(value), 0)
}

// outlineKey is pts's X/Y as a comparable key: a hash, so the keys asked
// for every frame cost no allocation. [INFERENCE] 2 outlines colliding in
// 64 bits, which would keep a stale profile, is not worth guarding.
func outlineKey(pts []zone.Point) uint64 {
	var h maphash.Hash
	h.SetSeed(keySeed)
	for _, p := range pts {
		maphash.WriteComparable(&h, [2]int{p.X, p.Y})
	}
	return h.Sum64()
}

// scenesKey hashes w's scenes, in order, as a comparable key.
func scenesKey(w *scene.World) uint64 {
	var h maphash.Hash
	h.SetSeed(keySeed)
	for _, s := range w.Scenes() {
		maphash.WriteComparable(&h, s)
	}
	return h.Sum64()
}

func roundF(v float64) int { return int(math.Round(v)) }
