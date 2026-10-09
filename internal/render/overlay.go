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
	// Ground is the floor line along the prism's walls, as segments (pairs
	// of points, server coordinates) drawn in the edge colour.
	Ground []geom.Vec3
}

// groundLift raises the ground line along the walls this many units over
// the floor it follows, so its visible pass wins the depth test against
// the very triangles it lies on instead of flickering to dashed.
const groundLift = 2

// problemColor is the edge and handle colour that flags a problem.
var problemColor = [4]float32{1, 0.12, 0.12, 1}

// Overlay alphas, wall ring spacing and handle sizes in pixels. The rings
// start every ringStep units over the shape's floor; the step doubles until
// they lie at least ringGap pixels apart on screen.
const (
	wallAlpha     = 0.55
	ringStep      = 64
	ringGap       = 6
	handleSize    = 7
	handleOutline = 11
	markedSize    = 11
	markedOutline = 15
	midSize       = 5
	midOutline    = 8
)

// overlayVertex is one overlay vertex: an absolute client-space position,
// a linear RGBA colour, for points the size in pixels, for lines the
// line's first end (the same on both vertices), where its dashes start,
// and for walls the shape's floor Z (client space), where its rings start.
type overlayVertex struct {
	Pos    [3]float32
	Color  [4]float32
	Size   float32
	Anchor [3]float32
	Base   float32
}

// segment is the line from a to b in colour c.
func segment(a, b [3]float32, c [4]float32) [2]overlayVertex {
	return [2]overlayVertex{{Pos: a, Color: c, Anchor: a}, {Pos: b, Color: c, Anchor: a}}
}

// Overlay draw modes, the uMode uniform: plain colour, wall rings in front
// of the scene and behind it (fainter), and buried lines (dashed along the
// line on screen).
const (
	modeVisible = iota
	modeWall
	modeBuriedWall
	modeBuriedLine
)

const overlayVert = `#version 300 es
layout(location = 0) in vec3 aPos;
layout(location = 1) in vec4 aColor;
layout(location = 2) in float aSize;
layout(location = 3) in vec3 aAnchor;
layout(location = 4) in float aBase;
uniform highp vec3 uOrigin;
uniform highp mat4 uViewProj;
out vec4 vColor;
// vAnchor is the clip position of the line's first end, equal on both
// vertices so it reaches the fragment unchanged (no flat varying: ANGLE's
// D3D11 backend drops lines that use one).
out highp vec4 vAnchor;
// vRise is the height over the shape's floor, where the wall rings count
// from; client and server Z differ by a constant, so the rings fall on the
// same server heights.
out highp float vRise;
void main() {
	vColor = aColor;
	gl_PointSize = aSize;
	gl_Position = uViewProj * vec4(aPos - uOrigin, 1.0);
	vAnchor = uViewProj * vec4(aAnchor - uOrigin, 1.0);
	vRise = aPos.z - aBase;
}
`

