package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Colors for terminal output
const (
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorBlue   = "\033[0;34m"
	colorCyan   = "\033[0;36m"
	colorReset  = "\033[0m"
)

// Source repository for base template
const baseSourceRepo = "https://github.com/rizrmd/base"

// Manifest represents the base.manifest structure
type Manifest struct {
	Version     string           `json:"version"`
	Description string           `json:"description"`
	Source      SourceConfig     `json:"source"`
	Base        ManifestSection  `json:"base"`
	User        ManifestSection  `json:"user"`
	Merge       ManifestSection  `json:"merge"`
	Migrations  MigrationConfig  `json:"migrations"`
}

type SourceConfig struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
}

type ManifestSection struct {
	Description string   `json:"description"`
	Files       []string `json:"files"`
	Exclude     []string `json:"exclude"`
	Strategy    string   `json:"strategy"`
}

type MigrationConfig struct {
	Description string `json:"description"`
	Path        string `json:"path"`
	Naming      string `json:"naming"`
}

func runUpgrade(rootDir string, args []string) {
	dryRun := false
	skipFetch := false

	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--skip-fetch":
			skipFetch = true
		case "--help", "-h":
			printUpgradeHelp()
			return
		}
	}

	fmt.Printf("%s=== Base Template %s ===%s\n\n", colorBlue, getActionText(skipFetch), colorReset)

	// Check git availability
	if !skipFetch && !isGitAvailable() {
		fmt.Fprintf(os.Stderr, "%sError: git is required. Install git or use --skip-fetch.%s\n", colorRed, colorReset)
		os.Exit(1)
	}

	// Get source repo
	sourceRepo := baseSourceRepo
	branch := "main"

	// Try to load local manifest for source config
	localManifestPath := filepath.Join(rootDir, "apps", "start", "base.manifest")
	if manifest, err := loadManifest(localManifestPath); err == nil {
		if manifest.Source.Repo != "" {
			sourceRepo = manifest.Source.Repo
		}
		if manifest.Source.Branch != "" {
			branch = manifest.Source.Branch
		}
	}

	// Check current state
	commitFile := filepath.Join(rootDir, ".base-commit")
	currentCommit := readFile(commitFile)
	isFreshInstall := currentCommit == ""

	if isFreshInstall {
		fmt.Printf("%sFresh install detected%s\n", colorYellow, colorReset)
	} else {
		fmt.Printf("Current commit:  %s\n", currentCommit[:min(8, len(currentCommit))])
	}

	fmt.Printf("Source repo:     %s\n", sourceRepo)
	fmt.Printf("Branch:          %s\n\n", branch)

	if dryRun {
		fmt.Printf("%s[DRY RUN MODE]%s\n\n", colorYellow, colorReset)
	}

	var tempDir string
	var remoteCommit string
	var err error

	if !skipFetch {
		// Check remote for updates
		fmt.Printf("%sChecking remote...%s\n", colorCyan, colorReset)
		remoteCommit, err = getRemoteHeadCommit(sourceRepo, branch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%sError: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}

		fmt.Printf("Remote commit:   %s\n", remoteCommit[:8])

		// Check if update needed
		if !isFreshInstall && remoteCommit == currentCommit {
			fmt.Printf("\n%sAlready up to date!%s\n", colorGreen, colorReset)
			return
		}

		if isFreshInstall {
			fmt.Printf("\n%sInstalling base template...%s\n", colorGreen, colorReset)
		} else {
			fmt.Printf("\n%sUpdate available!%s\n", colorGreen, colorReset)
		}

		if dryRun {
			fmt.Printf("\n%s[DRY RUN] Would clone and apply updates%s\n", colorYellow, colorReset)
			return
		}

		// Clone to temp
		fmt.Printf("\n%sCloning from remote...%s\n", colorCyan, colorReset)
		tempDir, err = cloneToTemp(sourceRepo, branch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%sError cloning: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		defer os.RemoveAll(tempDir)

		fmt.Printf("Cloned to: %s\n", tempDir)

		// Load manifest from cloned repo
		manifestPath := filepath.Join(tempDir, "apps", "start", "base.manifest")
		manifest, err := loadManifest(manifestPath)
		if err != nil {
			manifest = getDefaultManifest()
		}

		// Apply all files from clone
		fmt.Printf("\n%sApplying files...%s\n", colorCyan, colorReset)
		filesCopied, err := applyAllFiles(tempDir, rootDir, isFreshInstall)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%sError: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		fmt.Printf("Copied %d files\n", filesCopied)

		// Update commit tracking
		os.WriteFile(commitFile, []byte(remoteCommit), 0644)

		// Run migrations from cloned repo
		migrationsDir := filepath.Join(tempDir, manifest.Migrations.Path)
		pendingMigrations := findPendingMigrations(rootDir, migrationsDir)

		if len(pendingMigrations) > 0 {
			fmt.Printf("\n%sRunning migrations...%s\n", colorCyan, colorReset)
			for _, m := range pendingMigrations {
				name := filepath.Base(m)
				fmt.Printf("  %s\n", name)
				if err := runMigration(m, rootDir); err != nil {
					fmt.Fprintf(os.Stderr, "%sMigration failed: %v%s\n", colorRed, err, colorReset)
					os.Exit(1)
				}
				markMigrationApplied(rootDir, name)
			}
		}

		fmt.Printf("\n%sDone!%s\n", colorGreen, colorReset)
		fmt.Printf("Commit: %s\n", remoteCommit[:8])

		if isFreshInstall {
			fmt.Println("\nNext steps:")
			fmt.Println("  1. cd apps/frontend && bun install")
			fmt.Println("  2. ./dev.macos  # Start development")
		}
	} else {
		// --skip-fetch: just run local migrations
		manifest, _ := loadManifest(localManifestPath)
		if manifest == nil {
			manifest = getDefaultManifest()
		}
		migrationsDir := filepath.Join(rootDir, manifest.Migrations.Path)
		pendingMigrations := findPendingMigrations(rootDir, migrationsDir)

		if len(pendingMigrations) == 0 {
			fmt.Printf("%sNo pending migrations.%s\n", colorGreen, colorReset)
			return
		}

		fmt.Printf("%sPending migrations:%s\n", colorYellow, colorReset)
		for _, m := range pendingMigrations {
			fmt.Printf("  - %s\n", filepath.Base(m))
		}

		if dryRun {
			return
		}

		fmt.Printf("\n%sRunning migrations...%s\n", colorCyan, colorReset)
		for _, m := range pendingMigrations {
			name := filepath.Base(m)
			fmt.Printf("  %s\n", name)
			if err := runMigration(m, rootDir); err != nil {
				fmt.Fprintf(os.Stderr, "%sMigration failed: %v%s\n", colorRed, err, colorReset)
				os.Exit(1)
			}
			markMigrationApplied(rootDir, name)
		}
		fmt.Printf("\n%sDone!%s\n", colorGreen, colorReset)
	}
}

