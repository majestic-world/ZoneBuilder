package scene_test

import (
	"math"
	"math/rand/v2"
	"path/filepath"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/unreal"
)

// On Giran (22_22), at 1000 random columns where a vertical pick from above
// lands on the terrain, the highest floor triangle World.Floor gives under
// the column sits at the Z of that pick, within 0.01. Catches a floor that
// splits a quad on the wrong diagonal (EdgeTurn), keeps invisible quads,
// reads the heights off by a row or a column, or forgets ToServer.
func TestFloorTopMatchesTheVerticalPick(t *testing.T) {
	s := loadTile(t, "22_22")
	w := scene.NewWorld(geom.Vec3{})
	w.Add(s)
	w.HideMeshes = true
	b := s.Terrains[0].Bounds
	rng := rand.New(rand.NewPCG(22, 22))
	checked := 0
	for tries := 0; checked < 1000 && tries < 20000; tries++ {
		x := b.Min.X + rng.Float32()*(b.Max.X-b.Min.X)
		y := b.Min.Y + rng.Float32()*(b.Max.Y-b.Min.Y)
		h, ok := w.Pick(down(x, y))
		if !ok || h.Surface != scene.SurfaceTerrain {
			continue
		}
		checked++
		top, found := math.Inf(-1), false
		box := geom.Box{Min: geom.Vec3{X: x - 1, Y: y - 1}, Max: geom.Vec3{X: x + 1, Y: y + 1}}
		w.Floor(box, func(f scene.FloorTriangle) {
			if z, ok := zAt(f, float64(x), float64(y)); ok && z > top {
				top, found = z, true
			}
		})
		if !found {
			t.Errorf("column %v %v: pick hit terrain at z %v, floor has no triangle there", x, y, h.Pos.Z)
			continue
		}
		if d := math.Abs(top - float64(h.Pos.Z)); d > 0.01 {
			t.Errorf("column %v %v: floor top z %.4f, pick z %.4f (Δ %.4f)", x, y, top, h.Pos.Z, d)
		}
	}
	if checked < 1000 {
		t.Fatalf("only %d columns over the terrain", checked)
	}
}

// On Giran (22_22) with the static meshes shown, at random columns over
// the mesh actors, half of them over actors placed with a mirroring
// scale, where a vertical pick from above lands on a mesh or the BSP, on a
// face no steeper than 60° (its slope read off 2 more picks 2 units away,
// not off the winding), the highest floor triangle World.Floor gives under
// the column sits at the Z of that pick. Catches upward faces read off the
// wrong winding (a roof's top left out, its underside kept), a mirrored
// actor's faces turned over, or the pickables' triangles taken from the
// wrong batch run.
func TestFloorTopOverMeshesMatchesTheVerticalPick(t *testing.T) {
	s := loadTile(t, "22_22")
	w := scene.NewWorld(geom.Vec3{})
	w.Add(s)
	var actors [2][]geom.Box // plain, mirrored
	for _, a := range s.Actors {
		if !a.Hidden && len(a.Sections) > 0 {
			sc := a.Actor.DrawScale3D.Scale(a.Actor.DrawScale)
			mirrored := 0
			if sc.X*sc.Y*sc.Z < 0 {
				mirrored = 1
			}
			actors[mirrored] = append(actors[mirrored], a.Bounds)
		}
	}
	rng := rand.New(rand.NewPCG(4, 22))
	var checked [2]map[scene.Surface]int
	checked[0], checked[1] = map[scene.Surface]int{}, map[scene.Surface]int{}
	for tries := 0; min(checked[0][scene.SurfaceMesh], checked[1][scene.SurfaceMesh]) < 250 && tries < 50000; tries++ {
		kind := tries % 2
		b := actors[kind][rng.IntN(len(actors[kind]))]
		x := b.Min.X + rng.Float32()*(b.Max.X-b.Min.X)
		y := b.Min.Y + rng.Float32()*(b.Max.Y-b.Min.Y)
		h, ok := w.Pick(down(x, y))
		if !ok || h.Surface == scene.SurfaceTerrain {
			continue
		}
		// 4 more picks 1 unit around: on one plane through the hit (no
		// crease or face edge in between), and its slope.
		var z [4]float64
		flat := true
		for k, d := range [4][2]float32{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			hk, ok := w.Pick(down(x+d[0], y+d[1]))
			flat = flat && ok
			z[k] = float64(hk.Pos.Z)
		}
		z0 := float64(h.Pos.Z)
		if !flat || math.Abs(z[0]+z[1]-2*z0) > 0.02 || math.Abs(z[2]+z[3]-2*z0) > 0.02 {
			continue
		}
		if gx, gy := (z[0]-z[1])/2, (z[2]-z[3])/2; 1/math.Sqrt(1+gx*gx+gy*gy) < 0.55 {
			continue // steeper than 57°: no floor to compare with
		}
		checked[kind][h.Surface]++
		top, found := math.Inf(-1), false
		box := geom.Box{Min: geom.Vec3{X: x - 1, Y: y - 1}, Max: geom.Vec3{X: x + 1, Y: y + 1}}
		w.Floor(box, func(f scene.FloorTriangle) {
			if z, ok := zAt(f, float64(x), float64(y)); ok && z > top {
				top, found = z, true
			}
		})
		if d := math.Abs(top - float64(h.Pos.Z)); !found || d > 0.05 {
			t.Errorf("column %v %v: pick hit surface %d at z %.3f, floor top %.3f (found %v)", x, y, h.Surface, h.Pos.Z, top, found)
		}
	}
	for kind, name := range []string{"plain", "mirrored"} {
		if checked[kind][scene.SurfaceMesh] < 250 {
			t.Errorf("only %d columns on an upward mesh face over %s actors", checked[kind][scene.SurfaceMesh], name)
		}
		t.Logf("%s actors: %d mesh columns, %d BSP columns", name, checked[kind][scene.SurfaceMesh], checked[kind][scene.SurfaceBSP])
	}
}

