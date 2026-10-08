package l2pkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// assetDirectories is where a package is looked up by name under the client
// root, in resolution order: the first <dir>/<name>.<ext> that exists wins
// (UE2-Studio browse.rs ASSET_DIRECTORIES).
var assetDirectories = [...]struct{ dir, ext string }{
	{"Maps", "unr"},
	{"StaticMeshes", "usx"},
	{"Textures", "utx"},
	{"SysTextures", "utx"},
	{"Animations", "ukx"},
	{"Animations", "uix"},
	{"Sounds", "uax"},
	{"Voice", "uax"},
	{"system_en", "u"},
	{"system", "u"},
}

// MissingError reports a package the client does not ship, or an import
// whose object its package does not export. Callers that can go on without
// the object (a map without its terrain package) check for it with
// errors.As and warn instead of failing.
type MissingError struct {
	Package string
	Object  string // empty when the package itself is missing
}

func (e *MissingError) Error() string {
	if e.Object == "" {
		return "pacote não encontrado: " + e.Package
	}
	return fmt.Sprintf("objeto não encontrado: %s.%s", e.Package, e.Object)
}

// IsMissing reports whether err is, or wraps, a *MissingError.
func IsMissing(err error) bool {
	var m *MissingError
	return errors.As(err, &m)
}

// Client opens the packages of one client directory by name, the way
// packages name each other, and caches every package it opened (and every
// name it failed to find). It is safe for concurrent use.
type Client struct {
	// Root is the client directory, the folder above Maps.
	Root string

	mu       sync.Mutex
	packages map[string]*Package // keyed by lower-case name
	absent   map[string]bool
}

// NewClient returns a Client over the client directory root.
func NewClient(root string) *Client {
	return &Client{Root: root, packages: map[string]*Package{}, absent: map[string]bool{}}
}

// Path is the file the package called name resolves to, or "" when the
// client has none.
func (c *Client) Path(name string) string {
	for _, d := range assetDirectories {
		p := filepath.Join(c.Root, d.dir, name+"."+d.ext)
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

// Package opens the package called name (file name without extension),
// from the cache when it was opened before. A package the client does not
// ship is a *MissingError.
func (c *Client) Package(name string) (*Package, error) {
	key := strings.ToLower(name)
	c.mu.Lock()
	defer c.mu.Unlock()
	if p, ok := c.packages[key]; ok {
		return p, nil
	}
	if c.absent[key] {
		return nil, &MissingError{Package: name}
	}
	path := c.Path(name)
	if path == "" {
		c.absent[key] = true
		return nil, &MissingError{Package: name}
	}
	p, err := Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c.packages[key] = p
	return p, nil
}

// Resolve finds the object an object reference of p points at: the package
// that serializes it and its 0-based export index there. A positive ref is
// an export of p itself. A negative one is an import, whose owner chain is
// walked to its root package, which is opened by name and searched with
// ExportNamed. Object references inside the resolved export belong to the
// returned package, not to p. Port of UE2-Studio archive.rs
// resolve_reference.
func (c *Client) Resolve(p *Package, ref int32) (*Package, int, error) {
	switch {
	case ref == 0:
		return nil, 0, errors.New("referência nula")
	case ref > 0:
		if int(ref) > len(p.Exports) {
			return nil, 0, fmt.Errorf("%s: referência ao export %d fora de uma tabela de %d", p.Name, ref, len(p.Exports))
		}
		return p, int(ref - 1), nil
	}
	im, err := p.importAt(ref)
	if err != nil {
		return nil, 0, err
	}
	root := im
	for depth := 0; root.PackageIndex != 0; depth++ {
		if depth == maxChainDepth {
			return nil, 0, fmt.Errorf("%s: cadeia de imports não termina após %d níveis", p.Name, maxChainDepth)
		}
		if root, err = p.importAt(root.PackageIndex); err != nil {
			return nil, 0, err
		}
	}
	owner, err := c.Package(root.ObjectName)
	if err != nil {
		return nil, 0, err
	}
	i := owner.ExportNamed(im.ObjectName, im.ClassName)
	if i < 0 {
		return nil, 0, &MissingError{Package: root.ObjectName, Object: im.ObjectName}
	}
	return owner, i, nil
}

func (p *Package) importAt(ref int32) (*Import, error) {
	if ref >= 0 || int(-ref) > len(p.Imports) {
		return nil, fmt.Errorf("%s: referência ao import %d fora de uma tabela de %d", p.Name, -ref, len(p.Imports))
	}
	return &p.Imports[-ref-1], nil
}

// ExportNamed is the 0-based index of the first export whose object and
// class names match, ignoring ASCII case and skipping Package (group)
// exports, or -1. This is how an import names its object: groups are not
// consulted.
func (p *Package) ExportNamed(object, class string) int {
	p.exportsByNameOnce.Do(func() {
		p.exportsByName = make(map[string][]int, len(p.Exports))
		for i := range p.Exports {
			ex := &p.Exports[i]
			if strings.EqualFold(ex.ClassName, "Package") {
				continue
			}
			key := strings.ToLower(ex.ObjectName)
			p.exportsByName[key] = append(p.exportsByName[key], i)
		}
	})
	for _, i := range p.exportsByName[strings.ToLower(object)] {
		if strings.EqualFold(p.Exports[i].ClassName, class) {
			return i
		}
	}
	return -1
}
