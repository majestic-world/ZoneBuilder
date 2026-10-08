package main

import (
	"errors"
	"fmt"
	"image/color"
	"log"
	"math"
	"slices"
	"strconv"
	"strings"

	"zonebuilder/internal/camera"
	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/ui"
	"zonebuilder/internal/zone"
)

// goToHalfSize is half the side of the box a typed x y z is framed as: the
// camera stands about 3000 units off the point, above the roofs of a town
// such as Giran, with the ground around the point in view.
const goToHalfSize = 512

// zoneColors are the colours "Cor" steps a zone through after its type's.
var zoneColors = []zone.Color{
	{0xE6, 0x19, 0x4B}, {0xF5, 0x82, 0x31}, {0xFF, 0xE1, 0x19}, {0x3C, 0xB4, 0x4B},
	{0x42, 0xD4, 0xF4}, {0x43, 0x63, 0xD8}, {0xF0, 0x32, 0xE6}, {0xFF, 0xFF, 0xFF},
}

// linearColor is sRGB colour c in linear RGB, what the overlay takes.
func linearColor(c zone.Color) [3]float32 {
	ch := func(v uint8) float32 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return float32(s / 12.92)
		}
		return float32(math.Pow((s+0.055)/1.055, 2.4))
	}
	return [3]float32{ch(c[0]), ch(c[1]), ch(c[2])}
}

// rows is every zone as the zone list shows it.
func (e *zoneEditor) rows() []ui.ZoneRow {
	zones := e.doc.Zones()
	rows := make([]ui.ZoneRow, len(zones))
	counts := e.problemCounts()
	for i, z := range zones {
		c := z.DisplayColor()
		rows[i] = ui.ZoneRow{
			ID:       z.ID,
			Name:     z.Name,
			Type:     z.Type,
			Problems: counts[z.ID],
			Hidden:   z.Hidden,
			Color:    color.NRGBA{R: c[0], G: c[1], B: c[2], A: 0xFF},
			Compile:  !e.leftOut[z.ID],
		}
		if e.drawing && z.ID == e.zone {
			rows[i].Note = "desenhando"
		}
	}
	return rows
}

// selectedZone is the selected zone, the one the zone list highlights and
// the properties panel edits; false when none is.
func (e *zoneEditor) selectedZone() (zone.Zone, bool) {
	return e.doc.Zone(e.zone)
}

// listRequest carries out one zone list request; s and cam are the open
// scene (nil when none) and its camera. It returns the status line.
func (e *zoneEditor) listRequest(req any, s *scene.World, cam *camera.Camera) string {
	switch r := req.(type) {
	case ui.SelectZone:
		return e.selectZone(r.Zone, s, cam)
	case ui.HideZones:
		if e.apply(zone.SetHidden{Zones: r.Zones, Hidden: r.Hidden}) != nil {
			return "Não foi possível mudar a visibilidade"
		}
		verb := "exibida"
		if r.Hidden {
			verb = "oculta"
		}
		if len(r.Zones) == 1 {
			z, _ := e.doc.Zone(r.Zones[0])
			return fmt.Sprintf("Zona %s %s", z.Name, verb)
		}
		return inflect.Count(len(r.Zones), "zona "+verb, "zonas "+verb+"s")
	case ui.RenameZone:
		return e.rename(r.Zone, r.Name)
	case ui.DeleteZone:
		return e.deleteZone(r.Zone)
	case ui.DuplicateZone:
		return e.duplicate(r.Zone)
	case ui.CycleZoneColor:
		return e.cycleColor(r.Zone)
	case ui.GoTo:
		return goTo(r.Text, s, cam)
	case ui.SelectForCompile:
		return e.selectForCompile(r.Zones, r.Compile)
	}
	return ""
}

// selectZone selects zone id and frames it. While a polygon is being drawn
// the selection stays on its zone (the tool adds to it), but the camera
// still goes.
func (e *zoneEditor) selectZone(id zone.ZoneID, s *scene.World, cam *camera.Camera) string {
	z, ok := e.doc.Zone(id)
	if !ok {
		return ""
	}
	msg := "Zona " + z.Name + " selecionada"
	if e.drawing && id != e.zone {
		msg = "Feche o polígono atual para selecionar outra zona"
	} else {
		e.zone, e.shape = id, 0
	}
	if s == nil {
		return msg
	}
	b := geom.EmptyBox()
	for _, sh := range z.Shapes {
		for _, p := range sh.Points {
			b.Include(renderPoint(s, p))
		}
	}
	if b.Empty() {
		return msg + "; ela não tem vértices para enquadrar"
	}
	cam.Frame(b)
	log.Printf("zona: câmera em %s: %s", z.Name, formatPose(cam, s))
	return msg
}

