package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
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

func getConfig() *Config {
	// Generate unique port offset based on user's UID
	// This allows multiple users to run on the same machine without conflicts
	uid := os.Getuid()
	portOffset := uid % 1000 // Offset between 0-999

	cfg := &Config{
		FrontendPort: 5173 + portOffset,
		EncorePort:   4000 + portOffset,
	}

	// Allow manual override via environment variables
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
	// Get the directory where the binary is located
	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting executable path: %v\n", err)
		os.Exit(1)
	}

	rootDir := filepath.Dir(exePath)
	appsDir := filepath.Join(rootDir, "apps")
	frontendDir := filepath.Join(appsDir, "frontend")

	// Verify apps directory exists
	if _, err := os.Stat(appsDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: apps directory not found at %s\n", appsDir)
		os.Exit(1)
	}

	cfg := getConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var frontendCmd *exec.Cmd
	var encoreCmd *exec.Cmd

	if env == "prod" {
		fmt.Printf("Building %s frontend app...\n", name)

		// Build frontend app first
		if err := buildFrontendApp(ctx, frontendDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error building frontend app: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Starting %s in production mode (port %d)...\n", name, cfg.EncorePort)
		encoreCmd = exec.CommandContext(ctx, "encore", "run", "--env", "production", "--port", strconv.Itoa(cfg.EncorePort))
	} else {
		fmt.Printf("Starting %s in development mode (frontend: %d, encore: %d)...\n", name, cfg.FrontendPort, cfg.EncorePort)

		// Start frontend dev server in background
		frontendCmd = startFrontendDevServer(ctx, frontendDir, cfg.FrontendPort)

		// Start Encore
		encoreCmd = exec.CommandContext(ctx, "encore", "run", "--port", strconv.Itoa(cfg.EncorePort))
	}

	// Pass port config to Encore subprocess
	encoreCmd.Env = append(os.Environ(),
		fmt.Sprintf("FRONTEND_PORT=%d", cfg.FrontendPort),
		fmt.Sprintf("ENCORE_PORT=%d", cfg.EncorePort),
	)

	encoreCmd.Dir = appsDir
	encoreCmd.Stdout = os.Stdout
	encoreCmd.Stderr = os.Stderr
	encoreCmd.Stdin = os.Stdin

	// Start Encore
	if err := encoreCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting encore: %v\n", err)
		cancel()
		os.Exit(1)
	}

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down...")

	cancel()

	// Cleanup processes
	if frontendCmd != nil && frontendCmd.Process != nil {
		frontendCmd.Process.Signal(syscall.SIGTERM)
	}
	if encoreCmd.Process != nil {
		encoreCmd.Process.Signal(syscall.SIGTERM)
	}

	// Wait for processes to exit
	if frontendCmd != nil {
		frontendCmd.Wait()
	}
	encoreCmd.Wait()
}

func buildFrontendApp(ctx context.Context, frontendDir string) error {
	// Check if frontend directory exists
	if _, err := os.Stat(frontendDir); os.IsNotExist(err) {
		return fmt.Errorf("frontend directory not found at %s", frontendDir)
	}

	// Install dependencies if needed
	if _, err := os.Stat(filepath.Join(frontendDir, "node_modules")); os.IsNotExist(err) {
		installCmd := exec.CommandContext(ctx, "npm", "install")
		installCmd.Dir = frontendDir
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("npm install failed: %w", err)
		}
	}

	// Build the frontend app
	buildCmd := exec.CommandContext(ctx, "npm", "run", "build")
	buildCmd.Dir = frontendDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("npm build failed: %w", err)
	}

	return nil
}

func startFrontendDevServer(ctx context.Context, frontendDir string, port int) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "npm", "run", "dev", "--", "--port", strconv.Itoa(port))
	cmd.Dir = frontendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to start frontend dev server: %v\n", err)
		return nil
	}

	fmt.Printf("Frontend dev server started on port %d (PID: %d)\n", port, cmd.Process.Pid)
	return cmd
}
