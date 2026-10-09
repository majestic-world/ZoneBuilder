package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"strings"
	"time"

	"gioui.org/app"

	"zonebuilder/internal/coverage"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
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
	shape   shapeRef
	outline string
	world   *scene.World
	// scenes names the world's scenes, in order.
	scenes     string
	hideMeshes bool
	// bans is the outlines of the zone's other banned shapes; their Z
	// ranges are applied when classifying, so they are not in the key.
	bans string
}

// zoneBans is the zone's banned shapes other than the shape at index
// self, as shown (with any drag in progress), and the key of their
// outlines.
func zoneBans(e *zoneEditor, z zone.Zone, self int) ([]coverage.Ban, string) {
	var bans []coverage.Ban
	var key strings.Builder
	for i, s := range z.Shapes {
		if !s.Banned || i == self {
			continue
		}
		pts := outline(s.Kind, e.shownPoints(z.ID, i, s.Points))
		if len(pts) < 3 {
			continue
		}
		zmin, zmax := e.shownZRange(z.ID, i, s)
		b := coverage.Ban{Outline: make(coverage.Outline, len(pts)), ZMin: float64(zmin), ZMax: float64(zmax)}
		for k, p := range pts {
			b.Outline[k] = coverage.Point{X: float64(p.X), Y: float64(p.Y)}
		}
		bans = append(bans, b)
		key.WriteString(outlineKey(pts))
		key.WriteByte('|')
	}
	return bans, key.String()
}

// profileResult is a profile measured in the background.
type profileResult struct {
	key     coverageKey
	profile *coverage.Profile
	hist    coverage.Histogram
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
	// received counts the profiles received, so the floor warnings are
	// judged again when one comes in; warned are those warnings, judged
	// for warnedAt.
	received int
	warned   []floorWarning
	warnedAt warningsKey
}

// shapeCoverage is the latest profile of one shape and its report.
type shapeCoverage struct {
	// done is the key profile was measured for.
	done    coverageKey
	profile *coverage.Profile
	// hist is profile's floor area by Z, for the height window's ruler.
	hist coverage.Histogram
	// report is profile classified by [zmin, zmax]; fit is the floor
	// that range should span (coverage.Profile.Ground).
	report     coverage.Report
	fit        coverage.Ground
	zmin, zmax int
	banZ       string
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
	banZ := fmt.Sprint(bans)
	if s.classified != s.profile || zmin != s.zmin || zmax != s.zmax || banZ != s.banZ {
		p := s.profile.WithBanRanges(bans)
		s.report, s.classified, s.zmin, s.zmax, s.banZ = p.Classify(float64(zmin), float64(zmax)), s.profile, zmin, zmax, banZ
		s.fit = s.profile.Ground(float64(zmin), float64(zmax))
	}
	return s.report, key != s.done, true
}

// newCoverageKey is the key of shape ref's profile for outline pts over w,
// and the zone's other banned shapes it is measured with.
func newCoverageKey(e *zoneEditor, ref shapeRef, pts []zone.Point, w *scene.World) (coverageKey, []coverage.Ban) {
	z, _ := e.doc.Zone(ref.zone)
	bans, bansKey := zoneBans(e, z, ref.shape)
	return coverageKey{shape: ref, outline: outlineKey(pts), world: w, scenes: scenesKey(w), hideMeshes: w.HideMeshes, bans: bansKey}, bans
}

