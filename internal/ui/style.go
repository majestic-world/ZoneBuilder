package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"zonebuilder/internal/ui/fonts"
	"zonebuilder/internal/ui/icon"
)

// The palette: near-black translucent cards floating over the scene, one
// violet accent for the primary action and the active state, lime for
// creating and for what is valid, red for problems.
//
// Gio blends in linear light on the sRGB surface, where a faint white
// overlay comes out far brighter than the same alpha in a design tool;
// so the controls and lines on the cards use opaque colours, mixed in
// sRGB over the card surface, and only the cards themselves let the scene
// through.
var (
	cardSurface    = color.NRGBA{R: 0x12, G: 0x12, B: 0x16, A: 0xF0}
	hairline       = color.NRGBA{R: 0x2B, G: 0x2B, B: 0x33, A: 0xFF}
	strongLine     = color.NRGBA{R: 0x4A, G: 0x4A, B: 0x55, A: 0xFF}
	controlFill    = color.NRGBA{R: 0x1D, G: 0x1D, B: 0x24, A: 0xFF}
	controlHover   = color.NRGBA{R: 0x27, G: 0x27, B: 0x30, A: 0xFF}
	controlPress   = color.NRGBA{R: 0x30, G: 0x30, B: 0x3A, A: 0xFF}
	textColor      = color.NRGBA{R: 0xEC, G: 0xEC, B: 0xF1, A: 0xFF}
	dimText        = color.NRGBA{R: 0x9C, G: 0x9C, B: 0xAB, A: 0xFF}
	faintText      = color.NRGBA{R: 0x6E, G: 0x6E, B: 0x7C, A: 0xFF}
	accent         = color.NRGBA{R: 0x8B, G: 0x5C, B: 0xF6, A: 0xFF}
	accentHover    = color.NRGBA{R: 0x9D, G: 0x76, B: 0xF8, A: 0xFF}
	accentPress    = color.NRGBA{R: 0x7C, G: 0x4D, B: 0xE8, A: 0xFF}
	accentSoft     = color.NRGBA{R: 0x2A, G: 0x21, B: 0x43, A: 0xFF}
	accentText     = color.NRGBA{R: 0xC4, G: 0xB0, B: 0xFC, A: 0xFF}
	lime           = color.NRGBA{R: 0xA3, G: 0xE6, B: 0x35, A: 0xFF}
	limeHover      = color.NRGBA{R: 0xB5, G: 0xEE, B: 0x5C, A: 0xFF}
	limePress      = color.NRGBA{R: 0x8F, G: 0xCC, B: 0x2A, A: 0xFF}
	limeInk        = color.NRGBA{R: 0x1A, G: 0x2E, B: 0x05, A: 0xFF}
	errorText      = color.NRGBA{R: 0xF8, G: 0x71, B: 0x71, A: 0xFF}
	errorSoft      = color.NRGBA{R: 0x2E, G: 0x1D, B: 0x21, A: 0xFF}
	errorSoftHover = color.NRGBA{R: 0x49, G: 0x29, B: 0x2C, A: 0xFF}
	white          = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
)

// The metrics, in dp: cards, controls and the gaps between them.
const (
	cardRadius    = unit.Dp(6)
	controlRadius = unit.Dp(4)
	controlHeight = unit.Dp(32)
	iconSize      = unit.Dp(16)
	cardPadding   = unit.Dp(16)
	floatMargin   = unit.Dp(16)
)

// The type scale, in sp.
const (
	bodySize    = unit.Sp(13)
	smallSize   = unit.Sp(12)
	captionSize = unit.Sp(11)
	titleSize   = unit.Sp(15)
)

// NewTheme is the window's theme: Inter on the dark palette.
func NewTheme() *material.Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(fonts.Collection()))
	th.Face = fonts.Interface
	th.TextSize = bodySize
	th.Palette = material.Palette{Fg: textColor, Bg: cardSurface, ContrastBg: accent, ContrastFg: white}
	return th
}

// rrect is the rounded rectangle of size with corner radius r, in pixels.
func rrect(gtx layout.Context, size image.Point, r unit.Dp) clip.RRect {
	return clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(r))
}

// fillRRect paints the rounded rectangle of size in c, with a 1 px border
// in border unless its alpha is 0.
func fillRRect(gtx layout.Context, size image.Point, r unit.Dp, c, border color.NRGBA) {
	rr := rrect(gtx, size, r)
	paint.FillShape(gtx.Ops, c, rr.Op(gtx.Ops))
	if border.A != 0 {
		paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
	}
}

// pointerSink takes every pointer event over the area it is added to, so
// a card floating over the viewport keeps the clicks, drags and wheel
// that miss its controls from reaching the scene below.
type pointerSink struct{ _ byte }

