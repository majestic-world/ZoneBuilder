package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"zonebuilder/internal/ui/icon"
)

// ContextMenu is a popup card of items at a point of the window. While it
// is open a press anywhere off the card closes it, and so does Esc; the
// owner closes it when an item is chosen (Chosen).
type ContextMenu struct {
	open bool
	// at is the card's top-left corner, in window pixels.
	at      image.Point
	dismiss pointerSink
	sink    pointerSink
}

// MenuItem is one row of a ContextMenu: an icon and a text. A disabled
// item is greyed out and takes no click.
type MenuItem struct {
	Click    *widget.Clickable
	Icon     *icon.Icon
	Text     string
	Disabled bool
}

// Open opens the menu with its top-left corner at p, in window pixels.
func (m *ContextMenu) Open(p image.Point) {
	m.open, m.at = true, p
}

// Close closes the menu and reports whether it was open.
func (m *ContextMenu) Close() bool {
	was := m.open
	m.open = false
	return was
}

// IsOpen reports whether the menu is open.
func (m *ContextMenu) IsOpen() bool { return m.open }

// Chosen reports a click on item c since the last call, closing the menu
// after one.
func (m *ContextMenu) Chosen(gtx layout.Context, c *widget.Clickable) bool {
	if !c.Clicked(gtx) {
		return false
	}
	m.open = false
	return true
}

// contextMenu lays m out over everything drawn before it, kept inside the
// window, while it is open.
func (s *Shell) contextMenu(gtx layout.Context, m *ContextMenu, items ...MenuItem) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: &m.dismiss, Kinds: pointer.Press})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok && e.Kind == pointer.Press {
			m.open = false
		}
	}
	if m.open {
		// Without the viewport's focus Esc comes here; with it, the window
		// loop closes the menu (Shell.CloseMenus).
		for {
			ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
			if !ok {
				break
			}
			if e, ok := ev.(key.Event); ok && e.State == key.Press {
				m.open = false
			}
		}
	}
	if !m.open {
		return
	}
	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, &m.dismiss)
	area.Pop()

	rows := make([]layout.FlexChild, len(items))
	for i, it := range items {
		rows[i] = layout.Rigid(s.menuItem(it))
	}
	call, size := measure(gtx, func(gtx layout.Context) layout.Dimensions {
		return card(gtx, &m.sink, layout.UniformInset(unit.Dp(6)), func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		})
	})
	p := image.Pt(
		max(0, min(m.at.X, gtx.Constraints.Max.X-size.X)),
		max(0, min(m.at.Y, gtx.Constraints.Max.Y-size.Y)),
	)
	place(gtx, p, call)
}

// menuItem is a 220×34 dp row: the icon, then the text; hovering tints
// it, unless it is disabled.
func (s *Shell) menuItem(it MenuItem) layout.Widget {
	row := func(gtx layout.Context, hovered bool) layout.Dimensions {
		size := image.Pt(gtx.Dp(220), gtx.Dp(34))
		ink := textColor
		if it.Disabled {
			ink = dimText
		} else if hovered {
			fillRRect(gtx, size, controlRadius, controlHover, color.NRGBA{})
		}
		at(gtx, image.Pt(gtx.Dp(10), (size.Y-gtx.Dp(iconSize))/2), func(gtx layout.Context) layout.Dimensions {
			return it.Icon.Layout(gtx, iconSize, dimText)
		})
		call, t := measure(gtx, s.text(it.Text, bodySize, font.Normal, ink, 1))
		place(gtx, image.Pt(gtx.Dp(36), (size.Y-t.Y)/2), call)
		return layout.Dimensions{Size: size}
	}
	return func(gtx layout.Context) layout.Dimensions {
		if it.Disabled {
			return row(gtx, false)
		}
		return it.Click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			dims := row(gtx, it.Click.Hovered())
			pointerCursor(gtx, dims.Size)
			return dims
		})
	}
}
