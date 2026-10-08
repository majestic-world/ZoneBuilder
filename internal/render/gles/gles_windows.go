// Package gles binds the subset of OpenGL ES 3.0 the Zone Builder renderer
// uses, loaded from ANGLE's libGLESv2.dll at run time (no cgo).
//
// Every function must run on the OS thread that holds the current EGL
// context. Float arguments travel as their IEEE bits: the Go runtime copies
// the first four syscall arguments into XMM0-3 as well, which is what the
// Windows x64 ABI reads them from.
package gles

import (
	"fmt"
	"math"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"zonebuilder/internal/render/egl"
)

var (
	pActiveTexture, pAttachShader, pBindBuffer, pBindFramebuffer, pBindRenderbuffer,
	pBindTexture, pBindVertexArray, pBufferData, pCheckFramebufferStatus,
	pClear, pClearColor, pClearDepthf, pCompileShader, pCompressedTexImage2D, pCreateProgram,
	pCreateShader, pDeleteBuffers, pDeleteFramebuffers, pDeleteProgram,
	pDeleteRenderbuffers, pDeleteShader, pDeleteTextures, pDeleteVertexArrays, pDepthFunc,
	pDisable, pDrawArrays, pDrawElements, pEnable, pEnableVertexAttribArray,
	pFramebufferRenderbuffer, pFramebufferTexture2D, pGenBuffers, pGenFramebuffers, pGenRenderbuffers, pGenTextures,
	pGenVertexArrays, pGetError, pGetIntegerv, pGetProgramInfoLog, pGetProgramiv,
	pGetShaderInfoLog, pGetShaderiv, pGetString, pGetStringi, pGetUniformLocation,
	pLinkProgram, pRenderbufferStorage, pShaderSource,
	pTexImage2D, pTexParameteri, pUniform1i, pUniformMatrix4fv, pUseProgram,
	pVertexAttribPointer, pViewport uintptr

	// pClipControlEXT is GL_EXT_clip_control's entry point; zero when absent.
	pClipControlEXT uintptr
)

var loadOnce = sync.OnceValue(load)

// Load loads libGLESv2.dll and resolves the entry points. It is idempotent
// and must run after egl.Load (extension entry points come from
// eglGetProcAddress).
func Load() error { return loadOnce() }

