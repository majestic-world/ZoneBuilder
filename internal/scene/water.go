package scene

import (
	"fmt"
	"math"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/unreal"
)

// WaterVolume is one live WaterVolume actor of a map: a convex brush whose
// shape is every BSP node polygon of the Model its Brush names (spec D1),
// placed by the ABrush transform. Positions are client world coordinates,
// Unreal basis.
type WaterVolume struct {
	Tile Tile
	// Export is the actor's 0-based export index in the map package; with
	// Tile it identifies the volume.
	Export int
	Name   string
	// Faces are the brush's convex polygons, in node order.
	Faces [][]geom.Vec3
	// Planes[k] is the plane of Faces[k], its normal pointing out of the
	// volume.
	Planes []Plane
	// Bounds is the AABB of the faces' vertices.
	Bounds geom.Box
	// Exact is set when every wall is vertical and every other face is
	// horizontal: the server's prism (footprint, bottom, top) is then the
	// volume itself, else it covers the volume with room to spare.
	Exact bool
	// Unsupported is why the volume cannot be selected or compiled, "" when
	// it can.
	Unsupported string
}

// Top is the volume's highest Z, the water surface.
func (v *WaterVolume) Top() float32 { return v.Bounds.Max.Z }

// Bottom is the volume's lowest Z.
func (v *WaterVolume) Bottom() float32 { return v.Bounds.Min.Z }

// Plane is the plane of points p with Normal·p = D; Normal is unit length.
// A point is on the inner side when Normal·p <= D.
type Plane struct {
	Normal geom.Vec3
	D      float32
}

// minSolidFaces is the fewest faces that close a convex solid.
const minSolidFaces = 4

// axisEpsilon is how far a unit normal's component may be from 0 for the
// normal to still count as horizontal (wall) or vertical (cap).
const axisEpsilon = 1e-3

// NewWaterVolume builds the water volume of brush b, export export (name)
// of tile t's map, from m, the Model b.Brush names. A shear in either of
// b's scales, which BrushTransform does not apply, or a Model with too few
// faces to close a solid leaves the volume Unsupported, with the reason.
func NewWaterVolume(t Tile, export int, name string, b *unreal.Brush, m *unreal.Model) (WaterVolume, error) {
	v := WaterVolume{Tile: t, Export: export, Name: name, Bounds: geom.EmptyBox()}
	tr := b.Transform()
	err := m.Polygons(func(p unreal.Polygon) {
		face := make([]geom.Vec3, len(p.Points))
		for k, pt := range p.Points {
			face[k] = tr.Point(vec(pt))
			v.Bounds.Include(face[k])
		}
		v.Faces = append(v.Faces, face)
	})
	if err != nil {
		return WaterVolume{}, err
	}
	// The mean of every face vertex is strictly inside a convex solid, which
	// the AABB centre is not: on a wedge it lies on the slanted face, where
	// float noise would decide the plane's side.
	centre := vertexMean(v.Faces)
	v.Exact = true
	for _, f := range v.Faces {
		pl := facePlane(f)
		if float64(pl.Normal.X)*centre[0]+float64(pl.Normal.Y)*centre[1]+float64(pl.Normal.Z)*centre[2] > float64(pl.D) {
			pl = Plane{Normal: pl.Normal.Scale(-1), D: -pl.D}
		}
		v.Planes = append(v.Planes, pl)
		n := pl.Normal
		wall := abs32(n.Z) < axisEpsilon
		flatCap := abs32(n.X) < axisEpsilon && abs32(n.Y) < axisEpsilon
		if !wall && !flatCap {
			v.Exact = false
		}
	}
	switch {
	case b.MainScale.SheerRate != 0:
		v.Unsupported = fmt.Sprintf("o MainScale do brush tem cisalhamento (SheerRate %g), que o app não aplica", b.MainScale.SheerRate)
	case b.PostScale.SheerRate != 0:
		v.Unsupported = fmt.Sprintf("o PostScale do brush tem cisalhamento (SheerRate %g), que o app não aplica", b.PostScale.SheerRate)
	case len(v.Faces) < minSolidFaces:
		v.Unsupported = fmt.Sprintf("o brush tem %s, poucas para fechar um sólido", inflect.Count(len(v.Faces), "face", "faces"))
	}
	if len(v.Faces) < minSolidFaces {
		v.Exact = false
	}
	return v, nil
}

