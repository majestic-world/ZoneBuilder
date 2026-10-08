// Package egl binds the subset of EGL the Zone Builder needs to create a
// GLES 3.0 context on a Win32 window through ANGLE.
//
// The functions are loaded from libEGL.dll at run time (no cgo), with the
// same search rule Gio uses for its own ANGLE loading
// (LOAD_LIBRARY_SEARCH_DEFAULT_DIRS: the executable's directory first, then
// System32), so this package and Gio's GPU backend end up on the same DLL.
package egl

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// EGL enums used by this package.
const (
	eglSuccess                = 0x3000
	eglAlphaSize              = 0x3021
	eglBlueSize               = 0x3022
	eglGreenSize              = 0x3023
	eglRedSize                = 0x3024
	eglConfigCaveat           = 0x3027
	eglSurfaceType            = 0x3033
	eglNone                   = 0x3038
	eglRenderableType         = 0x3040
	eglVendor                 = 0x3053
	eglVersion                = 0x3054
	eglExtensions             = 0x3055
	eglContextClientVersion   = 0x3098
	eglGLColorspace           = 0x309D
	eglGLColorspaceSRGB       = 0x3089
	eglWindowBit              = 0x0004
	eglOpenGLES2Bit           = 0x0004
	eglOpenGLES3Bit           = 0x0040
	eglPlatformAngle          = 0x3202
	eglPlatformAngleType      = 0x3203
	eglPlatformAngleTypeD3D11 = 0x3208
)

var (
	libEGL windows.DLL

	procChooseConfig        *windows.Proc
	procCreateContext       *windows.Proc
	procCreateWindowSurface *windows.Proc
	procDestroyContext      *windows.Proc
	procDestroySurface      *windows.Proc
	procGetDisplay          *windows.Proc
	procGetError            *windows.Proc
	procGetProcAddress      *windows.Proc
	procInitialize          *windows.Proc
	procMakeCurrent         *windows.Proc
	procQueryString         *windows.Proc
	procSwapBuffers         *windows.Proc
	procSwapInterval        *windows.Proc
	procTerminate           *windows.Proc
	procWaitClient          *windows.Proc

	// eglGetPlatformDisplayEXT is an extension entry point; resolved through
	// eglGetProcAddress and zero when ANGLE does not offer it.
	getPlatformDisplayEXT uintptr
)

var loadOnce = sync.OnceValue(load)

// Load loads libEGL.dll and resolves its entry points. It is idempotent.
func Load() error { return loadOnce() }

func load() error {
	const name = "libEGL.dll"
	h, err := windows.LoadLibraryEx(name, 0, windows.LOAD_LIBRARY_SEARCH_DEFAULT_DIRS)
	if err != nil {
		return fmt.Errorf("egl: load %s (it must sit next to the executable): %w", name, err)
	}
	libEGL = windows.DLL{Name: name, Handle: h}
	procs := []struct {
		name string
		p    **windows.Proc
	}{
		{"eglChooseConfig", &procChooseConfig},
		{"eglCreateContext", &procCreateContext},
		{"eglCreateWindowSurface", &procCreateWindowSurface},
		{"eglDestroyContext", &procDestroyContext},
		{"eglDestroySurface", &procDestroySurface},
		{"eglGetDisplay", &procGetDisplay},
		{"eglGetError", &procGetError},
		{"eglGetProcAddress", &procGetProcAddress},
		{"eglInitialize", &procInitialize},
		{"eglMakeCurrent", &procMakeCurrent},
		{"eglQueryString", &procQueryString},
		{"eglSwapBuffers", &procSwapBuffers},
		{"eglSwapInterval", &procSwapInterval},
		{"eglTerminate", &procTerminate},
		{"eglWaitClient", &procWaitClient},
	}
	for _, p := range procs {
		if *p.p, err = libEGL.FindProc(p.name); err != nil {
			return fmt.Errorf("egl: %s: %w", name, err)
		}
	}
	getPlatformDisplayEXT = GetProcAddress("eglGetPlatformDisplayEXT")
	return nil
}

// GetProcAddress returns the address of an EGL or GLES extension entry
// point, or 0 when the implementation does not know it.
func GetProcAddress(name string) uintptr {
	cname, err := windows.BytePtrFromString(name)
	if err != nil {
		return 0
	}
	r, _, _ := syscall.SyscallN(procGetProcAddress.Addr(), uintptr(unsafe.Pointer(cname)))
	return r
}

// Context is an EGL display, config, GLES 3 context and window surface bound
// to one HWND. All methods except Release must run on the thread that calls
// MakeCurrent.
type Context struct {
	disp, cfg, ctx, surf uintptr
	// SRGB reports whether the window surface was created with an sRGB
	// colour space, so blits into it must come from an sRGB source to keep
	// colours unchanged.
	SRGB bool
}