// add takes the pointer input over size; controls laid out afterwards sit
// on top of it and still get theirs.
func (k *pointerSink) add(gtx layout.Context, size image.Point) {
	area := clip.Rect{Max: size}.Push(gtx.Ops)
	event.Op(gtx.Ops, k)
	area.Pop()
	for {
		if _, ok := gtx.Event(pointer.Filter{
			Target:  k,
			Kinds:   pointer.Press | pointer.Release | pointer.Drag | pointer.Move | pointer.Scroll,
			ScrollX: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20},
			ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20},
		}); !ok {
			return
		}
	}
}

// card lays w out on a floating card: translucent surface, hairline
// border, padding inset, and a pointer sink under it all.
func card(gtx layout.Context, sink *pointerSink, inset layout.Inset, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := inset.Layout(gtx, w)
	call := m.Stop()
	fillRRect(gtx, dims.Size, cardRadius, cardSurface, hairline)
	sink.add(gtx, dims.Size)
	call.Add(gtx.Ops)
	return dims
}

// at lays w out with its top-left corner at p, measured with no minimum
// size, and returns its size.
func at(gtx layout.Context, p image.Point, w layout.Widget) image.Point {
	defer op.Offset(p).Push(gtx.Ops).Pop()
	gtx.Constraints.Min = image.Point{}
	return w(gtx).Size
}

// measure records w with no minimum size, to place it once its size is
// known.
func measure(gtx layout.Context, w layout.Widget) (op.CallOp, image.Point) {
	m := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	dims := w(gtx)
	return m.Stop(), dims.Size
}

// pointerCursor shows the hand over size.
func pointerCursor(gtx layout.Context, size image.Point) {
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
}

// text is one run of text in the given size, weight and colour; maxLines
// 0 wraps freely.
func (s *Shell) text(txt string, size unit.Sp, weight font.Weight, c color.NRGBA, maxLines int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lbl := material.Label(s.Theme, size, txt)
		lbl.Font.Weight = weight
		lbl.Color = c
		lbl.MaxLines = maxLines
		return lbl.Layout(gtx)
	}
}

// label is body text with room under it.
func (s *Shell) label(txt string) layout.Widget {
	return s.spaced(s.text(txt, bodySize, font.Normal, textColor, 0))
}

// dimLabel is secondary text with room under it.
func (s *Shell) dimLabel(txt string) layout.Widget {
	return s.spaced(s.text(txt, smallSize, font.Normal, dimText, 0))
}

// errorLabel is an error message with room under it.
func (s *Shell) errorLabel(txt string) layout.Widget {
	return s.spaced(s.text(txt, smallSize, font.Normal, errorText, 0))
}

// fieldLabel names the control below it.
func (s *Shell) fieldLabel(txt string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, s.text(txt, smallSize, font.Medium, dimText, 1))
	}
}

// spaced puts the gap between stacked controls under w.
func (s *Shell) spaced(w layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, w)
	}
}

// buttonKind is a button's look.
type buttonKind int

const (
	// primaryButton is the violet call to action.
	primaryButton buttonKind = iota
	// secondaryButton is a quiet filled button.
	secondaryButton
	// limeButton creates something.
	limeButton
	// dangerButton destroys something: red outline.
	dangerButton
	// ghostButton has no surface until hovered.
	ghostButton
)

// colors are the button's surface, border and ink for its state.
func (k buttonKind) colors(hovered, pressed bool) (bg, border, ink color.NRGBA) {
	pick := func(rest, hover, press color.NRGBA) color.NRGBA {
		switch {
		case pressed:
			return press
		case hovered:
			return hover
		}
		return rest
	}
	switch k {
	case primaryButton:
		return pick(accent, accentHover, accentPress), color.NRGBA{}, white
	case limeButton:
		return pick(lime, limeHover, limePress), color.NRGBA{}, limeInk
	case dangerButton:
		return pick(color.NRGBA{}, errorSoft, errorSoftHover), errorText, errorText
	case ghostButton:
		return pick(color.NRGBA{}, controlHover, controlPress), color.NRGBA{}, textColor
	}
	return pick(controlFill, controlHover, controlPress), hairline, textColor
}

