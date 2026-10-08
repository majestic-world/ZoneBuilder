package l2pkg

import (
	"bytes"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"path/filepath"
)

// PackageMagic is the first u32 of a raw Unreal package (bytes C1 83 2A 9E).
const PackageMagic uint32 = 0x9e2a83c1

// containerHeaderLen is the "Lineage2Ver<NNN>" prologue: 14 UTF-16LE code
// units. Every offset inside the package counts from the byte after it.
const containerHeaderLen = 28

var containerPrefix = []byte("L\x00i\x00n\x00e\x00a\x00g\x00e\x002\x00V\x00e\x00r\x00")

// Container is what wraps a package on disk.
type Container struct {
	// Version is 0 for a raw Unreal package, else the NNN of the
	// Lineage2VerNNN prologue (111 or 121).
	Version int
	// Key is the single-byte XOR key the body is encrypted with; 0 when raw.
	Key byte
	// Recovered is set when a Ver121 key did not come from the file name
	// but from the encrypted magic: the package was renamed after it was
	// written.
	Recovered bool
}

func (c Container) String() string {
	switch {
	case c.Version == 0:
		return "cru"
	case c.Recovered:
		return fmt.Sprintf("Lineage2Ver%03d (chave 0x%02x recuperada pelo magic)", c.Version, c.Key)
	default:
		return fmt.Sprintf("Lineage2Ver%03d (chave 0x%02x)", c.Version, c.Key)
	}
}

// UnsupportedContainerError reports a Lineage2Ver container this reader has
// no cipher for: 120, 211/212 (Blowfish), 411-414 (RSA + zlib), or any other.
type UnsupportedContainerError struct {
	Version int
}

func (e *UnsupportedContainerError) Error() string {
	return fmt.Sprintf("versão de container não suportada: Lineage2Ver%03d", e.Version)
}

// decrypt strips the container from a file's bytes, decrypting in place, and
// returns the package plaintext (a subslice of data).
//
// name is the file name the package was found under; a Ver121 key is
// derived from it. A Ver121 package renamed after it was written still
// carries the key of its old name, which the known plaintext magic reveals.
func decrypt(name string, data []byte) ([]byte, Container, error) {
	if len(data) >= 4 && binary.LittleEndian.Uint32(data) == PackageMagic {
		return data, Container{}, nil
	}
	if len(data) < containerHeaderLen || !bytes.Equal(data[:len(containerPrefix)], containerPrefix) {
		return nil, Container{}, fmt.Errorf("não é um pacote Unreal cru nem um container Lineage2Ver (assinatura %s)", signature(data))
	}
	version := 0
	for _, slot := range []int{22, 24, 26} {
		digit := data[slot]
		if digit < '0' || digit > '9' || data[slot+1] != 0 {
			return nil, Container{}, fmt.Errorf("versão de container ilegível (assinatura %s)", signature(data))
		}
		version = version*10 + int(digit-'0')
	}
	encrypted := data[containerHeaderLen:]
	c := Container{Version: version}
	switch version {
	case 111:
		c.Key = 0xac
	case 121:
		c.Key = fileNameKey(name)
		if !decryptsMagic(encrypted, c.Key) {
			if len(encrypted) == 0 {
				return nil, c, fmt.Errorf("container Lineage2Ver121 sem conteúdo")
			}
			key := encrypted[0] ^ byte(PackageMagic&0xff)
			if !decryptsMagic(encrypted, key) {
				return nil, c, fmt.Errorf("a chave Lineage2Ver121 do nome do arquivo não revela um pacote Unreal, nem a chave recuperada pelo magic")
			}
			c.Key, c.Recovered = key, true
		}
	default:
		return nil, c, &UnsupportedContainerError{Version: version}
	}
	xorInPlace(encrypted, c.Key)
	return encrypted, c, nil
}

// fileNameKey is the Ver121 key: the low byte of the sum of the ASCII-
// lowercased file name, extension included.
func fileNameKey(name string) byte {
	var sum byte
	for _, c := range []byte(filepath.Base(name)) {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		sum += c
	}
	return sum
}

func decryptsMagic(encrypted []byte, key byte) bool {
	if len(encrypted) < 4 {
		return false
	}
	var plain [4]byte
	for i := range plain {
		plain[i] = encrypted[i] ^ key
	}
	return binary.LittleEndian.Uint32(plain[:]) == PackageMagic
}

// xorInPlace XORs every byte of data with key, a block at a time.
func xorInPlace(data []byte, key byte) {
	var block [64 << 10]byte
	for i := range block {
		block[i] = key
	}
	for len(data) > 0 {
		n := subtle.XORBytes(data, data, block[:min(len(data), len(block))])
		data = data[n:]
	}
}

// signature is a printable form of a file's first 8 bytes, so an unknown
// container is named in a way a user can act on.
func signature(data []byte) string {
	return escapeANSI(data[:min(len(data), 8)])
}