func getActionText(skipFetch bool) string {
	if skipFetch {
		return "Migration"
	}
	return "Upgrade"
}

func printUpgradeHelp() {
	fmt.Println("Usage: upgrade [options]")
	fmt.Println("")
	fmt.Println("Install or upgrade the base template from the source repository.")
	fmt.Println("")
	fmt.Println("For fresh installs, this command will:")
	fmt.Println("  1. Clone the base template from GitHub")
	fmt.Println("  2. Set up the project structure")
	fmt.Println("  3. Run any pending migrations")
	fmt.Println("")
	fmt.Println("Important: Migrations must not modify apps/backend or apps/frontend")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --dry-run       Preview changes without applying")
	fmt.Println("  --skip-fetch    Run local migrations only (no remote check)")
	fmt.Println("  --help, -h      Show this help message")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  ./dev.macos upgrade              # Install or update")
	fmt.Println("  ./dev.macos upgrade --dry-run    # Preview changes")
}

func isGitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func getRemoteHeadCommit(repo, branch string) (string, error) {
	cmd := exec.Command("git", "ls-remote", repo, "refs/heads/"+branch)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to query remote (is repo accessible?): %w", err)
	}

	parts := strings.Fields(string(output))
	if len(parts) < 1 {
		return "", fmt.Errorf("no commit found for branch %s", branch)
	}

	return parts[0], nil
}

