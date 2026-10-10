package main

import (
	"image"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/render"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zonexml"
)

// zoneMode is the zone editor ("Construir zonas"): the zone document and
// its editor, the floor coverage measured for its shapes, the water
// selection, and what the renderer last got from them.
type zoneMode struct {
	zones *zoneEditor
	cover *floorCoverage
	// waterSel is the water the user selected by clicking.
	waterSel waterSelection
	// zonesShown is the zones.version the renderer last got; waterShown
	// is the waterSel.version it last got.
	zonesShown, waterShown int
	// lineShown is the key of the ground line along the current shape's
	// walls when the zones were last sent.
	lineShown lineKey
	// groundShown is what the renderer's ground marking was last built
	// for; groundMark was last built for groundBuilt.
	groundShown, groundBuilt groundKey
	groundMark               render.Ground
	// pins are the current shape's worst points (spec D4e), as last laid
	// out; pinsShown are the ones the renderer's overlay has.
	pins, pinsShown []worstPin
}

func newZoneMode(w *app.Window) *zoneMode {
	return &zoneMode{
		zones:       newZoneEditor(),
		cover:       newFloorCoverage(w),
		zonesShown:  -1,
		waterShown:  -1,
		groundShown: groundKey{version: -1},
		groundBuilt: groundKey{version: -1},
	}
}

func (z *zoneMode) viewportEvent(ws *workspace, ev event.Event) bool {
	msg, used := z.zones.viewportEvent(ws.world(), &ws.cam, ev, ws.viewport())
	if used && msg != "" {
		ws.status = editorResult(msg, z.zones)
	}
	if e, ok := ev.(pointer.Event); ok && e.Kind == pointer.Press {
		z.waterSel.pressTaken = used
	}
	return used
}

func (z *zoneMode) click(ws *workspace, e pointer.Event, button pointer.Buttons) {
	w, vp := ws.world(), ws.viewport()
	switch button {
	case pointer.ButtonPrimary:
		if msg := z.zones.click(w, z.cover, &ws.cam, e.Position, vp, ws.probe.click, ws.probe.clickHit); msg != "" {
			ws.status = editorResult(msg, z.zones)
		} else if !z.zones.armed && !z.waterSel.pressTaken {
			if msg := z.waterSel.click(w, &ws.cam, e.Position, vp, ws.probe.click, ws.probe.clickHit, e.Modifiers.Contain(key.ModCtrl)); msg.render(ws.shell.Language) != "" {
				ws.status = msg
			}
		}
	case pointer.ButtonSecondary:
		msg, open := z.waterSel.rightClick(w, &ws.cam, e.Position, vp)
		if open {
			ws.shell.WaterMenu.Menu.Open(image.Pt(round(e.Position.X), round(e.Position.Y)))
			if z.zones.drawing {
				msg = action(locale.Message{Key: "actions.water.busy"})
			}
		}
		if msg.render(ws.shell.Language) != "" {
			ws.status = msg
		}
	}
}

func (z *zoneMode) key(ws *workspace, e key.Event) {
	if e.State != key.Press {
		return
	}
	switch e.Name {
	case key.NameReturn, key.NameEnter:
		if msg := z.zones.close(ws.world(), z.cover); msg != "" {
			ws.status = editorResult(msg, z.zones)
		}
	case key.NameEscape:
		if msg := z.zones.escape(); msg != "" {
			ws.status = editorResult(msg, z.zones)
		} else if z.waterSel.clear() {
			ws.status = action(locale.Message{Key: "actions.water.cleared"})
		}
	}
}

func (z *zoneMode) undo(ws *workspace) { ws.status = editorResult(z.zones.undo(), z.zones) }
func (z *zoneMode) redo(ws *workspace) { ws.status = editorResult(z.zones.redo(), z.zones) }

func (z *zoneMode) compile(ws *workspace) {
	msg, files := z.zones.compile()
	ws.status = editorResult(msg, z.zones)
	if len(files) > 0 {
		ws.shell.XML.Open(files, ui.ZoneXML)
	}
}

