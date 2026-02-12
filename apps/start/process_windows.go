//go:build windows

package main

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcessGroup sets up a new process group for proper signal handling on Windows
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// killProcess terminates the process on Windows
func killProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	// On Windows, use Kill to force terminate
	cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		cmd.Process.Kill()
		cmd.Wait()
	}
}
