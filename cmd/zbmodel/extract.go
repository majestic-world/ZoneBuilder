package main

import (
	"fmt"
	"image"
	"math"
	"slices"
	"strings"

	"zonebuilder/internal/geom"
	"zonebuilder/internal/inflect"
	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/model"
	"zonebuilder/internal/skeletal"
	"zonebuilder/internal/texture"
	"zonebuilder/internal/unreal"
)

// request names what to extract, by Package.Group.Name paths of the client
// (the group is not consulted, as UE2-Studio's resolved_material).
type request struct {
	Mesh      string // SkeletalMesh
	Animation string // MeshAnimation
	// Skins are materials (a Texture or a material node such as FinalBlend):
	// one for every section, or one per section in slot order.
	Skins []string
	// Clips are sequence names of Animation; the first is the pose the
	// feet, the height and the bundle's clip 0 come from.
	Clips     []string
	DrawScale float32
}

// result is the bundle and the preview constants measured on it.
type result struct {
	Bundle *model.Bundle
	// Radius is npcgrp.rs's collision radius times DrawScale: the lower
	// median of the vertices' horizontal distance from the actor axis, in
	// the bind pose through the mesh base.
	Radius float32
	// Height is the Z extent of the first clip's frame 0, times DrawScale.
	Height float32
	// Vertices, Bones are the part's sizes, for the report.
	Vertices, Bones int
}

// minClips is how many clips model.Decode requires; a shorter list is
// padded with clip 0.
const minClips = 3

// extract reads the request's mesh, animation and skins from c and builds a
// UE2HUM01 bundle the way UE2-Studio's Assets::load_client builds the human:
// root motion removed, skins resolved by name, and a placement that scales
// by DrawScale and puts the feet on Z = 0 in frame 0 of the first clip.
func extract(c *l2pkg.Client, req request) (*result, error) {
	if len(req.Clips) == 0 {
		return nil, fmt.Errorf("nenhum clip pedido")
	}
	if !(req.DrawScale > 0) || math.IsInf(float64(req.DrawScale), 0) {
		return nil, fmt.Errorf("drawscale %v inválido", req.DrawScale)
	}
	mp, mi, err := find(c, req.Mesh, func(class string) bool { return class == "SkeletalMesh" })
	if err != nil {
		return nil, err
	}
	mesh, err := skeletal.ReadMesh(mp, mi)
	if err != nil {
		return nil, err
	}
	ap, ai, err := find(c, req.Animation, func(class string) bool { return class == "MeshAnimation" })
	if err != nil {
		return nil, err
	}
	anim, err := skeletal.ReadAnimation(ap, ai)
	if err != nil {
		return nil, err
	}

	b := &model.Bundle{Placement: model.IdentityAffine}
	for _, name := range req.Clips {
		clip, err := readClip(anim, name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", req.Animation, err)
		}
		b.Clips = append(b.Clips, clip)
	}
	for len(b.Clips) < minClips {
		b.Clips = append(b.Clips, b.Clips[0])
	}
	// The monster never jumps; the slots only have to be valid.
	b.JumpClips = [2]uint32{0, 0}

	// names[track] is the mesh bone that track drives (bundle.rs encode).
	b.Names = make([]string, len(anim.Bones))
	bound := 0
	for bone, track := range anim.Bind(mesh.Bones) {
		if track >= 0 {
			b.Names[track] = mesh.Bones[bone].Name
			bound++
		}
	}
	if bound == 0 {
		return nil, fmt.Errorf("%s não anima nenhum osso de %s", req.Animation, req.Mesh)
	}

	part, err := buildPart(c, mesh, req.Skins)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", req.Mesh, err)
	}
	b.Parts = []model.Part{part}

	// Bounds of the first clip's frame 0 before placement, then the
	// placement that scales them and lifts the feet to Z = 0.
	positions := make([]geom.Vec3, len(part.Vertices))
	model.NewAnimator(b).Skin(0, positions, nil)
	box := geom.EmptyBox()
	for _, p := range positions {
		box.Include(p)
	}
	height := box.Max.Z - box.Min.Z
	if !(height >= 1) || math.IsInf(float64(height), 0) {
		return nil, fmt.Errorf("%s: a pose %s tem altura inválida %v", req.Mesh, req.Clips[0], height)
	}
	s := req.DrawScale
	b.Placement = model.Affine{
		Origin: geom.Vec3{Z: -box.Min.Z * s},
		Axis:   [3]geom.Vec3{{X: s}, {Y: s}, {Z: s}},
	}

	return &result{
		Bundle:   b,
		Radius:   collisionRadius(part) * s,
		Height:   height * s,
		Vertices: len(part.Vertices),
		Bones:    len(part.Bones),
	}, nil
}

