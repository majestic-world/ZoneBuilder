package render

import (
	"math"
	"strconv"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/render/gles"
	"zonebuilder/internal/scene"
)

// Ground shows where the ground is, to place zones on it: the terrain's
// cells drawn as a grid on the surfaces facing up, and the selected
// zone's footprint where its shapes meet the scene.
type Ground struct {
	// Grid draws the terrain cell lines, a stronger line every
	// GridMajor cells.
	Grid bool
	// Shapes are the selected zone's closed shapes. The scene inside a
	// shape's outline and Z range is tinted with Color; inside the
	// outline but above the range it gets a warm hatch, below it a cold
	// one; lines mark where it crosses each shape's ZMin and ZMax; the
	// outline is drawn on whatever surface it crosses. Banned shapes cut
	// holes. Shapes past the shader's room are left out (GroundLeftOut).
	Shapes []GroundShape
	// Color is the zone's linear RGB.
	Color [3]float32
}

// GroundShape is one shape of the footprint, in server coordinates (the
// numbers that go into the XML).
type GroundShape struct {
	// Points is the outline; only X and Y are used.
	Points     []geom.Vec3
	ZMin, ZMax float32
	Banned     bool
}

// GridMajor is how many terrain cells lie between the grid's strong
// lines.
const GridMajor = 8

// The footprint's capacity in the shader. Points past it, and the
// shapes they belong to, are left out (GroundLeftOut counts them).
const (
	groundMaxPoints = 128
	groundMaxShapes = 8
)

// groundFits reports whether shape s still fits in the shader after
// shapes shapes holding points points, and whether it is drawable at all.
func groundFits(s GroundShape, shapes, points int) (fits, drawable bool) {
	if len(s.Points) < 3 {
		return false, false
	}
	return shapes < groundMaxShapes && points+len(s.Points) <= groundMaxPoints, true
}

// GroundLeftOut is how many drawable shapes of g the shader leaves out
// of the footprint for lack of room.
func GroundLeftOut(g Ground) int {
	shapes, points, out := 0, 0, 0
	for _, s := range g.Shapes {
		fits, drawable := groundFits(s, shapes, points)
		switch {
		case fits:
			shapes, points = shapes+1, points+len(s.Points)
		case drawable:
			out++
		}
	}
	return out
}

// groundShader holds the ground uniforms of the scene program and the
// footprint in client coordinates, rebased on every draw.
type groundShader struct {
	// The uniform locations.
	mode, gridOrigin, gridStep             int32
	poly, shapes, shapeZ, count, zoneColor int32

	grid   bool
	color  [3]float32
	points []geom.Vec3
	ranges [][4]int32 // first point, count, banned
	zs     [][2]float32
	// The uniform values, reused across frames.
	packed    [][4]float32
	rebasedZs [][2]float32
}

func newGroundShader(prog uint32) groundShader {
	loc := func(name string) int32 { return gles.GetUniformLocation(prog, name) }
	return groundShader{
		mode:       loc("uGround"),
		gridOrigin: loc("uGridOrigin"),
		gridStep:   loc("uGridStep"),
		poly:       loc("uPoly"),
		shapes:     loc("uShapes"),
		shapeZ:     loc("uShapeZ"),
		count:      loc("uShapeCount"),
		zoneColor:  loc("uZoneColor"),
	}
}

// set takes g, converting the footprint to client coordinates.
func (gs *groundShader) set(g Ground) {
	gs.grid, gs.color = g.Grid, g.Color
	gs.points, gs.ranges, gs.zs = gs.points[:0], gs.ranges[:0], gs.zs[:0]
	for _, s := range g.Shapes {
		if fits, _ := groundFits(s, len(gs.ranges), len(gs.points)); !fits {
			continue
		}
		gs.ranges = append(gs.ranges, [4]int32{int32(len(gs.points)), int32(len(s.Points)), boolInt(s.Banned), 0})
		for _, p := range s.Points {
			gs.points = append(gs.points, scene.FromServer(p))
		}
		zmin := scene.FromServer(geom.Vec3{Z: s.ZMin}).Z
		zmax := scene.FromServer(geom.Vec3{Z: s.ZMax}).Z
		gs.zs = append(gs.zs, [2]float32{zmin, zmax})
	}
}

// upload sets the ground uniforms of the bound scene program for a frame
// rebased on rebase, with the grid of terrain t (none when t is nil).
func (gs *groundShader) upload(rebase geom.Vec3, t *scene.Terrain) {
	mode := int32(0)
	if gs.grid && t != nil && t.Scale.X > 0 && t.Scale.Y > 0 {
		mode |= 1
		// The grid origin is reduced to within one cell of the rebase
		// origin, in float64, so the shader works on small numbers.
		ox := math.Mod(float64(t.Position.X)-float64(rebase.X), float64(t.Scale.X))
		oy := math.Mod(float64(t.Position.Y)-float64(rebase.Y), float64(t.Scale.Y))
		gles.Uniform2f(gs.gridOrigin, float32(ox), float32(oy))
		gles.Uniform2f(gs.gridStep, t.Scale.X, t.Scale.Y)
	}
	if len(gs.ranges) > 0 {
		mode |= 2
		gs.packed = gs.packed[:0]
		for i := 0; i < len(gs.points); i += 2 {
			a := gs.points[i]
			v := [4]float32{a.X - rebase.X, a.Y - rebase.Y}
			if i+1 < len(gs.points) {
				b := gs.points[i+1]
				v[2], v[3] = b.X-rebase.X, b.Y-rebase.Y
			}
			gs.packed = append(gs.packed, v)
		}
		gles.Uniform4fv(gs.poly, gs.packed)
		gles.Uniform4iv(gs.shapes, gs.ranges)
		gs.rebasedZs = gs.rebasedZs[:0]
		for _, z := range gs.zs {
			gs.rebasedZs = append(gs.rebasedZs, [2]float32{z[0] - rebase.Z, z[1] - rebase.Z})
		}
		gles.Uniform2fv(gs.shapeZ, gs.rebasedZs)
		gles.Uniform1i(gs.count, int32(len(gs.ranges)))
		gles.Uniform3f(gs.zoneColor, gs.color[0], gs.color[1], gs.color[2])
	}
	gles.Uniform1i(gs.mode, mode)
}

