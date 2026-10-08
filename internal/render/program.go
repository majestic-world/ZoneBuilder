package render

import (
	"fmt"

	"zonebuilder/internal/render/gles"
)

// newProgram compiles and links a GLSL ES 3.00 vertex/fragment pair.
func newProgram(vertSrc, fragSrc string) (uint32, error) {
	vs, err := compileShader(gles.VERTEX_SHADER, vertSrc)
	if err != nil {
		return 0, fmt.Errorf("vertex shader: %w", err)
	}
	defer gles.DeleteShader(vs)
	fs, err := compileShader(gles.FRAGMENT_SHADER, fragSrc)
	if err != nil {
		return 0, fmt.Errorf("fragment shader: %w", err)
	}
	defer gles.DeleteShader(fs)
	p := gles.CreateProgram()
	gles.AttachShader(p, vs)
	gles.AttachShader(p, fs)
	gles.LinkProgram(p)
	if gles.GetProgrami(p, gles.LINK_STATUS) == 0 {
		log := gles.GetProgramInfoLog(p)
		gles.DeleteProgram(p)
		return 0, fmt.Errorf("link: %s", log)
	}
	return p, nil
}

func compileShader(kind uint32, src string) (uint32, error) {
	s := gles.CreateShader(kind)
	gles.ShaderSource(s, src)
	gles.CompileShader(s)
	if gles.GetShaderi(s, gles.COMPILE_STATUS) == 0 {
		log := gles.GetShaderInfoLog(s)
		gles.DeleteShader(s)
		return 0, fmt.Errorf("compile: %s", log)
	}
	return s, nil
}
