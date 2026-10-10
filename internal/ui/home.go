package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/icon"
)

// Mode is what the window shows: the home screen, or one of the 2
// editors over the shared map, viewport, camera, command bar and project.
type Mode uint8

const (
	// ModeHome is the home screen: one card per editor over the veiled
	// viewport.
	ModeHome Mode = iota
	// ModeZones is the zone editor ("Construir zonas").
	ModeZones
	// ModePopulate is the spawn area editor ("Popular zona").
	ModePopulate
)

// HomeScreen holds the home screen's 2 cards and the command bar's Início
// button. It only collects input: ModeRequested reports the picks and the
// window loop switches Shell.Mode.
type HomeScreen struct {
	Zones, Populate widget.Clickable
	// Back is the command bar's Início button.
	Back widget.Clickable
	veil pointerSink
}

// ModeRequested reports the mode picked since the last call: a home
// card's, or ModeHome from Início.
func (s *Shell) ModeRequested(gtx layout.Context) (Mode, bool) {
	h := &s.Home
	switch {
	case h.Zones.Clicked(gtx):
		return ModeZones, true
	case h.Populate.Clicked(gtx):
		return ModePopulate, true
	case h.Back.Clicked(gtx):
		return ModeHome, true
	}
	return 0, false
}

// homeVeilColor dims the scene behind the home screen; whatever map is
// open stays visible through it.
var homeVeilColor = color.NRGBA{R: 0x0B, G: 0x0B, B: 0x0F, A: 0xD8}

// homeCardWidth is the width of each home card.
const homeCardWidth = unit.Dp(300)

// homeVeil covers the viewport and takes its pointer input, so nothing
// reaches the scene while the home screen is up.
func (s *Shell) homeVeil(gtx layout.Context) {
	size := gtx.Constraints.Max
	paint.FillShape(gtx.Ops, homeVeilColor, clip.Rect{Max: size}.Op())
	s.Home.veil.add(gtx, size)
}

// homeOption is one home card: the clickable, its icon, title and phrase.
type homeOption struct {
	click         *widget.Clickable
	icon          *icon.Icon
	title, phrase string
}

// homeCards centres the heading and the 2 mode cards, equally tall, on
// the window.
func (s *Shell) homeCards(gtx layout.Context) {
	h := &s.Home
	options := [...]homeOption{
		{&h.Zones, icon.LandPlot, locale.Text(s.Language, "spawn.home.zones.title"), locale.Text(s.Language, "spawn.home.zones.phrase")},
		{&h.Populate, icon.Users, locale.Text(s.Language, "spawn.home.populate.title"), locale.Text(s.Language, "spawn.home.populate.phrase")},
	}
	w, gap := gtx.Dp(homeCardWidth), gtx.Dp(16)
	var contents [len(options)]op.CallOp
	height := 0
	for i, o := range options {
		var size image.Point
		contents[i], size = measure(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
			return s.homeOptionContent(gtx, o)
		})
		height = max(height, size.Y)
	}
	heading, headingSize := measure(gtx, s.text(locale.Text(s.Language, "spawn.home.title"), unit.Sp(20), font.SemiBold, textColor, 1))
	area := gtx.Constraints.Max
	below := headingSize.Y + gtx.Dp(20)
	total := image.Pt(len(options)*w+(len(options)-1)*gap, below+height)
	origin := image.Pt((area.X-total.X)/2, (area.Y-total.Y)/2)
	place(gtx, image.Pt((area.X-headingSize.X)/2, origin.Y), heading)
	for i, o := range options {
		at(gtx, origin.Add(image.Pt(i*(w+gap), below)), func(gtx layout.Context) layout.Dimensions {
			return o.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := image.Pt(w, height)
				border := hairline
				if o.click.Hovered() {
					border = accent
				}
				fillRRect(gtx, size, cardRadius, cardSurface, border)
				contents[i].Add(gtx.Ops)
				pointerCursor(gtx, size)
				return layout.Dimensions{Size: size}
			})
		})
	}
}

// homeOptionContent is a home card's inside: the icon on a violet tile,
// the title and the phrase under it.
func (s *Shell) homeOptionContent(gtx layout.Context, o homeOption) layout.Dimensions {
	return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				n := gtx.Dp(44)
				fillRRect(gtx, image.Pt(n, n), cardRadius, accentSoft, color.NRGBA{})
				off := (n - gtx.Dp(24)) / 2
				at(gtx, image.Pt(off, off), func(gtx layout.Context) layout.Dimensions { return o.icon.Layout(gtx, 24, accentText) })
				return layout.Dimensions{Size: image.Pt(n, n)}
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(s.text(o.title, titleSize, font.SemiBold, textColor, 1)),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(s.text(o.phrase, bodySize, font.Normal, dimText, 0)),
		)
	})
}