// NewContext initialises ANGLE on its D3D11 backend and creates a GLES 3.0
// context with a window surface for hwnd. The surface has no depth buffer:
// the 3D viewport renders into its own framebuffer (see ADR 0001).
func NewContext(hwnd uintptr) (*Context, error) {
	if err := Load(); err != nil {
		return nil, err
	}
	disp, err := display()
	if err != nil {
		return nil, err
	}
	var major, minor int32
	if r, _, _ := syscall.SyscallN(procInitialize.Addr(), disp, uintptr(unsafe.Pointer(&major)), uintptr(unsafe.Pointer(&minor))); r == 0 {
		return nil, fmt.Errorf("egl: eglInitialize: %w", lastError())
	}
	c := &Context{disp: disp}
	c.SRGB = HasExtension(c.QueryString(eglExtensions), "EGL_KHR_gl_colorspace")
	attribs := []int32{
		eglRenderableType, eglOpenGLES3Bit,
		eglSurfaceType, eglWindowBit,
		eglRedSize, 8,
		eglGreenSize, 8,
		eglBlueSize, 8,
		eglAlphaSize, 8,
		eglConfigCaveat, eglNone,
		eglNone,
	}
	var n int32
	r, _, _ := syscall.SyscallN(procChooseConfig.Addr(), disp, uintptr(unsafe.Pointer(&attribs[0])), uintptr(unsafe.Pointer(&c.cfg)), 1, uintptr(unsafe.Pointer(&n)))
	if r == 0 {
		return nil, fmt.Errorf("egl: eglChooseConfig: %w", lastError())
	}
	if n == 0 {
		return nil, errors.New("egl: no RGBA8 GLES 3 window config")
	}
	ctxAttribs := []int32{eglContextClientVersion, 3, eglNone}
	c.ctx, _, _ = syscall.SyscallN(procCreateContext.Addr(), disp, c.cfg, 0, uintptr(unsafe.Pointer(&ctxAttribs[0])))
	if c.ctx == 0 {
		return nil, fmt.Errorf("egl: eglCreateContext: %w", lastError())
	}
	surfAttribs := []int32{eglNone}
	if c.SRGB {
		surfAttribs = []int32{eglGLColorspace, eglGLColorspaceSRGB, eglNone}
	}
	c.surf, _, _ = syscall.SyscallN(procCreateWindowSurface.Addr(), disp, c.cfg, hwnd, uintptr(unsafe.Pointer(&surfAttribs[0])))
	if c.surf == 0 {
		err := lastError()
		c.Release()
		return nil, fmt.Errorf("egl: eglCreateWindowSurface: %w", err)
	}
	return c, nil
}

func display() (uintptr, error) {
	if getPlatformDisplayEXT != 0 {
		attrs := []int32{eglPlatformAngleType, eglPlatformAngleTypeD3D11, eglNone}
		d, _, _ := syscall.SyscallN(getPlatformDisplayEXT, eglPlatformAngle, 0, uintptr(unsafe.Pointer(&attrs[0])))
		if d != 0 {
			return d, nil
		}
	}
	d, _, _ := syscall.SyscallN(procGetDisplay.Addr(), 0)
	if d == 0 {
		return 0, fmt.Errorf("egl: no display: %w", lastError())
	}
	return d, nil
}

// MakeCurrent binds the context and surface to the calling OS thread.
func (c *Context) MakeCurrent() error {
	if r, _, _ := syscall.SyscallN(procMakeCurrent.Addr(), c.disp, c.surf, c.surf, c.ctx); r == 0 {
		return fmt.Errorf("egl: eglMakeCurrent: %w", lastError())
	}
	return nil
}

// SwapBuffers presents the back buffer.
func (c *Context) SwapBuffers() error {
	if r, _, _ := syscall.SyscallN(procSwapBuffers.Addr(), c.disp, c.surf); r == 0 {
		return fmt.Errorf("egl: eglSwapBuffers: %w", lastError())
	}
	return nil
}

// SetSwapInterval sets the vsync interval (1 = wait for vblank).
func (c *Context) SetSwapInterval(n int) {
	syscall.SyscallN(procSwapInterval.Addr(), c.disp, uintptr(n))
}

// WaitClient lets ANGLE notice a window resize before the frame is drawn, so
// the back buffer matches the new client area.
func (c *Context) WaitClient() {
	syscall.SyscallN(procWaitClient.Addr())
}

// QueryString returns an EGL display string (eglQueryString).
func (c *Context) QueryString(name int32) string {
	r, _, _ := syscall.SyscallN(procQueryString.Addr(), c.disp, uintptr(name))
	return cString(r)
}

// Describe returns the EGL vendor and version, for the startup log.
func (c *Context) Describe() string {
	return c.QueryString(eglVendor) + " EGL " + c.QueryString(eglVersion)
}

// Release unbinds and destroys the surface and context. Safe to call on a
// partially created context.
func (c *Context) Release() {
	if c.disp == 0 {
		return
	}
	syscall.SyscallN(procMakeCurrent.Addr(), c.disp, 0, 0, 0)
	if c.surf != 0 {
		syscall.SyscallN(procDestroySurface.Addr(), c.disp, c.surf)
	}
	if c.ctx != 0 {
		syscall.SyscallN(procDestroyContext.Addr(), c.disp, c.ctx)
	}
	*c = Context{}
}

// HasExtension reports whether the space-separated list exts contains ext.
func HasExtension(exts, ext string) bool {
	for e := range strings.FieldsSeq(exts) {
		if e == ext {
			return true
		}
	}
	return false
}

// cString reads the NUL-terminated string at the address r returned by a
// DLL call. EGL owns the memory; it stays valid for the display's lifetime.
func cString(r uintptr) string {
	return windows.BytePtrToString(*(**byte)(unsafe.Pointer(&r)))
}

func lastError() error {
	r, _, _ := syscall.SyscallN(procGetError.Addr())
	if r == eglSuccess {
		return errors.New("unknown EGL error")
	}
	return fmt.Errorf("EGL error 0x%x", r)
}
