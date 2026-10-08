package scene_test

import (
	"math"
	"path/filepath"
	"testing"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/scene"
	"zonebuilder/internal/unreal"
)

// The BSP surfaces Load draws are the ones UE2-Studio's load_visual_scene
// draws, at the same positions: the same surfaces in the same order, the
// same fan triangles and a position checksum over every index in order
// (Σ (i%7+1)·(x+2y+3z)). The reference numbers come from a probe over
// UE2-Studio's VisualScene.surfaces on the Fafurion client (2026-10-08).
// 18_23 and 23_18 are the tiles with the most BSP triangles; every map
// here also carries the backdrop quad and the off-map helper box the
// region filters drop. Catches a wrong Model layout, licensee gate, fan,
// visibility skip or region filter.
func TestBSPMatchesUE2Studio(t *testing.T) {
	for _, want := range []struct {
		tile                  string
		surfaces, tris, verts int
		ids                   uint64
		sum                   float64
	}{
		{"22_22", 1009, 5615, 9857, 6559704701411890061, 23290752176.0},
		{"22_22_Classic", 1009, 5429, 9539, 7675361110403231564, 22475383347.1},
		{"18_23", 2712, 17531, 29643, 16160471257425746329, 59914856511.8},
		{"23_18", 1846, 16250, 27516, 15198782072325984249, 28854105790.5},
		{"17_22_Classic", 982, 5707, 9823, 8387539673205628799, 13568858763.4},
	} {
		s := loadTile(t, want.tile)
		tris, verts := 0, 0
		var ids uint64
		sum := 0.0
		for _, sf := range s.BSPSurfaces {
			ids = ids*31 + uint64(sf.Index+1)
			b := &s.Batches[sf.Batch]
			idx := b.Indices[sf.First : sf.First+sf.Count]
			tris += len(idx) / 3
			lo, hi := idx[0], idx[0]
			for i, k := range idx {
				lo, hi = min(lo, k), max(hi, k)
				p := b.Vertices[k].Pos
				sum += float64(i%7+1) * (float64(p.X) + 2*float64(p.Y) + 3*float64(p.Z))
			}
			verts += int(hi-lo) + 1
		}
		if len(s.BSPSurfaces) != want.surfaces || tris != want.tris || verts != want.verts || ids != want.ids {
			t.Errorf("%s: %d superfícies, %d triângulos, %d vértices, ids %d; UE2-Studio: %d, %d, %d, %d",
				want.tile, len(s.BSPSurfaces), tris, verts, ids, want.surfaces, want.tris, want.verts, want.ids)
		}
		if math.Abs(sum-want.sum) > 1 {
			t.Errorf("%s: soma das posições %.1f, UE2-Studio %.1f", want.tile, sum, want.sum)
		}
	}
}

// Invisible, portal and backdrop surfaces are neither drawn nor hit, and
// the region filters drop Giran's 655360-wide backdrop quad (z -16384) and
// the helper box parked ~8 tiles away, as UE2-Studio does.
func TestBSPSkipsHiddenSurfaces(t *testing.T) {
	s := loadTile(t, "22_22")
	foot := s.Terrains[0].Bounds
	for _, sf := range s.BSPSurfaces {
		if sf.PolyFlags&unreal.PFNotVisible != 0 {
			t.Errorf("superfície %d desenhada com flags %#x", sf.Index, sf.PolyFlags)
		}
		size, c := sf.Bounds.Size(), sf.Bounds.Center()
		if max(size.X, size.Y) > 2*scene.TileSpan {
			t.Errorf("superfície %d com %v de largura não foi filtrada", sf.Index, max(size.X, size.Y))
		}
		if c.X < foot.Min.X-scene.TileSpan || c.X > foot.Max.X+2*scene.TileSpan ||
			c.Y < foot.Min.Y-scene.TileSpan || c.Y > foot.Max.Y+2*scene.TileSpan {
			t.Errorf("superfície %d fora do mapa em %v", sf.Index, c)
		}
	}
	// The backdrop quad lies under the whole tile, below the terrain: a
	// ray up from beneath it meets the terrain, not the quad.
	if h, ok := s.Pick(scene.Ray{Origin: geom.Vec3{X: 70000, Y: 140000, Z: -20000}, Dir: geom.Vec3{Z: 1}}); !ok || h.Surface != scene.SurfaceTerrain || h.Pos.Z < -5000 {
		t.Errorf("raio sob o backdrop: %+v %v, quero o terreno", h, ok)
	}

	// Every hidden polygon of the Model: a ray at its centroid from one
	// unit away along its normal must not stop there, unless a drawn
	// surface also passes through the centroid (a visible twin of a floor,
	// or a wall crossing it).
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
	type plane struct {
		n      geom.Vec3
		d      float32
		bounds geom.Box
	}
	var drawn []plane
	for _, sf := range s.BSPSurfaces {
		b := &s.Batches[sf.Batch]
		a := b.Vertices[b.Indices[sf.First]].Pos
		n := vec3(model.Vectors[model.Surfs[sf.Index].Normal])
		drawn = append(drawn, plane{n, n.Dot(a), sf.Bounds})
	}
	onDrawnSurface := func(p geom.Vec3) bool {
		for _, q := range drawn {
			in := true
			for a := range 3 {
				in = in && p.Axis(a) >= q.bounds.Min.Axis(a)-0.5 && p.Axis(a) <= q.bounds.Max.Axis(a)+0.5
			}
			if in && math.Abs(float64(q.n.Dot(p)-q.d)) < 0.5 {
				return true
			}
		}
		return false
	}
	hidden := 0
	for _, node := range model.Nodes {
		surf := model.Surfs[node.Surf]
		if surf.PolyFlags&unreal.PFNotVisible == 0 || node.NumVertices < 3 {
			continue
		}
		var c geom.Vec3
		for _, v := range model.Verts[node.VertPool : int(node.VertPool)+int(node.NumVertices)] {
			c = c.Add(vec3(model.Points[v.Point]))
		}
		c = c.Scale(1 / float32(node.NumVertices))
		n := vec3(model.Vectors[surf.Normal])
		h, ok := s.Pick(scene.Ray{Origin: c.Add(n), Dir: n.Scale(-1)})
		if ok && math.Abs(float64(h.Distance-1)) < 0.01 && !onDrawnSurface(c) {
			t.Errorf("superfície oculta (flags %#x) atingida em %v", surf.PolyFlags, h.Pos)
		}
		hidden++
	}
	if hidden == 0 {
		t.Fatal("22_22 sem polígonos ocultos: o teste não verifica nada")
	}
	t.Logf("%d polígonos ocultos verificados", hidden)
}

