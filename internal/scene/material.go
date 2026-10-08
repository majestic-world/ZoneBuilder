package scene

import (
	"fmt"

	"zonebuilder/internal/l2pkg"
	"zonebuilder/internal/texture"
)

// loader is what one Load shares across its tiles: the client's packages
// and every Texture export already read from them.
type loader struct {
	c        *l2pkg.Client
	textures map[textureKey]*texture.Texture
}

type textureKey struct {
	pkg    *l2pkg.Package
	export int
}

func newLoader(c *l2pkg.Client) *loader {
	return &loader{c: c, textures: make(map[textureKey]*texture.Texture)}
}

// texture is the Texture that object reference ref of p points at, read
// once per Load: every batch drawn with one Texture export shares the
// pointer, which is the batch key's texture identity. It fails when the
// reference is not a Texture or the texture cannot be drawn
// (texture.Drawable).
func (ld *loader) texture(p *l2pkg.Package, ref int32) (*texture.Texture, error) {
	owner, i, err := ld.c.Resolve(p, ref)
	if err != nil {
		return nil, err
	}
	key := textureKey{owner, i}
	if t, ok := ld.textures[key]; ok {
		return t, nil
	}
	if cls := owner.Exports[i].ClassName; cls != "Texture" {
		return nil, fmt.Errorf("%s.%s é %s, não Texture", owner.Name, owner.Exports[i].ObjectName, cls)
	}
	t, err := texture.Read(owner, i)
	if err != nil {
		return nil, fmt.Errorf("%s.%s: %w", owner.Name, owner.Exports[i].ObjectName, err)
	}
	if err := t.Drawable(); err != nil {
		return nil, fmt.Errorf("%s: %w", t.Path, err)
	}
	ld.textures[key] = t
	return t, nil
}