func load() error {
	if err := egl.Load(); err != nil {
		return err
	}
	const name = "libGLESv2.dll"
	h, err := windows.LoadLibraryEx(name, 0, windows.LOAD_LIBRARY_SEARCH_DEFAULT_DIRS)
	if err != nil {
		return fmt.Errorf("gles: load %s (it must sit next to the executable): %w", name, err)
	}
	dll := windows.DLL{Name: name, Handle: h}
	procs := []struct {
		name string
		p    *uintptr
	}{
		{"glActiveTexture", &pActiveTexture},
		{"glAttachShader", &pAttachShader},
		{"glBindBuffer", &pBindBuffer},
		{"glBindFramebuffer", &pBindFramebuffer},
		{"glBindRenderbuffer", &pBindRenderbuffer},
		{"glBindTexture", &pBindTexture},
		{"glBindVertexArray", &pBindVertexArray},
		{"glBufferData", &pBufferData},
		{"glCheckFramebufferStatus", &pCheckFramebufferStatus},
		{"glClear", &pClear},
		{"glClearColor", &pClearColor},
		{"glClearDepthf", &pClearDepthf},
		{"glCompileShader", &pCompileShader},
		{"glCompressedTexImage2D", &pCompressedTexImage2D},
		{"glCreateProgram", &pCreateProgram},
		{"glCreateShader", &pCreateShader},
		{"glDeleteBuffers", &pDeleteBuffers},
		{"glDeleteFramebuffers", &pDeleteFramebuffers},
		{"glDeleteProgram", &pDeleteProgram},
		{"glDeleteRenderbuffers", &pDeleteRenderbuffers},
		{"glDeleteShader", &pDeleteShader},
		{"glDeleteTextures", &pDeleteTextures},
		{"glDeleteVertexArrays", &pDeleteVertexArrays},
		{"glDepthFunc", &pDepthFunc},
		{"glDisable", &pDisable},
		{"glDrawArrays", &pDrawArrays},
		{"glDrawElements", &pDrawElements},
		{"glEnable", &pEnable},
		{"glEnableVertexAttribArray", &pEnableVertexAttribArray},
		{"glFramebufferRenderbuffer", &pFramebufferRenderbuffer},
		{"glFramebufferTexture2D", &pFramebufferTexture2D},
		{"glGenBuffers", &pGenBuffers},
		{"glGenFramebuffers", &pGenFramebuffers},
		{"glGenRenderbuffers", &pGenRenderbuffers},
		{"glGenTextures", &pGenTextures},
		{"glGenVertexArrays", &pGenVertexArrays},
		{"glGetError", &pGetError},
		{"glGetIntegerv", &pGetIntegerv},
		{"glGetProgramInfoLog", &pGetProgramInfoLog},
		{"glGetProgramiv", &pGetProgramiv},
		{"glGetShaderInfoLog", &pGetShaderInfoLog},
		{"glGetShaderiv", &pGetShaderiv},
		{"glGetString", &pGetString},
		{"glGetStringi", &pGetStringi},
		{"glGetUniformLocation", &pGetUniformLocation},
		{"glLinkProgram", &pLinkProgram},
		{"glRenderbufferStorage", &pRenderbufferStorage},
		{"glShaderSource", &pShaderSource},
		{"glTexImage2D", &pTexImage2D},
		{"glTexParameteri", &pTexParameteri},
		{"glUniform1i", &pUniform1i},
		{"glUniformMatrix4fv", &pUniformMatrix4fv},
		{"glUseProgram", &pUseProgram},
		{"glVertexAttribPointer", &pVertexAttribPointer},
		{"glViewport", &pViewport},
	}
	for _, p := range procs {
		proc, err := dll.FindProc(p.name)
		if err != nil {
			return fmt.Errorf("gles: %s: %w", name, err)
		}
		*p.p = proc.Addr()
	}
	pClipControlEXT = egl.GetProcAddress("glClipControlEXT")
	return nil
}

