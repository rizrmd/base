package main

import (
	"bufio"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	env     = "dev" // overridden at build time via ldflags
	version = "1.0.0"
	name    = ""
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

	// Parse subcommand
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "upgrade":
			runUpgrade(rootDir, os.Args[2:])
			return
		case "version", "-v", "--version":
			fmt.Printf("%s version %s (env: %s)\n", name, version, env)
			return
		case "help", "-h", "--help":
			printHelp()
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
			printHelp()
			os.Exit(1)
		}
	}

	// Default: run the app
	runApp(rootDir)
}

func printHelp() {
	fmt.Printf("Usage: %s [command]\n\n", filepath.Base(os.Args[0]))
	fmt.Println("Commands:")
	fmt.Println("  (none)    Start the application (default)")
	fmt.Println("  upgrade   Upgrade base template to latest version")
	fmt.Println("  version   Show version information")
	fmt.Println("  help      Show this help message")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --dry-run    Preview upgrade changes without applying")
}

// loadEnvFile reads and parses the .env file at the given path
// Returns a map of environment variables, or nil if the file doesn't exist
func loadEnvFile(envPath string) map[string]string {
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return nil
	}

	f, err := os.Open(envPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	envMap := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE format
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])

			// Remove quotes if present
			if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
				value = value[1 : len(value)-1]
			}

			envMap[key] = value
		}
	}

	if len(envMap) > 0 {
		fmt.Printf("Loaded %d environment variables from %s\n", len(envMap), envPath)
	}

	return envMap
}

// mergeEnv merges existing environment variables with .env file variables
// .env variables take precedence over existing ones
func mergeEnv(envVars []string, envFile map[string]string) []string {
	if envFile == nil {
		return envVars
	}

	// Create a map from existing environment
	existing := make(map[string]string)
	for _, envVar := range envVars {
		if idx := strings.Index(envVar, "="); idx > 0 {
			existing[envVar[:idx]] = envVar[idx+1:]
		}
	}

	// Merge with .env (overriding existing values)
	for key, value := range envFile {
		existing[key] = value
	}

	// Convert back to slice
	result := make([]string, 0, len(existing))
	for key, value := range existing {
		result = append(result, fmt.Sprintf("%s=%s", key, value))
	}

	return result
}

func runApp(rootDir string) {
	appsDir := filepath.Join(rootDir, "apps")
	frontendDir := filepath.Join(appsDir, "frontend")

	if _, err := os.Stat(appsDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: apps directory not found at %s\n", appsDir)
		os.Exit(1)
	}

	cfg := getConfig(rootDir)

	// Load root .env file if it exists
	envFile := loadEnvFile(filepath.Join(rootDir, ".env"))

	// Kill any processes using our ports before starting
	if err := killProcessesOnPorts(cfg.FrontendPort, cfg.EncorePort); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to kill existing processes: %v\n", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var frontendCmd *exec.Cmd
	var encoreCmd *exec.Cmd

	if env == "prod" {
		fmt.Printf("Building %s frontend app...\n", name)

		if err := buildFrontendApp(frontendDir, envFile); err != nil {
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

		frontendCmd = startFrontendDevServer(frontendDir, cfg.FrontendPort, cfg.EncorePort, envFile)

		encoreCmd = exec.Command("encore", "run",
			"--port", strconv.Itoa(cfg.EncorePort),
			"--browser=never",
		)
	}

	encoreCmd.Env = mergeEnv(os.Environ(), envFile)
	encoreCmd.Env = append(encoreCmd.Env,
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

func buildFrontendApp(frontendDir string, envFile map[string]string) error {
	if _, err := os.Stat(frontendDir); os.IsNotExist(err) {
		return fmt.Errorf("frontend directory not found at %s", frontendDir)
	}

	if _, err := os.Stat(filepath.Join(frontendDir, "node_modules")); os.IsNotExist(err) {
		installCmd := exec.Command("bun", "install")
		installCmd.Dir = frontendDir
		installCmd.Env = mergeEnv(os.Environ(), envFile)
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("bun install failed: %w", err)
		}
	}

	buildCmd := exec.Command("bun", "run", "build")
	buildCmd.Dir = frontendDir
	buildCmd.Env = mergeEnv(os.Environ(), envFile)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("bun build failed: %w", err)
	}

	return nil
}

func startFrontendDevServer(frontendDir string, port int, encorePort int, envFile map[string]string) *exec.Cmd {
	// Install dependencies if node_modules doesn't exist
	if _, err := os.Stat(filepath.Join(frontendDir, "node_modules")); os.IsNotExist(err) {
		fmt.Println("Installing frontend dependencies...")
		installCmd := exec.Command("bun", "install")
		installCmd.Dir = frontendDir
		installCmd.Env = mergeEnv(os.Environ(), envFile)
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: bun install failed: %v\n", err)
			os.Exit(1)
		}
	}

	cmd := exec.Command("bun", "run", "dev", "--port", strconv.Itoa(port))
	cmd.Dir = frontendDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Merge .env variables with current environment
	cmd.Env = mergeEnv(os.Environ(), envFile)
	cmd.Env = append(cmd.Env, fmt.Sprintf("ENCORE_PORT=%d", encorePort))

	setProcessGroup(cmd)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to start frontend dev server: %v\n", err)
		return nil
	}

	fmt.Printf("Frontend dev server started on port %d (PID: %d)\n", port, cmd.Process.Pid)
	return cmd
}