// profileNow is the floor profile of shape i of zone id over w, for its
// outline as shown: the one kept when it is up to date, else measured on
// the spot and kept, so a click that fits a Z range to the floor never
// reads a stale outline's profile (spec D5). nil for a shape with no
// outline.
func (c *floorCoverage) profileNow(e *zoneEditor, w *scene.World, id zone.ZoneID, i int, sh zone.Shape) *coverage.Profile {
	c.receive(e)
	pts := outline(sh.Kind, e.shownPoints(id, i, sh.Points))
	if len(pts) < 3 {
		return nil
	}
	ref := shapeRef{id, i}
	key, bans := newCoverageKey(e, ref, pts, w)
	if s := c.shapes[ref]; s != nil && s.done == key {
		return s.profile
	}
	began := time.Now()
	p := coverage.Measure(w, coverageOutline(pts), bans)
	log.Printf("cobertura: perfil do shape %d medido na hora em %v", i+1, time.Since(began).Round(time.Millisecond))
	c.shapes[ref] = &shapeCoverage{done: key, profile: p, hist: p.Histogram()}
	return p
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
	c.running = true
	go func() {
		began := time.Now()
		p := coverage.Measure(snap, o, bans)
		h := p.Histogram()
		if d := time.Since(began); d > profileBudget {
			log.Printf("cobertura: perfil do chão medido em %v, acima do orçamento de %v", d.Round(time.Millisecond), profileBudget)
		}
		c.results <- profileResult{key, p, h}
		c.win.Invalidate()
	}()
}

