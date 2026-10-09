package ui

import (
	"image"
	"image/color"
	"math"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Ruler is the height window's vertical scale of the selected zone (spec
// D7), read the same from any camera angle: Z runs up from Lo to Hi, the
// zone's range is a bar, the floor's area by Z is a histogram coloured by
// state beside it and Marks label the lowest and highest floor. It only
// shows; the window loop fills it.
type Ruler struct {
	// Lo and Hi are the Z at the bottom and the top of the scale; Hi ≤ Lo
	// hides the ruler.
	Lo, Hi float64
	// ZMin and ZMax are the zone's range, drawn as a bar in Color.
	ZMin, ZMax float64
	Color      color.NRGBA
	// Bars are the histogram's rows, bottom to top, each an equal slice of
	// [Lo, Hi]: the floor area there inside the range, above and below it.
	Bars []RulerBar
	// Marks are lines across the histogram at a Z, with a label.
	Marks []RulerMark
}

// RulerBar is the floor area in one row of the ruler, by state.
type RulerBar struct{ Inside, Above, Below float64 }

// RulerMark is a line across the ruler at Z, labelled Text; Alert writes
// it in the error colour.
type RulerMark struct {
	Z     float64
	Text  string
	Alert bool
}

// The ruler's colours for floor above and below the range: the hues of the
// viewport's hatches (render's groundAbove and groundBelow).
var (
	rulerAbove = color.NRGBA{R: 0xFF, G: 0x5A, B: 0x1E, A: 0xFF}
	rulerBelow = color.NRGBA{R: 0x28, G: 0xAA, B: 0xFF, A: 0xFF}
)

const (
	rulerHeight = unit.Dp(190)
	// rulerAxis is the room for the range's Z on the left, rulerBar the
	// range bar's width and rulerHist the histogram's longest bar.
	rulerAxis = unit.Dp(46)
	rulerBar  = unit.Dp(10)
	rulerHist = unit.Dp(96)
)

// ruler lays r out across the width, rulerHeight tall.
func (s *Shell) ruler(r Ruler) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if r.Hi <= r.Lo {
			return layout.Dimensions{}
		}
		w := gtx.Constraints.Max.X
		pad := gtx.Dp(7) // room for the labels at the ends
		h := gtx.Dp(rulerHeight)
		span := float64(h - 2*pad)
		y := func(z float64) int { return pad + int(math.Round((r.Hi-z)/(r.Hi-r.Lo)*span)) }
		fill := func(rect image.Rectangle, c color.NRGBA) {
			paint.FillShape(gtx.Ops, c, clip.Rect(rect).Op())
		}
		axis, barW, histW := gtx.Dp(rulerAxis), gtx.Dp(rulerBar), gtx.Dp(rulerHist)
		x0 := axis + barW + gtx.Dp(6) // the histogram's left edge

		// The axis line and the range bar with its Z on the left.
		fill(image.Rect(axis+barW/2, pad, axis+barW/2+1, h-pad), strongLine)
		top, bottom := y(r.ZMax), y(r.ZMin)
		fill(image.Rect(axis, top, axis+barW, max(bottom, top+1)), r.Color)
		label := func(z float64, yy int, c color.NRGBA) {
			call, size := measure(gtx, s.text(strconv.Itoa(int(math.Round(z))), captionSize, font.Medium, c, 1))
			place(gtx, image.Pt(axis-gtx.Dp(4)-size.X, yy-size.Y/2), call)
		}
		label(r.ZMax, top, textColor)
		if bottom-top > gtx.Dp(14) {
			label(r.ZMin, bottom, textColor)
		}

		// The histogram: each row's bar, below then inside then above the
		// range, as long as its area against the fullest row.
		var most float64
		for _, b := range r.Bars {
			most = max(most, b.Inside+b.Above+b.Below)
		}
		if n := len(r.Bars); n > 0 && most > 0 {
			for i, b := range r.Bars {
				yb := pad + int(math.Round(float64(n-i)*span/float64(n)))
				yt := pad + int(math.Round(float64(n-i-1)*span/float64(n)))
				yt = min(yt, yb-1)
				x := float64(x0)
				for _, part := range []struct {
					area float64
					c    color.NRGBA
				}{{b.Below, rulerBelow}, {b.Inside, r.Color}, {b.Above, rulerAbove}} {
					if part.area <= 0 {
						continue
					}
					next := x + part.area/most*float64(histW)
					fill(image.Rect(int(math.Round(x)), yt, max(int(math.Round(next)), int(math.Round(x))+1), yb), part.c)
					x = next
				}
			}
		}

		// The marks, top first: a line across the histogram and the label
		// after it, pushed down off the label above.
		below := math.MinInt
		for _, m := range r.Marks {
			ym := y(m.Z)
			ink := dimText
			if m.Alert {
				ink = errorText
			}
			fill(image.Rect(axis, ym, x0+histW, ym+1), ink)
			call, size := measure(gtx, s.text(m.Text, captionSize, font.Medium, ink, 1))
			at := max(ym-size.Y/2, below)
			place(gtx, image.Pt(min(x0+histW+gtx.Dp(4), w-size.X), at), call)
			below = at + size.Y
		}
		return layout.Dimensions{Size: image.Pt(w, h)}
	}
}
