package scene

import (
	"errors"
	"fmt"
	"sort"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/unreal"
)

// MeshActor is one placed static mesh actor of a map.
type MeshActor struct {
	Tile Tile
	// Export is the actor's 0-based export index in the map package.
	Export int
	Name   string
	Class  string
	// Mesh is the StaticMesh's object path ("Package.Group.Name").
	Mesh  string
	Actor unreal.Actor
	// Hidden is set for a bHidden or bDeleteMe actor: listed, but neither
	// drawn nor pickable.
	Hidden bool
	// Sections are the mesh sections the actor draws, in mesh order;
	// sections dropped by the region filters are absent.
	Sections []MeshActorSection
	// Batch and First/Count locate the actor's triangles in
	// Scene.Batches[Batch].Indices (Count is 0 for a hidden actor).
	Batch, First, Count int
	// Bounds is the world AABB of the kept sections' vertices.
	Bounds geom.Box
}

// MeshActorSection is one kept mesh section of an actor.
type MeshActorSection struct {
	// Section is the section's index in the mesh.
	Section int
	// Material is the material the section is drawn with: the actor's Skins
	// override when set, else the mesh's Materials slot.
	Material MaterialRef
	// First and Count locate its triangles in the actor's batch indices;
	// both are 0 for a hidden actor.
	First, Count int
}

// MaterialRef is an unresolved material: an object reference and the
// package whose index space it belongs to (the map for a Skins override,
// the mesh's package for a mesh default). Ref 0 means no material.
type MaterialRef struct {
	Package *l2pkg.Package
	Ref     int32
}

type meshKey struct {
	pkg    *l2pkg.Package
	export int
}

// meshCache holds every StaticMesh one Load decoded, by owning export.
type meshCache map[meshKey]*unreal.StaticMesh

// addMeshes adds the static mesh actors of map m (tile t): every export of
// a MeshActorClasses class that the Level places, in UE2-Studio's order
// (class by class, then export order). An actor whose mesh package or
// object the client lacks is skipped; each missing package becomes one
// warning naming it.
func (s *Scene) addMeshes(c *l2pkg.Client, meshes meshCache, m *l2pkg.Package, t Tile) error {
	li := unreal.FindLevel(m)
	if li < 0 {
		return nil
	}
	level, err := unreal.ReadLevel(m, li)
	if err != nil {
		return err
	}
	placed := make(map[int]bool, len(level.Actors))
	for _, ref := range level.Actors {
		if ref > 0 {
			placed[int(ref-1)] = true
		}
	}
	footprint, hasMap := s.footprint(t)
	missing := map[string]int{}
	for _, class := range unreal.MeshActorClasses {
		for i := range m.Exports {
			if !placed[i] || m.Exports[i].ClassName != class {
				continue
			}
			a, err := unreal.ReadActor(m, i)
			if err != nil {
				return err
			}
			if a.StaticMesh == 0 {
				continue
			}
			owner, mesh, err := loadMesh(c, meshes, m, a.StaticMesh)
			if err != nil {
				var miss *l2pkg.MissingError
				if errors.As(err, &miss) {
					missing[miss.Error()]++
					continue
				}
				return fmt.Errorf("%s %s: %w", class, m.Exports[i].ObjectName, err)
			}
			path, _ := m.ObjectPath(a.StaticMesh)
			ma := MeshActor{
				Tile: t, Export: i, Name: m.Exports[i].ObjectName, Class: class, Mesh: path,
				Actor: *a, Hidden: a.Hidden || a.DeleteMe, Bounds: geom.EmptyBox(),
			}
			if s.placeMesh(&ma, m, owner, mesh, footprint, hasMap) {
				s.Actors = append(s.Actors, ma)
			}
		}
	}
	reasons := make([]string, 0, len(missing))
	for r := range missing {
		reasons = append(reasons, r)
	}
	sort.Strings(reasons)
	for _, r := range reasons {
		skipped := "1 ator de static mesh ignorado"
		if n := missing[r]; n != 1 {
			skipped = fmt.Sprintf("%d atores de static mesh ignorados", n)
		}
		s.Warnings = append(s.Warnings, fmt.Sprintf("%s: %s: %s", t.Name(), r, skipped))
	}
	return nil
}

