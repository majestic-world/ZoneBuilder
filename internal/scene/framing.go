package scene

import "zonebuilder/internal/geom"

// Framing histogram parameters, UE2-Studio scene.rs framing_bounds: keep
// 99.9% of the vertices on each axis, trimming at least 32 off each end, and
// leave scenes under 1024 vertices untrimmed. A stray quad parked far from
// the map then no longer pulls the opening camera away from it.
const (
	framingBins           = 2048
	framingVertexFraction = 0.999
	framingMinVertices    = 1024
	framingMinTrim        = 32
)

// framingBounds is full with vertex outliers cut away by a per-axis
// histogram.
func framingBounds(batches []Batch, full geom.Box) geom.Box {
	if full.Empty() {
		return full
	}
	size := full.Size()
	lo := [3]float32{full.Min.X, full.Min.Y, full.Min.Z}
	width := [3]float32{max(size.X, 1), max(size.Y, 1), max(size.Z, 1)}
	var bins [3][framingBins]uint32
	const scale = framingBins - 1
	var total uint64
	for i := range batches {
		for _, v := range batches[i].Vertices {
			for axis := range 3 {
				b := int((v.Pos.Axis(axis) - lo[axis]) / width[axis] * scale)
				bins[axis][min(b, framingBins-1)]++
			}
			total++
		}
	}
	if total < framingMinVertices {
		return full
	}
	spare := max(uint64(float64(total)*(1-framingVertexFraction)/2), framingMinTrim)
	var mn, mx [3]float32
	for axis := range 3 {
		first, last := trimmedSpan(bins[axis][:], spare)
		mn[axis] = lo[axis] + width[axis]*(float32(first)/scale)
		mx[axis] = lo[axis] + width[axis]*(float32(last)/scale)
	}
	return geom.Box{Min: vec(mn), Max: vec(mx)}
}

// trimmedSpan is the first and last bin left after spare vertices are
// dropped off each end.
func trimmedSpan(bins []uint32, spare uint64) (int, int) {
	first, dropped := 0, uint64(0)
	for first+1 < len(bins) && dropped+uint64(bins[first]) <= spare {
		dropped += uint64(bins[first])
		first++
	}
	last := len(bins) - 1
	dropped = 0
	for last > first && dropped+uint64(bins[last]) <= spare {
		dropped += uint64(bins[last])
		last--
	}
	return first, last
}
