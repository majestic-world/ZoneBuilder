package texture_test

import (
	"hash/fnv"
	"os"
	"testing"

	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/texture"
)

type pixel [4]uint8 // R, G, B, A

// Real client textures decode to the pixels UE2-Studio's decoder
// (texture-engine decode_texture_rgba, reached through
// PackageLoader::visual_material_ref) produced from the same bytes on the
// Fafurion client on 2026-10-08: the FNV-1a 64 of the whole mip-0 RGBA
// buffer, plus four pixels of one row spelled out. Catches a swapped
// BGRA/RGBA channel order, a DXT1 1-bit alpha that is not cut (or cuts
// opaque texels), a DXT3 nibble order or DXT5 alpha palette error, and a
// wrong RGB565 expansion or block walk.
func TestRealTexturesDecodeLikeUE2Studio(t *testing.T) {
	root := os.Getenv("ZB_CLIENT")
	if root == "" {
		t.Skip("ZB_CLIENT não definido: testes contra o cliente real pulados")
	}
	c := l2pkg.NewClient(root)
	for _, tc := range []struct {
		what          string
		pkg, name     string
		format        uint8
		width, height int
		fnv           uint64
		x, y          int
		row           [4]pixel
	}{
		{
			// Texels 1-3 sit in a c0 <= c1 block with colour index 3:
			// transparent black, while texel 0 stays opaque.
			what: "DXT1 com alfa de 1 bit", pkg: "Oren_DEV_T", name: "DE_V_fence",
			format: 3, width: 256, height: 256, fnv: 0xfc17e536d5aa435f,
			x: 96, y: 0, row: [4]pixel{{49, 42, 36, 255}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}},
		},
		{
			what: "DXT3", pkg: "Giran_Devil_island_T", name: "devil_island_rope02",
			format: 7, width: 128, height: 128, fnv: 0x2d8373a0cf011d54,
			x: 12, y: 0, row: [4]pixel{{92, 83, 65, 0}, {65, 48, 24, 255}, {148, 153, 148, 255}, {120, 118, 106, 255}},
		},
		{
			what: "DXT5", pkg: "Annihilation_Dungeon_Boss_T", name: "Annihilation_F_boss_egg_b02",
			format: 8, width: 256, height: 256, fnv: 0x844f9be10d6796a3,
			x: 0, y: 0, row: [4]pixel{{87, 71, 65, 28}, {87, 71, 65, 24}, {74, 56, 49, 20}, {87, 71, 65, 26}},
		},
		{
			// Stored B, G, R, A: a missed swap turns this grass purple.
			what: "RGBA8 gravado como BGRA", pkg: "T_New_Speaking", name: "respgrass02",
			format: 5, width: 256, height: 256, fnv: 0xbfed4ab2b21850f9,
			x: 0, y: 0, row: [4]pixel{{121, 125, 76, 255}, {87, 96, 52, 255}, {19, 33, 3, 255}, {53, 61, 27, 255}},
		},
	} {
		t.Run(tc.what, func(t *testing.T) {
			p, err := c.Package(tc.pkg)
			if err != nil {
				t.Fatal(err)
			}
			i := p.ExportNamed(tc.name, "Texture")
			if i < 0 {
				t.Fatalf("%s não tem a Texture %s", tc.pkg, tc.name)
			}
			tex, err := texture.Read(p, i)
			if err != nil {
				t.Fatal(err)
			}
			if tex.Format != tc.format {
				t.Fatalf("%s.%s: formato %d, quero %d", tc.pkg, tc.name, tex.Format, tc.format)
			}
			img, err := tex.RGBA()
			if err != nil {
				t.Fatal(err)
			}
			if img.Width != tc.width || img.Height != tc.height || len(img.Pix) != 4*tc.width*tc.height {
				t.Fatalf("%dx%d com %d bytes, quero %dx%d", img.Width, img.Height, len(img.Pix), tc.width, tc.height)
			}
			for k, want := range tc.row {
				o := 4 * (tc.y*img.Width + tc.x + k)
				if got := pixel(img.Pix[o : o+4]); got != want {
					t.Errorf("pixel (%d,%d) = %v, quero %v", tc.x+k, tc.y, got, want)
				}
			}
			h := fnv.New64a()
			h.Write(img.Pix)
			if got := h.Sum64(); got != tc.fnv {
				t.Errorf("FNV-1a 64 do mip 0 = %#016x, quero %#016x", got, tc.fnv)
			}
		})
	}
}

// P8 editor icons of the client's Engine.u decode, through their Palette,
// to the RGBA UE2-Studio ships in assets/ (copied to testdata/), which were
// extracted from UnrealEd's own Engine.u. Catches a palette read in the
// wrong channel order, trusting the palette's stored alpha, and a masked
// texture whose index 0 is not cut out.
func TestP8DecodesThroughItsPalette(t *testing.T) {
	root := os.Getenv("ZB_CLIENT")
	if root == "" {
		t.Skip("ZB_CLIENT não definido: testes contra o cliente real pulados")
	}
	p, err := l2pkg.NewClient(root).Package("Engine")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"S_Light", "S_Actor", "S_ZoneInfo"} {
		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile("testdata/" + name + ".rgba")
			if err != nil {
				t.Fatal(err)
			}
			i := p.ExportNamed(name, "Texture")
			if i < 0 {
				t.Fatalf("Engine não tem a Texture %s", name)
			}
			tex, err := texture.Read(p, i)
			if err != nil {
				t.Fatal(err)
			}
			if tex.Format != texture.FormatP8 {
				t.Fatalf("formato %d, quero P8", tex.Format)
			}
			if tex.PaletteRef <= 0 {
				t.Fatalf("Palette %d não é um export de Engine", tex.PaletteRef)
			}
			if tex.Palette, err = texture.ReadPalette(p, int(tex.PaletteRef-1)); err != nil {
				t.Fatal(err)
			}
			img, err := tex.RGBA()
			if err != nil {
				t.Fatal(err)
			}
			if img.Width != 32 || img.Height != 32 {
				t.Fatalf("%d×%d, quero 32×32", img.Width, img.Height)
			}
			bad := 0
			for k := 0; k < len(want); k += 4 {
				if got := pixel(img.Pix[k : k+4]); got != pixel(want[k:k+4]) {
					if bad < 4 {
						t.Errorf("texel %d = %v, quero %v", k/4, got, pixel(want[k:k+4]))
					}
					bad++
				}
			}
			if bad > 0 {
				t.Errorf("%d de %d texels diferentes", bad, len(want)/4)
			}
		})
	}
}
