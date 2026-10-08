// Package unreal decodes the Unreal objects a map is built from (TerrainInfo
// so far) out of l2pkg packages. Port of UE2-Studio's src/unreal/objects.rs.
package unreal

import (
	"fmt"

	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/texture"
)

// TerrainInfo is a map's terrain actor.
type TerrainInfo struct {
	// TerrainMap is the object reference of the G16 heightmap Texture, in
	// the map package's index space (usually an import from T_<tile>).
	TerrainMap int32
	// TerrainScale is the raw property: X/Y are world units per sample and
	// Z is world units per 256 height steps. A zero component is broken.
	TerrainScale [3]float32
	// QuadVisibilityBitmap and EdgeTurnBitmap hold one bit per quad,
	// indexed x + y*width with the heightmap's width as stride.
	QuadVisibilityBitmap []byte
	EdgeTurnBitmap       []byte
	MapX, MapY           int32
	Location             [3]float32
}

// FindTerrainInfo is the 0-based index of the first TerrainInfo export of
// p, or -1 when the map has none (an interior).
func FindTerrainInfo(p *l2pkg.Package) int {
	for i := range p.Exports {
		if p.Exports[i].ClassName == "TerrainInfo" {
			return i
		}
	}
	return -1
}

// ReadTerrainInfo decodes export i (0-based) of p as a TerrainInfo.
func ReadTerrainInfo(p *l2pkg.Package, i int) (*TerrainInfo, error) {
	r, err := p.ExportReader(i)
	if err != nil {
		return nil, err
	}
	props, err := l2pkg.ReadProperties(p, r, p.Exports[i].Flags)
	if err != nil {
		return nil, fmt.Errorf("TerrainInfo %s: %w", p.Exports[i].ObjectName, err)
	}
	t := &TerrainInfo{}
	t.TerrainMap, _ = props.Index("TerrainMap")
	t.TerrainScale, _ = props.Vector("TerrainScale")
	t.QuadVisibilityBitmap, _ = props.Bytes("QuadVisibilityBitmap")
	t.EdgeTurnBitmap, _ = props.Bytes("EdgeTurnBitmap")
	t.MapX, _ = props.Int("MapX")
	t.MapY, _ = props.Int("MapY")
	t.Location, _ = props.Vector("Location")
	return t, nil
}

// BrokenScale reports a TerrainScale with a zero component. Such a terrain
// is placed by MapX/MapY instead (see Scale and Position).
func (t *TerrainInfo) BrokenScale() bool {
	s := t.TerrainScale
	return s[0] == 0 || s[1] == 0 || s[2] == 0
}

// Scale is the world size of one heightmap step on each axis: a sample
// apart on X/Y, one height unit on Z. A broken TerrainScale falls back to
// the stock (128, 128, 76/256).
func (t *TerrainInfo) Scale() [3]float32 {
	if t.BrokenScale() {
		return [3]float32{128, 128, 76.0 / 256}
	}
	s := t.TerrainScale
	return [3]float32{s[0], s[1], s[2] / 256}
}

// Position is the world position of heightmap sample (0, 0) at height 0,
// for a heightmap of usize×vsize samples. A broken TerrainScale places the
// terrain at its tile's corner from MapX/MapY, at height 0.
func (t *TerrainInfo) Position(usize, vsize int) [3]float32 {
	s := t.Scale()
	if t.BrokenScale() {
		return [3]float32{
			float32(t.MapX-20) * float32(usize) * 128,
			float32(t.MapY-18) * float32(vsize) * 128,
			0,
		}
	}
	l := t.Location
	return [3]float32{
		l[0] - float32(usize)/2*s[0],
		l[1] - float32(vsize)/2*s[1],
		l[2] - 32768*s[2],
	}
}

// Heightmap resolves and decodes the TerrainMap of t, a TerrainInfo of the
// map package p. A heightmap package the client lacks is an
// l2pkg.MissingError.
func (t *TerrainInfo) Heightmap(c *l2pkg.Client, p *l2pkg.Package) (*texture.Texture, error) {
	if t.TerrainMap == 0 {
		return nil, fmt.Errorf("TerrainInfo sem TerrainMap")
	}
	owner, i, err := c.Resolve(p, t.TerrainMap)
	if err != nil {
		return nil, fmt.Errorf("TerrainMap: %w", err)
	}
	if cls := owner.Exports[i].ClassName; cls != "Texture" {
		return nil, fmt.Errorf("TerrainMap aponta para %s, não Texture", cls)
	}
	tex, err := texture.Read(owner, i)
	if err != nil {
		return nil, fmt.Errorf("TerrainMap %s.%s: %w", owner.Name, owner.Exports[i].ObjectName, err)
	}
	return tex, nil
}