var overlayFrag = `#version 300 es
precision mediump float;
uniform int uMode;
uniform highp vec2 uViewport;
in vec4 vColor;
in highp vec4 vAnchor;
in highp float vRise;
out vec4 oColor;

// ring is how much of a ring of the rings g (in steps, fw its fwidth) the
// fragment covers: a 1 px line, antialiased.
float ring(highp float g, highp float fw) {
	return 1.0 - min(abs(fract(g - 0.5) - 0.5) / fw, 1.0);
}

// rings is the coverage of the wall rings. The step is the base step
// doubled until the rings lie ringGap px apart; the finer level fades out
// as its rings close from 2 ringGap to ringGap px, so the step never jumps.
float rings() {
	highp float fz = max(fwidth(vRise), 1e-4); // units per pixel
	highp float lod = max(log2(2.0 * ` + itoa(ringGap) + `.0 * fz / ` + itoa(ringStep) + `.0), 0.0);
	highp float k = floor(lod);
	highp float s = ` + itoa(ringStep) + `.0 * exp2(k);
	float fine = ring(vRise / s, fz / s) * (1.0 - (lod - k));
	float coarse = ring(vRise / (2.0 * s), fz / (2.0 * s));
	return max(fine, coarse);
}

void main() {
	vec4 c = vColor;
	if (uMode == ` + itoa(modeWall) + ` || uMode == ` + itoa(modeBuriedWall) + `) {
		float r = rings();
		if (r <= 0.0) discard;
		c.a *= r;
		if (uMode == ` + itoa(modeBuriedWall) + `) {
			c.a *= 0.35; // the rings behind the scene keep 35% of wallAlpha
		}
	} else if (uMode == ` + itoa(modeBuriedLine) + `) {
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

// zoneOverlay draws zone shapes over the scene as hollow prisms: rings
// along the walls, outlines and vertical edges, no fill. Walls and lines
// draw in 2 passes: where they are in front of the scene, plain; where the
// scene hides them, rings faint and lines dashed. Vertex handles go on top
// of everything.
type zoneOverlay struct {
	prog     uint32
	viewProj int32
	origin   int32
	mode     int32
	viewport int32
	vao, vbo uint32
	// verts holds the wall triangle run, then the line run, then the
	// ground line run, then the points.
	verts               []overlayVertex
	tris, lines, ground int
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
	gles.EnableVertexAttribArray(4)
	gles.VertexAttribPointer(4, 1, gles.FLOAT, false, stride, unsafe.Offsetof(overlayVertex{}.Base))
	gles.BindVertexArray(0)
	gles.BindBuffer(gles.ARRAY_BUFFER, 0)
	return o, nil
}

// set rebuilds the vertex runs for shapes: wall triangles, then lines, then
// points, so each draws with one call.
func (o *zoneOverlay) set(shapes []ZoneShape) {
	var tris, lines, ground, points []overlayVertex
	for _, s := range shapes {
		if len(s.Points) == 0 {
			continue
		}
		c := s.Color
		wall := [4]float32{c[0], c[1], c[2], wallAlpha}
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
					tris = append(tris, overlayVertex{Pos: v, Color: wall, Base: zmin})
				}
				for _, l := range [3][2]overlayVertex{
					segment(at(a, zmin), at(b, zmin), edge),
					segment(at(a, zmax), at(b, zmax), edge),
					segment(at(a, zmin), at(a, zmax), edge),
				} {
					lines = append(lines, l[:]...)
				}
			}
			for k := 0; k+1 < len(s.Ground); k += 2 {
				a, b := scene.FromServer(s.Ground[k]), scene.FromServer(s.Ground[k+1])
				l := segment(at(a, a.Z+groundLift), at(b, b.Z+groundLift), edge)
				ground = append(ground, l[:]...)
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
	o.verts = append(append(append(append(o.verts[:0], tris...), lines...), ground...), points...)
	o.tris, o.lines, o.ground = len(tris), len(lines), len(ground)
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

	// Walls: rings in front of the scene, faint rings behind it.
	o.pass(gles.TRIANGLES, 0, o.tris, gles.GEQUAL, modeWall)
	o.pass(gles.TRIANGLES, 0, o.tris, gles.LESS, modeBuriedWall)
	// Lines: solid in front of the scene, dashed behind it; the ground
	// line along the walls the same way.
	o.pass(gles.LINES, o.tris, o.lines, gles.GEQUAL, modeVisible)
	o.pass(gles.LINES, o.tris, o.lines, gles.LESS, modeBuriedLine)
	o.pass(gles.LINES, o.tris+o.lines, o.ground, gles.GEQUAL, modeVisible)
	o.pass(gles.LINES, o.tris+o.lines, o.ground, gles.LESS, modeBuriedLine)

	// Handles: over everything.
	gles.Disable(gles.DEPTH_TEST)
	gles.Uniform1i(o.mode, modeVisible)
	run := o.tris + o.lines + o.ground
	gles.DrawArrays(gles.POINTS, run, len(o.verts)-run)

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
