package render

import (
	"fmt"
	"unsafe"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/render/gles"
	"zonebuilder/internal/scene"
)

// ZoneShape is one zone shape for the overlay, in server coordinates (the
// numbers that go into the XML); the overlay converts them to the client's
// space with scene.FromServer (ADR 0003).
type ZoneShape struct {
	// Points are the vertices in order; each Z is the height it was
	// picked at, where its handle is drawn.
	Points []geom.Vec3
	// ZMin and ZMax bound the prism of a closed shape.
	ZMin, ZMax float32
	// Closed draws the prism and its outline; an open shape (still being
	// drawn) is a polyline through Points.
	Closed bool
	// Color is the shape's linear RGB.
	Color [3]float32
	// Marked is the vertex whose handle stands out (the first vertex,
	// which closes the polygon when clicked), or -1.
	Marked int
}

// Overlay alphas and handle sizes in pixels.
const (
	prismAlpha    = 0.22
	handleSize    = 7
	handleOutline = 11
	markedSize    = 11
	markedOutline = 15
)

// overlayVertex is one overlay vertex: an absolute client-space position,
// a linear RGBA colour and, for points, the size in pixels.
type overlayVertex struct {
	Pos   [3]float32
	Color [4]float32
	Size  float32
}

const overlayVert = `#version 300 es
layout(location = 0) in vec3 aPos;
layout(location = 1) in vec4 aColor;
layout(location = 2) in float aSize;
uniform highp vec3 uOrigin;
uniform highp mat4 uViewProj;
out vec4 vColor;
void main() {
	vColor = aColor;
	gl_PointSize = aSize;
	gl_Position = uViewProj * vec4(aPos - uOrigin, 1.0);
}
`

const overlayFrag = `#version 300 es
precision mediump float;
in vec4 vColor;
out vec4 oColor;
void main() {
	oColor = vColor;
}
`

// zoneOverlay draws zone shapes over the scene: translucent prism faces
// depth-tested against the scene, then edges and vertex handles on top of
// everything.
type zoneOverlay struct {
	prog     uint32
	viewProj int32
	origin   int32
	vao, vbo uint32
	// verts holds the triangle run, then the line run, then the points.
	verts       []overlayVertex
	tris, lines int
	dirty       bool
}

func newZoneOverlay() (*zoneOverlay, error) {
	p, err := newProgram(overlayVert, overlayFrag)
	if err != nil {
		return nil, fmt.Errorf("render: overlay program: %w", err)
	}
	o := &zoneOverlay{
		prog:     p,
		viewProj: gles.GetUniformLocation(p, "uViewProj"),
		origin:   gles.GetUniformLocation(p, "uOrigin"),
		vao:      gles.GenVertexArray(),
		vbo:      gles.GenBuffer(),
	}
	gles.BindVertexArray(o.vao)
	gles.BindBuffer(gles.ARRAY_BUFFER, o.vbo)
	stride := int(unsafe.Sizeof(overlayVertex{}))
	gles.EnableVertexAttribArray(0)
	gles.VertexAttribPointer(0, 3, gles.FLOAT, false, stride, unsafe.Offsetof(overlayVertex{}.Pos))
	gles.EnableVertexAttribArray(1)
	gles.VertexAttribPointer(1, 4, gles.FLOAT, false, stride, unsafe.Offsetof(overlayVertex{}.Color))
	gles.EnableVertexAttribArray(2)
	gles.VertexAttribPointer(2, 1, gles.FLOAT, false, stride, unsafe.Offsetof(overlayVertex{}.Size))
	gles.BindVertexArray(0)
	gles.BindBuffer(gles.ARRAY_BUFFER, 0)
	return o, nil
}

// set rebuilds the vertex runs for shapes: triangles, then lines, then
// points, so each draws with one call.
func (o *zoneOverlay) set(shapes []ZoneShape) {
	var tris, lines, points []overlayVertex
	for _, s := range shapes {
		if len(s.Points) == 0 {
			continue
		}
		c := s.Color
		face := [4]float32{c[0], c[1], c[2], prismAlpha}
		edge := [4]float32{c[0], c[1], c[2], 1}
		client := make([]geom.Vec3, len(s.Points))
		for i, p := range s.Points {
			client[i] = scene.FromServer(p)
		}
		if s.Closed && len(client) >= 2 {
			zmin := scene.FromServer(geom.Vec3{Z: s.ZMin}).Z
			zmax := scene.FromServer(geom.Vec3{Z: s.ZMax}).Z
			at := func(p geom.Vec3, z float32) [3]float32 { return [3]float32{p.X, p.Y, z} }
			n := len(client)
			for i := range n {
				a, b := client[i], client[(i+1)%n]
				for _, v := range [6][3]float32{at(a, zmin), at(b, zmin), at(b, zmax), at(a, zmin), at(b, zmax), at(a, zmax)} {
					tris = append(tris, overlayVertex{Pos: v, Color: face})
				}
				lines = append(lines,
					overlayVertex{Pos: at(a, zmin), Color: edge}, overlayVertex{Pos: at(b, zmin), Color: edge},
					overlayVertex{Pos: at(a, zmax), Color: edge}, overlayVertex{Pos: at(b, zmax), Color: edge},
					overlayVertex{Pos: at(a, zmin), Color: edge}, overlayVertex{Pos: at(a, zmax), Color: edge},
				)
			}
			caps := triangulate(client)
			for _, z := range [2]float32{zmin, zmax} {
				for _, k := range caps {
					tris = append(tris, overlayVertex{Pos: at(client[k], z), Color: face})
				}
			}
		} else {
			for i := 1; i < len(client); i++ {
				a, b := client[i-1], client[i]
				lines = append(lines,
					overlayVertex{Pos: [3]float32{a.X, a.Y, a.Z}, Color: edge},
					overlayVertex{Pos: [3]float32{b.X, b.Y, b.Z}, Color: edge})
			}
		}
		for i, p := range client {
			pos := [3]float32{p.X, p.Y, p.Z}
			size, outline := float32(handleSize), float32(handleOutline)
			fill := [4]float32{1, 1, 1, 1}
			if i == s.Marked {
				size, outline = markedSize, markedOutline
				fill = [4]float32{1, 0.85, 0.1, 1}
			}
			points = append(points,
				overlayVertex{Pos: pos, Color: [4]float32{0, 0, 0, 1}, Size: outline},
				overlayVertex{Pos: pos, Color: fill, Size: size})
		}
	}
	o.verts = append(append(append(o.verts[:0], tris...), lines...), points...)
	o.tris, o.lines = len(tris), len(lines)
	o.dirty = true
}

