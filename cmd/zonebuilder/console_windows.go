package main

import (
	"log"
	"os"
	"syscall"
)

// The release build links with -H windowsgui (scripts/build.ps1), so Windows
// opens no console window for the app. Without one, stderr is invalid and the
// log is lost; when the app is started from a terminal, attach to that
// terminal's console so the log and flag errors still show up there.
// Inherited handles (pipes, redirected files) are kept as they are.
func init() {
	// A GUI process started from a console can get a stale handle value with
	// no file behind it (GetFileType: FILE_TYPE_UNKNOWN); only real pipes and
	// files count as inherited.
	if h, err := syscall.GetStdHandle(syscall.STD_ERROR_HANDLE); err == nil {
		if t, err := syscall.GetFileType(h); err == nil && t != syscall.FILE_TYPE_UNKNOWN {
			return
		}
	}
	const attachParentProcess = uintptr(^uint32(0)) // ATTACH_PARENT_PROCESS, (DWORD)-1
	attach := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	if ok, _, _ := attach.Call(attachParentProcess); ok == 0 {
		return
	}
	// Read access lets GetConsoleMode succeed, so os writes through
	// WriteConsoleW and accents survive the console code page.
	out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		return
	}
	os.Stdout, os.Stderr = out, out
	log.SetOutput(out)
}
