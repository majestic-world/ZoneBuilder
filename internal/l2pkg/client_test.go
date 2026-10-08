package l2pkg_test

import (
	"os"
	"path/filepath"
	"testing"

	"zonebuilder/internal/l2pkg"
)

// clientRoot is the real client the tests read (the folder above Maps),
// named by ZB_CLIENT. The tests skip without it.
func clientRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("ZB_CLIENT")
	if root == "" {
		t.Skip("ZB_CLIENT não definido: testes contra o cliente real pulados")
	}
	return root
}

// Every .unr, .utx and .usx of the client opens and verifies. Catches a
// broken container, header or table read, and any ArVer/licensee pair the
// tables are read wrong under.
func TestSweepOpensEveryClientPackage(t *testing.T) {
	root := clientRoot(t)
	var opened int
	err := l2pkg.Sweep(root, func(r l2pkg.SweepResult) {
		if r.Err != nil {
			t.Errorf("%s: %v", r.Path, r.Err)
			return
		}
		opened++
	})
	if err != nil {
		t.Fatal(err)
	}
	if opened == 0 {
		t.Fatalf("nenhum pacote encontrado em %s", root)
	}
	t.Logf("%d pacotes abertos", opened)
}

// A Ver121 package keys its XOR on its own file name. Under its own name the
// key must come from the name; renamed, it must be recovered from the
// encrypted magic and read the same tables. Catches a regression in either
// derivation.
func TestRenamedVer121OpensByKeyRecovery(t *testing.T) {
	original := filepath.Join(clientRoot(t), "Textures", "Giran_Village_T.utx")
	want, err := l2pkg.Open(original)
	if err != nil {
		t.Fatal(err)
	}
	if want.Container.Version != 121 || want.Container.Recovered {
		t.Fatalf("original: container %v, quer Lineage2Ver121 com a chave do nome", want.Container)
	}

	data, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(t.TempDir(), "Renomeado.utx")
	if err := os.WriteFile(renamed, data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := l2pkg.Open(renamed)
	if err != nil {
		t.Fatalf("renomeado: %v", err)
	}
	if !got.Container.Recovered || got.Container.Key != want.Container.Key {
		t.Errorf("renomeado: container %v, quer a chave 0x%02x recuperada", got.Container, want.Container.Key)
	}
	if len(got.Exports) != len(want.Exports) || len(got.Names) != len(want.Names) {
		t.Errorf("renomeado: %d nomes/%d exports, quer %d/%d", len(got.Names), len(got.Exports), len(want.Names), len(want.Exports))
	}
}

// Export counts of one map, one .utx and one .usx, as UE2-Studio's
// package-engine reports them (Archive::parse over decrypt, run on the
// Fafurion client on 2026-10-08). Catches a shifted table read.
func TestExportCountsMatchUE2Studio(t *testing.T) {
	root := clientRoot(t)
	for _, c := range []struct {
		path                    string
		names, imports, exports int
	}{
		{"Maps/22_22.unr", 11779, 1040, 10186},
		{"Textures/Giran_Village_T.utx", 247, 8, 213},
		{"StaticMeshes/Giran_Village_S.usx", 344, 223, 109},
	} {
		pkg, err := l2pkg.Open(filepath.Join(root, c.path))
		if err != nil {
			t.Errorf("%s: %v", c.path, err)
			continue
		}
		if len(pkg.Names) != c.names || len(pkg.Imports) != c.imports || len(pkg.Exports) != c.exports {
			t.Errorf("%s: %d nomes, %d imports, %d exports; UE2-Studio: %d, %d, %d",
				c.path, len(pkg.Names), len(pkg.Imports), len(pkg.Exports), c.names, c.imports, c.exports)
		}
	}
}
