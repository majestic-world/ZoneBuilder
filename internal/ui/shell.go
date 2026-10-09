package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	appicon "zonebuilder/assets/icon"
	"zonebuilder/internal/ui/icon"
)

// InspectorWidth is the inspector card's width.
const InspectorWidth = unit.Dp(360)

// logo is the app icon, drawn as the brand mark.
var logo = func() *icon.Icon {
	ic, err := icon.Parse(appicon.SVG)
	if err != nil {
		// The file is embedded: a parse failure is a broken build.
		panic(err)
	}
	return ic
}()

// Shell arranges the window: the 3D viewport fills it edge to edge and
// every control floats over it on cards: the brand mark top left, the tool
// dock under it, the command bar at the top, the inspector down the right
// side (map, new zone, zone list, problems, selected zone, editing), the
// status pill at the bottom with the message card over it, and the height
// and XML windows.
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
	// Warnings are what failed to load in the open tiles.
	Warnings []string
	// Project holds the project menu and the recent maps.
	Project ProjectPanel
	// Zone holds the new zone controls, the tool dock and Compile.
	Zone ZonePanel
	// Zones is the zone list.
	Zones ZoneList
	// Problems is the problem list.
	Problems ProblemList
	// Props edits the selected zone's type and parameters.
	Props PropertiesPanel
	// Edit holds undo/redo and the shape/vertex editing controls.
	Edit EditPanel
	// Height raises, lowers and sizes the selected zone from a window
	// floating over the viewport.
	Height HeightPanel
	// Arrow is the viewport's handle that raises and lowers the selected
	// zone.
	Arrow ZArrow
	// Meshes is the command bar's switch that hides the static meshes
	// while On.
	Meshes Switch
	// Ground is the command bar's switch that shows, while On, the
	// terrain grid, the selected zone's footprint on the ground and
	// EdgeLabels.
	Ground Switch
	// EdgeLabels are the lengths of the selected zone's edges, drawn on
	// the viewport at their midpoints.
	EdgeLabels []EdgeLabel
	// Pins are the labels of the selected shape's worst-point pins (its
	// highest and lowest floor); unlike EdgeLabels a click on one is
	// reported by PinClicked.
	Pins     []EdgeLabel
	pinPicks []widget.Clickable
	// XML shows the last compilation, to copy.
	XML XMLWindow
	// WaterMenu is the viewport's context menu over the water.
	WaterMenu WaterMenu
	// Message is the last outcome ("" for none), shown over the status
	// pill with the armed tool's hints (Zone.Info).
	Message string
	// Cursor and Click are the status pill's positions under the cursor
	// and at the last click ("" hides Click); Tiles names the open tiles.
	Cursor, Click, Tiles string

	list                                    widget.List
	brandSink, dockSink, barSink, inspector pointerSink
	statusSink, messageSink                 pointerSink
}

// NewShell returns a shell with single-line fields holding client and
// tile.
func NewShell(th *material.Theme, client, tile string) *Shell {
	s := &Shell{Theme: th}
	s.Client.SingleLine = true
	s.Client.SetText(client)
	s.Tile.SingleLine = true
	s.Tile.Submit = true
	s.Tile.SetText(tile)
	s.Zone.init()
	s.Zones.init()
	s.Props.init()
	s.Edit.init()
	s.Height.init()
	s.list.Axis = layout.Vertical
	return s
}

// OpenRequested reports a click on Open or Enter in the tile field since
// the last call.
func (s *Shell) OpenRequested(gtx layout.Context) bool {
	return requested(gtx, &s.Tile, &s.Open)
}

// Layout lays the window out and returns the viewport rectangle in window
// pixels (origin top-left): all of it, as the cards float over the scene.
// Nothing is painted under the viewport but the cards, so the 3D content
// drawn before Gio's frame shows through and around them.
func (s *Shell) Layout(gtx layout.Context) image.Rectangle {
	gtx.Constraints.Min = gtx.Constraints.Max
	area := gtx.Constraints.Max
	s.Viewport.Layout(gtx)
	// The arrow only paints; every card takes the pointer input over its
	// bounds, so the viewport gets what lands between them.
	s.Arrow.Layout(gtx)
	s.edgeLabels(gtx)
	s.pinLabels(gtx)

	m := gtx.Dp(floatMargin)
	brand := at(gtx, image.Pt(m, m), s.brand)
	dock := at(gtx, image.Pt(m, m+brand.Y+gtx.Dp(12)), s.dock)

	iw := gtx.Dp(InspectorWidth)
	ix := area.X - m - iw
	at(gtx, image.Pt(ix, m), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(image.Pt(iw, max(0, area.Y-2*m)))
		return s.inspectorCard(gtx)
	})

	// The command bar and the status pill centre on the part of the
	// scene the inspector leaves, kept clear of the brand mark and the
	// dock.
	left := m + dock.X + gtx.Dp(16)
	call, bar := measure(gtx, s.commandBar)
	barAt := image.Pt(max(m+brand.X+gtx.Dp(16), (ix-bar.X)/2), m)
	place(gtx, barAt, call)
	s.Project.anchor = barAt.Add(image.Pt(gtx.Dp(6), bar.Y+gtx.Dp(6)))

	call, status := measure(gtx, s.statusPill)
	statusAt := image.Pt(max(left, (ix-status.X)/2), area.Y-m-status.Y)
	place(gtx, statusAt, call)
	if call, msg := measure(gtx, s.messageCard); msg.Y > 0 {
		place(gtx, image.Pt(max(left, (ix-msg.X)/2), statusAt.Y-gtx.Dp(10)-msg.Y), call)
	}

	s.heightWindow(gtx)
	s.xmlWindow(gtx)
	s.projectMenu(gtx)
	s.waterMenu(gtx)
	return image.Rectangle{Max: s.Viewport.Size()}
}

