// Package appicon embeds the app icon's SVG source, which the window also
// draws as its brand mark.
package appicon

import _ "embed"

// SVG is zonebuilder.svg.
//
//go:embed zonebuilder.svg
var SVG []byte
