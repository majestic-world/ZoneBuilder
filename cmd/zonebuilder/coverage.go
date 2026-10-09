package main

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"gioui.org/app"

	"zonebuilder/internal/coverage"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/scene"
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
}

// profileResult is a profile measured in the background.
type profileResult struct {
	key     coverageKey
	profile *coverage.Profile
}

// floorCoverage keeps the floor profiles of the selected zone's shapes:
// measured in the background, one at a time, each for its latest key;
// classified by each shape's Z range on the event loop. Every method runs
// on the event loop.
type floorCoverage struct {
	win     *app.Window
	results chan profileResult
	running bool
	shapes  map[shapeRef]*shapeCoverage
}

// shapeCoverage is the latest profile of one shape and its report.
type shapeCoverage struct {
	// done is the key profile was measured for.
	done    coverageKey
	profile *coverage.Profile
	// report is profile classified by [zmin, zmax].
	report     coverage.Report
	zmin, zmax int
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
	key := coverageKey{shape: ref, outline: outlineKey(pts), world: w, scenes: scenesKey(w), hideMeshes: w.HideMeshes}
	s := c.shapes[ref]
	if (s == nil || key != s.done) && !c.running {
		c.start(key, pts, w)
	}
	if s == nil || s.done.world != w {
		return coverage.Report{}, true, false
	}
	zmin, zmax := e.shownZRange(id, i, sh)
	if s.classified != s.profile || zmin != s.zmin || zmax != s.zmax {
		s.report, s.classified, s.zmin, s.zmax = s.profile.Classify(float64(zmin), float64(zmax)), s.profile, zmin, zmax
	}
	return s.report, key != s.done, true
}

// start measures key's profile in the background, on a world of the same
// scenes: the event loop may add tiles to w or drop them meanwhile, and
// the scenes themselves are only read.
func (c *floorCoverage) start(key coverageKey, pts []zone.Point, w *scene.World) {
	snap := scene.NewWorld(w.Origin)
	for _, s := range w.Scenes() {
		snap.Add(s)
	}
	snap.HideMeshes = w.HideMeshes
	o := make(coverage.Outline, len(pts))
	for i, p := range pts {
		o[i] = coverage.Point{X: float64(p.X), Y: float64(p.Y)}
	}
	c.running = true
	go func() {
		began := time.Now()
		p := coverage.Measure(snap, o)
		if d := time.Since(began); d > profileBudget {
			log.Printf("cobertura: perfil do chão medido em %v, acima do orçamento de %v", d.Round(time.Millisecond), profileBudget)
		}
		c.results <- profileResult{key, p}
		c.win.Invalidate()
	}()
}

// receive takes the profile measured in the background, if it is in, and
// forgets the profiles of shapes outside e's selected zone.
func (c *floorCoverage) receive(e *zoneEditor) {
	select {
	case r := <-c.results:
		c.running = false
		c.shapes[r.key.shape] = &shapeCoverage{done: r.key, profile: r.profile}
	default:
	}
	for ref := range c.shapes {
		if ref.zone != e.zone {
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
// w, "" when there is none.
func (c *floorCoverage) inspector(e *zoneEditor, w *scene.World) string {
	r, measuring, ok := c.current(e, w)
	if !ok {
		if measuring {
			return "Cobertura do chão: medindo…"
		}
		return ""
	}
	return coverageText("Cobertura do chão", "Nenhum chão medido sob o shape", r, measuring)
}

// heightWindow is the height window's coverage lines for e's selected
// zone over w, "" when there is none.
func (c *floorCoverage) heightWindow(e *zoneEditor, w *scene.World) string {
	r, measuring, ok := c.zone(e, w)
	if !ok {
		if measuring {
			return "Cobertura da zona: medindo…"
		}
		return ""
	}
	return coverageText("Cobertura da zona (soma dos shapes)", "Nenhum chão medido sob a zona", r, measuring)
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
	}{{"Dentro da faixa", r.Inside}, {"Acima do topo", r.Above}, {"Abaixo do piso", r.Below}, {"Sem chão", r.NoGround}} {
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
