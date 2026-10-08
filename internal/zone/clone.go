package zone

import "slices"

// clone is a deep copy of z: none of its slices is shared with z. Nil
// slices stay nil, so a clone is reflect.DeepEqual to its source. A slice
// field added to Zone or Shape must be copied here too.
func (z Zone) clone() Zone {
	z.Params = slices.Clone(z.Params)
	z.RestartPoints = slices.Clone(z.RestartPoints)
	z.PKRestartPoints = slices.Clone(z.PKRestartPoints)
	if z.Shapes != nil {
		shapes := make([]Shape, len(z.Shapes))
		for i, s := range z.Shapes {
			s.Points = slices.Clone(s.Points)
			shapes[i] = s
		}
		z.Shapes = shapes
	}
	return z
}

// cloneZones is a deep copy of zs (see Zone.clone).
func cloneZones(zs []Zone) []Zone {
	if zs == nil {
		return nil
	}
	out := make([]Zone, len(zs))
	for i, z := range zs {
		out[i] = z.clone()
	}
	return out
}
