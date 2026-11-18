package main

import (
	"os"
	"syscall"
)

// attachConsole connects stdout/stderr to the launching terminal. The release
// build is a GUI-subsystem exe (-H windowsgui), which otherwise has no console,
// so --install / --version output would be lost.
func attachConsole() {
	const attachParentProcess = ^uintptr(0) // ATTACH_PARENT_PROCESS (DWORD -1)
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	if r, _, _ := proc.Call(attachParentProcess); r == 0 {
		return // already has a console, or launched without one
	}
	if out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout, os.Stderr = out, out
	}
}

func isRoot() bool { return false }