// find is the first export of the package path's first segment named by its
// last segment whose class accept takes.
func find(c *l2pkg.Client, path string, accept func(class string) bool) (*l2pkg.Package, int, error) {
	pkgName, _, ok := strings.Cut(path, ".")
	name := path[strings.LastIndexByte(path, '.')+1:]
	if !ok || pkgName == "" || name == "" {
		return nil, 0, fmt.Errorf("%q não é um caminho Pacote.Objeto", path)
	}
	p, err := c.Package(pkgName)
	if err != nil {
		return nil, 0, err
	}
	for i := range p.Exports {
		if ex := &p.Exports[i]; strings.EqualFold(ex.ObjectName, name) && accept(ex.ClassName) {
			return p, i, nil
		}
	}
	return nil, 0, fmt.Errorf("%s não existe no cliente", path)
}

// readClip is sequence name of a as a looping clip with the root motion
// removed: track 0's position keys keep the first key's X and Y, so the
// capsule owns locomotion and the vertical bob stays (avatar.rs
// load_client).
func readClip(a *skeletal.Animation, name string) (model.Clip, error) {
	k := a.SequenceNamed(name)
	if k < 0 {
		return model.Clip{}, fmt.Errorf("não tem a sequência %s", name)
	}
	seq := a.Sequences[k]
	if seq.Frames <= 0 || !(seq.Rate > 0) || math.IsInf(float64(seq.Rate), 0) {
		return model.Clip{}, fmt.Errorf("a sequência %s tem %s a %v fps", name, inflect.Count(seq.Frames, "frame", "frames"), seq.Rate)
	}
	tracks, err := a.Tracks(k)
	if err != nil {
		return model.Clip{}, err
	}
	clip := model.Clip{Frames: int32(seq.Frames), Rate: seq.Rate, Looping: true, Tracks: make([]model.Track, len(tracks))}
	for i, t := range tracks {
		out := &clip.Tracks[i]
		out.KeyQuat = make([]model.Quat, len(t.Quats))
		for j, q := range t.Quats {
			out.KeyQuat[j] = model.Quat{X: q[0], Y: q[1], Z: q[2], W: q[3]}
		}
		out.KeyPos = slices.Clone(t.Positions)
		out.KeyTime = slices.Clone(t.Times)
	}
	if len(clip.Tracks) > 0 && len(clip.Tracks[0].KeyPos) > 0 {
		root := clip.Tracks[0].KeyPos
		for j := range root {
			root[j].X, root[j].Y = root[0].X, root[0].Y
		}
	}
	return clip, nil
}

// buildPart is mesh as a bundle part: its skeleton, base and vertices, and
// each section drawn with its skin.
func buildPart(c *l2pkg.Client, mesh *skeletal.Mesh, skins []string) (model.Part, error) {
	if len(skins) != 1 && len(skins) != len(mesh.Sections) {
		return model.Part{}, fmt.Errorf("%s para %s", inflect.Count(len(skins), "skin", "skins"), inflect.Count(len(mesh.Sections), "seção", "seções"))
	}
	part := model.Part{Base: base(mesh), Bones: make([]model.Bone, len(mesh.Bones))}
	for i, b := range mesh.Bones {
		parent := model.NoParent
		if b.Parent >= 0 {
			parent = uint16(b.Parent)
		}
		part.Bones[i] = model.Bone{
			Name:        b.Name,
			Parent:      parent,
			Position:    b.Position,
			Orientation: model.Quat{X: b.Orientation[0], Y: b.Orientation[1], Z: b.Orientation[2], W: b.Orientation[3]},
			Length:      b.Length,
			Size:        b.Size,
		}
	}
	part.InverseBind = model.InverseBind(part.Bones)
	part.Vertices = make([]model.Vertex, len(mesh.Vertices))
	for i, v := range mesh.Vertices {
		part.Vertices[i] = model.Vertex{Position: v.Position, Normal: v.Normal, UV: v.UV, Bones: v.Bones, Weights: v.Weights}
	}
	part.Indices = mesh.Indices
	for i, s := range mesh.Sections {
		skin := skins[min(i, len(skins)-1)]
		section, err := skinSection(c, skin)
		if err != nil {
			return model.Part{}, fmt.Errorf("skin %s: %w", skin, err)
		}
		section.FirstIndex, section.IndexCount, section.PolygonFlags = uint32(s.FirstIndex), uint32(s.IndexCount), s.PolyFlags
		part.Sections = append(part.Sections, section)
	}
	return part, nil
}