// draw renders the overlay into the bound viewport target, whose depth
// buffer holds the scene (reversed Z). It leaves blending off, depth
// writes on and the depth test GREATER, as the scene pass expects.
func (o *zoneOverlay) draw(viewProj mat4, origin [3]float32) {
	if len(o.verts) == 0 {
		return
	}
	gles.BindVertexArray(o.vao)
	if o.dirty {
		gles.BindBuffer(gles.ARRAY_BUFFER, o.vbo)
		gles.BufferData(gles.ARRAY_BUFFER, o.verts, gles.DYNAMIC_DRAW)
		gles.BindBuffer(gles.ARRAY_BUFFER, 0)
		o.dirty = false
	}
	gles.UseProgram(o.prog)
	gles.UniformMatrix4fv(o.viewProj, (*[16]float32)(&viewProj))
	gles.Uniform3f(o.origin, origin[0], origin[1], origin[2])
	gles.Enable(gles.BLEND)
	gles.BlendFunc(gles.SRC_ALPHA, gles.ONE_MINUS_SRC_ALPHA)

	// Faces: hidden by the scene in front of them, never hiding it.
	gles.DepthMask(false)
	gles.DepthFunc(gles.GEQUAL)
	gles.DrawArrays(gles.TRIANGLES, 0, o.tris)

	// Edges and handles: over everything.
	gles.Disable(gles.DEPTH_TEST)
	gles.DrawArrays(gles.LINES, o.tris, o.lines)
	gles.DrawArrays(gles.POINTS, o.tris+o.lines, len(o.verts)-o.tris-o.lines)

	gles.Enable(gles.DEPTH_TEST)
	gles.DepthFunc(gles.GREATER)
	gles.DepthMask(true)
	gles.Disable(gles.BLEND)
	gles.UseProgram(0)
	gles.BindVertexArray(0)
}

func (o *zoneOverlay) release() {
	gles.DeleteProgram(o.prog)
	gles.DeleteVertexArray(o.vao)
	gles.DeleteBuffer(o.vbo)
}

// triangulate ear-clips the XY outline of pts, a simple polygon of either
// winding, into triangles (indices into pts). A self-intersecting outline
// may have no ear left; what was clipped so far is returned.
func triangulate(pts []geom.Vec3) []int {
	n := len(pts)
	if n < 3 {
		return nil
	}
	var area float32
	for i := range n {
		a, b := pts[i], pts[(i+1)%n]
		area += a.X*b.Y - b.X*a.Y
	}
	ccw := area > 0
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	cross := func(a, b, c geom.Vec3) float32 {
		return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
	}
	convex := func(a, b, c geom.Vec3) bool {
		if ccw {
			return cross(a, b, c) > 0
		}
		return cross(a, b, c) < 0
	}
	inside := func(p, a, b, c geom.Vec3) bool {
		d1, d2, d3 := cross(a, b, p), cross(b, c, p), cross(c, a, p)
		neg := d1 < 0 || d2 < 0 || d3 < 0
		pos := d1 > 0 || d2 > 0 || d3 > 0
		return !(neg && pos)
	}
	var out []int
	for len(idx) > 3 {
		clipped := false
		for i := range idx {
			ia, ib, ic := idx[(i+len(idx)-1)%len(idx)], idx[i], idx[(i+1)%len(idx)]
			a, b, c := pts[ia], pts[ib], pts[ic]
			if !convex(a, b, c) {
				continue
			}
			ear := true
			for _, k := range idx {
				if k != ia && k != ib && k != ic && inside(pts[k], a, b, c) {
					ear = false
					break
				}
			}
			if !ear {
				continue
			}
			out = append(out, ia, ib, ic)
			idx = append(idx[:i], idx[i+1:]...)
			clipped = true
			break
		}
		if !clipped {
			return out
		}
	}
	return append(out, idx[0], idx[1], idx[2])
}
