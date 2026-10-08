package scene_test

import (
	"math"
	"testing"

	"zonebuilder/internal/inflect"
	"zonebuilder/internal/scene"
)

type uv = [2]float32

// The terrain draws the layers UE2-Studio draws, in its order, each with its
// own texture and UVs: the first drawn layer opaque, every later one a
// TerrainLayer blended by its alpha map. The expected values were measured
// with UE2-Studio's load_visual_scene on the Fafurion client on 2026-10-08
// (texture path, render mode, the UV of grid samples (1,0), (0,1) and
// (5,7), and the sum of every vertex UV). 21_20 pans, 22_22 and 16_25
// scale, 23_22 rotates, scales and pans one layer. Catches a layer dropped,
// reordered or given the wrong texture, a rotation/pan/scale applied in the
// wrong order or unit, and a mask UV that does not span the tile.
func TestTerrainLayersMatchUE2Studio(t *testing.T) {
	type layer struct {
		texture    string
		uv10, uv01 uv
		uv57       uv
		sumU, sumV float64
	}
	plain := func(tex string) layer {
		return layer{tex, uv{1, 0}, uv{0, 1}, uv{5, 7}, 8355840, 8355840}
	}
	for _, c := range []struct {
		tile   string
		layers []layer
		// only, when set, checks just that drawn layer (0-based) of a map
		// whose other layers the plain cases already cover.
		only int
	}{
		{tile: "22_22", only: -1, layers: []layer{
			plain("T_Innadrile.INNS_05"),
			plain("T_Innadrile.INNS_06"),
			plain("T_texture.Texture.g_01"),
			plain("T_Giran.GI_G"),
			{"T_Giran.GI_C1", uv{0.6666667, 0}, uv{0, 0.6666667}, uv{3.3333333, 4.6666665}, 5570560.000015259, 5570560.000015259},
			plain("T_Innadrile.IG_03"),
			plain("T_ADEN.AS_N_03"),
			plain("T_Giran.GI_S1"),
		}},
		{tile: "16_25", only: -1, layers: []layer{
			plain("T_texture.Texture.Base"),
			plain("T_New_Speaking.respgrass02"),
			{"T_New_Speaking.spdstoneside", uv{0.33333334, 0}, uv{0, 0.33333334}, uv{1.6666666, 2.3333333}, 2785280.0000076294, 2785280.0000076294},
			{"T_New_Speaking.spdstoneside", uv{0.33333334, 0}, uv{0, 0.33333334}, uv{1.6666666, 2.3333333}, 2785280.0000076294, 2785280.0000076294},
			plain("T_New_Speaking.respgrass03"),
			plain("T_New_Speaking.respgrass10"),
			plain("T_New_Speaking.respgrass06"),
			plain("T_New_Speaking.respgrass08"),
			plain("T_New_Speaking.respgrass09"),
			plain("T_New_Speaking.respTR"),
			plain("T_oren.OG_03"),
		}},
		{tile: "23_22", only: 6, layers: []layer{
			{"T_Giran.GI_C1", uv{0.027873786, 0.49878234}, uv{-0.5057823, 0.034873765}, uv{-3.3241074, 2.738028}, -3876804.66077617, 4459145.001474533},
		}},
		{tile: "21_20", only: 6, layers: []layer{
			{"T_oren.OS_01", uv{0.70125026, 0.057499945}, uv{-0.5487497, 1.3075}, uv{5.7012506, 8.8075}, 10408837.233505249, 10448568.396530151},
		}},
	} {
		s := loadTile(t, c.tile)
		ter := s.Terrains[0]
		if c.only < 0 && len(ter.Layers) != len(c.layers) {
			t.Errorf("%s: %s drawn, want %d", c.tile, inflect.Count(len(ter.Layers), "layer", "layers"), len(c.layers))
			continue
		}
		for k, want := range c.layers {
			n := k
			if c.only >= 0 {
				n = c.only
			}
			if n >= len(ter.Layers) {
				t.Errorf("%s: layer %d not drawn", c.tile, n)
				continue
			}
			b := s.Batches[ter.Layers[n]]
			wantMode := scene.TerrainLayer
			if n == 0 {
				wantMode = scene.Opaque
			}
			if b.Texture == nil || b.Texture.Path != want.texture || b.Mode != wantMode {
				path := "<no texture>"
				if b.Texture != nil {
					path = b.Texture.Path
				}
				t.Errorf("%s layer %d: %s mode %d, want %s mode %d", c.tile, n, path, b.Mode, want.texture, wantMode)
				continue
			}
			at := func(x, y int) scene.Vertex { return b.Vertices[x+y*ter.Width] }
			for _, p := range []struct {
				x, y int
				want uv
			}{{1, 0, want.uv10}, {0, 1, want.uv01}, {5, 7, want.uv57}} {
				if got := at(p.x, p.y).UV; !near(got, p.want, 1e-5) {
					t.Errorf("%s layer %d: UV of (%d,%d) = %v, want %v", c.tile, n, p.x, p.y, got, p.want)
				}
			}
			if got, want := at(5, 7).MaskUV, (uv{5.0 / 255, 7.0 / 255}); !near(got, want, 1e-7) {
				t.Errorf("%s layer %d: MaskUV of (5,7) = %v, want %v", c.tile, n, got, want)
			}
			var su, sv float64
			for _, v := range b.Vertices {
				su += float64(v.UV[0])
				sv += float64(v.UV[1])
			}
			if !nearSum(su, want.sumU) || !nearSum(sv, want.sumV) {
				t.Errorf("%s layer %d: UV sum (%v, %v), want (%v, %v)", c.tile, n, su, sv, want.sumU, want.sumV)
			}
		}
	}
}

// A layer with no alpha map covers the whole terrain, as in UE2-Studio,
// which masks it with a 1×1 white bitmap: 20_24's second layer
// (T_Hellbound.HC4) has none. Catches a layer skipped, or drawn
// transparent, for lack of a mask.
func TestLayerWithoutAlphaMapIsDrawnWhole(t *testing.T) {
	s := loadTile(t, "20_24")
	ter := s.Terrains[0]
	if len(ter.Layers) != 2 {
		t.Fatalf("%s drawn, want 2", inflect.Count(len(ter.Layers), "layer", "layers"))
	}
	b := s.Batches[ter.Layers[1]]
	if b.Texture == nil || b.Texture.Path != "T_Hellbound.HC4" {
		t.Fatalf("layer 1 with texture %v, want T_Hellbound.HC4", b.Texture)
	}
	if b.Mode != scene.TerrainLayer || b.Mask != nil {
		t.Errorf("layer 1: mode %d with mask %v, want TerrainLayer without mask (full coverage)", b.Mode, b.Mask)
	}
	if base := s.Batches[ter.Layers[0]].Indices; len(b.Indices) != len(base) || len(base) != 390150 {
		t.Errorf("layer 1 has %s, the base %s; want the whole terrain's 390150",
			inflect.Count(len(b.Indices), "index", "indices"), inflect.Count(len(base), "index", "indices"))
	}
}

func near(a, b uv, tol float64) bool {
	return math.Abs(float64(a[0]-b[0])) <= tol && math.Abs(float64(a[1]-b[1])) <= tol
}

// nearSum compares UV sums over 65536 vertices that UE2-Studio computed in
// f32 trigonometry: a relative 1e-6 absorbs the last-bit differences.
func nearSum(a, b float64) bool {
	return math.Abs(a-b) <= 1e-6*math.Max(1, math.Abs(b))
}
