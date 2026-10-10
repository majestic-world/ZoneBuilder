package unreal

import (
	"fmt"
	"strings"
	"sync"

	"zonebuilder/internal/l2pkg"
)

// Collision is the part of an actor's collision settings that decides
// whether it stops a walking player. Port of the flags UE2-Studio's
// Scene::play_collision reads (src/scene/play_map.rs).
type Collision struct {
	CollideActors, BlockActors, BlockPlayers, WorldGeometry, BlockNonZeroExtentTraces bool
}

// collisionNames are the property names of Collision's fields, in order.
var collisionNames = [...]string{"bCollideActors", "bBlockActors", "bBlockPlayers", "bWorldGeometry", "bBlockNonZeroExtentTraces"}

func (c *Collision) fields() [5]*bool {
	return [5]*bool{&c.CollideActors, &c.BlockActors, &c.BlockPlayers, &c.WorldGeometry, &c.BlockNonZeroExtentTraces}
}

// Blocks reports whether a player capsule collides with the actor:
// it collides with actors, blocks extent traces, and blocks actors,
// players or counts as world geometry (UE2-Studio play_collision).
func (c Collision) Blocks() bool {
	return c.CollideActors && c.BlockNonZeroExtentTraces && (c.BlockActors || c.BlockPlayers || c.WorldGeometry)
}

// CollisionSettings is what an actor's own property list says about
// Collision: Value for the flags Set marks as serialized.
type CollisionSettings struct {
	Value, Set Collision
}

// readCollision collects the collision flags props carries.
func readCollision(props l2pkg.Properties) CollisionSettings {
	var s CollisionSettings
	vals, set := s.Value.fields(), s.Set.fields()
	for k, name := range collisionNames {
		*vals[k], *set[k] = props.BoolOK(name)
	}
	return s
}

// Over is the actor's effective collision: every serialized flag, and the
// class default d for the rest.
func (s CollisionSettings) Over(d Collision) Collision {
	out := d
	o, vals, set := out.fields(), s.Value.fields(), s.Set.fields()
	for k := range o {
		if *set[k] {
			*o[k] = *vals[k]
		}
	}
	return out
}

// maxClassChain bounds a superclass walk (Engine.L2NMover is 4 deep).
const maxClassChain = 32

// ClassDefaults reads the collision defaults of script classes out of the
// client's compiled packages (system/Engine.u and friends), once per class.
// Port of UE2-Studio's collision_defaults (crates/package-engine/src/
// defaults.rs): the class chain is walked from the class up, and each flag
// takes the first defaultproperties that declares it; a flag no class
// declares is false. It is safe for concurrent use.
type ClassDefaults struct {
	c     *l2pkg.Client
	mu    sync.Mutex
	cache map[string]classResult
}

type classResult struct {
	c   Collision
	err error
}

// NewClassDefaults reads class defaults through c.
func NewClassDefaults(c *l2pkg.Client) *ClassDefaults {
	return &ClassDefaults{c: c, cache: map[string]classResult{}}
}

// ActorClass is the script class (package, class name) of export i of p.
func ActorClass(p *l2pkg.Package, i int) (pkg, class string, err error) {
	return classOwner(p, p.Exports[i].ClassIndex)
}

// classOwner names the class an object reference of p points at: the
// package that declares it and its name.
func classOwner(p *l2pkg.Package, ref int32) (string, string, error) {
	switch {
	case ref > 0 && int(ref) <= len(p.Exports):
		return p.Name, p.Exports[ref-1].ObjectName, nil
	case ref < 0 && int(-ref) <= len(p.Imports):
		im := &p.Imports[-ref-1]
		root := im
		for depth := 0; root.PackageIndex != 0; depth++ {
			if depth == maxClassChain || root.PackageIndex > 0 || int(-root.PackageIndex) > len(p.Imports) {
				return "", "", fmt.Errorf("%s: cadeia de imports inválida para %s", p.Name, im.ObjectName)
			}
			root = &p.Imports[-root.PackageIndex-1]
		}
		return root.ObjectName, im.ObjectName, nil
	}
	return "", "", fmt.Errorf("%s: referência de classe %d inválida", p.Name, ref)
}

// Collision is the default collision of class pkg.class.
func (d *ClassDefaults) Collision(pkg, class string) (Collision, error) {
	key := strings.ToLower(pkg + "." + class)
	d.mu.Lock()
	r, ok := d.cache[key]
	d.mu.Unlock()
	if ok {
		return r.c, r.err
	}
	r.c, r.err = d.read(pkg, class)
	d.mu.Lock()
	d.cache[key] = r
	d.mu.Unlock()
	return r.c, r.err
}

