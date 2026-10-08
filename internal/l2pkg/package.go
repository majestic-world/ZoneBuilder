// Package l2pkg reads Lineage II Unreal Engine 2 packages (.unr, .utx, .usx,
// .u): the Lineage2Ver container, the package header, and the name, import
// and export tables. It is a Go port of the read path of UE2-Studio's
// package-engine crate (archive.rs, reader.rs, patch.rs split_container).
package l2pkg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxTableCount rejects the absurd element counts a malformed package
// produces before they become a huge allocation (UE2-Studio check_count).
const maxTableCount = 10_000_000

// maxChainDepth bounds an owner chain walk, so a table that points in a
// circle is refused instead of walked forever.
const maxChainDepth = 32

// Header is the package file summary.
type Header struct {
	// FileVersion and LicenseeVersion are the ArVer/LicenseeVer pair that
	// gates object layouts. A Lineage II client mixes many pairs; the
	// measured range is 117/0 to 133/40.
	FileVersion     uint16
	LicenseeVersion uint16
	Flags           uint32
	NameCount       int32
	NameOffset      int32
	ExportCount     int32
	ExportOffset    int32
	ImportCount     int32
	ImportOffset    int32
	GUID            [16]byte
	Generations     []Generation
}

// Generation is one FGenerationInfo record of the header.
type Generation struct {
	ExportCount int32
	NameCount   int32
}

// Import is one row of the import table: an object of another package.
type Import struct {
	ClassPackage string
	ClassName    string
	// PackageIndex is the import's owner: negative into the import table,
	// zero when the import is itself a root package (a file name).
	PackageIndex int32
	ObjectName   string
}

// Export is one row of the export table: an object serialized in this
// package.
type Export struct {
	// ClassIndex is the class reference: negative into the import table,
	// positive into the export table, zero for a class export (whose class
	// is Core.Class, left implicit).
	ClassIndex int32
	// ClassName is the object name ClassIndex resolves to, "None" for zero.
	ClassName  string
	SuperIndex int32
	// PackageIndex is the export's owner (a group, or another object) in the
	// same index space as ClassIndex; zero at the package root.
	PackageIndex int32
	ObjectName   string
	Flags        uint32
	SerialSize   int32
	SerialOffset int32
}

// Package is one parsed package. Data is the whole decrypted plaintext;
// every offset in the header and tables is relative to it.
type Package struct {
	// Name is the file name without extension, as packages name each other.
	Name      string
	Container Container
	Header    Header
	Names     []string
	Imports   []Import
	Exports   []Export
	Data      []byte
}

// Open reads, decrypts and parses the package file at path. Errors carry the
// reason only; callers add the path.
func Open(path string) (*Package, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	plain, container, err := decrypt(filepath.Base(path), data)
	if err != nil {
		return nil, err
	}
	base := filepath.Base(path)
	pkg, err := Parse(strings.TrimSuffix(base, filepath.Ext(base)), plain)
	if err != nil {
		return nil, err
	}
	pkg.Container = container
	return pkg, nil
}

// Parse parses decrypted package plaintext.
func Parse(name string, data []byte) (*Package, error) {
	r := NewReader(data, 0)
	if magic := r.U32(); r.Err() == nil && magic != PackageMagic {
		return nil, fmt.Errorf("magic de pacote Unreal inválido: 0x%08x", magic)
	}
	var h Header
	h.FileVersion = r.U16()
	h.LicenseeVersion = r.U16()
	h.Flags = r.U32()
	h.NameCount = r.I32()
	h.NameOffset = r.I32()
	h.ExportCount = r.I32()
	h.ExportOffset = r.I32()
	h.ImportCount = r.I32()
	h.ImportOffset = r.I32()
	copy(h.GUID[:], r.Bytes(16))
	generations := r.I32()
	if err := r.Err(); err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	if generations < 0 || int64(generations)*8 > int64(len(data)) {
		return nil, fmt.Errorf("header: quantidade de gerações inválida: %d", generations)
	}
	h.Generations = make([]Generation, generations)
	for i := range h.Generations {
		h.Generations[i] = Generation{ExportCount: r.I32(), NameCount: r.I32()}
	}
	if err := r.Err(); err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	for _, t := range []struct {
		label         string
		count, offset int32
	}{
		{"nomes", h.NameCount, h.NameOffset},
		{"imports", h.ImportCount, h.ImportOffset},
		{"exports", h.ExportCount, h.ExportOffset},
	} {
		if t.count < 0 || t.count > maxTableCount {
			return nil, fmt.Errorf("tabela de %s: quantidade inválida: %d", t.label, t.count)
		}
		if t.offset < 0 || int64(t.offset) > int64(len(data)) {
			return nil, fmt.Errorf("tabela de %s: offset %d fora do pacote (%d bytes)", t.label, t.offset, len(data))
		}
	}

	p := &Package{Name: name, Header: h, Data: data}

	r = NewReader(data, int(h.NameOffset))
	p.Names = make([]string, h.NameCount)
	for i := range p.Names {
		p.Names[i] = r.String()
		r.U32() // object flags of the name
		if err := r.Err(); err != nil {
			return nil, fmt.Errorf("tabela de nomes, nome %d: %w", i, err)
		}
	}

	r = NewReader(data, int(h.ImportOffset))
	p.Imports = make([]Import, h.ImportCount)
	for i := range p.Imports {
		im := &p.Imports[i]
		classPackage, classPackageErr := p.name(r.Index())
		className, classNameErr := p.name(r.Index())
		im.PackageIndex = r.I32()
		objectName, objectNameErr := p.name(r.Index())
		if err := firstError(r.Err(), classPackageErr, classNameErr, objectNameErr); err != nil {
			return nil, fmt.Errorf("tabela de imports, import %d: %w", i, err)
		}
		im.ClassPackage, im.ClassName, im.ObjectName = classPackage, className, objectName
	}

	r = NewReader(data, int(h.ExportOffset))
	p.Exports = make([]Export, h.ExportCount)
	for i := range p.Exports {
		ex := &p.Exports[i]
		ex.ClassIndex = r.Index()
		ex.SuperIndex = r.Index()
		ex.PackageIndex = r.I32()
		objectName, objectNameErr := p.name(r.Index())
		ex.Flags = r.U32()
		ex.SerialSize = r.Index()
		if ex.SerialSize > 0 {
			ex.SerialOffset = r.Index()
		}
		if err := firstError(r.Err(), objectNameErr); err != nil {
			return nil, fmt.Errorf("tabela de exports, export %d: %w", i+1, err)
		}
		ex.ObjectName = objectName
		className, err := p.classNameOf(ex.ClassIndex, i)
		if err != nil {
			return nil, fmt.Errorf("tabela de exports, export %d (%s): %w", i+1, objectName, err)
		}
		ex.ClassName = className
	}
	return p, nil
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Package) name(index int32) (string, error) {
	if index < 0 || int(index) >= len(p.Names) {
		return "", fmt.Errorf("índice de nome %d fora de uma tabela de %d nomes", index, len(p.Names))
	}
	return p.Names[index], nil
}