func cloneToTemp(repo, branch string) (string, error) {
	tempDir, err := os.MkdirTemp("", "base-upgrade-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", branch, repo, tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("clone failed: %s", string(output))
	}

	return tempDir, nil
}

func applyAllFiles(srcDir, dstDir string, isFresh bool) (int, error) {
	count := 0

	err := filepath.Walk(srcDir, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		// Skip .git directory
		if strings.Contains(srcPath, "/.git/") || strings.HasSuffix(srcPath, "/.git") {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, srcPath)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dstDir, relPath)

		// For non-fresh installs, preserve user files
		if !isFresh && isUserFile(relPath) {
			return nil
		}

		// Create directory
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return err
		}

		// Copy file (preserve executable bit)
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}

		count++
		return nil
	})

	return count, err
}

func isUserFile(relPath string) bool {
	// Files that should be preserved during upgrade
	userPatterns := []string{
		"apps/backend/",
		"apps/frontend/app/",
		"apps/frontend/public/",
		"apps/frontend/.env",
		".env",
	}

	for _, p := range userPatterns {
		if strings.HasPrefix(relPath, p) {
			// But don't preserve spa (it's base)
			if strings.HasPrefix(relPath, "apps/backend/spa/") {
				return false
			}
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func loadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func getDefaultManifest() *Manifest {
	return &Manifest{
		Version: "1.0.0",
		Source: SourceConfig{
			Repo:   baseSourceRepo,
			Branch: "main",
		},
		Migrations: MigrationConfig{
			Path: "apps/start/migrations",
		},
	}
}

// Migration tracking using .applied files
func getMigrationsDir(rootDir string) string {
	return filepath.Join(rootDir, ".base-migrations")
}

func isMigrationApplied(rootDir, migrationName string) bool {
	appliedFile := filepath.Join(getMigrationsDir(rootDir), migrationName+".applied")
	_, err := os.Stat(appliedFile)
	return err == nil
}

func markMigrationApplied(rootDir, migrationName string) error {
	dir := getMigrationsDir(rootDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	appliedFile := filepath.Join(dir, migrationName+".applied")
	content := fmt.Sprintf("applied: %s\n", time.Now().Format(time.RFC3339))
	return os.WriteFile(appliedFile, []byte(content), 0644)
}

func findPendingMigrations(rootDir, migrationsDir string) []string {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sh"))
	if err != nil || len(files) == 0 {
		return nil
	}

	sort.Strings(files)

	var pending []string
	for _, f := range files {
		name := filepath.Base(f)
		if !isMigrationApplied(rootDir, name) {
			pending = append(pending, f)
		}
	}

	return pending
}

func runMigration(path string, rootDir string) error {
	// Check if we're in a git repo for validation
	gitAvailable := isGitAvailable()
	var gitStatusBefore []string

	if gitAvailable {
		cmd := exec.Command("git", "status", "--porcelain", "apps/backend", "apps/frontend")
		cmd.Dir = rootDir
		output, err := cmd.Output()
		if err == nil {
			gitStatusBefore = strings.Fields(string(output))
		}
	}

	// Run the migration
	cmd := exec.Command("bash", path)
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	runErr := cmd.Run()

	// Validate no changes to protected directories
	if gitAvailable {
		cmd := exec.Command("git", "status", "--porcelain", "apps/backend", "apps/frontend")
		cmd.Dir = rootDir
		output, err := cmd.Output()
		if err == nil {
			gitStatusAfter := strings.Fields(string(output))
			if len(gitStatusAfter) > len(gitStatusBefore) {
				fmt.Fprintf(os.Stderr, "%sERROR: Migration modified protected directories (apps/backend or apps/frontend)%s\n", colorRed, colorReset)
				fmt.Fprintf(os.Stderr, "Migrations must not modify user code directories.\n")
				fmt.Fprintf(os.Stderr, "Please revert changes with: git checkout -- apps/backend apps/frontend\n")
				return fmt.Errorf("migration modified protected directories")
			}
		}
	}

	return runErr
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