// receive takes the profile measured in the background, if it is in, and
// then forgets the profiles of shapes no longer in e's document. The slot
// one past a zone's last shape stays: it holds the profile of the shape
// being placed (floorCoverage.suggest).
func (c *floorCoverage) receive(e *zoneEditor) {
	select {
	case r := <-c.results:
		c.running = false
		c.received++
		c.shapes[r.key.shape] = &shapeCoverage{done: r.key, profile: r.profile, hist: r.hist}
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

// inspector is the inspector's coverage lines for e's current shape over
// w, "" when there is none: the report, then the floor layers the range
// rule leaves out (spec D5).
func (c *floorCoverage) inspector(e *zoneEditor, w *scene.World) string {
	r, measuring, ok := c.current(e, w)
	if !ok {
		if measuring {
			return "Cobertura do chão: medindo…"
		}
		return ""
	}
	text := coverageText("Cobertura do chão", "Nenhum chão medido sob o shape", r, measuring)
	if others := c.shapes[shapeRef{e.zone, e.shape}].fit.Others; len(others) > 0 {
		text += "\n" + othersText(others)
	}
	return text
}

// othersShown is the most left-out layers the inspector lists by Z.
const othersShown = 4

// othersText names the floor layers left out of a Z range's fit, with
// the Z of the first othersShown: "Outra camada: z 3256" or "Outras
// camadas: 2 (z 3256; z 4000 … 4120)".
func othersText(others []coverage.Layer) string {
	var zs []string
	for _, l := range others[:min(len(others), othersShown)] {
		lo, hi := int(math.Floor(l.Low)), int(math.Ceil(l.High))
		if hi == lo {
			zs = append(zs, fmt.Sprintf("z %d", lo))
		} else {
			zs = append(zs, fmt.Sprintf("z %d … %d", lo, hi))
		}
	}
	if len(others) == 1 {
		return "Outra camada: " + zs[0]
	}
	if len(others) > othersShown {
		zs = append(zs, "…")
	}
	return fmt.Sprintf("Outras camadas: %d (%s)", len(others), strings.Join(zs, "; "))
}

// rulerRows is how many histogram bars the height window's ruler draws
// over its Z span.
const rulerRows = 48

// heightWindow fills p's coverage of e's selected zone over w (spec D7):
// the ruler and the text lines under it, both empty when there is none.
func (c *floorCoverage) heightWindow(e *zoneEditor, w *scene.World, p *ui.HeightPanel) {
	p.Coverage, p.Ruler = "", ui.Ruler{}
	r, measuring, ok := c.zone(e, w)
	if !ok {
		if measuring {
			p.Coverage = "Cobertura da zona: medindo…"
		}
		return
	}
	title := "Cobertura da zona (soma dos shapes)"
	if measuring {
		title += ": medindo…"
	}
	lines := []string{title}
	if r.Measured {
		lines = append(lines,
			fmt.Sprintf("Chão sob a zona: %s … %s", units(roundF(r.GroundMin.Z)), units(roundF(r.GroundMax.Z))),
			fmt.Sprintf("Folga do piso: %s · Folga do topo: %s", clearance(r.FloorClearance), clearance(r.TopClearance)),
		)
		p.Ruler = c.ruler(e, w, r)
	} else {
		lines = append(lines, "Nenhum chão medido sob a zona")
	}
	total := r.Total()
	lines = append(lines, fmt.Sprintf("Cobertura %s · acima %s · abaixo %s · sem chão %s",
		percent(r.Inside, total), percent(r.Above, total), percent(r.Below, total), percent(r.NoGround, total)))
	p.Coverage = strings.Join(lines, "\n")
}

// clearance writes a clearance, marked "(fura)" when the floor pierces
// that side of the range.
func clearance(v float64) string {
	if n := roundF(v); n < 0 {
		return "−" + units(-n) + " (fura)"
	}
	return units(roundF(v))
}

// ruler is the height window's ruler for e's selected zone over w, whose
// summed report is r: Z from the lowest of the range and the floor to the
// highest, padded; the range bar from the lowest zmin to the highest zmax;
// each row's floor by state, every shape's histogram split by its own
// range; and marks at the lowest and highest floor with their clearances.
// Only the split depends on the range, so a Z drag redraws it every frame.
func (c *floorCoverage) ruler(e *zoneEditor, w *scene.World, r coverage.Report) ui.Ruler {
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
	top, floor := roundF(r.TopClearance), roundF(r.FloorClearance)
	u.Marks = []ui.RulerMark{
		{Z: r.GroundMax.Z, Text: fmt.Sprintf("%s topo %s", units(roundF(r.GroundMax.Z)), signed(top)), Alert: top < 0},
		{Z: r.GroundMin.Z, Text: fmt.Sprintf("%s piso %s", units(roundF(r.GroundMin.Z)), signed(floor)), Alert: floor < 0},
	}
	return u
}

// coverageText writes report r under title, marked as still being
// measured when measuring; none replaces the floor lines when r has no
// floor.
func coverageText(title, none string, r coverage.Report, measuring bool) string {
	if measuring {
		title += ": medindo…"
	}
	lines := []string{title}
	if r.Measured {
		lo, hi := r.GroundMin, r.GroundMax
		lines = append(lines,
			fmt.Sprintf("Chão mais baixo: %d %d %d", roundF(lo.X), roundF(lo.Y), roundF(lo.Z)),
			fmt.Sprintf("Chão mais alto: %d %d %d", roundF(hi.X), roundF(hi.Y), roundF(hi.Z)),
			fmt.Sprintf("Folga do piso: %s · folga do topo: %s", units(roundF(r.FloorClearance)), units(roundF(r.TopClearance))),
			"Chão em "+inflect.Count(r.Layers, "camada", "camadas"),
		)
	} else {
		lines = append(lines, none)
	}
	total := r.Total()
	for _, a := range []struct {
		name string
		area float64
	}{{"Dentro da faixa", r.Inside}, {"Acima do topo", r.Above}, {"Abaixo do piso", r.Below}, {"Excluída", r.Excluded}, {"Sem chão", r.NoGround}} {
		lines = append(lines, fmt.Sprintf("%s: %s u² (%s)", a.name, units(roundF(a.area)), percent(a.area, total)))
	}
	return strings.Join(lines, "\n")
}

// percent is part of whole as a pt-BR percentage with 1 decimal.
func percent(part, whole float64) string {
	if whole <= 0 {
		return "0%"
	}
	return strings.Replace(fmt.Sprintf("%.1f%%", 100*part/whole), ".", ",", 1)
}

// outlineKey is pts as a comparable key.
func outlineKey(pts []zone.Point) string {
	var b strings.Builder
	for _, p := range pts {
		fmt.Fprintf(&b, "%d,%d;", p.X, p.Y)
	}
	return b.String()
}

// scenesKey names w's scenes, in order, as a comparable key.
func scenesKey(w *scene.World) string {
	var b strings.Builder
	for _, s := range w.Scenes() {
		fmt.Fprintf(&b, "%p;", s)
	}
	return b.String()
}

func roundF(v float64) int { return int(math.Round(v)) }