// button is a button of kind k with an optional icon before its text (no
// text: an icon-only square). It is as wide as its content, or as the
// minimum width when wider, with the content centred.
func (s *Shell) button(c *widget.Clickable, k buttonKind, ic *icon.Icon, txt string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			bg, border, ink := k.colors(c.Hovered(), c.Pressed())
			h := gtx.Dp(controlHeight)
			call, content := measure(gtx, func(gtx layout.Context) layout.Dimensions {
				pad := unit.Dp(12)
				if txt == "" {
					pad = unit.Dp(8)
				}
				return layout.Inset{Left: pad, Right: pad}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if ic == nil {
								return layout.Dimensions{}
							}
							return ic.Layout(gtx, iconSize, ink)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if ic == nil || txt == "" {
								return layout.Dimensions{}
							}
							return layout.Spacer{Width: unit.Dp(6)}.Layout(gtx)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if txt == "" {
								return layout.Dimensions{}
							}
							return s.text(txt, bodySize, font.Medium, ink, 1)(gtx)
						}),
					)
				})
			})
			size := image.Pt(max(content.X, gtx.Constraints.Min.X), h)
			if txt == "" {
				size.X = max(size.X, h)
			}
			fillRRect(gtx, size, controlRadius, bg, border)
			off := op.Offset(image.Pt((size.X-content.X)/2, (size.Y-content.Y)/2)).Push(gtx.Ops)
			call.Add(gtx.Ops)
			off.Pop()
			pointerCursor(gtx, size)
			return layout.Dimensions{Size: size, Baseline: (size.Y - content.Y) / 2}
		})
	}
}

// fullButton is button stretched over the width it is given.
func (s *Shell) fullButton(c *widget.Clickable, k buttonKind, ic *icon.Icon, txt string) layout.Widget {
	b := s.button(c, k, ic, txt)
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return b(gtx)
	}
}

// iconToggle is a square icon button that shows active as a violet tint.
func (s *Shell) iconToggle(c *widget.Clickable, ic *icon.Icon, active bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			n := gtx.Dp(controlHeight)
			size := image.Pt(n, n)
			bg, _, ink := ghostButton.colors(c.Hovered(), c.Pressed())
			if active {
				bg, ink = accentSoft, accentText
			}
			fillRRect(gtx, size, controlRadius, bg, color.NRGBA{})
			off := (n - gtx.Dp(iconSize)) / 2
			at(gtx, image.Pt(off, off), func(gtx layout.Context) layout.Dimensions { return ic.Layout(gtx, iconSize, ink) })
			pointerCursor(gtx, size)
			return layout.Dimensions{Size: size}
		})
	}
}

// buttonRow lays buttons out side by side, sharing the width equally, with
// a gap between them.
func buttonRow(buttons ...layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 2*len(buttons))
		for i, b := range buttons {
			if i > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout))
			}
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return b(gtx)
			}))
		}
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx, children...)
		})
	}
}

// field is a single-line text field with an optional icon inside on the
// left; the border turns violet while it has the focus.
func (s *Shell) field(e *widget.Editor, hint string, lead *icon.Icon) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return s.bareField(gtx, e, hint, lead)
		})
	}
}

// bareField is field without the gap under it.
func (s *Shell) bareField(gtx layout.Context, e *widget.Editor, hint string, lead *icon.Icon) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	h := gtx.Dp(controlHeight)
	size := image.Pt(gtx.Constraints.Max.X, h)
	border := hairline
	if gtx.Source.Focused(e) {
		border = accent
	}
	fillRRect(gtx, size, controlRadius, controlFill, border)
	left := unit.Dp(10)
	if lead != nil {
		off := (h - gtx.Dp(iconSize)) / 2
		at(gtx, image.Pt(gtx.Dp(10), off), func(gtx layout.Context) layout.Dimensions { return lead.Layout(gtx, iconSize, dimText) })
		left = unit.Dp(10) + iconSize + unit.Dp(8)
	}
	ed := material.Editor(s.Theme, e, hint)
	ed.Color = textColor
	ed.HintColor = faintText
	ed.SelectionColor = accentSoft
	ed.TextSize = bodySize
	call, content := measure(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = max(0, size.X-gtx.Dp(left)-gtx.Dp(10))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return ed.Layout(gtx)
	})
	off := op.Offset(image.Pt(gtx.Dp(left), (h-content.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	off.Pop()
	return layout.Dimensions{Size: size}
}

// fieldButton is a field with a button after it, on one line.
func (s *Shell) fieldButton(e *widget.Editor, hint string, lead *icon.Icon, b layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return s.bareField(gtx, e, hint, lead) }),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(b),
			)
		})
	}
}

// checkBox is a rounded box, violet with a check mark when set, and its
// text.
func (s *Shell) checkBox(b *widget.Bool, txt string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return b.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return checkMark(gtx, b.Value, b.Hovered()) }),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if txt == "" {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, s.text(txt, bodySize, font.Normal, textColor, 1))
				}),
			)
		})
	}
}

// checkMark is the 16 dp box of a check box.
func checkMark(gtx layout.Context, on, hovered bool) layout.Dimensions {
	n := gtx.Dp(16)
	size := image.Pt(n, n)
	switch {
	case on:
		fillRRect(gtx, size, 3, accent, color.NRGBA{})
		off := (n - gtx.Dp(12)) / 2
		at(gtx, image.Pt(off, off), func(gtx layout.Context) layout.Dimensions { return icon.Check.Layout(gtx, 12, white) })
	case hovered:
		fillRRect(gtx, size, 3, controlHover, strongLine)
	default:
		fillRRect(gtx, size, 3, controlFill, strongLine)
	}
	pointerCursor(gtx, size)
	return layout.Dimensions{Size: size}
}