// vertexMean is the mean of the vertices of faces, in float64 to keep
// precision at world coordinates.
func vertexMean(faces [][]geom.Vec3) [3]float64 {
	var c [3]float64
	n := 0
	for _, f := range faces {
		for _, p := range f {
			c[0] += float64(p.X)
			c[1] += float64(p.Y)
			c[2] += float64(p.Z)
			n++
		}
	}
	if n > 0 {
		for k := range c {
			c[k] /= float64(n)
		}
	}
	return c
}

// facePlane is the plane of convex polygon f by Newell's method, about the
// polygon's centroid to keep float precision at world coordinates. The
// normal follows f's winding.
func facePlane(f []geom.Vec3) Plane {
	var c [3]float64
	for _, p := range f {
		c[0] += float64(p.X)
		c[1] += float64(p.Y)
		c[2] += float64(p.Z)
	}
	for k := range c {
		c[k] /= float64(len(f))
	}
	var n [3]float64
	for i, p := range f {
		q := f[(i+1)%len(f)]
		px, py, pz := float64(p.X)-c[0], float64(p.Y)-c[1], float64(p.Z)-c[2]
		qx, qy, qz := float64(q.X)-c[0], float64(q.Y)-c[1], float64(q.Z)-c[2]
		n[0] += (py - qy) * (pz + qz)
		n[1] += (pz - qz) * (px + qx)
		n[2] += (px - qx) * (py + qy)
	}
	l := math.Sqrt(n[0]*n[0] + n[1]*n[1] + n[2]*n[2])
	if l == 0 {
		return Plane{}
	}
	for k := range n {
		n[k] /= l
	}
	return Plane{
		Normal: geom.Vec3{X: float32(n[0]), Y: float32(n[1]), Z: float32(n[2])},
		D:      float32(n[0]*c[0] + n[1]*c[1] + n[2]*c[2]),
	}
}

func abs32(x float32) float32 { return float32(math.Abs(float64(x))) }

// addWaterVolumes adds the live WaterVolumes of map m (tile t): the
// WaterVolume exports the Level places and that are not bDeleteMe. bHidden
// does not count: a volume is invisible in game by nature.
func (s *Scene) addWaterVolumes(m *l2pkg.Package, t Tile) error {
	li := unreal.FindLevel(m)
	if li < 0 {
		return nil
	}
	level, err := unreal.ReadLevel(m, li)
	if err != nil {
		return err
	}
	placed := make(map[int]bool, len(level.Actors))
	for _, ref := range level.Actors {
		if ref > 0 {
			placed[int(ref-1)] = true
		}
	}
	for i := range m.Exports {
		if !placed[i] || m.Exports[i].ClassName != "WaterVolume" {
			continue
		}
		name := m.Exports[i].ObjectName
		b, err := unreal.ReadBrush(m, i)
		if err != nil {
			return err
		}
		if b.DeleteMe {
			continue
		}
		if b.Brush <= 0 || int(b.Brush) > len(m.Exports) {
			s.WaterVolumes = append(s.WaterVolumes, WaterVolume{
				Tile: t, Export: i, Name: name, Bounds: geom.EmptyBox(),
				Unsupported: fmt.Sprintf("o Brush %d não é um export do mapa", b.Brush),
			})
			continue
		}
		model, err := unreal.ReadModel(m, int(b.Brush)-1)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		v, err := NewWaterVolume(t, i, name, b, model)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		s.WaterVolumes = append(s.WaterVolumes, v)
	}
	return nil
}
