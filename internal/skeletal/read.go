// Package skeletal reads a SkeletalMesh and a MeshAnimation out of a Lineage
// II package of file version 132, licensee 40: the reference skeleton, LOD 0
// assembled into one skinned vertex per wedge, and the key tracks of each
// animation sequence. It is a port of the parts of UE2-Studio's skeletal.rs,
// skin.rs, meshanim.rs and pose.rs that this one layout exercises; other
// versions, writing, and PSK/PSA are out of scope. Only cmd/zbmodel uses it:
// the pose math lives in internal/model.
package skeletal

import (
	"fmt"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
)

// The one package layout this reader knows. Every version gate of the Rust
// reader is resolved for it in place, with the gate named beside the field.
const (
	fileVersion     = 132
	licenseeVersion = 40
)

// maxCount bounds every serialized array count, as reader.rs read_array does.
const maxCount = 10_000_000

func checkVersion(p *l2pkg.Package) error {
	if p.Header.FileVersion != fileVersion || p.Header.LicenseeVersion != licenseeVersion {
		return fmt.Errorf("pacote %s na versão %d/%d: só a %d/%d é suportada",
			p.Name, p.Header.FileVersion, p.Header.LicenseeVersion, fileVersion, licenseeVersion)
	}
	return nil
}

// count reads a compact-index array count.
func count(r *l2pkg.Reader, what string) int {
	n := r.Index()
	if r.Err() == nil && (n < 0 || n > maxCount) {
		r.Fail(fmt.Errorf("contagem inválida de %s: %d", what, n))
	}
	if r.Err() != nil {
		return 0
	}
	return int(n)
}

// skipArray walks a TArray of fixed-size elements.
func skipArray(r *l2pkg.Reader, what string, size int) int {
	n := count(r, what)
	r.Skip(n * size)
	return n
}

// lazyArray reads a TLazyArray (file version > 61): an absolute end offset, a
// count, and the elements, which must end exactly at that offset. each is
// called once per element.
func lazyArray(r *l2pkg.Reader, what string, each func()) {
	end := int(r.I32())
	n := count(r, what)
	for range n {
		if r.Err() != nil {
			return
		}
		each()
	}
	if r.Err() == nil && r.Pos != end {
		r.Fail(fmt.Errorf("TLazyArray de %s termina em %d, mas declara o fim em %d", what, r.Pos, end))
	}
}

func vector(r *l2pkg.Reader) geom.Vec3 {
	return geom.Vec3{X: r.F32(), Y: r.F32(), Z: r.F32()}
}

// name reads an FName: a compact index into the name table.
func name(p *l2pkg.Package, r *l2pkg.Reader) string {
	i := r.Index()
	if r.Err() != nil {
		return ""
	}
	if i < 0 || int(i) >= len(p.Names) {
		r.Fail(fmt.Errorf("nome %d fora de uma tabela de %d", i, len(p.Names)))
		return ""
	}
	return p.Names[i]
}

// checkEnd fails unless r stopped exactly at the end of export i: the
// whole-layout check that every field before it was read in step.
func checkEnd(p *l2pkg.Package, r *l2pkg.Reader, i int) {
	ex := &p.Exports[i]
	end := int(ex.SerialOffset) + int(ex.SerialSize)
	if r.Err() == nil && r.Pos != end {
		r.Fail(fmt.Errorf("a leitura parou em %d, mas o corpo termina em %d", r.Pos, end))
	}
}