// A ray straight down from 64 above a Giran point that stands on a BSP
// floor lands on that floor, within 16 units of the server's x y z. The
// points are the Giran Castle Town teleport (charmanage.htm:56) and NPCs
// the datapack spawns in the town (spawn/22_22.xml, line in the name),
// several inside buildings (warehouse, temple, guild hall); Giran's terrain
// has a hole under the town, so only BSP can answer here.
func TestPickLandsOnGiranBSPFloors(t *testing.T) {
	s := loadTile(t, "22_22")
	for _, p := range []struct {
		name    string
		x, y, z float32
	}{
		{"teleporte da cidade", 83400, 147943, -3404},
		{"warehouse_chief_gesto :320", 83263, 146667, -3464},
		{"jurek :330", 85823, 153248, -3494},
		{"sir_kristof_rodemai :341", 84521, 146372, -3404},
		{"gabrielle :342", 81296, 149728, -3469},
		{"maximilian :345", 87058, 148634, -3392},
		{"priest_bandellos :351", 85829, 148365, -3392},
		{"groot :316", 83305, 150521, -3514},
	} {
		h, ok := s.Pick(scene.Ray{Origin: scene.FromServer(geom.Vec3{X: p.x, Y: p.y, Z: p.z + 64}), Dir: geom.Vec3{Z: -1}})
		if !ok {
			t.Errorf("%s: sem acerto", p.name)
			continue
		}
		if h.Surface != scene.SurfaceBSP {
			t.Errorf("%s: superfície %v, quero BSP", p.name, h.Surface)
		}
		d := math.Sqrt(float64(sq(h.Pos.X-p.x) + sq(h.Pos.Y-p.y) + sq(h.Pos.Z-p.z)))
		t.Logf("%s: servidor %v %v %v, pick %.1f %.1f %.1f, Δz %+.1f", p.name, p.x, p.y, p.z, h.Pos.X, h.Pos.Y, h.Pos.Z, h.Pos.Z-p.z)
		if d >= 16 {
			t.Errorf("%s: pick %v a %.1f unidades do servidor, quero menos de 16", p.name, h.Pos, d)
		}
	}
}

// Where a BSP floor lies over the terrain, the nearer of the two wins:
// from above, the floor (the NPC helvetia stands on it, spawn/22_22.xml
// :304, the terrain is ~63 below); from beneath, starting between the
// terrain and the BSP that lies further down (~-3748), the terrain.
func TestPickNearestOfTerrainAndBSP(t *testing.T) {
	s := loadTile(t, "22_22")
	const x, y, z = 80518, 147922, -3506
	h, ok := s.Pick(scene.Ray{Origin: scene.FromServer(geom.Vec3{X: x, Y: y, Z: z + 64}), Dir: geom.Vec3{Z: -1}})
	if !ok || h.Surface != scene.SurfaceBSP || math.Abs(float64(h.Pos.Z-z)) >= 16 {
		t.Errorf("de cima: %+v %v, quero o piso BSP perto de z %v", h, ok, z)
	}
	h, ok = s.Pick(scene.Ray{Origin: scene.FromServer(geom.Vec3{X: x, Y: y, Z: z - 150}), Dir: geom.Vec3{Z: 1}})
	if !ok || h.Surface != scene.SurfaceTerrain || h.Pos.Z > z-32 {
		t.Errorf("de baixo: %+v %v, quero o terreno abaixo do piso", h, ok)
	}
}

func vec3(v [3]float32) geom.Vec3 { return geom.Vec3{X: v[0], Y: v[1], Z: v[2]} }