func (e *zoneEditor) rename(id zone.ZoneID, name string) string {
	name = strings.TrimSpace(name)
	z, ok := e.doc.Zone(id)
	switch {
	case !ok:
		return "Selecione uma zona na lista"
	case name == "":
		return "Digite o novo nome"
	case name == z.Name:
		return ""
	}
	old := z.Name
	if e.apply(zone.Rename{Zone: id, Name: name}) != nil {
		return "Não foi possível renomear a zona"
	}
	log.Printf("zona: %s renomeada para %s", old, name)
	return fmt.Sprintf("Zona %s renomeada para %s", old, name)
}

func (e *zoneEditor) deleteZone(id zone.ZoneID) string {
	z, ok := e.doc.Zone(id)
	if !ok {
		return "Selecione uma zona na lista"
	}
	if e.apply(zone.DeleteZone{Zone: id}) != nil {
		return "Não foi possível apagar a zona"
	}
	if e.zone == id {
		e.drawing, e.zone, e.shape = false, 0, 0
	}
	log.Printf("zona: %s apagada", z.Name)
	return "Zona " + z.Name + " apagada"
}

func (e *zoneEditor) duplicate(id zone.ZoneID) string {
	z, ok := e.doc.Zone(id)
	switch {
	case !ok:
		return "Selecione uma zona na lista"
	case e.drawing:
		return "Feche o polígono antes de duplicar"
	}
	dup := e.doc.NewZoneID()
	if e.apply(zone.DuplicateZone{Zone: id, ID: dup}) != nil {
		return "Não foi possível duplicar a zona"
	}
	e.zone, e.shape = dup, 0
	c, _ := e.doc.Zone(dup)
	log.Printf("zona: %s duplicada como %s", z.Name, c.Name)
	return fmt.Sprintf("Zona %s duplicada como %s", z.Name, c.Name)
}

// cycleColor steps a zone's colour: its type's, then each of zoneColors,
// then its type's again.
func (e *zoneEditor) cycleColor(id zone.ZoneID) string {
	z, ok := e.doc.Zone(id)
	if !ok {
		return "Selecione uma zona na lista"
	}
	next := zone.Color{} // back to the type's
	if i := slices.Index(zoneColors, z.Color); z.Color == (zone.Color{}) {
		next = zoneColors[0]
	} else if i >= 0 && i+1 < len(zoneColors) {
		next = zoneColors[i+1]
	}
	if e.apply(zone.SetColor{Zone: id, Color: next}) != nil {
		return "Não foi possível mudar a cor"
	}
	if next == (zone.Color{}) {
		return "Zona " + z.Name + " com a cor do tipo"
	}
	return fmt.Sprintf("Zona %s com a cor #%02X%02X%02X", z.Name, next[0], next[1], next[2])
}

// goTo frames the server point typed as "x y z" (spaces, commas or
// semicolons between the numbers). It returns the status line.
func goTo(text string, s *scene.World, cam *camera.Camera) string {
	p, err := parsePoint(text)
	if err != nil {
		return err.Error()
	}
	if s == nil {
		return "Abra um tile antes de ir para um ponto"
	}
	c := scene.ToRender(scene.FromServer(p).Sub(s.Origin))
	h := geom.Vec3{X: goToHalfSize, Y: goToHalfSize, Z: goToHalfSize}
	cam.Frame(geom.Box{Min: c.Sub(h), Max: c.Add(h)})
	log.Printf("zona: câmera indo para %g %g %g: %s", p.X, p.Y, p.Z, formatPose(cam, s))
	return fmt.Sprintf("Câmera em %g %g %g", p.X, p.Y, p.Z)
}

// parsePoint reads a server x y z.
func parsePoint(text string) (geom.Vec3, error) {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\t' || r == ',' || r == ';'
	})
	if len(fields) != 3 {
		return geom.Vec3{}, errors.New("Digite x y z, 3 números separados por espaço")
	}
	var v [3]float32
	for i, f := range fields {
		n, err := strconv.ParseFloat(f, 32)
		if err != nil {
			return geom.Vec3{}, fmt.Errorf("%q não é um número", f)
		}
		v[i] = float32(n)
	}
	return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, nil
}
