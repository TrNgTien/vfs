//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	modkernel32                      = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess                  = modkernel32.NewProc("OpenProcess")
	procGetExitCodeProcess           = modkernel32.NewProc("GetExitCodeProcess")
	procCloseHandle                  = modkernel32.NewProc("CloseHandle")
)

const (
	processQueryLimitedInformation = 0x1000
	stillActive                    = 259
)

func isRunning(pid int) bool {
	h, _, err := procOpenProcess.Call(
		processQueryLimitedInformation, 0, uintptr(pid),
	)
	if h == 0 || err != nil && err != syscall.Errno(0) {
		return false
	}
	defer procCloseHandle.Call(h)

	var exitCode uint32
	ret, _, _ := procGetExitCodeProcess.Call(h, uintptr(unsafe.Pointer(&exitCode)))
	if ret == 0 {
		return false
	}
	return exitCode == stillActive
}

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func terminateProcess(proc *os.Process) error {
	return proc.Kill()
}
