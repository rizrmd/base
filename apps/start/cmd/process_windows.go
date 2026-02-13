//go:build windows

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

// killProcessesOnPorts kills any processes listening on the specified ports
func killProcessesOnPorts(ports ...int) error {
	for _, port := range ports {
		if err := killProcessOnPort(port); err != nil {
			return fmt.Errorf("port %d: %w", port, err)
		}
	}
	return nil
}

// killProcessOnPort finds and kills the process listening on the specified port on Windows
func killProcessOnPort(port int) error {
	// Use netstat to find the PID
	cmd := exec.Command("netstat", "-ano")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	// Get our own PID to avoid killing ourselves
	ourPID := os.Getpid()

	// Parse netstat output to find PID listening on the port
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		// Check if line matches our port (format: TCP 0.0.0.0:PORT ... PID)
		localAddr := fields[1]
		if strings.HasSuffix(localAddr, fmt.Sprintf(":%d", port)) {
			pidStr := fields[len(fields)-1]
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				continue
			}

			// Skip our own process
			if pid == ourPID {
				continue
			}

			// Kill the process using taskkill
			killCmd := exec.Command("taskkill", "/F", "/PID", pidStr)
			if err := killCmd.Run(); err != nil {
				return fmt.Errorf("failed to kill process %d: %w", pid, err)
			}
			fmt.Printf("Killed process %d on port %d\n", pid, port)

			// Give it a moment to gracefully shut down
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}
