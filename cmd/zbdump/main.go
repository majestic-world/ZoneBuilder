// Command zbdump is the package diagnostic CLI.
//
//	zbdump <pacote>          lists the package's names, imports and exports
//	zbdump <pasta do cliente> opens every .unr, .utx and .usx below the folder
//	                         and reports each failure with its reason
package main

import (
	"bufio"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"

	"zonebuilder/internal/l2pkg"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "uso: zbdump <pacote .unr/.utx/.usx/.u> | <pasta do cliente>")
		os.Exit(2)
	}
	target := os.Args[1]
	info, err := os.Stat(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	if info.IsDir() {
		if failures := sweep(out, target); failures > 0 {
			out.Flush()
			os.Exit(1)
		}
		return
	}
	pkg, err := l2pkg.Open(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", target, err)
		os.Exit(1)
	}
	dump(out, pkg)
}

func dump(w io.Writer, p *l2pkg.Package) {
	h := &p.Header
	fmt.Fprintf(w, "pacote %s\n", p.Name)
	fmt.Fprintf(w, "container %s\n", p.Container)
	fmt.Fprintf(w, "versão %d/%d (ArVer/licensee), flags 0x%08x, %d %s\n",
		h.FileVersion, h.LicenseeVersion, h.Flags, len(h.Generations), plural(len(h.Generations), "geração", "gerações"))

	fmt.Fprintf(w, "\n%d %s\n", len(p.Names), plural(len(p.Names), "nome", "nomes"))
	for i, name := range p.Names {
		fmt.Fprintf(w, "%6d  %s\n", i, name)
	}

	fmt.Fprintf(w, "\n%d %s\n", len(p.Imports), plural(len(p.Imports), "import", "imports"))
	fmt.Fprintf(w, "%6s  %-24s  %-40s  %s\n", "índice", "classe", "pacote pai", "nome")
	for i, im := range p.Imports {
		fmt.Fprintf(w, "%6d  %-24s  %-40s  %s\n", -(i + 1), im.ClassPackage+"."+im.ClassName, parent(p, im.PackageIndex, ""), im.ObjectName)
	}

	fmt.Fprintf(w, "\n%d %s\n", len(p.Exports), plural(len(p.Exports), "export", "exports"))
	fmt.Fprintf(w, "%6s  %-24s  %-40s  %10s  %10s  %s\n", "índice", "classe", "pacote pai", "tamanho", "offset", "nome")
	for i, ex := range p.Exports {
		fmt.Fprintf(w, "%6d  %-24s  %-40s  %10d  %10d  %s\n", i+1, ex.ClassName, parent(p, ex.PackageIndex, p.Name), ex.SerialSize, ex.SerialOffset, ex.ObjectName)
	}
}

// parent spells an owner reference as a dotted path; root is what a zero
// owner means for the row (the package itself for an export, nothing for a
// root import).
func parent(p *l2pkg.Package, ref int32, root string) string {
	if ref == 0 {
		if root == "" {
			return "-"
		}
		return root
	}
	path, err := p.ObjectPath(ref)
	if err != nil {
		return fmt.Sprintf("<%v>", err)
	}
	return path
}

func sweep(w io.Writer, root string) int {
	start := time.Now()
	var opened, failures int
	containers := map[string]int{}
	pairs := map[string]int{}
	err := l2pkg.Sweep(root, func(r l2pkg.SweepResult) {
		if r.Err != nil {
			failures++
			fmt.Fprintf(w, "FALHA %s: %v\n", r.Path, r.Err)
			return
		}
		opened++
		folder := filepath.Dir(r.Path)
		containers[fmt.Sprintf("%-14s %s", folder, containerName(r.Package.Container))]++
		pairs[fmt.Sprintf("%-14s %d/%d", folder, r.Package.Header.FileVersion, r.Package.Header.LicenseeVersion)]++
	})
	if err != nil {
		fmt.Fprintf(w, "FALHA na varredura de %s: %v\n", root, err)
		return failures + 1
	}
	total := opened + failures
	fmt.Fprintf(w, "\ncontainers por pasta:\n")
	histogram(w, containers)
	fmt.Fprintf(w, "\npares ArVer/licensee por pasta:\n")
	histogram(w, pairs)
	fmt.Fprintf(w, "\n%d %s, %d %s, %s\n",
		total, plural(total, "pacote", "pacotes"),
		failures, plural(failures, "falha", "falhas"),
		time.Since(start).Round(time.Millisecond))
	return failures
}

func containerName(c l2pkg.Container) string {
	if c.Version == 0 {
		return "cru"
	}
	name := fmt.Sprintf("Lineage2Ver%03d", c.Version)
	if c.Recovered {
		name += " (chave recuperada)"
	}
	return name
}

func histogram(w io.Writer, counts map[string]int) {
	for _, key := range slices.Sorted(maps.Keys(counts)) {
		fmt.Fprintf(w, "  %s  %d\n", key, counts[key])
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
