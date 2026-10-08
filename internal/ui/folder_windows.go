package ui

import (
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32                = windows.NewLazySystemDLL("shell32.dll")
	procSHBrowseForFolderW = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDW   = shell32.NewProc("SHGetPathFromIDListW")
	user32                 = windows.NewLazySystemDLL("user32.dll")
	procSendMessageW       = user32.NewProc("SendMessageW")

	// browseCallback selects the starting folder once the dialog is up. One
	// callback for the process: syscall.NewCallback slots are never freed.
	browseCallback = syscall.NewCallback(func(hwnd, msg, _, data uintptr) uintptr {
		const bffmInitialized, bffmSetSelectionW = 1, 0x467
		if msg == bffmInitialized && data != 0 {
			procSendMessageW.Call(hwnd, bffmSetSelectionW, 1, data)
		}
		return 0
	})
)

// browseInfo is BROWSEINFOW.
type browseInfo struct {
	owner       uintptr
	root        uintptr
	displayName *uint16
	title       *uint16
	flags       uint32
	callback    uintptr
	lParam      uintptr
	image       int32
}

// PickFolder shows the Windows folder picker starting at start and returns
// the chosen folder, or ok = false when the user cancels. It blocks until
// the dialog closes, so call it off the window's event loop.
func PickFolder(title, start string) (path string, ok bool) {
	// The dialog needs a single-threaded COM apartment on its own thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err == nil {
		defer windows.CoUninitialize()
	}
	const (
		bifReturnOnlyFSDirs = 0x1
		bifEditBox          = 0x10
		bifNewDialogStyle   = 0x40
	)
	display := make([]uint16, windows.MAX_PATH)
	titlePtr, _ := windows.UTF16PtrFromString(title)
	var startPtr *uint16
	if start != "" {
		// The shell only selects a path spelled with backslashes.
		startPtr, _ = windows.UTF16PtrFromString(filepath.Clean(filepath.FromSlash(start)))
	}
	bi := browseInfo{
		displayName: &display[0],
		title:       titlePtr,
		flags:       bifReturnOnlyFSDirs | bifEditBox | bifNewDialogStyle,
		callback:    browseCallback,
		lParam:      uintptr(unsafe.Pointer(startPtr)),
	}
	pidl, _, _ := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	runtime.KeepAlive(startPtr)
	if pidl == 0 {
		return "", false
	}
	defer windows.CoTaskMemFree(*(*unsafe.Pointer)(unsafe.Pointer(&pidl)))
	buf := make([]uint16, windows.MAX_PATH)
	if r, _, _ := procSHGetPathFromIDW.Call(pidl, uintptr(unsafe.Pointer(&buf[0]))); r == 0 {
		return "", false
	}
	return windows.UTF16ToString(buf), true
}
