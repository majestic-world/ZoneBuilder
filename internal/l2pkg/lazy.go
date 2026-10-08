package l2pkg

import "fmt"

// LazyBytes reads a TLazyArray<BYTE> (a texture mip) without copying it:
// from file version 62 an absolute end offset, then a compact byte count and
// the bytes. The end offset is checked against where the bytes end, which
// catches anything in front of the array read out of step. Port of
// UE2-Studio's reader.rs read_lazy_bytes.
func (r *Reader) LazyBytes(fileVersion uint16) []byte {
	if fileVersion >= 266 {
		r.Fail(fmt.Errorf("TLazyArray com bulk data do UE3 (versão %d)", fileVersion))
		return nil
	}
	end := -1
	if fileVersion > 61 {
		end = int(r.I32())
		if end < 0 && r.Err() == nil {
			r.Fail(fmt.Errorf("TLazyArray com offset final negativo %d", end))
		}
		if fileVersion >= 251 {
			r.U32() // lazy loader flags
		}
		if fileVersion >= 254 {
			r.I32() // storage size
		}
		if fileVersion >= 260 {
			r.Index() // package name
		}
	}
	count := r.Index()
	if count < 0 {
		r.Fail(fmt.Errorf("TLazyArray com quantidade negativa %d", count))
		return nil
	}
	b := r.Bytes(int(count))
	if end >= 0 && r.Err() == nil && r.Pos != end {
		r.Fail(fmt.Errorf("TLazyArray termina em %d, mas declara o fim em %d", r.Pos, end))
	}
	return b
}

// SkipMaterialData skips the native UMaterial/UBitmapMaterial fields between
// a texture's property list and its mip chain. The layout depends on the
// package's file and licensee versions; every branch is UE2-Studio's
// props.rs skip_material_data, which was measured on real client packages.
func SkipMaterialData(r *Reader, fileVersion, licenseeVersion uint16) {
	fv, lv := int(fileVersion), int(licenseeVersion)
	if fv < 123 {
		return
	}
	if lv >= 16 && lv < 37 {
		r.U32()
	}
	if lv >= 30 && lv < 37 {
		if lv >= 33 && lv < 36 {
			r.U8()
		}
		r.Skip(6)
		r.Skip(3 * 4)
		for range 8 {
			r.U8()
			if lv < 36 {
				r.U8()
			}
			r.Skip(126)
		}
		r.Skip(8)
		r.Skip(3 * 4)
		for range 17 {
			r.SkipString()
		}
	}
	if lv >= 37 {
		r.Skip(2)
		if fv < 129 {
			r.Skip(2 + 12)
		} else {
			r.Skip(5 * (6 + 8))
		}
		r.Skip(8 + 12)
		stages := r.Index()
		if stages < 0 || stages > maxTableCount {
			r.Fail(fmt.Errorf("material com quantidade de estágios inválida: %d", stages))
			return
		}
		for range stages {
			r.SkipString()
			values := r.Index()
			if values < 0 || values > maxTableCount {
				r.Fail(fmt.Errorf("estágio de material com quantidade de textos inválida: %d", values))
				return
			}
			for range values {
				r.SkipString()
			}
		}
		r.SkipString()
	}
	if lv >= 31 {
		r.U16()
		r.U16()
	}
}