// skinSection resolves the material at path to its texture and render mode
// (UE2-Studio skeletal_material_ref) with the texture's top mip as PNG.
func skinSection(c *l2pkg.Client, path string) (model.Section, error) {
	p, i, err := find(c, path, func(class string) bool { return class == "Texture" || unreal.IsMaterialNode(class) })
	if err != nil {
		return model.Section{}, err
	}
	m, err := unreal.WalkMaterial(c, p, int32(i+1), func(pkg *l2pkg.Package, i int) (*texture.Texture, bool, error) {
		t, err := texture.Read(pkg, i)
		if err != nil {
			return nil, false, err
		}
		if t.Format == texture.FormatP8 && t.PaletteRef != 0 {
			owner, j, err := c.Resolve(pkg, t.PaletteRef)
			if err != nil {
				return t, false, nil
			}
			if t.Palette, err = texture.ReadPalette(owner, j); err != nil {
				return t, false, nil
			}
		}
		return t, t.Drawable() == nil, nil
	})
	if err != nil {
		return model.Section{}, err
	}
	if m.Texture == nil {
		return model.Section{}, fmt.Errorf("o grafo de material não chega a uma textura desenhável")
	}
	rgba, err := m.Texture.RGBA()
	if err != nil {
		return model.Section{}, fmt.Errorf("%s: %w", m.Texture.Path, err)
	}
	s := model.Section{
		RenderMode:    renderMode(m),
		Masked:        m.Masked,
		VertexOpacity: m.VertexOpacity,
		Texture:       m.Texture.Path,
		PNG: model.EncodePNG(&image.NRGBA{
			Pix:    rgba.Pix,
			Stride: 4 * rgba.Width,
			Rect:   image.Rect(0, 0, rgba.Width, rgba.Height),
		}),
	}
	// A Shader with neither Opacity nor OutputBlending draws its texture
	// opaque whatever the alpha says (skeletal_material_ref).
	if sh := m.Shader; sh != nil && sh.Opacity == 0 && sh.OutputBlending == 0 {
		s.Masked, s.RenderMode = false, model.Opaque
	}
	return s, nil
}

// renderMode is UE2-Studio's RenderMode::from_material_semantics.
func renderMode(m unreal.Material) model.RenderMode {
	switch {
	case m.Brighten:
		return model.Brighten
	case m.Translucent && m.Water:
		return model.Water
	case m.Translucent:
		return model.Translucent
	case m.Masked:
		return model.Masked
	}
	return model.Opaque
}

// base is the mesh's actor-space placement (skin.rs base_from_parts): the
// RotOrigin basis with row k scaled by MeshScale component k, and the origin
// that basis applied to -MeshOrigin.
func base(m *skeletal.Mesh) model.Affine {
	axis := rotatorAxis(m.Rotation)
	axis[0] = axis[0].Scale(m.Scale.X)
	axis[1] = axis[1].Scale(m.Scale.Y)
	axis[2] = axis[2].Scale(m.Scale.Z)
	b := model.Affine{Axis: axis}
	b.Origin = b.Vector(m.Origin.Scale(-1))
	return b
}

// rotatorAxis is a mesh RotOrigin as basis rows (UEViewer RotatorToAxis,
// math.rs rotator_to_axis): forward, negated right and up of the Euler
// angles, then four sign flips from Unreal's left-handed rotator.
func rotatorAxis(r l2pkg.Rotator) [3]geom.Vec3 {
	const toRadians = math.Pi / 32768
	sincos := func(units int32) (float32, float32) {
		s, c := math.Sincos(float64(float32(units) * float32(toRadians)))
		return float32(s), float32(c)
	}
	sp, cp := sincos(r.Pitch)
	sy, cy := sincos(r.Yaw)
	sr, cr := sincos(r.Roll)
	forward := geom.Vec3{X: cp * cy, Y: cp * sy, Z: -sp}
	right := geom.Vec3{X: -sr*sp*cy + cr*sy, Y: -sr*sp*sy - cr*cy, Z: -sr * cp}
	up := geom.Vec3{X: cr*sp*cy + sr*sy, Y: cr*sp*sy - sr*cy, Z: cr * cp}
	axis := [3]geom.Vec3{forward, right.Scale(-1), up}
	axis[0].Z = -axis[0].Z
	axis[1].Z = -axis[1].Z
	axis[2].X = -axis[2].X
	axis[2].Y = -axis[2].Y
	return axis
}

// collisionRadius is npcgrp.rs collision's radius at drawscale 1: over the
// bind-pose vertices through the part's base, the lower median of the
// horizontal distance from the actor's Z axis (wings, weapons and effects do
// not dominate it).
func collisionRadius(part model.Part) float32 {
	samples := make([]float32, 0, len(part.Vertices))
	for _, v := range part.Vertices {
		p := part.Base.Point(v.Position)
		if finite(p.X) && finite(p.Y) && finite(p.Z) {
			samples = append(samples, float32(math.Hypot(float64(p.X), float64(p.Y))))
		}
	}
	if len(samples) == 0 {
		return 0
	}
	slices.Sort(samples)
	return samples[(len(samples)+1)/2-1]
}

func finite(f float32) bool { return !math.IsNaN(float64(f)) && !math.IsInf(float64(f), 0) }
