// Package fonts holds the typefaces the window draws with: Inter (SIL Open
// Font License, LICENSE-Inter.txt) for the interface, in the weights the
// theme uses, and Go Mono, from Gio, for code.
package fonts

import (
	_ "embed"
	"fmt"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
)

// Interface is the interface typeface's name; Mono is the code typeface's.
const (
	Interface = "Inter"
	Mono      = "Go Mono"
)

var (
	//go:embed Inter-Regular.ttf
	interRegular []byte
	//go:embed Inter-Medium.ttf
	interMedium []byte
	//go:embed Inter-SemiBold.ttf
	interSemiBold []byte
)

// Collection is Inter in its regular, medium and semibold weights, then
// the Go fonts, which bring Go Mono and fill in the glyphs Inter lacks.
func Collection() []font.FontFace {
	faces := make([]font.FontFace, 0, 3+len(gofont.Collection()))
	for _, f := range []struct {
		data   []byte
		weight font.Weight
	}{{interRegular, font.Normal}, {interMedium, font.Medium}, {interSemiBold, font.SemiBold}} {
		face, err := opentype.Parse(f.data)
		if err != nil {
			// The files are embedded: a parse failure is a broken build.
			panic(fmt.Errorf("fonts: Inter: %w", err))
		}
		faces = append(faces, font.FontFace{Font: font.Font{Typeface: Interface, Weight: f.weight}, Face: face})
	}
	return append(faces, gofont.Collection()...)
}