// place adds the recorded call at p.
func place(gtx layout.Context, p image.Point, call op.CallOp) {
	defer op.Offset(p).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

// brand is the logo card: the app icon, its name and what it edits.
func (s *Shell) brand(gtx layout.Context) layout.Dimensions {
	inset := layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(10), Right: unit.Dp(16)}
	return card(gtx, &s.brandSink, inset, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return logo.Layout(gtx, 30, white) }),
			layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(s.text("Zone Builder", titleSize, font.SemiBold, textColor, 1)),
					layout.Rigid(s.text("Editor de zonas · Lineage II", captionSize, font.Normal, dimText, 1)),
				)
			}),
		)
	})
}

// inspectorCard is the right-hand card: every section in one scrolling
// column, filling the height it is given.
func (s *Shell) inspectorCard(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	return card(gtx, &s.inspector, layout.Inset{}, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Exact(size)
		var children []layout.FlexChild
		children = append(children, s.mapSection()...)
		children = append(children, s.zonePanel()...)
		children = append(children, s.zoneList()...)
		children = append(children, s.problemList()...)
		children = append(children, s.selectedZone()...)
		children = append(children, s.editPanel()...)
		// One list item holding the whole column: the card scrolls when
		// the window is too short for it.
		s.scrollList(&s.list).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
			return layout.UniformInset(cardPadding).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
		return layout.Dimensions{Size: size}
	})
}

// mapSection is the inspector's first section: the client folder, the
// tile to open, the recent maps, the go-to field and the load warnings.
func (s *Shell) mapSection() []layout.FlexChild {
	l := &s.Zones
	children := []layout.FlexChild{
		layout.Rigid(s.section(true, icon.Map, "Mapa", s.Tiles)),
		layout.Rigid(s.fieldLabel("Pasta do cliente")),
		layout.Rigid(s.fieldButton(&s.Client, "pasta acima de Maps", icon.Folder, s.button(&s.Browse, secondaryButton, icon.FolderOpen, ""))),
		layout.Rigid(s.fieldLabel("Tile (X_Y ou X_Y_Classic)")),
		layout.Rigid(s.fieldButton(&s.Tile, "22_22", icon.Grid2x2, s.button(&s.Open, primaryButton, nil, "Abrir"))),
		layout.Rigid(s.spaced(s.checkBox(&s.Neighbours, "Abrir com os vizinhos (até 3×3)"))),
		layout.Rigid(s.loading),
	}
	children = append(children, s.recentMaps()...)
	children = append(children,
		layout.Rigid(s.fieldLabel("Ir para x y z (coordenadas do servidor)")),
		layout.Rigid(s.fieldButton(&l.GoTo, "83400 147943 -3400", icon.Crosshair, s.button(&l.Go, secondaryButton, icon.Navigation, ""))),
	)
	for _, w := range s.Warnings {
		children = append(children, layout.Rigid(s.dimLabel(w)))
	}
	return children
}

// loading is the map loading line and its progress bar, while Loading is
// set.
func (s *Shell) loading(gtx layout.Context) layout.Dimensions {
	if s.Loading == "" {
		return layout.Dimensions{}
	}
	return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(s.dimLabel(s.Loading)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return progressBar(gtx, s.Progress) }),
		)
	})
}

// statusPill is the bottom bar: the server position under the cursor, at
// the last click, and the open tiles.
func (s *Shell) statusPill(gtx layout.Context) layout.Dimensions {
	item := func(ic *icon.Icon, txt string) []layout.FlexChild {
		if txt == "" {
			return nil
		}
		return []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ic.Layout(gtx, 14, dimText) }),
			layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
			layout.Rigid(s.text(txt, smallSize, font.Normal, textColor, 1)),
			layout.Rigid(layout.Spacer{Width: unit.Dp(18)}.Layout),
		}
	}
	var children []layout.FlexChild
	children = append(children, item(icon.Crosshair, s.Cursor)...)
	children = append(children, item(icon.MousePointer2, s.Click)...)
	children = append(children, item(icon.Layers, s.Tiles)...)
	if len(children) > 0 {
		children = children[:len(children)-1]
	}
	inset := layout.Inset{Top: unit.Dp(9), Bottom: unit.Dp(9), Left: unit.Dp(16), Right: unit.Dp(16)}
	return card(gtx, &s.statusSink, inset, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

// messageCard is the last outcome and the armed tool's hints, over the
// status pill; nothing when there are neither.
func (s *Shell) messageCard(gtx layout.Context) layout.Dimensions {
	if s.Message == "" && len(s.Zone.Info) == 0 {
		return layout.Dimensions{}
	}
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(560))
	line := func(ic *icon.Icon, ink layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(1), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return ic.Layout(gtx, 14, dimText)
					})
				}),
				layout.Rigid(ink),
			)
		})
	}
	var children []layout.FlexChild
	if s.Message != "" {
		children = append(children, line(icon.Info, s.text(s.Message, smallSize, font.Normal, textColor, 3)))
	}
	for i, h := range s.Zone.Info {
		if i > 0 || s.Message != "" {
			children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout))
		}
		children = append(children, line(icon.MousePointer2, s.text(h, smallSize, font.Normal, accentText, 3)))
	}
	inset := layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(14), Right: unit.Dp(16)}
	return card(gtx, &s.messageSink, inset, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}
