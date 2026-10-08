// Package unreal decodes the Unreal objects a map is built from (TerrainInfo
// so far) out of l2pkg packages. Port of UE2-Studio's src/unreal/objects.rs.
package unreal

import (
	"fmt"
	"math"

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
	// Layers are the configured slots of the Layers array, in order, with
	// the slots that have no Texture dropped.
	Layers []TerrainLayer
}

// TerrainLayer is one entry of TerrainInfo.Layers. Port of UE2-Studio
// objects.rs TerrainLayer, defaults included.
type TerrainLayer struct {
	// Texture is the layer's material, AlphaMap the Texture whose R channel
	// is its coverage (0: none, the layer covers everything). Both are
	// object references in the map package's index space.
	Texture, AlphaMap int32
	// UScale and VScale divide the UV (a zero in the file reads as 1);
	// UPan and VPan offset it before the division; TextureRotation turns
	// it, read as degrees like UE2-Studio does.
	UScale, VScale, UPan, VPan, TextureRotation float32
}

// UVMapping is the layer's material texture coordinate as a function of
// heightmap sample (x, y). Port of UE2-Studio's terrain_layer_uv: the
// sample's grid position rotated, panned, then divided by the scale.
func (l *TerrainLayer) UVMapping() func(x, y int) [2]float32 {
	// The angle is rounded to float32 first, as UE2-Studio's f32
	// to_radians does: at the ~65000 degrees some maps store, that
	// rounding moves the UVs visibly.
	angle := l.TextureRotation * float32(math.Pi/180)
	sin, cos := math.Sincos(float64(angle))
	s, c := float32(sin), float32(cos)
	return func(x, y int) [2]float32 {
		fx, fy := float32(x), float32(y)
		return [2]float32{
			(fx*c - fy*s + l.UPan) / l.UScale,
			(fx*s + fy*c + l.VPan) / l.VScale,
		}
	}
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
	for _, slot := range props.StructSlots("Layers") {
		tex, _ := slot.Index("Texture")
		if tex == 0 {
			continue
		}
		l := TerrainLayer{Texture: tex, UScale: 1, VScale: 1}
		l.AlphaMap, _ = slot.Index("AlphaMap")
		if v, ok := slot.Float("UScale"); ok && v != 0 {
			l.UScale = v
		}
		if v, ok := slot.Float("VScale"); ok && v != 0 {
			l.VScale = v
		}
		l.UPan, _ = slot.Float("UPan")
		l.VPan, _ = slot.Float("VPan")
		l.TextureRotation, _ = slot.Float("TextureRotation")
		t.Layers = append(t.Layers, l)
	}
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
