package l2pkg

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
)

// sweptExtensions are the package kinds a client sweep opens.
var sweptExtensions = []string{".unr", ".utx", ".usx"}

// SweepResult is the outcome of opening one package of a client sweep.
type SweepResult struct {
	// Path is the package file, relative to the swept root.
	Path string
	// Package is the parsed package, nil when Err is set.
	Package *Package
	Err     error
}

// Sweep opens and verifies every .unr, .utx and .usx under root, in any
// subfolder, calling visit once per package in path order. visit runs on the
// calling goroutine, one result at a time; a package's bytes are released as
// soon as visit returns and drops it.
//
// The walk itself failing (an unreadable folder) is the only returned error;
// a package that does not open is a result, not an error.
func Sweep(root string, visit func(SweepResult)) error {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && slices.Contains(sweptExtensions, strings.ToLower(filepath.Ext(path))) {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		return err
	}
	slices.Sort(paths)

	// Results are delivered in path order through one slot per package,
	// with at most `workers` packages open at once: a package can run to
	// hundreds of MiB, so few are held.
	workers := min(runtime.GOMAXPROCS(0), 4)
	slots := make([]chan SweepResult, len(paths))
	for i := range slots {
		slots[i] = make(chan SweepResult, 1)
	}
	tokens := make(chan struct{}, workers)
	var wg sync.WaitGroup
	go func() {
		for i, rel := range paths {
			tokens <- struct{}{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				pkg, err := Open(filepath.Join(root, rel))
				if err == nil {
					err = pkg.Verify()
				}
				if err != nil {
					pkg = nil
				}
				slots[i] <- SweepResult{Path: rel, Package: pkg, Err: err}
			}()
		}
	}()
	for _, slot := range slots {
		visit(<-slot)
		<-tokens
	}
	wg.Wait()
	return nil
}

// Verify checks what Open leaves to the first reader of each export: that
// every export's body lies inside the package and that every object
// reference in the tables names an existing row.
func (p *Package) Verify() error {
	for i := range p.Exports {
		if _, err := p.ExportReader(i); err != nil {
			return err
		}
		ex := &p.Exports[i]
		for _, ref := range []int32{ex.SuperIndex, ex.PackageIndex} {
			if _, err := p.ObjectPath(ref); err != nil {
				return fmt.Errorf("export %d (%s): %w", i+1, ex.ObjectName, err)
			}
		}
	}
	for i := range p.Imports {
		im := &p.Imports[i]
		if _, err := p.ObjectPath(im.PackageIndex); err != nil {
			return fmt.Errorf("import %d (%s): %w", i+1, im.ObjectName, err)
		}
	}
	return nil
}
