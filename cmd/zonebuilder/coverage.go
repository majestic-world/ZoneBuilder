package main

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"gioui.org/app"

	"zonebuilder/internal/coverage"
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

// floorCoverage keeps the floor profile of the current shape: measured in
// the background, one at a time, for the latest key; classified by the
// shape's Z range on the event loop. Every method runs on the event loop.
type floorCoverage struct {
	win     *app.Window
	results chan profileResult
	running bool
	// done is the key profile was measured for; profile is nil before the
	// first one.
	done    coverageKey
	profile *coverage.Profile
	// report is profile classified by [zmin, zmax].
	report     coverage.Report
	zmin, zmax int
	classified *coverage.Profile
}

func newFloorCoverage(win *app.Window) *floorCoverage {
	return &floorCoverage{win: win, results: make(chan profileResult, 1)}
}

// current is the floor coverage of e's current shape over w: its report
// and whether it is still being measured, for a profile measured for an
// earlier outline or scene of the same shape. ok is false when there is no
// closed shape or no world, or when the shape's first profile is not in
// yet: a report of another shape is never given.
func (c *floorCoverage) current(e *zoneEditor, w *scene.World) (r coverage.Report, measuring, ok bool) {
	c.receive()
	z, sh, found := e.currentShape()
	if !found || e.drawing || w == nil {
		return coverage.Report{}, false, false
	}
	pts := outline(sh.Kind, e.shownPoints(z.ID, e.shape, sh.Points))
	if len(pts) < 3 {
		return coverage.Report{}, false, false
	}
	key := coverageKey{shape: shapeRef{z.ID, e.shape}, outline: outlineKey(pts), world: w, scenes: scenesKey(w), hideMeshes: w.HideMeshes}
	if key != c.done && !c.running {
		c.start(key, pts, w)
	}
	if c.profile == nil || c.done.shape != key.shape || c.done.world != w {
		return coverage.Report{}, true, false
	}
	zmin, zmax := e.shownZRange(z.ID, e.shape, sh)
	if c.classified != c.profile || zmin != c.zmin || zmax != c.zmax {
		c.report, c.classified, c.zmin, c.zmax = c.profile.Classify(float64(zmin), float64(zmax)), c.profile, zmin, zmax
	}
	return c.report, key != c.done, true
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

// receive takes the profile measured in the background, if it is in.
func (c *floorCoverage) receive() {
	select {
	case r := <-c.results:
		c.running = false
		c.done, c.profile = r.key, r.profile
	default:
	}
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
	return coverageText(r, measuring)
}

// coverageText writes report r for the inspector, marked as still being
// measured when measuring.
func coverageText(r coverage.Report, measuring bool) string {
	title := "Cobertura do chão (terreno)"
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
		)
	} else {
		lines = append(lines, "Nenhum chão medido sob o shape")
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
