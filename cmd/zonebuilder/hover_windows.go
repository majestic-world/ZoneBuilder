package main

import (
	"sync/atomic"

	"golang.org/x/sys/windows"
)

// Gio's Windows backend calls SetFocus on every pointer message while its
// window is not focused (os_windows.go, pointerUpdate). Moving the real
// mouse over the app makes its thread the one that received the last
// input, so Windows lets that SetFocus activate the window: hovering the
// app took the focus from the Task Manager, from the app's own file
// dialog, from any other window. hoverGuard subclasses the window and
// drops the pointer moves Gio would see while another window is in the
// foreground. Clicks still go through and activate the app as usual; only
// hover feedback waits for the focus.

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	procSetWindowLongPtr = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProc   = user32.NewProc("CallWindowProcW")

	// hoverGuarded is the subclassed window and hoverNext Gio's window
	// procedure, which every other message goes to.
	hoverGuarded, hoverNext atomic.Uintptr
	hoverProc               = windows.NewCallback(guardHover)
)

const (
	gwlpWndProc     = ^uintptr(3) // GWLP_WNDPROC, -4
	wmPointerUpdate = 0x0245
)

// guardHover is the subclass procedure: it drops WM_POINTERUPDATE while
// hwnd is not the foreground window and passes everything else to Gio.
func guardHover(hwnd, msg, wParam, lParam uintptr) uintptr {
	if msg == wmPointerUpdate && uintptr(windows.GetForegroundWindow()) != hwnd {
		return 0
	}
	r, _, _ := procCallWindowProc.Call(hoverNext.Load(), hwnd, msg, wParam, lParam)
	return r
}

// guardHoverFocus subclasses hwnd, the app's window, once. It must run on
// the window's thread (Window.Run), so no message reaches guardHover
// before hoverNext is set.
func guardHoverFocus(hwnd uintptr) {
	if hwnd == 0 || hoverGuarded.Load() == hwnd {
		return
	}
	prev, _, _ := procSetWindowLongPtr.Call(hwnd, gwlpWndProc, hoverProc)
	if prev == 0 {
		return
	}
	hoverNext.Store(prev)
	hoverGuarded.Store(hwnd)
}
