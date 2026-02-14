package main

import (
	"bufio"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/KennethanCeyer/ptyx"
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

// ANSI escape code regex - matches all ANSI escape sequences
// Matches: ESC [ ... (CSI sequences), ESC ] ... (OSC sequences), and other ESC sequences
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07\x1b]*[\x07\x1b\\]|\x1b.`)

// stripANSI removes ANSI escape codes from a string
func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// Logger handles writing to both file and console
type Logger struct {
	file   *os.File
	mu     sync.Mutex
	prefix string
}

// NewLogger creates a new logger that writes to both file and stdout/stderr
func NewLogger(logPath, prefix string) (*Logger, error) {
	// Ensure logs directory exists
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Open (or create) log file
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &Logger{
		file:   file,
		prefix: prefix,
	}, nil
}

// Write writes to both file and console
func (l *Logger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data := string(p)

	// Write to file with timestamp (no ANSI codes)
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	cleanData := stripANSI(data)
	logLine := fmt.Sprintf("[%s] %s%s", timestamp, l.prefix, cleanData)

	if _, err := l.file.WriteString(logLine); err != nil {
		return len(p), err
	}

	// Write to console (with ANSI codes for colors)
	return fmt.Print(data)
}

// WriteStderr writes to both file and stderr
func (l *Logger) WriteStderr(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data := string(p)

	// Write to file with timestamp (no ANSI codes)
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	cleanData := stripANSI(data)
	logLine := fmt.Sprintf("[%s] %s%s", timestamp, l.prefix, cleanData)

	if _, err := l.file.WriteString(logLine); err != nil {
		return len(p), err
	}

	// Write to stderr (with ANSI codes for colors)
	return fmt.Fprint(os.Stderr, data)
}

// Close closes the log file
func (l *Logger) Close() error {
	return l.file.Close()
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

// copyPtyOutput copies data from PTY session to both logger and console
func copyPtyOutput(session ptyx.Session, logger *Logger) {
	io.Copy(logger, session.PtyReader())
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

	// Create loggers
	backendLogger, err := NewLogger(filepath.Join(rootDir, "logs", "backend.log"), "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating backend logger: %v\n", err)
		os.Exit(1)
	}
	defer backendLogger.Close()

	frontendLogger, err := NewLogger(filepath.Join(rootDir, "logs", "frontend.log"), "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating frontend logger: %v\n", err)
		os.Exit(1)
	}
	defer frontendLogger.Close()

	ctx := context.Background()
	var frontendSession ptyx.Session
	var encoreSession ptyx.Session

	if env == "prod" {
		fmt.Printf("Building %s frontend app...\n", name)

		if err := buildFrontendApp(frontendDir, envFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error building frontend app: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Starting %s in production mode (port %d)...\n", name, cfg.EncorePort)
	} else {
		fmt.Printf("Starting %s in development mode:\n", name)
		fmt.Printf("  Frontend: http://localhost:%d\n", cfg.FrontendPort)
		fmt.Printf("  API:      http://localhost:%d\n", cfg.EncorePort)

		// Start frontend with PTY
		_, frontendSession, err = startFrontendDevServerWithPTY(ctx, frontendDir, cfg.FrontendPort, cfg.EncorePort, envFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting frontend: %v\n", err)
			os.Exit(1)
		}
		defer frontendSession.Close()

		// Start copying frontend output
		go copyPtyOutput(frontendSession, frontendLogger)
	}

	// Build Encore environment and arguments
	encoreEnv := mergeEnv(os.Environ(), envFile)
	encoreEnv = append(encoreEnv,
		fmt.Sprintf("FRONTEND_PORT=%d", cfg.FrontendPort),
		fmt.Sprintf("ENCORE_PORT=%d", cfg.EncorePort),
	)

	// Build Encore args
	var encoreArgs []string
	if env == "prod" {
		encoreArgs = []string{"run", "--env", "production", "--port", strconv.Itoa(cfg.EncorePort), "--browser=never"}
	} else {
		encoreArgs = []string{"run", "--port", strconv.Itoa(cfg.EncorePort), "--browser=never"}
	}

	// Start Encore with PTY
	encoreSession, err = ptyx.Spawn(ctx, ptyx.SpawnOpts{
		Prog: "encore",
		Args: encoreArgs,
		Env:  encoreEnv,
		Dir:  appsDir,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting encore with PTY: %v\n", err)
		if frontendSession != nil {
			frontendSession.Close()
		}
		os.Exit(1)
	}
	defer encoreSession.Close()

	// Start copying Encore output
	go copyPtyOutput(encoreSession, backendLogger)

	// Wait for encore command to finish or signal to shut down
	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- encoreSession.Wait()
		close(cmdDone)
	}()

	select {
	case <-sigChan:
		fmt.Println("\nShutting down...")
		if frontendSession != nil {
			frontendSession.Kill()
		}
		encoreSession.Kill()
	case err := <-cmdDone:
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nEncore exited: %v\n", err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	// Exit cleanly to avoid signal exit codes
	os.Exit(0)
}

func buildFrontendApp(frontendDir string, envFile map[string]string) error {
	if _, err := os.Stat(frontendDir); os.IsNotExist(err) {
		return fmt.Errorf("frontend directory not found at %s", frontendDir)
	}

	// Create logger for build output
	rootDir := filepath.Dir(filepath.Dir(frontendDir))
	logger, err := NewLogger(filepath.Join(rootDir, "logs", "frontend.log"), "[Frontend Build] ")
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer logger.Close()

	ctx := context.Background()

	if _, err := os.Stat(filepath.Join(frontendDir, "node_modules")); os.IsNotExist(err) {
		fmt.Println("Installing frontend dependencies...")
		session, err := ptyx.Spawn(ctx, ptyx.SpawnOpts{
			Prog: "bun",
			Args: []string{"install"},
			Dir:  frontendDir,
			Env:  mergeEnv(os.Environ(), envFile),
		})
		if err != nil {
			return fmt.Errorf("bun install failed: %w", err)
		}
		defer session.Close()

		go copyPtyOutput(session, logger)
		if err := session.Wait(); err != nil {
			return fmt.Errorf("bun install failed: %w", err)
		}
	}

	fmt.Println("Building frontend...")
	buildSession, err := ptyx.Spawn(ctx, ptyx.SpawnOpts{
		Prog: "bun",
		Args: []string{"run", "build"},
		Dir:  frontendDir,
		Env:  mergeEnv(os.Environ(), envFile),
	})
	if err != nil {
		return fmt.Errorf("bun build failed: %w", err)
	}
	defer buildSession.Close()

	go copyPtyOutput(buildSession, logger)
	if err := buildSession.Wait(); err != nil {
		return fmt.Errorf("bun build failed: %w", err)
	}

	return nil
}

func startFrontendDevServerWithPTY(ctx context.Context, frontendDir string, port int, encorePort int, envFile map[string]string) (*exec.Cmd, ptyx.Session, error) {
	// Install dependencies if node_modules doesn't exist
	if _, err := os.Stat(filepath.Join(frontendDir, "node_modules")); os.IsNotExist(err) {
		fmt.Println("Installing frontend dependencies...")
		session, err := ptyx.Spawn(ctx, ptyx.SpawnOpts{
			Prog: "bun",
			Args: []string{"install"},
			Dir:  frontendDir,
			Env:  mergeEnv(os.Environ(), envFile),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("bun install failed: %w", err)
		}

		// Copy output to console during install
		go io.Copy(os.Stdout, session.PtyReader())

		if err := session.Wait(); err != nil {
			return nil, nil, fmt.Errorf("bun install failed: %w", err)
		}
		session.Close()
	}

	// Build command args
	env := mergeEnv(os.Environ(), envFile)
	env = append(env, fmt.Sprintf("ENCORE_PORT=%d", encorePort))

	// Start with PTY
	session, err := ptyx.Spawn(ctx, ptyx.SpawnOpts{
		Prog: "bun",
		Args: []string{"run", "dev", "--port", strconv.Itoa(port)},
		Dir:  frontendDir,
		Env:  env,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start frontend with PTY: %w", err)
	}

	// Monitor for early failures (within first 3 seconds)
	processDone := make(chan error, 1)
	go func() {
		processDone <- session.Wait()
	}()

	select {
	case err := <-processDone:
		// Frontend exited early - show error
		session.Close()
		return nil, nil, fmt.Errorf("frontend failed to start (exited with code %w)", err)
	case <-time.After(2 * time.Second):
		// Frontend is still running - good!
		fmt.Printf("✓ Frontend dev server started on port %d (PID: %d)\n", port, session.Pid())
		return nil, session, nil
	}
}