// groundGLSL is the scene fragment shader's ground marking: ground(c)
// returns colour c of the fragment at vPos (rebased, Unreal basis) with
// the grid and the footprint drawn over it.
var groundGLSL = `
uniform int uGround;
uniform highp vec2 uGridOrigin;
uniform highp vec2 uGridStep;
uniform highp vec4 uPoly[` + itoa(groundMaxPoints/2) + `];
uniform ivec4 uShapes[` + itoa(groundMaxShapes) + `];
uniform highp vec2 uShapeZ[` + itoa(groundMaxShapes) + `];
uniform int uShapeCount;
uniform vec3 uZoneColor;

highp vec2 polyPoint(int i) {
	highp vec4 v = uPoly[i / 2];
	return (i % 2 == 0) ? v.xy : v.zw;
}

// gridLine is how much of a line of the grid of cell coordinates g the
// fragment covers, faded out where the cells shrink under a few pixels.
float gridLine(highp vec2 g) {
	highp vec2 fw = max(fwidth(g), vec2(1e-6));
	vec2 d = abs(fract(g - 0.5) - 0.5) / fw;
	float line = 1.0 - min(min(d.x, d.y), 1.0);
	return line * (1.0 - smoothstep(0.12, 0.3, max(fw.x, fw.y)));
}

// The footprint's fixed colours above and below a shape's Z range, apart
// from every zone type's colour: a hot red-orange and an ice blue.
const vec3 groundAbove = vec3(1.0, 0.18, 0.0);
const vec3 groundBelow = vec3(0.0, 0.55, 1.0);

vec3 ground(vec3 c) {
	// The derivatives are taken here, in uniform control flow.
	highp vec3 n = normalize(cross(dFdx(vPos), dFdy(vPos)));
	float up = smoothstep(0.3, 0.5, abs(n.z));
	highp vec2 g = (vPos.xy - uGridOrigin) / max(uGridStep, vec2(1.0));
	float minor = gridLine(g);
	float major = gridLine(g / ` + itoa(GridMajor) + `.0);
	highp float px = length(fwidth(vPos.xy));
	highp float fz = max(fwidth(vPos.z), 1e-4);
	if ((uGround & 1) != 0) {
		c = mix(c, c * 0.25, minor * up * 0.75);
		c = mix(c, vec3(0.95, 0.85, 0.35), major * up * 0.55);
	}
	if ((uGround & 2) != 0) {
		bool inside = false, above = false, below = false, cut = false;
		highp float edge = 1e20;
		// top and bottom are the nearest crossings, in pixels, of the ZMax
		// and ZMin of a shape whose outline holds the fragment.
		highp float top = 1e20, bottom = 1e20;
		for (int s = 0; s < uShapeCount; s++) {
			ivec4 sh = uShapes[s];
			bool in_ = false;
			for (int k = 0; k < sh.y; k++) {
				highp vec2 a = polyPoint(sh.x + k);
				highp vec2 b = polyPoint(sh.x + (k + 1) % sh.y);
				if ((a.y > vPos.y) != (b.y > vPos.y) && vPos.x < (b.x - a.x) * (vPos.y - a.y) / (b.y - a.y) + a.x) {
					in_ = !in_;
				}
				highp vec2 ab = b - a;
				highp float t = clamp(dot(vPos.xy - a, ab) / max(dot(ab, ab), 1e-6), 0.0, 1.0);
				edge = min(edge, length(vPos.xy - a - ab * t));
			}
			bool zin = vPos.z >= uShapeZ[s].x && vPos.z <= uShapeZ[s].y;
			if (in_ && sh.z != 0) {
				cut = cut || zin;
			} else if (in_) {
				inside = inside || zin;
				above = above || vPos.z > uShapeZ[s].y;
				below = below || vPos.z < uShapeZ[s].x;
				top = min(top, abs(vPos.z - uShapeZ[s].y) / fz);
				bottom = min(bottom, abs(vPos.z - uShapeZ[s].x) / fz);
			}
		}
		if (!cut) {
			if (inside) {
				c = mix(c, uZoneColor, 0.3);
			} else if (above) {
				// Warm hatch, rising to the right.
				float h = step(fract((gl_FragCoord.x + gl_FragCoord.y) / 12.0), 0.4);
				c = mix(c, groundAbove, 0.15 + 0.5 * h);
			} else if (below) {
				// Cold hatch, falling to the right.
				float h = step(fract((gl_FragCoord.x - gl_FragCoord.y) / 12.0), 0.4);
				c = mix(c, groundBelow, 0.15 + 0.5 * h);
			}
			// The ZMax and ZMin lines, in the hatch colours: the exact edge
			// of the covered area.
			c = mix(c, groundAbove * 1.1 + 0.1, (1.0 - smoothstep(1.0, 2.0, top)) * up);
			c = mix(c, groundBelow * 1.1 + 0.1, (1.0 - smoothstep(1.0, 2.0, bottom)) * up);
		}
		float line = 1.0 - smoothstep(px * 1.2, px * 2.4, edge);
		c = mix(c, uZoneColor * 1.3 + 0.15, line);
	}
	return c;
}
`

func itoa(n int) string { return strconv.Itoa(n) }
