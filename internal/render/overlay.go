package render

import (
	"fmt"
	"slices"
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
	// which closes the polygon when clicked, or the selected one), or -1.
	Marked int
	// Midpoints adds a small handle in the middle of each edge of a
	// closed shape, where a click inserts a vertex.
	Midpoints bool
	// Problem draws the edges in problemColor: the shape's zone has a
	// problem. BadVertices are the vertices with a problem of their own,
	// whose handles are filled with problemColor.
	Problem     bool
	BadVertices []int
}

// problemColor is the edge and handle colour that flags a problem.
var problemColor = [4]float32{1, 0.12, 0.12, 1}

// Overlay alphas and handle sizes in pixels.
const (
	prismAlpha    = 0.22
	handleSize    = 7
	handleOutline = 11
	markedSize    = 11
	markedOutline = 15
	midSize       = 5
	midOutline    = 8
)

// overlayVertex is one overlay vertex: an absolute client-space position,
// a linear RGBA colour, for points the size in pixels and, for lines, the
// line's first end (the same on both vertices), where its dashes start.
type overlayVertex struct {
	Pos    [3]float32
	Color  [4]float32
	Size   float32
	Anchor [3]float32
}

// segment is the line from a to b in colour c.
func segment(a, b [3]float32, c [4]float32) [2]overlayVertex {
	return [2]overlayVertex{{Pos: a, Color: c, Anchor: a}, {Pos: b, Color: c, Anchor: a}}
}

// Overlay draw modes, the uMode uniform: plain colour, buried faces
// (fainter, striped) and buried lines (dashed along the line on screen).
const (
	modeVisible = iota
	modeBuriedFace
	modeBuriedLine
)

const overlayVert = `#version 300 es
layout(location = 0) in vec3 aPos;
layout(location = 1) in vec4 aColor;
layout(location = 2) in float aSize;
layout(location = 3) in vec3 aAnchor;
uniform highp vec3 uOrigin;
uniform highp mat4 uViewProj;
out vec4 vColor;
// vAnchor is the clip position of the line's first end, equal on both
// vertices so it reaches the fragment unchanged (no flat varying: ANGLE's
// D3D11 backend drops lines that use one).
out highp vec4 vAnchor;
void main() {
	vColor = aColor;
	gl_PointSize = aSize;
	gl_Position = uViewProj * vec4(aPos - uOrigin, 1.0);
	vAnchor = uViewProj * vec4(aAnchor - uOrigin, 1.0);
}
`

const overlayFrag = `#version 300 es
precision mediump float;
uniform int uMode;
uniform highp vec2 uViewport;
in vec4 vColor;
in highp vec4 vAnchor;
out vec4 oColor;
void main() {
	vec4 c = vColor;
	if (uMode == 1) {
		// Diagonal stripes, 3 px on, 3 px off.
		if (mod(gl_FragCoord.x + gl_FragCoord.y, 6.0) >= 3.0) discard;
		c.a *= 0.6; // the buried faces keep 60% of prismAlpha in their stripes
	} else if (uMode == 2) {
		// Dashes 8 px on, 6 px off, measured on screen from the line's
		// first end; a first end behind the camera falls back to a fixed
		// diagonal pattern.
		highp float d = gl_FragCoord.x + gl_FragCoord.y;
		if (vAnchor.w > 0.0) {
			highp vec2 a = (vAnchor.xy / vAnchor.w * 0.5 + 0.5) * uViewport;
			d = distance(gl_FragCoord.xy, a);
		}
		if (mod(d, 14.0) >= 8.0) discard;
		c.a *= 0.85;
	}
	oColor = c;
}
`