// classNameOf resolves a class reference while the export table is still
// being read: only the first parsed exports may be named.
func (p *Package) classNameOf(ref int32, parsed int) (string, error) {
	switch {
	case ref < 0:
		if int(-ref-1) >= len(p.Imports) {
			return "", fmt.Errorf("classe aponta para o import %d, fora de uma tabela de %d", -ref, len(p.Imports))
		}
		return p.Imports[-ref-1].ObjectName, nil
	case ref > 0:
		if int(ref-1) >= parsed {
			return "", fmt.Errorf("classe aponta para o export %d, que ainda não foi lido", ref)
		}
		return p.Exports[ref-1].ObjectName, nil
	}
	return "None", nil
}

// ObjectName is the object name an object reference points at: negative
// into the import table, positive into the export table, "None" for zero.
func (p *Package) ObjectName(ref int32) (string, error) {
	switch {
	case ref < 0:
		if int(-ref-1) >= len(p.Imports) {
			return "", fmt.Errorf("referência ao import %d fora de uma tabela de %d", -ref, len(p.Imports))
		}
		return p.Imports[-ref-1].ObjectName, nil
	case ref > 0:
		if int(ref-1) >= len(p.Exports) {
			return "", fmt.Errorf("referência ao export %d fora de uma tabela de %d", ref, len(p.Exports))
		}
		return p.Exports[ref-1].ObjectName, nil
	}
	return "None", nil
}

// ObjectPath is the full dotted path of an object reference, outermost first:
// "Textures.Wind.g_01" for an import from Textures.utx, "<Name>.Group.Obj"
// for an export of this package, "" for the null reference.
func (p *Package) ObjectPath(ref int32) (string, error) {
	var segments []string
	for depth := 0; ref != 0; depth++ {
		if depth == maxChainDepth {
			return "", fmt.Errorf("cadeia de donos não termina após %d níveis", maxChainDepth)
		}
		name, err := p.ObjectName(ref)
		if err != nil {
			return "", err
		}
		segments = append(segments, name)
		if ref > 0 {
			ref = p.Exports[ref-1].PackageIndex
			if ref == 0 {
				segments = append(segments, p.Name)
			}
		} else {
			ref = p.Imports[-ref-1].PackageIndex
		}
	}
	for i, j := 0, len(segments)-1; i < j; i, j = i+1, j-1 {
		segments[i], segments[j] = segments[j], segments[i]
	}
	return strings.Join(segments, "."), nil
}

// ExportReader returns a reader over the serialized body of export i (0-based
// into Exports), positioned at its absolute serial offset and bounded at its
// end. An export with no body yields an empty reader at offset 0.
func (p *Package) ExportReader(i int) (*Reader, error) {
	if i < 0 || i >= len(p.Exports) {
		return nil, fmt.Errorf("export %d fora de uma tabela de %d", i+1, len(p.Exports))
	}
	ex := &p.Exports[i]
	if ex.SerialSize <= 0 {
		return NewReader(nil, 0), nil
	}
	start, end := int64(ex.SerialOffset), int64(ex.SerialOffset)+int64(ex.SerialSize)
	if start < 0 || end > int64(len(p.Data)) {
		return nil, fmt.Errorf("export %d (%s.%s): corpo [%d, %d) fora do pacote (%d bytes)",
			i+1, ex.ClassName, ex.ObjectName, start, end, len(p.Data))
	}
	return NewReader(p.Data[:end], int(start)), nil
}
