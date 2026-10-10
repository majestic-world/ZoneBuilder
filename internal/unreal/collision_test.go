package unreal

import (
	"os"
	"testing"

	"zonebuilder/internal/l2pkg"
)

// TestCollisionFlagsOfATileFollowTheClassDefaults reads the mesh actors of
// tile 22_22 from the real client named by ZB_CLIENT (skipped without it).
// Bugs it catches: a flag the actor does not serialize read as false
// instead of its class default (every plain StaticMeshActor would stop
// blocking); defaults taken from the wrong class in the chain (Mover
// inherits no bWorldGeometry, StaticMeshActor declares it, as in stock
// UE2 script); a serialized flag losing to the class default (the meshes
// saved with bBlockNonZeroExtentTraces=False would block).
func TestCollisionFlagsOfATileFollowTheClassDefaults(t *testing.T) {
	root := os.Getenv("ZB_CLIENT")
	if root == "" {
		t.Skip("ZB_CLIENT not set: tests against the real client skipped")
	}
	c := l2pkg.NewClient(root)
	m, err := c.Package("22_22")
	if err != nil {
		t.Fatal(err)
	}
	d := NewClassDefaults(c)
	all := Collision{true, true, true, true, true}
	sma, err := d.Collision("Engine", "StaticMeshActor")
	if err != nil {
		t.Fatal(err)
	}
	if sma != all {
		t.Fatalf("Engine.StaticMeshActor defaults = %+v, want every flag set", sma)
	}
	mover, err := d.Collision("Engine", "Mover")
	if err != nil {
		t.Fatal(err)
	}
	if want := (Collision{true, true, true, false, true}); mover != want {
		t.Fatalf("Engine.Mover defaults = %+v, want %+v", mover, want)
	}

	var silentBlocks, savedOff int
	for i := range m.Exports {
		if m.Exports[i].ClassName != "StaticMeshActor" {
			continue
		}
		a, err := ReadActor(m, i)
		if err != nil {
			t.Fatal(err)
		}
		pkg, class, err := ActorClass(m, i)
		if err != nil {
			t.Fatal(err)
		}
		def, err := d.Collision(pkg, class)
		if err != nil {
			t.Fatal(err)
		}
		got := a.Collision.Over(def)
		switch {
		case a.Collision.Set == Collision{}:
			if got != sma || !got.Blocks() {
				t.Fatalf("%s carries no flag: got %+v, want the class default %+v", m.Exports[i].ObjectName, got, sma)
			}
			silentBlocks++
		case a.Collision.Set.BlockNonZeroExtentTraces && !a.Collision.Value.BlockNonZeroExtentTraces:
			if got.Blocks() || got.BlockNonZeroExtentTraces {
				t.Fatalf("%s saves bBlockNonZeroExtentTraces=False: got %+v, blocks", m.Exports[i].ObjectName, got)
			}
			savedOff++
		}
	}
	if silentBlocks == 0 || savedOff == 0 {
		t.Fatalf("tile 22_22: %d blocking actors on class defaults, %d saved as not blocking; want both > 0", silentBlocks, savedOff)
	}
}
