package ui

import (
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	comdlg32             = windows.NewLazySystemDLL("comdlg32.dll")
	procGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
	procGetSaveFileNameW = comdlg32.NewProc("GetSaveFileNameW")
)

// openFileName is OPENFILENAMEW.
type openFileName struct {
	structSize    uint32
	owner         uintptr
	instance      uintptr
	filter        *uint16
	customFilter  *uint16
	maxCustFilter uint32
	filterIndex   uint32
	file          *uint16
	maxFile       uint32
	fileTitle     *uint16
	maxFileTitle  uint32
	initialDir    *uint16
	title         *uint16
	flags         uint32
	fileOffset    uint16
	fileExtension uint16
	defExt        *uint16
	custData      uintptr
	hook          uintptr
	templateName  *uint16
	reserved      uintptr
	reserved2     uint32
	flagsEx       uint32
}

// FileType is the file kind a file dialog lists: its description and its
// extension with the dot (".zbproj").
type FileType struct {
	Name, Ext string
}

// PickOpenFile shows the Windows open-file dialog for files of type t,
// starting at start (a file or a folder), and returns the chosen file, or
// ok = false when the user cancels. It blocks until the dialog closes, so
// call it off the window's event loop.
func PickOpenFile(title, start string, t FileType) (path string, ok bool) {
	const ofnFileMustExist = 0x1000
	return pickFile(procGetOpenFileNameW, title, start, t, ofnFileMustExist)
}

// PickSaveFile is PickOpenFile for a file to write: it asks before
// replacing an existing file and adds t's extension when the name has
// none.
func PickSaveFile(title, start string, t FileType) (path string, ok bool) {
	const ofnOverwritePrompt = 0x2
	return pickFile(procGetSaveFileNameW, title, start, t, ofnOverwritePrompt)
}

func pickFile(proc *windows.LazyProc, title, start string, t FileType, flags uint32) (string, bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err == nil {
		defer windows.CoUninitialize()
	}
	const (
		ofnHideReadOnly  = 0x4
		ofnNoChangeDir   = 0x8
		ofnPathMustExist = 0x800
		ofnExplorer      = 0x80000
	)
	// The filter is pairs of NUL-terminated strings ending in an empty one
	// (UTF16FromString refuses the inner NULs).
	pattern := "*" + t.Ext
	filter := utf16.Encode([]rune(t.Name + " (" + pattern + ")\x00" + pattern + "\x00\x00"))
	file := make([]uint16, 4096)
	var dir string
	if start != "" {
		start = filepath.Clean(filepath.FromSlash(start))
		if strings.EqualFold(filepath.Ext(start), t.Ext) {
			dir = filepath.Dir(start)
			copy(file[:len(file)-1], windows.StringToUTF16(filepath.Base(start)))
		} else {
			dir = start
		}
	}
	titlePtr, _ := windows.UTF16PtrFromString(title)
	defExt, _ := windows.UTF16PtrFromString(strings.TrimPrefix(t.Ext, "."))
	ofn := openFileName{
		filter:  &filter[0],
		file:    &file[0],
		maxFile: uint32(len(file)),
		title:   titlePtr,
		flags:   flags | ofnHideReadOnly | ofnNoChangeDir | ofnPathMustExist | ofnExplorer,
		defExt:  defExt,
	}
	ofn.structSize = uint32(unsafe.Sizeof(ofn))
	if dir != "" {
		ofn.initialDir, _ = windows.UTF16PtrFromString(dir)
	}
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return "", false
	}
	return windows.UTF16ToString(file), true
}