func (d *ClassDefaults) read(pkg, class string) (Collision, error) {
	var out Collision
	var found Collision // flags already taken from a more derived class
	o, f := out.fields(), found.fields()
	seen := map[string]bool{}
	for depth := 0; ; depth++ {
		key := strings.ToLower(pkg + "." + class)
		if seen[key] || depth == maxClassChain {
			break
		}
		seen[key] = true
		p, err := d.c.Package(pkg)
		if err != nil {
			if depth == 0 {
				return Collision{}, err
			}
			break
		}
		i := classExport(p, class)
		if i < 0 {
			if depth == 0 {
				return Collision{}, fmt.Errorf("%s não declara a classe %s", pkg, class)
			}
			break
		}
		props, err := classDefaultProperties(p, i)
		if err != nil {
			if depth == 0 {
				return Collision{}, err
			}
			break
		}
		for k, name := range collisionNames {
			if *f[k] {
				continue
			}
			if v, ok := props.BoolOK(name); ok {
				*o[k], *f[k] = v, true
			}
		}
		super := p.Exports[i].SuperIndex
		if super == 0 {
			break
		}
		if pkg, class, err = classOwner(p, super); err != nil {
			break
		}
	}
	return out, nil
}

// classExport is the class export called class: Unreal leaves a class's
// own class (Core.Class) implicit.
func classExport(p *l2pkg.Package, class string) int {
	for i := range p.Exports {
		ex := &p.Exports[i]
		if ex.ClassIndex == 0 && ex.SerialSize > 0 && strings.EqualFold(ex.ObjectName, class) {
			return i
		}
	}
	return -1
}

// Sentinels of a class's state header: a class ignores every probe, has no
// label table and no state flags. They anchor the end of the compiled
// script, whose on-disk size is unknown.
const (
	ignoreMaskNone = ^uint64(0)
	labelTableNone = 0xffff
	classGUIDLen   = 16
	maxListLen     = 65536
)

// classDefaultProperties is the defaultproperties list of class export i.
// The compiled script in front of it cannot be measured without decoding
// bytecode, so the class trailer is found by search: the first offset
// whose bytes read as a trailer (inert state header, the class as its own
// first dependency, valid name indices) and whose property list then ends
// exactly on the export's last byte. Port of UE2-Studio class_trailer_at.
func classDefaultProperties(p *l2pkg.Package, i int) (l2pkg.Properties, error) {
	r, err := p.ExportReader(i)
	if err != nil {
		return l2pkg.Properties{}, err
	}
	ex := &p.Exports[i]
	end := int(ex.SerialOffset) + int(ex.SerialSize)
	body := p.Data[:end]
	if ex.Flags&l2pkg.RFHasStack != 0 {
		node := r.Index()
		r.Index()
		r.U64()
		r.I32()
		if node != 0 {
			r.Index()
		}
	}
	for range 6 { // UField super, next; UStruct script text, children, friendly name, C++ text
		r.Index()
	}
	r.I32() // source line
	r.I32() // source text position
	r.I32() // script size, counted in memory, not on disk
	if err := r.Err(); err != nil {
		return l2pkg.Properties{}, err
	}
	self := int32(i + 1)
	for off := r.Pos; off < end; off++ {
		c := l2pkg.NewReader(body, off)
		if !trailerMatches(p, c, self) {
			continue
		}
		props, err := l2pkg.ReadProperties(p, c, 0)
		if err == nil && c.Pos == end {
			return props, nil
		}
	}
	return l2pkg.Properties{}, fmt.Errorf("%s: propriedades padrão da classe %s não encontradas", p.Name, ex.ObjectName)
}

// trailerMatches reports whether r is at a class trailer, leaving it on
// the first byte of the default property list when so.
func trailerMatches(p *l2pkg.Package, r *l2pkg.Reader, self int32) bool {
	name := func(i int32) bool { return i >= 0 && int(i) < len(p.Names) }
	r.U64() // probe mask
	if r.U64() != ignoreMaskNone || r.U16() != labelTableNone || r.I32() != 0 {
		return false
	}
	r.I32() // class flags
	r.Skip(classGUIDLen)
	deps := r.Index()
	if deps < 1 || deps > maxListLen || r.Index() != self {
		return false
	}
	r.I32() // dependency depth
	r.I32() // script text CRC
	for range deps - 1 {
		r.Index()
		r.I32()
		r.I32()
	}
	imports := r.Index()
	if imports < 0 || imports > maxListLen {
		return false
	}
	for range imports {
		if !name(r.Index()) {
			return false
		}
	}
	r.Index() // within
	if !name(r.Index()) { // config name
		return false
	}
	hidden := r.Index()
	if hidden < 0 || hidden > maxListLen {
		return false
	}
	for range hidden {
		if !name(r.Index()) {
			return false
		}
	}
	return r.Err() == nil
}
