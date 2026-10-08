package scene_test

import (
	"math"
	"testing"

	"zonebuilder/internal/scene"
	"zonebuilder/internal/texture"
)

// drawn is the texture path ("-" for none) and render mode one BSP surface
// or mesh section is drawn with.
type drawn struct {
	texture string
	mode    scene.RenderMode
}

func batchDrawn(s *scene.Scene, b int) drawn {
	d := drawn{texture: "-", mode: s.Batches[b].Mode}
	if t := s.Batches[b].Texture; t != nil {
		d.texture = t.Path
	}
	return d
}

func meshSection(t *testing.T, s *scene.Scene, export, k int) scene.MeshActorSection {
	t.Helper()
	for _, a := range s.Actors {
		if a.Export == export {
			if k >= len(a.Sections) {
				t.Fatalf("ator %d tem %d seções, quero a %d", export, len(a.Sections), k)
			}
			return a.Sections[k]
		}
	}
	t.Fatalf("ator %d não está na cena", export)
	return scene.MeshActorSection{}
}

func bspSurface(t *testing.T, s *scene.Scene, index int) scene.BSPSurface {
	t.Helper()
	for _, sf := range s.BSPSurfaces {
		if sf.Index == index {
			return sf
		}
	}
	t.Fatalf("superfície BSP %d não está na cena", index)
	return scene.BSPSurface{}
}

// BSP surfaces and mesh sections reach the texture and render mode
// UE2-Studio's load_visual_scene gives them (measured on the Fafurion
// client on 2026-10-08; the same holds for all 1 125 746 surfaces and
// sections of the 309 tile maps, see the ticket 09 notes). Leaves and
// fences cut out by mask, flags and the mood light's flame add light, and
// water draws in the Water pass, whether it is a BSP sheet or a mesh. The
// flag reaches its texture through a Shader set in the actor's Skins, over
// the mesh's own material. Catches a material graph not followed (an
// untextured surface), a blend flag lost on the way (an opaque leaf), Skins
// ignored, or water drawn as plain translucency.
func TestMaterialsResolveLikeUE2Studio(t *testing.T) {
	scenes := map[string]*scene.Scene{}
	for _, c := range []struct {
		tile string
		what string
		bsp  int // BSP surface index, or -1 for the mesh section below
		mesh struct{ export, section int }
		want drawn
	}{
		{"22_22", "água BSP", 1086, struct{ export, section int }{}, drawn{"FX_E_T.WaterSurfaceSet.ocean011", scene.Water}},
		{"22_22", "parede BSP", 500, struct{ export, section int }{}, drawn{"Giran_Village_T.Giran_wall07", scene.Opaque}},
		{"22_22", "água em mesh", -1, struct{ export, section int }{1020, 0}, drawn{"interior_B_t.interior_B_Water01", scene.Water}},
		{"22_22", "folhas", -1, struct{ export, section int }{1041, 0}, drawn{"Superion_T.superion_tree_01_leaf", scene.Masked}},
		{"22_22", "chama", -1, struct{ export, section int }{1569, 0}, drawn{"FX_E_T.Flameset.de_fire_0000", scene.Brighten}},
		{"22_22", "translúcido", -1, struct{ export, section int }{543, 0}, drawn{"interior_A_t.interior_A_deco27", scene.Translucent}},
		{"22_22", "bandeira por Skins", -1, struct{ export, section int }{2615, 0}, drawn{"Giran_Village_T.Giran_flag", scene.Masked}},
		{"22_22", "placa por Skins", -1, struct{ export, section int }{868, 2}, drawn{"Field_Deco_Artifact2_T.refined_obj.refined_22_22_t003", scene.Opaque}},
		{"22_20", "cerca", -1, struct{ export, section int }{1042, 1}, drawn{"Hunter_Village_T.Ht_vi_fence_001", scene.Masked}},
	} {
		s := scenes[c.tile]
		if s == nil {
			s = loadTile(t, c.tile)
			scenes[c.tile] = s
		}
		var got drawn
		if c.bsp >= 0 {
			got = batchDrawn(s, bspSurface(t, s, c.bsp).Batch)
		} else {
			got = batchDrawn(s, meshSection(t, s, c.mesh.export, c.mesh.section).Batch)
		}
		if got != c.want {
			t.Errorf("%s %s: %s modo %d, quero %s modo %d", c.tile, c.what, got.texture, got.mode, c.want.texture, c.want.mode)
		}
	}
}