func b2u(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

func f2u(f float32) uintptr { return uintptr(math.Float32bits(f)) }

func ActiveTexture(unit uint32) { syscall.SyscallN(pActiveTexture, uintptr(unit)) }
func AttachShader(prog, shader uint32) {
	syscall.SyscallN(pAttachShader, uintptr(prog), uintptr(shader))
}
func BindBuffer(target, buf uint32) { syscall.SyscallN(pBindBuffer, uintptr(target), uintptr(buf)) }
func BindFramebuffer(target, fb uint32) {
	syscall.SyscallN(pBindFramebuffer, uintptr(target), uintptr(fb))
}
func BindRenderbuffer(target, rb uint32) {
	syscall.SyscallN(pBindRenderbuffer, uintptr(target), uintptr(rb))
}
func BindTexture(target, tex uint32)   { syscall.SyscallN(pBindTexture, uintptr(target), uintptr(tex)) }
func BindVertexArray(vao uint32)       { syscall.SyscallN(pBindVertexArray, uintptr(vao)) }
func Clear(mask uint32)                { syscall.SyscallN(pClear, uintptr(mask)) }
func ClearDepthf(d float32)            { syscall.SyscallN(pClearDepthf, f2u(d)) }
func CompileShader(s uint32)           { syscall.SyscallN(pCompileShader, uintptr(s)) }
func DeleteProgram(p uint32)           { syscall.SyscallN(pDeleteProgram, uintptr(p)) }
func DeleteShader(s uint32)            { syscall.SyscallN(pDeleteShader, uintptr(s)) }
func DepthFunc(f uint32)               { syscall.SyscallN(pDepthFunc, uintptr(f)) }
func Disable(cap uint32)               { syscall.SyscallN(pDisable, uintptr(cap)) }
func Enable(cap uint32)                { syscall.SyscallN(pEnable, uintptr(cap)) }
func EnableVertexAttribArray(i uint32) { syscall.SyscallN(pEnableVertexAttribArray, uintptr(i)) }
func LinkProgram(p uint32)             { syscall.SyscallN(pLinkProgram, uintptr(p)) }
func Uniform1i(loc int32, v int32)     { syscall.SyscallN(pUniform1i, uintptr(loc), uintptr(v)) }
func UseProgram(p uint32)              { syscall.SyscallN(pUseProgram, uintptr(p)) }

func ClearColor(r, g, b, a float32) {
	syscall.SyscallN(pClearColor, f2u(r), f2u(g), f2u(b), f2u(a))
}

func Viewport(x, y, w, h int) {
	syscall.SyscallN(pViewport, uintptr(x), uintptr(y), uintptr(w), uintptr(h))
}

// BufferData uploads data (a slice of fixed-size elements) into the bound buffer.
func BufferData[T any](target uint32, data []T, usage uint32) {
	var size uintptr
	var ptr unsafe.Pointer
	if len(data) > 0 {
		size = uintptr(len(data)) * unsafe.Sizeof(data[0])
		ptr = unsafe.Pointer(&data[0])
	}
	syscall.SyscallN(pBufferData, uintptr(target), size, uintptr(ptr), uintptr(usage))
}

func CheckFramebufferStatus(target uint32) uint32 {
	r, _, _ := syscall.SyscallN(pCheckFramebufferStatus, uintptr(target))
	return uint32(r)
}

func CompressedTexImage2D(target uint32, level int, internalFormat uint32, w, h int, data []byte) {
	syscall.SyscallN(pCompressedTexImage2D, uintptr(target), uintptr(level), uintptr(internalFormat),
		uintptr(w), uintptr(h), 0, uintptr(len(data)), uintptr(unsafe.Pointer(unsafe.SliceData(data))))
}

func CreateProgram() uint32 {
	r, _, _ := syscall.SyscallN(pCreateProgram)
	return uint32(r)
}

func CreateShader(kind uint32) uint32 {
	r, _, _ := syscall.SyscallN(pCreateShader, uintptr(kind))
	return uint32(r)
}

func gen(proc uintptr) uint32 {
	var id uint32
	syscall.SyscallN(proc, 1, uintptr(unsafe.Pointer(&id)))
	return id
}

func del(proc uintptr, id uint32) {
	syscall.SyscallN(proc, 1, uintptr(unsafe.Pointer(&id)))
}

func GenBuffer() uint32       { return gen(pGenBuffers) }
func GenFramebuffer() uint32  { return gen(pGenFramebuffers) }
func GenRenderbuffer() uint32 { return gen(pGenRenderbuffers) }
func GenTexture() uint32      { return gen(pGenTextures) }
func GenVertexArray() uint32  { return gen(pGenVertexArrays) }

func DeleteBuffer(id uint32)       { del(pDeleteBuffers, id) }
func DeleteFramebuffer(id uint32)  { del(pDeleteFramebuffers, id) }
func DeleteRenderbuffer(id uint32) { del(pDeleteRenderbuffers, id) }
func DeleteTexture(id uint32)      { del(pDeleteTextures, id) }
func DeleteVertexArray(id uint32)  { del(pDeleteVertexArrays, id) }

func DrawArrays(mode uint32, first, count int) {
	syscall.SyscallN(pDrawArrays, uintptr(mode), uintptr(first), uintptr(count))
}

func DrawElements(mode uint32, count int, typ uint32, offset uintptr) {
	syscall.SyscallN(pDrawElements, uintptr(mode), uintptr(count), uintptr(typ), offset)
}

func FramebufferRenderbuffer(target, attachment, rbTarget, rb uint32) {
	syscall.SyscallN(pFramebufferRenderbuffer, uintptr(target), uintptr(attachment), uintptr(rbTarget), uintptr(rb))
}

func FramebufferTexture2D(target, attachment, texTarget, tex uint32, level int) {
	syscall.SyscallN(pFramebufferTexture2D, uintptr(target), uintptr(attachment), uintptr(texTarget), uintptr(tex), uintptr(level))
}

func GetError() uint32 {
	r, _, _ := syscall.SyscallN(pGetError)
	return uint32(r)
}

func GetInteger(pname uint32) int32 {
	var v [4]int32 // some queries write more than one value
	syscall.SyscallN(pGetIntegerv, uintptr(pname), uintptr(unsafe.Pointer(&v[0])))
	return v[0]
}

func GetString(name uint32) string {
	r, _, _ := syscall.SyscallN(pGetString, uintptr(name))
	return cString(r)
}

func GetStringi(name, index uint32) string {
	r, _, _ := syscall.SyscallN(pGetStringi, uintptr(name), uintptr(index))
	return cString(r)
}

// cString reads the NUL-terminated string at the address r returned by
// glGetString; GL owns the memory.
func cString(r uintptr) string {
	return windows.BytePtrToString(*(**byte)(unsafe.Pointer(&r)))
}

func GetShaderi(s, pname uint32) int32 {
	var v int32
	syscall.SyscallN(pGetShaderiv, uintptr(s), uintptr(pname), uintptr(unsafe.Pointer(&v)))
	return v
}

func GetProgrami(p, pname uint32) int32 {
	var v int32
	syscall.SyscallN(pGetProgramiv, uintptr(p), uintptr(pname), uintptr(unsafe.Pointer(&v)))
	return v
}

func GetShaderInfoLog(s uint32) string {
	n := GetShaderi(s, INFO_LOG_LENGTH)
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	syscall.SyscallN(pGetShaderInfoLog, uintptr(s), uintptr(n), 0, uintptr(unsafe.Pointer(&buf[0])))
	return windows.ByteSliceToString(buf)
}

func GetProgramInfoLog(p uint32) string {
	n := GetProgrami(p, INFO_LOG_LENGTH)
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	syscall.SyscallN(pGetProgramInfoLog, uintptr(p), uintptr(n), 0, uintptr(unsafe.Pointer(&buf[0])))
	return windows.ByteSliceToString(buf)
}

func GetUniformLocation(p uint32, name string) int32 {
	cname, err := windows.BytePtrFromString(name)
	if err != nil {
		return -1
	}
	r, _, _ := syscall.SyscallN(pGetUniformLocation, uintptr(p), uintptr(unsafe.Pointer(cname)))
	return int32(r)
}

func RenderbufferStorage(target, internalFormat uint32, w, h int) {
	syscall.SyscallN(pRenderbufferStorage, uintptr(target), uintptr(internalFormat), uintptr(w), uintptr(h))
}

func ShaderSource(s uint32, src string) {
	csrc, err := windows.BytePtrFromString(src)
	if err != nil {
		panic(fmt.Sprintf("gles: shader source contains NUL: %v", err))
	}
	syscall.SyscallN(pShaderSource, uintptr(s), 1, uintptr(unsafe.Pointer(&csrc)), 0)
}

// TexImage2D uploads pixels (nil allocates storage only).
func TexImage2D(target uint32, level int, internalFormat uint32, w, h int, format, typ uint32, pixels []byte) {
	syscall.SyscallN(pTexImage2D, uintptr(target), uintptr(level), uintptr(internalFormat), uintptr(w), uintptr(h), 0,
		uintptr(format), uintptr(typ), uintptr(unsafe.Pointer(unsafe.SliceData(pixels))))
}

func TexParameteri(target, pname uint32, v int32) {
	syscall.SyscallN(pTexParameteri, uintptr(target), uintptr(pname), uintptr(v))
}

func UniformMatrix4fv(loc int32, m *[16]float32) {
	syscall.SyscallN(pUniformMatrix4fv, uintptr(loc), 1, 0, uintptr(unsafe.Pointer(m)))
}

func VertexAttribPointer(index uint32, size int, typ uint32, normalized bool, stride int, offset uintptr) {
	syscall.SyscallN(pVertexAttribPointer, uintptr(index), uintptr(size), uintptr(typ), b2u(normalized), uintptr(stride), offset)
}

// HasClipControl reports whether ANGLE exports glClipControlEXT. The
// extension must also be listed in GL_EXTENSIONS for the context.
func HasClipControl() bool { return pClipControlEXT != 0 }

// ClipControlEXT is glClipControlEXT (GL_EXT_clip_control).
func ClipControlEXT(origin, depth uint32) {
	syscall.SyscallN(pClipControlEXT, uintptr(origin), uintptr(depth))
}

// Extensions lists GL_EXTENSIONS of the current context.
func Extensions() []string {
	n := GetInteger(NUM_EXTENSIONS)
	exts := make([]string, 0, n)
	for i := range uint32(n) {
		exts = append(exts, GetStringi(EXTENSIONS, i))
	}
	return exts
}
