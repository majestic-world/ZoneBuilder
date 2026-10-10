package play_test

import (
	"fmt"
	"math"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/play"
)

// terrainGrid is a continuous height field with alternating cell diagonals,
// like the terrain consumed by the game mode. The lower layer distinguishes
// support on the visible ground from falling through to buried geometry.
func terrainGrid(origin geom.Vec3, slope float32) []play.Triangle {
	const spacing = 128
	var tris []play.Triangle
	vertex := func(x, y int) geom.Vec3 {
		return origin.Add(geom.Vec3{X: float32(x * spacing), Y: float32(y * spacing), Z: float32(x*spacing) * slope})
	}
	for y := -4; y < 4; y++ {
		for x := -4; x < 12; x++ {
			a, b, c, d := vertex(x, y), vertex(x+1, y), vertex(x+1, y+1), vertex(x, y+1)
			if (x+y)&1 == 0 {
				tris = append(tris, play.Triangle{a, b, c}, play.Triangle{a, c, d})
			} else {
				tris = append(tris, play.Triangle{a, b, d}, play.Triangle{b, c, d})
			}
		}
	}
	lower := quad(origin.Add(geom.Vec3{X: -512, Y: -512, Z: -160}),
		origin.Add(geom.Vec3{X: 1536, Y: -512, Z: -160}),
		origin.Add(geom.Vec3{X: 1536, Y: 512, Z: -160}),
		origin.Add(geom.Vec3{X: -512, Y: 512, Z: -160}))
	return append(tris, lower...)
}

func TestOpenTerrainTraversalAtClientWorldOffsets(t *testing.T) {
	for _, origin := range []geom.Vec3{{}, {X: -114688, Y: -245760, Z: -18000}, {X: 196608, Y: 262144, Z: 12000}} {
		for _, slope := range []float32{0, 0.125, 0.5} {
			t.Run(fmtTerrainCase(origin, slope), func(t *testing.T) {
				s := play.NewSession(play.NewWorld(terrainGrid(origin, slope)), origin.Add(geom.Vec3{X: -64, Y: 37, Z: play.EyeHeight}), 0, 0)
				start := s.Feet()
				for i := range 180 {
					s.Step(play.Input{Seconds: frame, Forward: 1})
					feet := s.Feet()
					floor := origin.Z + (feet.X-origin.X)*slope
					if feet.Z < floor-0.5 || feet.Z > floor+4 || s.Motion() != play.Grounded {
						t.Fatalf("frame %d: feet %v, surface z %v, motion %v; want support on the upper terrain", i, feet, floor, s.Motion())
					}
				}
				if distance := s.Feet().X - start.X; distance < 400 {
					t.Fatalf("open terrain traversal arrested after %v units; want at least 400 in 3 seconds", distance)
				}
				if slope == 0 && math.Abs(float64(s.Feet().X-start.X-780)) > 8 {
					t.Fatalf("flat terrain traversal covered %v units, want 780", s.Feet().X-start.X)
				}
			})
		}
	}
}

func fmtTerrainCase(origin geom.Vec3, slope float32) string {
	return fmt.Sprintf("origin_%g_%g_%g/slope_%g", origin.X, origin.Y, origin.Z, slope)
}

func TestClientWorldTerrainStillBlocksWallsAndSteepSlopes(t *testing.T) {
	origin := geom.Vec3{X: -114688, Y: -245760, Z: -18000}
	for _, obstacle := range []struct {
		name string
		tris []play.Triangle
	}{{"wall", wall(300)}, {"steep_slope", ramp(60)}} {
		t.Run(obstacle.name, func(t *testing.T) {
			tris := terrainGrid(origin, 0)
			for _, triangle := range obstacle.tris {
				for i := range triangle {
					triangle[i] = triangle[i].Add(origin)
				}
				tris = append(tris, triangle)
			}
			world := play.NewWorld(tris)
			for _, seconds := range []float32{frame, 0.1} {
				s := play.NewSession(world, origin.Add(geom.Vec3{Z: play.EyeHeight}), 0, 0)
				run(s, int(math.Round(4/float64(seconds))), play.Input{Seconds: seconds, Forward: 1})
				if x := s.Feet().X - origin.X; x > 284.5 {
					t.Fatalf("frame %v crossed %s at client-world offset: x %v", seconds, obstacle.name, x)
				}
				if z := s.Feet().Z - origin.Z; z < -0.5 || z > 20 {
					t.Fatalf("frame %v climbed or fell through %s: feet z %v", seconds, obstacle.name, z)
				}
			}
		})
	}
}

// This ramp extrudes the walkable face measured in 23_14 at
// (127360, -116736, -2567.005): it rises 128.24988 units in 128 units,
// normal Z 0.7064. Extending the plane removes unrelated mesh and edge
// contacts while retaining the real flat-to-ramp seam and client offset.
func TestWalkableTerrainTraversalDoesNotDependOnFrameSubdivision(t *testing.T) {
	for _, origin := range []geom.Vec3{{}, {X: 127360, Y: -116736, Z: -2695.2549}} {
		t.Run(fmt.Sprintf("origin_%g_%g_%g", origin.X, origin.Y, origin.Z), func(t *testing.T) {
			at := func(x, y, z float32) geom.Vec3 {
				return origin.Add(geom.Vec3{X: x, Y: y, Z: z})
			}
			tris := quad(at(128, 0, 0), at(384, 0, 0), at(384, 128, 0), at(128, 128, 0))
			tris = append(tris, quad(at(-128, 0, 256.49976), at(128, 0, 0),
				at(128, 128, 0), at(-128, 128, 256.49976))...)
			world := play.NewWorld(tris)
			eye := at(170.66667, 42.66667, play.EyeHeight)
			coarse := play.NewSession(world, eye, math.Pi, 0)
			fine := play.NewSession(world, eye, math.Pi, 0)
			start := coarse.Feet()
			run(coarse, 10, play.Input{Seconds: 0.1, Forward: 1})
			run(fine, 60, play.Input{Seconds: frame, Forward: 1})
			coarseDistance := start.X - coarse.Feet().X
			fineDistance := start.X - fine.Feet().X
			if coarse.Motion() != play.Grounded || fine.Motion() != play.Grounded {
				t.Fatalf("walkable ramp lost support: coarse %v, fine %v", coarse.Motion(), fine.Motion())
			}
			if coarseDistance < 100 || fineDistance < 100 || coarseDistance < 0.8*fineDistance {
				t.Fatalf("walking input exhausted on a walkable ramp: 10 x 0.1 s traveled %v, 60 x 1/60 s traveled %v; want at least 100 units and comparable traversal", coarseDistance, fineDistance)
			}
		})
	}
}
