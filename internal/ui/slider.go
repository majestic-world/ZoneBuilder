package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// sliderLabelWidth is the column the slider names take on their left.
const sliderLabelWidth = unit.Dp(92)

// slider is a horizontal slider over f with its name on the left: a track
// filled violet up to the thumb, which follows the pointer while dragged.
func (s *Shell) slider(f *widget.Float, name string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					w := gtx.Dp(sliderLabelWidth)
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
					return s.text(name, smallSize, font.Normal, dimText, 1)(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return sliderTrack(gtx, f)
				}),
			)
		})
	}
}

// sliderTrack lays f out over the width it is given: the thumb's center
// runs from one end of the track to the other, a thumb radius in from the
// edges, and presses up to a radius past them still count.
func sliderTrack(gtx layout.Context, f *widget.Float) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(22))
	r := gtx.Dp(7)
	length := max(size.X-2*r, 1)
	pos := min(max(f.Value, 0), 1)
	thumb := r + int(float32(length)*pos)
	bar := gtx.Dp(4)
	y := (size.Y - bar) / 2
	paint.FillShape(gtx.Ops, controlHover, clip.UniformRRect(image.Rect(r, y, r+length, y+bar), bar/2).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, accent, clip.UniformRRect(image.Rect(r, y, thumb, y+bar), bar/2).Op(gtx.Ops))
	c := image.Pt(thumb, size.Y/2)
	paint.FillShape(gtx.Ops, accent, clip.Ellipse{Min: c.Sub(image.Pt(r, r)), Max: c.Add(image.Pt(r, r))}.Op(gtx.Ops))
	in := r - gtx.Dp(3)
	paint.FillShape(gtx.Ops, white, clip.Ellipse{Min: c.Sub(image.Pt(in, in)), Max: c.Add(image.Pt(in, in))}.Op(gtx.Ops))

	off := op.Offset(image.Pt(r, 0)).Push(gtx.Ops)
	g := gtx
	g.Constraints.Min = image.Pt(length, size.Y)
	f.Layout(g, layout.Horizontal, unit.Dp(7))
	off.Pop()
	pointerCursor(gtx, size)
	return layout.Dimensions{Size: size}
}