// BSP UVs are texels along the surface's U/V vectors from its base point,
// divided by the texture's size, and mesh UVs are the mesh's first stream,
// as UE2-Studio uploads them (the first vertex and the sum over a surface's
// vertices, or over a section's triangle corners). Catches a BSP UV left in
// texels, divided by the wrong size or measured from the wrong point, and
// mesh UVs from the wrong stream or remapped to the wrong vertices.
func TestUVsMatchUE2Studio(t *testing.T) {
	s := loadTile(t, "22_22")
	for _, c := range []struct {
		surf       int
		first      [2]float32
		sumU, sumV float64
	}{
		{500, [2]float32{0, 0.0078125}, 7.90625, -1.296875},
		{900, [2]float32{-0.0859375, -2.5}, 1.84375, -10.4921875},
		{1086, [2]float32{1.7421875, -19.46289}, 4704.586212158203, -8832.93637084961},
	} {
		sf := bspSurface(t, s, c.surf)
		b := &s.Batches[sf.Batch]
		idx := b.Indices[sf.First : sf.First+sf.Count]
		lo, hi := idx[0], idx[0]
		for _, k := range idx {
			lo, hi = min(lo, k), max(hi, k)
		}
		var su, sv float64
		for _, v := range b.Vertices[lo : hi+1] {
			su += float64(v.UV[0])
			sv += float64(v.UV[1])
		}
		first := b.Vertices[lo].UV
		if math.Abs(float64(first[0]-c.first[0])) > 1e-4 || math.Abs(float64(first[1]-c.first[1])) > 1e-4 ||
			math.Abs(su-c.sumU) > 1e-3 || math.Abs(sv-c.sumV) > 1e-3 {
			t.Errorf("superfície %d: primeiro UV %v, soma (%v, %v); quero %v, (%v, %v)", c.surf, first, su, sv, c.first, c.sumU, c.sumV)
		}
	}
	for _, c := range []struct {
		export, section int
		sumU, sumV      float64
	}{
		{1041, 0, 12.655200004577637, 12},
		{1041, 1, 12.655200004577637, 12},
		{2615, 0, 9, 9},
	} {
		sec := meshSection(t, s, c.export, c.section)
		b := &s.Batches[sec.Batch]
		var su, sv float64
		for _, k := range b.Indices[sec.First : sec.First+sec.Count] {
			su += float64(b.Vertices[k].UV[0])
			sv += float64(b.Vertices[k].UV[1])
		}
		if math.Abs(su-c.sumU) > 1e-4 || math.Abs(sv-c.sumV) > 1e-4 {
			t.Errorf("ator %d seção %d: soma dos UV (%v, %v), quero (%v, %v)", c.export, c.section, su, sv, c.sumU, c.sumV)
		}
	}
}

// A terrain layer whose alpha map is P8 is drawn, blended by that alpha
// map decoded through its Palette, where UE2-Studio, which does not decode
// P8, leaves it out: 12_22 draws 3 layers to its 2, the second one masked
// by T_12_22.Height.12_22_ST_00. Catches a P8 alpha map skipped again, or
// one whose palette was not found.
func TestP8AlphaMapLayerIsDrawn(t *testing.T) {
	s := loadTile(t, "12_22")
	ter := s.Terrains[0]
	if len(ter.Layers) != 3 {
		t.Fatalf("%d camadas desenhadas, quero 3 (as 2 do UE2-Studio e a de alpha map P8)", len(ter.Layers))
	}
	m := s.Batches[ter.Layers[1]].Mask
	if m == nil || m.Path != "T_12_22.Height.12_22_ST_00" || m.Format != texture.FormatP8 {
		t.Fatalf("camada 1 com máscara %v, quero o P8 T_12_22.Height.12_22_ST_00", m)
	}
	img, err := m.RGBA()
	if err != nil {
		t.Fatal(err)
	}
	if img.Width != 512 || img.Height != 512 {
		t.Errorf("máscara de %d×%d, quero 512×512", img.Width, img.Height)
	}
}
