package scene

import (
	"errors"
	"fmt"
	"sort"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/unreal"
)

// MeshActor is one placed static mesh actor of a map.
type MeshActor struct {
	// Export is the actor's 0-based export index in the map package.
	Export int
	Name   string
	Class  string
	Actor  unreal.Actor
	// Hidden is set for a bHidden or bDeleteMe actor: listed, but neither
	// drawn nor pickable.
	Hidden bool
	// Sections are the mesh sections the actor draws, in mesh order;
	// sections dropped by the region filters are absent.
	Sections []MeshActorSection
	// Bounds is the world AABB of the kept sections' vertices.
	Bounds geom.Box
}

// Triangles is how many triangles the actor draws (0 when hidden).
func (a *MeshActor) Triangles() int {
	n := 0
	for _, sec := range a.Sections {
		n += sec.Count / 3
	}
	return n
}

// MeshActorSection is one kept mesh section of an actor.
type MeshActorSection struct {
	// Section is the section's index in the mesh.
	Section int
	// Material is the object path ("Package.Group.Name") of the material
	// the section is drawn with, "" for none: the actor's Skins override
	// when set, else the mesh's Materials slot. A path, not a package
	// reference, so the scene holds no package in memory.
	Material string
	// Batch, First and Count locate its triangles in
	// Scene.Batches[Batch].Indices, the batch of its material; Batch is -1
	// and Count 0 for a hidden actor.
	Batch, First, Count int
	// Bounds is the world AABB of the section's vertices.
	Bounds geom.Box
}

// addMeshes adds the static mesh actors of map m (tile t): every export of
// a MeshActorClasses class that the Level places, in UE2-Studio's order
// (class by class, then export order). footprint is the tile's terrain
// footprint for the region filters, nil without terrain. An actor whose
// mesh package or object the client lacks is skipped; each missing package
// becomes one warning naming it.
func (s *Scene) addMeshes(ld *loader, m *l2pkg.Package, t Tile, footprint *geom.Box) error {
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
			owner, mesh, err := ld.mesh(m, a.StaticMesh)
			if err != nil {
				var miss *l2pkg.MissingError
				if errors.As(err, &miss) {
					missing[miss.Error()]++
					continue
				}
				return fmt.Errorf("%s %s: %w", class, m.Exports[i].ObjectName, err)
			}
			ma := MeshActor{
				Export: i, Name: m.Exports[i].ObjectName, Class: class,
				Actor: *a, Hidden: a.Hidden || a.DeleteMe, Bounds: geom.EmptyBox(),
			}
			kept, err := s.placeMesh(ld, &ma, m, owner, mesh, footprint)
			if err != nil {
				return fmt.Errorf("%s %s: %w", class, m.Exports[i].ObjectName, err)
			}
			if kept {
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
			skipped = inflect.Count(n, "ator de static mesh ignorado", "atores de static mesh ignorados")
		}
		s.Warnings = append(s.Warnings, fmt.Sprintf("%s: %s: %s", t.Name(), r, skipped))
	}
	return nil
}

// placeMesh transforms the actor's mesh sections, minus those the region
// filters drop, into the batches of their materials, and registers each for
// picking. Port of UE2-Studio's mesh placement: a section's material is the
// actor's Skins slot when set, else the mesh's Materials slot; a material
// that asks for a cutout anywhere along its graph draws Masked whatever its
// blend (map_static_mesh_mode); a vertex-colour opacity takes the mesh's
// ColorStream alpha. It reports false when no section survives
// (UE2-Studio then drops the actor).
func (s *Scene) placeMesh(ld *loader, ma *MeshActor, m, owner *l2pkg.Package, mesh *unreal.StaticMesh, footprint *geom.Box) (bool, error) {
	xf := ma.Actor.Transform()
	world := make([]geom.Vec3, len(mesh.Positions))
	for k, p := range mesh.Positions {
		world[k] = xf.Point(p)
	}
	// remap[v] is mesh vertex v's index in the current section's batch, -1
	// before the section uses it.
	remap := make([]int32, len(mesh.Positions))
	var tris []uint16
	for si, sec := range mesh.Sections {
		tris = tris[:0]
		box := geom.EmptyBox()
		for k := range sec.Triangles {
			tri, ok := mesh.Triangle(sec, k)
			if !ok {
				continue
			}
			for _, v := range tri {
				tris = append(tris, v)
				box.Include(world[v])
			}
		}
		if len(tris) == 0 || outsideRegion(box, footprint) {
			continue
		}
		pkg, ref := owner, int32(0)
		if si < len(mesh.Materials) {
			ref = mesh.Materials[si]
		}
		if si < len(ma.Actor.Skins) && ma.Actor.Skins[si] != 0 {
			pkg, ref = m, ma.Actor.Skins[si]
		}
		kept := MeshActorSection{Section: si, Batch: -1, Bounds: box}
		if ref != 0 {
			kept.Material, _ = pkg.ObjectPath(ref)
		}
		ma.Bounds.Union(box)
		if !ma.Hidden {
			mat, err := ld.material(pkg, ref)
			if err != nil {
				return false, fmt.Errorf("material da seção %d: %w", si, err)
			}
			mode := mat.mode
			if mat.masked {
				mode = Masked
			}
			kept.Batch = s.batch(ld, batchKey{tex: mat.texture, mode: mode, opaque: mat.vertexOpacity, mesh: true})
			b := &s.Batches[kept.Batch]
			kept.First, kept.Count = len(b.Indices), len(tris)
			for k := range remap {
				remap[k] = -1
			}
			for _, v := range tris {
				if remap[v] < 0 {
					remap[v] = int32(len(b.Vertices))
					vert := Vertex{Pos: world[v], Alpha: 1}
					if int(v) < len(mesh.UVs) {
						vert.UV = mesh.UVs[v]
					}
					if mat.vertexOpacity && int(v) < len(mesh.Alpha) {
						vert.Alpha = float32(mesh.Alpha[v]) / 255
					}
					b.Vertices = append(b.Vertices, vert)
				}
				b.Indices = append(b.Indices, uint32(remap[v]))
			}
			b.Bounds.Union(box)
			s.addPickable(triangleSet{Surface: SurfaceMesh, Batch: kept.Batch, First: kept.First, Count: kept.Count, Bounds: box, Mirrored: xf.Scale.X*xf.Scale.Y*xf.Scale.Z < 0})
		}
		ma.Sections = append(ma.Sections, kept)
	}
	return len(ma.Sections) > 0, nil
}

// mesh resolves the StaticMesh reference ref of map m and decodes it, once
// per Load.
func (ld *loader) mesh(m *l2pkg.Package, ref int32) (*l2pkg.Package, *unreal.StaticMesh, error) {
	owner, i, err := ld.c.Resolve(m, ref)
	if err != nil {
		return nil, nil, err
	}
	key := objectKey{owner, i}
	if mesh, ok := ld.meshes[key]; ok {
		return owner, mesh, nil
	}
	if cls := owner.Exports[i].ClassName; cls != "StaticMesh" {
		return nil, nil, fmt.Errorf("StaticMesh aponta para %s, não StaticMesh", cls)
	}
	mesh, err := unreal.ReadStaticMesh(owner, i)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", owner.Name, err)
	}
	ld.meshes[key] = mesh
	return owner, mesh, nil
}
