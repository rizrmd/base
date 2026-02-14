//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// setProcessGroup sets up a new process group for proper signal handling on Unix
// Creates a new session to isolate child processes from terminal signals
func setProcessGroup(cmd *exec.Cmd) {
	// Create a new session for child processes
	// This isolates them from Ctrl+C (SIGINT) sent to the parent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}

// killProcess sends SIGINT to gracefully terminate the process, then SIGKILL if needed
// Using SIGINT instead of SIGTERM avoids exit code 143 errors from bun.
func killProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	// Send SIGINT (like Ctrl+C) for graceful shutdown
	// Processes handle SIGINT more gracefully than SIGTERM
	cmd.Process.Signal(syscall.SIGINT)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// Process exited
	case <-time.After(2 * time.Second):
		// Process didn't exit, force kill with SIGKILL
		cmd.Process.Kill()
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

	// Get our own PID to avoid killing ourselves
	ourPID := os.Getpid()

	for _, pidStr := range pidStrings {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// Skip our own process
		if pid == ourPID {
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
