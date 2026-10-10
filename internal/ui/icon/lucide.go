package icon

import (
	"embed"
	"fmt"
)

// lucideFS holds the Lucide icons, kept byte-for-byte as published
// (ISC license, see lucide/LICENSE).
//
//go:embed lucide/*.svg
var lucideFS embed.FS

// lucideStrokeScale thins Lucide's 2-unit strokes for the UI's look.
const lucideStrokeScale = 0.8

// lucide parses an embedded Lucide icon. The files are build-time assets, so a
// failure is a programming error.
func lucide(name string) *Icon {
	data, err := lucideFS.ReadFile("lucide/" + name + ".svg")
	if err != nil {
		panic(fmt.Sprintf("icon: lucide %s: %v", name, err))
	}
	ic, err := Parse(data)
	if err != nil {
		panic(fmt.Sprintf("icon: lucide %s: %v", name, err))
	}
	ic.StrokeScale = lucideStrokeScale
	return ic
}

// Lucide icons (https://lucide.dev), named after their Lucide names.
var (
	ArrowDown         = lucide("arrow-down")
	ArrowUp           = lucide("arrow-up")
	Box               = lucide("box")
	Check             = lucide("check")
	ChevronDown       = lucide("chevron-down")
	ChevronLeft       = lucide("chevron-left")
	ChevronRight      = lucide("chevron-right")
	Circle            = lucide("circle")
	CircleAlert       = lucide("circle-alert")
	CircleCheck       = lucide("circle-check")
	CodeXML           = lucide("code-xml")
	Copy              = lucide("copy")
	Crosshair         = lucide("crosshair")
	Droplets          = lucide("droplets")
	Eye               = lucide("eye")
	EyeOff            = lucide("eye-off")
	FileCode          = lucide("file-code")
	Folder            = lucide("folder")
	FolderOpen        = lucide("folder-open")
	Grid2x2           = lucide("grid-2x2")
	Grid3x3           = lucide("grid-3x3")
	House             = lucide("house")
	Info              = lucide("info")
	LandPlot          = lucide("land-plot")
	Layers            = lucide("layers")
	List              = lucide("list")
	Map               = lucide("map")
	MapPin            = lucide("map-pin")
	Minus             = lucide("minus")
	Mountain          = lucide("mountain")
	MousePointer2     = lucide("mouse-pointer-2")
	Move              = lucide("move")
	Navigation        = lucide("navigation")
	Palette           = lucide("palette")
	Pencil            = lucide("pencil")
	Pentagon          = lucide("pentagon")
	Plus              = lucide("plus")
	Redo2             = lucide("redo-2")
	Ruler             = lucide("ruler")
	Save              = lucide("save")
	SaveAll           = lucide("save-all")
	Search            = lucide("search")
	Settings2         = lucide("settings-2")
	SlidersHorizontal = lucide("sliders-horizontal")
	Square            = lucide("square")
	SquareDashed      = lucide("square-dashed")
	Swords            = lucide("swords")
	Trash2            = lucide("trash-2")
	TriangleAlert     = lucide("triangle-alert")
	Undo2             = lucide("undo-2")
	Users             = lucide("users")
	X                 = lucide("x")
)
