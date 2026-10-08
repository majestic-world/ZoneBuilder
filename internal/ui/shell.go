package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// PanelWidth is the side panel's fixed width.
const PanelWidth = unit.Dp(300)

var (
	panelBackground  = color.NRGBA{R: 0x22, G: 0x24, B: 0x28, A: 0xFF}
	panelText        = color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
	fieldBackground  = color.NRGBA{R: 0x33, G: 0x36, B: 0x3C, A: 0xFF}
	statusBackground = color.NRGBA{R: 0x18, G: 0x19, B: 0x1C, A: 0xFF}
)

// Shell arranges the window: the viewport fills the top left, a
// fixed-width, scrolling panel on the right holds the project controls,
// the map controls (client folder, tile, recent maps), the info lines and
// the zone controls, and a status bar
// runs along the bottom.
type Shell struct {
	Theme    *material.Theme
	Viewport Viewport
	// Client is the client folder (the folder above Maps); Browse opens
	// the folder picker for it.
	Client widget.Editor
	Browse widget.Clickable
	// Tile is the map tile X_Y to open; Open (or Enter in Tile) opens it.
	Tile widget.Editor
	Open widget.Clickable
	// Neighbours opens the tile with its neighbours, up to 3×3 around the
	// camera's tile.
	Neighbours widget.Bool
	// Loading is the map loading line shown under Open, with Progress (0
	// to 1) as a bar; "" shows neither.
	Loading  string
	Progress float32
	// Project holds the project controls and the recent maps.
	Project ProjectPanel
	// Zone holds the zone controls.
	Zone ZonePanel
	// Zones is the zone list.
	Zones ZoneList
	// Props edits the selected zone's type and parameters.
	Props PropertiesPanel
	// Edit holds the undo and shape/vertex editing controls.
	Edit EditPanel
	// Status is the status bar's text.
	Status string
	list   widget.List
}

// NewShell returns a shell with single-line fields holding client, tile and
// the XML output folder.
func NewShell(th *material.Theme, client, tile, output string) *Shell {
	s := &Shell{Theme: th}
	s.Client.SingleLine = true
	s.Client.SetText(client)
	s.Tile.SingleLine = true
	s.Tile.Submit = true
	s.Tile.SetText(tile)
	s.Neighbours.Value = true
	s.Zone.init(output)
	s.Zones.init()
	s.Props.init()
	s.Edit.init()
	s.list.Axis = layout.Vertical
	return s
}

// OpenRequested reports a click on Open or Enter in the tile field since
// the last call.
func (s *Shell) OpenRequested(gtx layout.Context) bool {
	open := s.Open.Clicked(gtx)
	for {
		ev, ok := s.Tile.Update(gtx)
		if !ok {
			return open
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			open = true
		}
	}
}

// Layout lays the window out and returns the viewport rectangle in window
// pixels (origin top-left), which is where the renderer must draw. Nothing
// is painted under the viewport, so the 3D content drawn before Gio's frame
// shows through. lines fill the side panel below the controls; Status
// fills the status bar.
func (s *Shell) Layout(gtx layout.Context, lines []string) image.Rectangle {
	var vp image.Rectangle
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					// The viewport is the first child of both Flexes, so
					// its origin is the window's origin.
					gtx.Constraints.Min = gtx.Constraints.Max
					dims := s.Viewport.Layout(gtx)
					vp = image.Rectangle{Max: s.Viewport.Size()}
					return dims
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return s.panel(gtx, lines)
				}),
			)
		}),
		layout.Rigid(s.statusBar),
	)
	return vp
}

func (s *Shell) statusBar(gtx layout.Context) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	dims := layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(s.Theme, s.Status)
			lbl.Color = panelText
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		})
	call := macro.Stop()
	paint.FillShape(gtx.Ops, statusBackground, clip.Rect{Max: dims.Size}.Op())
	call.Add(gtx.Ops)
	return dims
}

func (s *Shell) panel(gtx layout.Context, lines []string) layout.Dimensions {
	w := gtx.Dp(PanelWidth)
	size := image.Point{X: w, Y: gtx.Constraints.Max.Y}
	paint.FillShape(gtx.Ops, panelBackground, clip.Rect{Max: size}.Op())
	gtx.Constraints = layout.Exact(size)
	layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := append(s.projectControls(),
			layout.Rigid(s.label("Pasta do cliente")),
			layout.Rigid(s.field(&s.Client, "pasta acima de Maps")),
			layout.Rigid(s.button(&s.Browse, "Procurar…")),
			layout.Rigid(s.label("Tile (X_Y ou X_Y_Classic)")),
			layout.Rigid(s.field(&s.Tile, "22_22")),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, s.checkBox(&s.Neighbours, "Abrir com os vizinhos (até 3×3)"))
			}),
			layout.Rigid(s.button(&s.Open, "Abrir")),
			layout.Rigid(s.loading),
		)
		children = append(children, s.recentMaps()...)
		for _, l := range lines {
			children = append(children, layout.Rigid(s.label(l)))
		}
		children = append(children, s.zonePanel()...)
		children = append(children, s.zoneList()...)
		children = append(children, s.propertiesPanel()...)
		children = append(children, s.editPanel()...)
		// One list item holding the whole column: the panel scrolls when
		// the window is too short for it.
		return material.List(s.Theme, &s.list).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
	return layout.Dimensions{Size: size}
}

func (s *Shell) label(text string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lbl := material.Body2(s.Theme, text)
		lbl.Color = panelText
		return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, lbl.Layout)
	}
}

func (s *Shell) field(e *widget.Editor, hint string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					paint.FillShape(gtx.Ops, fieldBackground, clip.Rect{Max: gtx.Constraints.Min}.Op())
					return layout.Dimensions{Size: gtx.Constraints.Min}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					ed := material.Editor(s.Theme, e, hint)
					ed.Color = panelText
					ed.HintColor = dimText
					return layout.UniformInset(unit.Dp(6)).Layout(gtx, ed.Layout)
				}),
			)
		})
	}
}

func (s *Shell) button(c *widget.Clickable, text string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, material.Button(s.Theme, c, text).Layout)
	}
}

// loading is the map loading line and its progress bar, while Loading is
// set.
func (s *Shell) loading(gtx layout.Context) layout.Dimensions {
	if s.Loading == "" {
		return layout.Dimensions{}
	}
	return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(s.label(s.Loading)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return material.ProgressBar(s.Theme, s.Progress).Layout(gtx)
			}),
		)
	})
}