// placeMesh transforms the actor's mesh into the scene's mesh batch, minus
// the sections the region filters drop, and registers it for picking. It
// reports false when no section survives (UE2-Studio then drops the actor).
func (s *Scene) placeMesh(ma *MeshActor, m, owner *l2pkg.Package, mesh *unreal.StaticMesh, footprint geom.Box, hasMap bool) bool {
	xf := ma.Actor.Transform()
	world := make([]geom.Vec3, len(mesh.Positions))
	for k, p := range mesh.Positions {
		world[k] = xf.Point(p)
	}
	ma.Batch = s.meshBatch()
	b := &s.Batches[ma.Batch]
	base := uint32(len(b.Vertices))
	ma.First = len(b.Indices)
	var tris []uint32
	for si, sec := range mesh.Sections {
		tris = tris[:0]
		box := geom.EmptyBox()
		for k := range sec.Triangles {
			tri, ok := mesh.Triangle(sec, k)
			if !ok {
				continue
			}
			for _, v := range tri {
				tris = append(tris, uint32(v))
				box.Include(world[v])
			}
		}
		if len(tris) == 0 || exceedsRegionTile(box) || hasMap && offMap(box, footprint) {
			continue
		}
		mat := MaterialRef{Package: owner}
		if si < len(mesh.Materials) {
			mat.Ref = mesh.Materials[si]
		}
		if si < len(ma.Actor.Skins) && ma.Actor.Skins[si] != 0 {
			mat = MaterialRef{Package: m, Ref: ma.Actor.Skins[si]}
		}
		kept := MeshActorSection{Section: si, Material: mat}
		ma.Bounds.Union(box)
		if !ma.Hidden {
			kept.First, kept.Count = len(b.Indices), len(tris)
			for _, v := range tris {
				b.Indices = append(b.Indices, base+v)
			}
		}
		ma.Sections = append(ma.Sections, kept)
	}
	if len(ma.Sections) == 0 {
		return false
	}
	ma.Count = len(b.Indices) - ma.First
	if ma.Count > 0 {
		for _, p := range world {
			b.Vertices = append(b.Vertices, Vertex{Pos: p})
		}
		b.Bounds.Union(ma.Bounds)
		s.addPickable(SurfaceMesh, ma.Batch, ma.First, ma.Count, ma.Bounds)
	}
	return true
}

// meshBatch is the index of the batch static meshes are drawn in, created
// on first use. Untextured, every mesh shares one opaque batch; the
// sections keep their MaterialRef for the textured renderer.
func (s *Scene) meshBatch() int {
	if s.meshes == 0 {
		s.Batches = append(s.Batches, Batch{Mode: Opaque, Bounds: geom.EmptyBox()})
		s.meshes = len(s.Batches)
	}
	return s.meshes - 1
}

// loadMesh resolves the StaticMesh reference ref of map m and decodes it,
// once per Load.
func loadMesh(c *l2pkg.Client, meshes meshCache, m *l2pkg.Package, ref int32) (*l2pkg.Package, *unreal.StaticMesh, error) {
	owner, i, err := c.Resolve(m, ref)
	if err != nil {
		return nil, nil, err
	}
	key := meshKey{owner, i}
	if mesh, ok := meshes[key]; ok {
		return owner, mesh, nil
	}
	if cls := owner.Exports[i].ClassName; cls != "StaticMesh" {
		return nil, nil, fmt.Errorf("StaticMesh aponta para %s, não StaticMesh", cls)
	}
	mesh, err := unreal.ReadStaticMesh(owner, i)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", owner.Name, err)
	}
	meshes[key] = mesh
	return owner, mesh, nil
}

// footprint is the horizontal extent of tile t's terrain, the map the
// off-map filter measures against; false when the tile has no terrain.
func (s *Scene) footprint(t Tile) (geom.Box, bool) {
	for i := range s.Terrains {
		ter := &s.Terrains[i]
		if ter.Tile == t {
			size := geom.Vec3{X: float32(ter.Width) * ter.Scale.X, Y: float32(ter.Height) * ter.Scale.Y}
			return geom.Box{Min: ter.Position, Max: ter.Position.Add(size)}, true
		}
	}
	return geom.Box{}, false
}

// exceedsRegionTile drops a section wider than two tiles on X or Y: zone
// backdrop sheets, not tile detail (UE2-Studio exceeds_region_tile).
func exceedsRegionTile(b geom.Box) bool {
	size := b.Size()
	return max(size.X, size.Y) > 2*TileSpan
}

// offMap drops a section whose horizontal centre lies more than a tile
// outside the terrain's footprint: props parked far from the map
// (UE2-Studio is_off_map).
func offMap(b, footprint geom.Box) bool {
	c := b.Center()
	return c.X < footprint.Min.X-TileSpan || c.X > footprint.Max.X+TileSpan ||
		c.Y < footprint.Min.Y-TileSpan || c.Y > footprint.Max.Y+TileSpan
}