func (z *zoneMode) update(gtx layout.Context, ws *workspace) {
	shell, zones, w := ws.shell, z.zones, ws.world()
	if shell.EditorLocked {
		shell.Zone.Tools.Discard(gtx)
		shell.Zones.Discard(gtx)
		shell.Props.Discard(gtx)
		for {
			if _, ok := shell.Problems.Clicked(gtx); !ok {
				break
			}
		}
		for {
			if _, ok := shell.PinClicked(gtx); !ok {
				break
			}
		}
		return
	}
	if shell.Zone.CreateRequested(gtx) {
		ws.status = editorResult(zones.create(shell.Zone.Name.Text(), shell.Zone.Type(), shell.Zone.Tools.Shape), zones)
	}
	if t, ok := shell.Zone.Tools.Requested(gtx); ok {
		ws.status = editorResult(zones.arm(t, shell.Zone.Tools.Banned.Value), zones)
	}
	if shell.Zone.Tools.WholeTile.Clicked(gtx) {
		ws.status = editorResult(wholeTile(zones, z.cover, ws.tiles, &ws.cam, ws.viewport(), shell.Zone.Tools.Banned.Value), zones)
	}
	if shell.WaterMenu.CompileRequested(gtx) {
		var files []zonexml.File
		sel, live := z.waterSel.selected(w)
		ws.status, files = zones.compileWater(sel, live)
		if len(files) > 0 {
			shell.XML.Open(files, ui.ZoneXML)
		}
	}
	for _, req := range shell.Zones.Update(gtx) {
		if msg := zones.listRequest(req, w, &ws.cam); msg != "" {
			ws.status = editorResult(msg, zones)
		}
	}
	if i, ok := shell.Problems.Clicked(gtx); ok {
		if msg := zones.goToProblem(i, w, &ws.cam, shell.Language); msg != "" {
			ws.status = actionStatus{raw: msg, problemClick: true}
		}
	}
	if i, ok := shell.PinClicked(gtx); ok && i < len(z.pins) {
		pin := z.pins[i]
		goToPin(pin, w, &ws.cam, shell.Language)
		ws.status = actionStatus{message: pinMessage(pin), pin: &pin}
	}
	sel, selOK := zones.selectedZone()
	for _, req := range shell.Props.Update(gtx, sel, selOK) {
		if msg := shell.Props.Applied(req, zones.apply(req.Command)); msg.Key != "" {
			ws.status = actionStatus{message: msg}
		}
	}
}

func (z *zoneMode) present(gtx layout.Context, ws *workspace) {
	shell, zones, w, vp := ws.shell, z.zones, ws.world(), ws.viewport()
	z.waterSel.prune(w)
	shell.WaterMenu.Disabled = zones.drawing
	shell.Zone.Info = zones.info()
	sel, _ := zones.selectedZone()
	shell.Zones.Rows, shell.Zones.Selected = zones.rows(), sel.ID
	if k := (groundKey{version: zones.version, zone: zones.zone, on: shell.Ground.On}); k != z.groundBuilt {
		z.groundMark = zones.ground(shell.Ground.On)
		shell.Zones.LeftOut = render.GroundLeftOut(z.groundMark)
		z.groundBuilt = k
	}
	if msg := zones.panel(gtx, &shell.Edit, w, z.cover); msg != "" {
		ws.status = editorResult(msg, zones)
	}
	shell.Edit.Coverage = z.cover.inspector(zones, w, shell.Language)
	if msg := zones.heightPanel(gtx, &shell.Height, w, z.cover); msg != "" {
		ws.status = editorResult(msg, zones)
	}
	z.cover.heightWindow(zones, w, &shell.Height, shell.Language)
	shell.Gizmo = zones.layoutGizmo(w, &ws.cam, vp, gtx.Dp(90))
	shell.EdgeLabels = nil
	if shell.Ground.On && w != nil {
		shell.EdgeLabels = zones.edgeLabels(w, &ws.cam, vp, shell.Language)
	}
	z.pins = z.cover.pins(zones, w)
	// After the selected zone's coverage asked for its profiles, so they
	// are measured first.
	warnings, changed := z.cover.warnings(zones, w)
	if rows, ok := zones.problemRows(warnings, changed, shell.Language); ok {
		shell.Problems.Rows = rows
	}
	shell.Pins = nil
	if w != nil {
		shell.Pins = pinLabels(z.pins, w, &ws.cam, vp, shell.Language)
	}
	if zones.anchored && w != nil && ws.probe.inside {
		h, ok := pickAt(w, &ws.cam, ws.probe.cursor, vp)
		if msg := zones.hoverAt(w, z.cover, h, ok); msg != "" {
			ws.status = editorResult(msg, zones)
		}
	} else {
		zones.hoverAt(nil, z.cover, scene.Hit{}, false)
	}
	shell.Zone.Tools.Armed, shell.Zone.Tools.Active = zones.tool, zones.armed
}

func (z *zoneMode) sync(r *render.Renderer, ws *workspace) {
	w := ws.world()
	if line, from := z.cover.groundLine(z.zones, w); z.zonesShown != z.zones.version || z.waterShown != z.waterSel.version || z.lineShown != from || !samePinShapes(z.pins, z.pinsShown) {
		shapes := append(z.zones.overlay(line), pinShapes(z.pins)...)
		r.SetZones(append(shapes, z.waterSel.overlay(w)...))
		z.zonesShown, z.waterShown, z.lineShown, z.pinsShown = z.zones.version, z.waterSel.version, from, z.pins
	}
	if z.groundShown != z.groundBuilt {
		r.SetGround(z.groundMark)
		z.groundShown = z.groundBuilt
	}
}

// forget makes the next sync send everything, and refills the height
// window's fields, which the population mode shares.
func (z *zoneMode) forget() {
	z.zonesShown, z.waterShown, z.groundShown = -1, -1, groundKey{version: -1}
	z.zones.heightFilled = heightKey{version: -1}
}
