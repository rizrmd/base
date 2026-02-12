//go:build !windows

package main

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcessGroup sets up a new process group for proper signal handling on Unix
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// killProcess sends SIGTERM to the process group, then SIGKILL if needed
func killProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		cmd.Process.Signal(syscall.SIGTERM)
		return
	}

	// Kill the entire process group
	syscall.Kill(-pgid, syscall.SIGTERM)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		syscall.Kill(-pgid, syscall.SIGKILL)
		cmd.Wait()
	}
}