// chip is a small pill button; current shows it violet.
func (s *Shell) chip(c *widget.Clickable, txt string, current bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			bg, border, ink := secondaryButton.colors(c.Hovered(), c.Pressed())
			if current {
				bg, border, ink = accentSoft, accent, accentText
			}
			call, content := measure(gtx, s.text(txt, smallSize, font.Medium, ink, 1))
			h := gtx.Dp(26)
			size := image.Pt(content.X+gtx.Dp(24), h)
			fillRRect(gtx, size, controlRadius, bg, border)
			off := op.Offset(image.Pt(gtx.Dp(12), (h-content.Y)/2)).Push(gtx.Ops)
			call.Add(gtx.Ops)
			off.Pop()
			pointerCursor(gtx, size)
			return layout.Dimensions{Size: size}
		})
	}
}

// stepper is "‹ text ›" in a field-like box: prev and next step through a
// closed list; with toggle set, clicking the text clicks it (opening the
// whole list).
func (s *Shell) stepper(prev, next, toggle *widget.Clickable, txt string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(controlHeight))
			fillRRect(gtx, size, controlRadius, controlFill, hairline)
			gtx.Constraints = layout.Exact(size)
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(s.iconToggle(prev, icon.ChevronLeft, false)),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					w := func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min = gtx.Constraints.Max
						return layout.Center.Layout(gtx, s.text(txt, bodySize, font.Medium, textColor, 1))
					}
					if toggle == nil {
						return w(gtx)
					}
					return toggle.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						dims := w(gtx)
						pointerCursor(gtx, dims.Size)
						return dims
					})
				}),
				layout.Rigid(s.iconToggle(next, icon.ChevronRight, false)),
			)
		})
	}
}

// listItem is one clickable row of a closed list; the current one is
// violet.
func (s *Shell) listItem(c *widget.Clickable, txt string, current bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			call, content := measure(gtx, func(gtx layout.Context) layout.Dimensions {
				ink := dimText
				if current {
					ink = accentText
				}
				return layout.Inset{Top: unit.Dp(5), Bottom: unit.Dp(5), Left: unit.Dp(12)}.Layout(gtx, s.text(txt, bodySize, font.Normal, ink, 1))
			})
			size := image.Pt(gtx.Constraints.Max.X, content.Y)
			switch {
			case current:
				fillRRect(gtx, size, controlRadius, accentSoft, color.NRGBA{})
			case c.Hovered():
				fillRRect(gtx, size, controlRadius, controlHover, color.NRGBA{})
			}
			call.Add(gtx.Ops)
			pointerCursor(gtx, size)
			return layout.Dimensions{Size: size}
		})
	}
}

// section is an inspector section: a divider above (unless first), the
// icon and title, an optional trailing note, then its controls.
func (s *Shell) section(first bool, ic *icon.Icon, title, note string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		top := unit.Dp(0)
		if !first {
			top = unit.Dp(18)
			w := gtx.Constraints.Max.X
			at(gtx, image.Pt(0, gtx.Dp(8)), func(gtx layout.Context) layout.Dimensions {
				size := image.Pt(w, gtx.Dp(1))
				paint.FillShape(gtx.Ops, hairline, clip.Rect{Max: size}.Op())
				return layout.Dimensions{Size: size}
			})
		}
		return layout.Inset{Top: top, Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ic.Layout(gtx, iconSize, dimText) }),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Flexed(1, s.text(title, bodySize, font.SemiBold, textColor, 1)),
				layout.Rigid(s.text(note, smallSize, font.Normal, dimText, 1)),
			)
		})
	}
}

// progressBar is a thin violet bar filled to p (0 to 1).
func progressBar(gtx layout.Context, p float32) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(4))
	fillRRect(gtx, size, 2, controlHover, color.NRGBA{})
	done := image.Pt(int(float32(size.X)*min(max(p, 0), 1)), size.Y)
	if done.X > 0 {
		fillRRect(gtx, done, 2, accent, color.NRGBA{})
	}
	return layout.Dimensions{Size: size}
}

// scrollList is the material list styled to the palette: a slim overlay
// scrollbar.
func (s *Shell) scrollList(l *widget.List) material.ListStyle {
	ls := material.List(s.Theme, l)
	ls.AnchorStrategy = material.Overlay
	ls.Indicator.Color = strongLine
	ls.Indicator.HoverColor = faintText
	ls.Indicator.MinorWidth = 4
	ls.Indicator.CornerRadius = 2
	ls.Track.Color = color.NRGBA{}
	return ls
}
