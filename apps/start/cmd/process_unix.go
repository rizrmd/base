//go:build !windows

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
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

// killProcessesOnPorts kills any processes listening on the specified ports
func killProcessesOnPorts(ports ...int) error {
	for _, port := range ports {
		if err := killProcessOnPort(port); err != nil {
			return fmt.Errorf("port %d: %w", port, err)
		}
	}
	return nil
}

// killProcessOnPort finds and kills the process listening on the specified port
func killProcessOnPort(port int) error {
	// Find process ID using lsof
	cmd := exec.Command("lsof", "-t", "-i", fmt.Sprintf(":%d", port))
	output, err := cmd.CombinedOutput()
	if err != nil {
		// No process found on port (lsof returns non-zero when nothing found)
		return nil
	}

	// Get PIDs from output (may be multiple)
	pidStrings := strings.Fields(strings.TrimSpace(string(output)))
	if len(pidStrings) == 0 {
		return nil
	}

	for _, pidStr := range pidStrings {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// Kill the process
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			if err == syscall.ESRCH {
				// Process already gone
				continue
			}
			return fmt.Errorf("failed to kill process %d: %w", pid, err)
		}
		fmt.Printf("Killed process %d on port %d\n", pid, port)

		// Give it a moment to gracefully shut down
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}
