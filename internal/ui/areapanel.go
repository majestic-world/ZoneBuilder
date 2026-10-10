package ui

import (
	"gioui.org/layout"

	"zonebuilder/internal/locale"
	"zonebuilder/internal/ui/icon"
)

// areaPanel is the population mode's inspector panel, under the map
// section: the project's spawn areas.
func (s *Shell) areaPanel() []layout.FlexChild {
	return []layout.FlexChild{
		layout.Rigid(s.section(false, icon.Users, locale.Text(s.Language, "spawn.areas.title"), "")),
		layout.Rigid(s.dimLabel(locale.Text(s.Language, "spawn.areas.empty"))),
	}
}