// On Giran (22_22), no triangle of a BSP surface whose Model normal is a
// wall (|z| < 0.5) or a ceiling (z ≤ −0.5) comes as floor, and every
// surface whose normal faces up gives floor. Catches walls or ceilings
// counted as floor, and a surface's facing read off each fan triangle's
// winding, which a sliver of a ceiling's fan turns up.
func TestBSPWallsAndCeilingsAreNotFloor(t *testing.T) {
	s := loadTile(t, "22_22")
	m, err := l2pkg.Open(filepath.Join(clientRoot(t), "Maps", "22_22.unr"))
	if err != nil {
		t.Fatal(err)
	}
	level, err := unreal.ReadLevel(m, unreal.FindLevel(m))
	if err != nil {
		t.Fatal(err)
	}
	model, err := unreal.ReadModel(m, int(level.Model)-1)
	if err != nil {
		t.Fatal(err)
	}
	w := scene.NewWorld(geom.Vec3{})
	w.Add(s)
	floor := map[[3]geom.Vec3]bool{}
	w.Floor(s.Bounds, func(f scene.FloorTriangle) {
		if f.Surface == scene.SurfaceBSP {
			floor[[3]geom.Vec3{f.A, f.B, f.C}] = true
		}
	})
	kinds := map[string]int{}
	for _, sf := range s.BSPSurfaces {
		n := vec3(model.Vectors[model.Surfs[sf.Index].Normal])
		kind := "floor"
		switch {
		case n.Z <= -0.5:
			kind = "ceiling"
		case n.Z < 0.5:
			kind = "wall"
		}
		kinds[kind]++
		b := &s.Batches[sf.Batch]
		idx := b.Indices[sf.First : sf.First+sf.Count]
		got := 0
		for k := 0; k+2 < len(idx); k += 3 {
			key := [3]geom.Vec3{scene.ToServer(b.Vertices[idx[k]].Pos), scene.ToServer(b.Vertices[idx[k+1]].Pos), scene.ToServer(b.Vertices[idx[k+2]].Pos)}
			if floor[key] {
				got++
			}
		}
		if kind != "floor" && got > 0 {
			t.Errorf("%s surface %d (normal %v): %d floor triangles", kind, sf.Index, n, got)
		}
		if kind == "floor" && got == 0 {
			t.Errorf("floor surface %d (normal %v): no floor triangle", sf.Index, n)
		}
	}
	if kinds["wall"] == 0 || kinds["ceiling"] == 0 || kinds["floor"] == 0 {
		t.Fatalf("22_22 surfaces %v: the test needs walls, ceilings and floors", kinds)
	}
	t.Logf("surfaces %v", kinds)
}

// On Giran (22_22), the static meshes give floor while shown and none
// while hidden, and the BSP gives the same floor either way: what is seen
// is what is measured.
func TestHiddenMeshesGiveNoFloor(t *testing.T) {
	s := loadTile(t, "22_22")
	w := scene.NewWorld(geom.Vec3{})
	w.Add(s)
	count := func() map[scene.Surface]int {
		n := map[scene.Surface]int{}
		w.Floor(s.Bounds, func(f scene.FloorTriangle) { n[f.Surface]++ })
		return n
	}
	shown := count()
	w.HideMeshes = true
	hidden := count()
	if shown[scene.SurfaceMesh] == 0 || hidden[scene.SurfaceMesh] != 0 || hidden[scene.SurfaceBSP] != shown[scene.SurfaceBSP] {
		t.Errorf("floor triangles shown %v, hidden %v; want meshes only while shown, the same BSP", shown, hidden)
	}
}

// zAt is the Z of triangle f at column (x, y), false when the column
// misses it.
func zAt(f scene.FloorTriangle, x, y float64) (float64, bool) {
	ax, ay, az := float64(f.A.X), float64(f.A.Y), float64(f.A.Z)
	bx, by, bz := float64(f.B.X), float64(f.B.Y), float64(f.B.Z)
	cx, cy, cz := float64(f.C.X), float64(f.C.Y), float64(f.C.Z)
	det := (bx-ax)*(cy-ay) - (cx-ax)*(by-ay)
	if det == 0 {
		return 0, false
	}
	u := ((x-ax)*(cy-ay) - (cx-ax)*(y-ay)) / det
	v := ((bx-ax)*(y-ay) - (x-ax)*(by-ay)) / det
	const slack = 1e-9
	if u < -slack || v < -slack || u+v > 1+slack {
		return 0, false
	}
	return az + u*(bz-az) + v*(cz-az), true
}