// zoneOverlay draws zone shapes over the scene. Faces and lines draw in 2
// passes: where they are in front of the scene, plain; where the scene
// hides them, faces faint and striped, lines dashed. Vertex handles go on
// top of everything.
type zoneOverlay struct {
	prog     uint32
	viewProj int32
	origin   int32
	mode     int32
	viewport int32
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
		mode:     gles.GetUniformLocation(p, "uMode"),
		viewport: gles.GetUniformLocation(p, "uViewport"),
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
	gles.EnableVertexAttribArray(3)
	gles.VertexAttribPointer(3, 3, gles.FLOAT, false, stride, unsafe.Offsetof(overlayVertex{}.Anchor))
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
		if s.Problem {
			edge = problemColor
		}
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
				for _, l := range [3][2]overlayVertex{
					segment(at(a, zmin), at(b, zmin), edge),
					segment(at(a, zmax), at(b, zmax), edge),
					segment(at(a, zmin), at(a, zmax), edge),
				} {
					lines = append(lines, l[:]...)
				}
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
				l := segment([3]float32{a.X, a.Y, a.Z}, [3]float32{b.X, b.Y, b.Z}, edge)
				lines = append(lines, l[:]...)
			}
		}
		for i, p := range client {
			pos := [3]float32{p.X, p.Y, p.Z}
			size, outline := float32(handleSize), float32(handleOutline)
			fill := [4]float32{1, 1, 1, 1}
			if slices.Contains(s.BadVertices, i) {
				fill = problemColor
			}
			if i == s.Marked {
				size, outline = markedSize, markedOutline
				fill = [4]float32{1, 0.85, 0.1, 1}
			}
			points = append(points,
				overlayVertex{Pos: pos, Color: [4]float32{0, 0, 0, 1}, Size: outline},
				overlayVertex{Pos: pos, Color: fill, Size: size})
		}
		if s.Closed && s.Midpoints && len(client) >= 2 {
			for i, a := range client {
				b := client[(i+1)%len(client)]
				pos := [3]float32{(a.X + b.X) / 2, (a.Y + b.Y) / 2, (a.Z + b.Z) / 2}
				points = append(points,
					overlayVertex{Pos: pos, Color: [4]float32{0, 0, 0, 1}, Size: midOutline},
					overlayVertex{Pos: pos, Color: edge, Size: midSize})
			}
		}
	}
	o.verts = append(append(append(o.verts[:0], tris...), lines...), points...)
	o.tris, o.lines = len(tris), len(lines)
	o.dirty = true
}

// draw renders the overlay into the bound viewport target of size w×h,
// whose depth buffer holds the scene (reversed Z, so LESS is behind the
// scene). It leaves blending off, depth writes on and the depth test
// GREATER, as the scene pass expects.
func (o *zoneOverlay) draw(viewProj mat4, origin [3]float32, w, h int) {
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
	gles.Uniform2f(o.viewport, float32(w), float32(h))
	gles.Enable(gles.BLEND)
	gles.BlendFunc(gles.SRC_ALPHA, gles.ONE_MINUS_SRC_ALPHA)
	gles.DepthMask(false)

	// Faces: plain in front of the scene, faint stripes behind it.
	o.pass(gles.TRIANGLES, 0, o.tris, gles.GEQUAL, modeVisible)
	o.pass(gles.TRIANGLES, 0, o.tris, gles.LESS, modeBuriedFace)
	// Lines: solid in front of the scene, dashed behind it. Another line
	// run (a ground line per wall, say) draws the same 2 passes.
	o.pass(gles.LINES, o.tris, o.lines, gles.GEQUAL, modeVisible)
	o.pass(gles.LINES, o.tris, o.lines, gles.LESS, modeBuriedLine)

	// Handles: over everything.
	gles.Disable(gles.DEPTH_TEST)
	gles.Uniform1i(o.mode, modeVisible)
	gles.DrawArrays(gles.POINTS, o.tris+o.lines, len(o.verts)-o.tris-o.lines)

	gles.Enable(gles.DEPTH_TEST)
	gles.DepthFunc(gles.GREATER)
	gles.DepthMask(true)
	gles.Disable(gles.BLEND)
	gles.UseProgram(0)
	gles.BindVertexArray(0)
}

// pass draws count vertices from first as prim with the depth test fn and
// the fragment mode.
func (o *zoneOverlay) pass(prim uint32, first, count int, fn uint32, mode int32) {
	if count == 0 {
		return
	}
	gles.DepthFunc(fn)
	gles.Uniform1i(o.mode, mode)
	gles.DrawArrays(prim, first, count)
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
