package main

import (
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

var (
	env  = "dev" // overridden at build time via ldflags
	name = ""
)

// Config holds runtime configuration
type Config struct {
	FrontendPort int
	EncorePort   int
}

// getPortOffset returns a unique offset based on user identity + project path
func getPortOffset(rootDir string) int {
	h := fnv.New32a()

	uid := os.Getuid()
	if uid == -1 {
		h.Write([]byte(os.Getenv("USERNAME")))
	} else {
		h.Write([]byte(strconv.Itoa(uid)))
	}

	h.Write([]byte(rootDir))

	return int(h.Sum32() % 1000)
}

func getConfig(rootDir string) *Config {
	portOffset := getPortOffset(rootDir)

	cfg := &Config{
		FrontendPort: 5173 + portOffset,
		EncorePort:   4000 + portOffset,
	}

	if port := os.Getenv("FRONTEND_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.FrontendPort = p
		}
	}
	if port := os.Getenv("ENCORE_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.EncorePort = p
		}
	}

	return cfg
}

func main() {
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting executable path: %v\n", err)
		os.Exit(1)
	}

	rootDir := filepath.Dir(exePath)
	appsDir := filepath.Join(rootDir, "apps")
	frontendDir := filepath.Join(appsDir, "frontend")

	if _, err := os.Stat(appsDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: apps directory not found at %s\n", appsDir)
		os.Exit(1)
	}

	cfg := getConfig(rootDir)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var frontendCmd *exec.Cmd
	var encoreCmd *exec.Cmd

	if env == "prod" {
		fmt.Printf("Building %s frontend app...\n", name)

		if err := buildFrontendApp(frontendDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error building frontend app: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Starting %s in production mode (port %d)...\n", name, cfg.EncorePort)
		encoreCmd = exec.Command("encore", "run", "--env", "production",
			"--port", strconv.Itoa(cfg.EncorePort),
			"--browser=never",
		)
	} else {
		fmt.Printf("Starting %s in development mode:\n", name)
		fmt.Printf("  Frontend: http://localhost:%d\n", cfg.FrontendPort)
		fmt.Printf("  API:      http://localhost:%d\n", cfg.EncorePort)

		frontendCmd = startFrontendDevServer(frontendDir, cfg.FrontendPort, cfg.EncorePort)

		encoreCmd = exec.Command("encore", "run",
			"--port", strconv.Itoa(cfg.EncorePort),
			"--browser=never",
		)
	}

	encoreCmd.Env = append(os.Environ(),
		fmt.Sprintf("FRONTEND_PORT=%d", cfg.FrontendPort),
		fmt.Sprintf("ENCORE_PORT=%d", cfg.EncorePort),
	)

	encoreCmd.Dir = appsDir
	encoreCmd.Stdout = os.Stdout
	encoreCmd.Stderr = os.Stderr
	encoreCmd.Stdin = os.Stdin

	// Set platform-specific process attributes
	setProcessGroup(encoreCmd)

	if err := encoreCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting encore: %v\n", err)
		killProcess(frontendCmd)
		os.Exit(1)
	}

	<-sigChan
	fmt.Println("\nShutting down...")

	killProcess(frontendCmd)
	killProcess(encoreCmd)

	time.Sleep(500 * time.Millisecond)
}

func buildFrontendApp(frontendDir string) error {
	if _, err := os.Stat(frontendDir); os.IsNotExist(err) {
		return fmt.Errorf("frontend directory not found at %s", frontendDir)
	}

	if _, err := os.Stat(filepath.Join(frontendDir, "node_modules")); os.IsNotExist(err) {
		installCmd := exec.Command("npm", "install")
		installCmd.Dir = frontendDir
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("npm install failed: %w", err)
		}
	}

	buildCmd := exec.Command("npm", "run", "build")
	buildCmd.Dir = frontendDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("npm build failed: %w", err)
	}

	return nil
}

func startFrontendDevServer(frontendDir string, port int, encorePort int) *exec.Cmd {
	cmd := exec.Command("npm", "run", "dev", "--", "--port", strconv.Itoa(port))
	cmd.Dir = frontendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Pass ENCORE_PORT for Vite HMR configuration
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("ENCORE_PORT=%d", encorePort),
	)

	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to start frontend dev server: %v\n", err)
		return nil
	}

	fmt.Printf("Frontend dev server started on port %d (PID: %d)\n", port, cmd.Process.Pid)
	return cmd
}
