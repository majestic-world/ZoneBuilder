package l2pkg_test

import (
	"path/filepath"
	"testing"

	"zonebuilder/internal/l2pkg"
)

// TestWaterVolumeScaleDecodesAsTaggedStruct reads the MainScale and
// PostScale of Giran's WaterVolume0, which the client stores as tagged
// Scale structs holding the unit scale and no shear (measured with the
// probe of the water-zone spec, 2026-10-09). Catches the Scale struct
// skipped (no fields at all), read as 17 raw bytes, or decoded off by a
// field.
func TestWaterVolumeScaleDecodesAsTaggedStruct(t *testing.T) {
	p, err := l2pkg.Open(filepath.Join(clientRoot(t), "Maps", "22_22.unr"))
	if err != nil {
		t.Fatal(err)
	}
	i := -1
	for k, e := range p.Exports {
		if e.ClassName == "WaterVolume" && e.ObjectName == "WaterVolume0" {
			i = k
		}
	}
	if i < 0 {
		t.Fatal("22_22 has no WaterVolume0")
	}
	r, err := p.ExportReader(i)
	if err != nil {
		t.Fatal(err)
	}
	props, err := l2pkg.ReadProperties(p, r, p.Exports[i].Flags)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"MainScale", "PostScale"} {
		m := props.Maps(name)
		if len(m) != 1 {
			t.Errorf("%s: %d property lists, want 1", name, len(m))
			continue
		}
		scale, ok := m[0].Vector("Scale")
		if !ok || scale != [3]float32{1, 1, 1} {
			t.Errorf("%s.Scale = %v (present %v), want (1, 1, 1)", name, scale, ok)
		}
		if rate, ok := m[0].Float("SheerRate"); !ok || rate != 0 {
			t.Errorf("%s.SheerRate = %v (present %v), want 0", name, rate, ok)
		}
	}
}
