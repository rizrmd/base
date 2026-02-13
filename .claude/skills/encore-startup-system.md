# Encore Startup System

The `apps/start` directory contains a Go project that builds platform-specific startup binaries. These binaries handle the complete development workflow including port management, process orchestration, and template upgrades.

## Architecture

**Directory Structure:**
```
apps/start/
  Makefile              # Build system for cross-compilation
  base.manifest        # Base template upgrade configuration
  go.mod               # Go module definition
  migrations/          # Template upgrade scripts
  cmd/
    main.go            # Main entry point, port calculation, process management
    upgrade.go         # Base template upgrade system
    process_unix.go    # Unix-specific process handling
    process_windows.go # Windows-specific process handling
```

## Build System (Makefile)

The `Makefile` uses Go's cross-compilation with `-ldflags` to embed compile-time variables:

```makefile
# Example: dev.macos
dev.macos:
    GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.env=dev -X main.version=$(VERSION)" -o ../../dev.macos ./cmd

# Example: prod.linux (stripped)
prod.linux:
    GOOS=linux GOARCH=amd64 go build -ldflags "-X main.env=prod -X main.version=$(VERSION) -s -w" -o ../../prod.linux ./cmd
```

**Build Flags:**
- `GOOS=darwin GOARCH=arm64`: Target macOS ARM64
- `-X main.env=dev`: Embed the environment variable
- `-X main.version=$(VERSION)`: Embed version string
- `-s -w` (prod only): Strip debug symbols to reduce binary size

**Build Commands:**
```bash
cd apps/start
make all         # Build all binaries
make dev.macos   # Build macOS dev binary only
make clean       # Remove all built binaries
```

## Main Binary (main.go)

### 1. Port Calculation (Multi-User Support)

The binary automatically calculates unique ports to support multiple users/projects on shared machines:

```go
func getPortOffset(rootDir string) int {
    h := fnv.New32a()
    uid := os.Getuid()  // Unix: user ID, Windows: USERNAME env
    h.Write([]byte(strconv.Itoa(uid)))
    h.Write([]byte(rootDir))  // Project path ensures uniqueness per project
    return int(h.Sum32() % 1000)
}
```

**Port Assignment:**
- Frontend: `5173 + offset`
- API/Encore: `4000 + offset`
- Offset range: 0-999 (via hash modulo)

**Manual Override:**
```bash
FRONTEND_PORT=5174 ENCORE_PORT=4001 ./dev.macos
```

### 2. Configuration

```go
type Config struct {
    FrontendPort int
    EncorePort   int
}
```

Configuration priority:
1. Environment variables (`FRONTEND_PORT`, `ENCORE_PORT`)
2. Calculated ports from hash
3. Defaults (5173, 4000)

### 3. Development Mode Flow

When `env=dev` (embedded at build time):

1. **Kill existing processes**: Uses `lsof` (Unix) or equivalent to find and kill processes on allocated ports
2. **Start Bun dev server**:
   ```go
   bun run dev --port <calculated_port>
   ```
   - Installs dependencies if `node_modules` doesn't exist
   - Passes `ENCORE_PORT` for Vite HMR configuration
3. **Start Encore dev server**:
   ```go
   encore run --port <calculated_port> --browser=never
   ```
4. **Setup signal handling**: Listens for SIGINT/SIGTERM for graceful shutdown
5. **Wait for signals**: Blocks until shutdown signal received

### 4. Production Mode Flow

When `env=prod`:

1. **Build frontend**:
   ```go
   bun install  # If node_modules missing
   bun run build
   ```
2. **Start Encore production server**:
   ```go
   encore run --env=production --port <port> --browser=never
   ```
3. **Serves static files**: Encore serves the `frontend/build/client` directory

### 5. Signal Handling & Process Management

**Process Groups (Unix):**
```go
func setProcessGroup(cmd *exec.Cmd) {
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
```
- Creates a new process group for proper signal propagation
- Ensures child processes (Bun) receive shutdown signals

**Graceful Shutdown:**
```go
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
// ...
killProcess(frontendCmd)  // SIGTERM first
killProcess(encoreCmd)
time.Sleep(500 * time.Millisecond)
```
- Sends SIGTERM to process groups
- Waits 2 seconds, then sends SIGKILL if needed

## Subcommands

The binary supports several subcommands:

| Command | Description |
|---------|-------------|
| (none) | Start the application (default) |
| `upgrade` | Upgrade base template to latest version |
| `version` | Show version information |
| `help` | Show help message |

## Upgrade System (upgrade.go)

Handles base template updates from a remote repository:

**Features:**
1. **Remote sync**: Clones template from GitHub (default: `rizrmd/base`)
2. **Commit tracking**: Stores current commit in `.base-commit` file
3. **File merging**: Preserves user files during upgrades:
   - `apps/backend/` (except `apps/backend/spa/`)
   - `apps/frontend/app/`
   - `apps/frontend/public/`
   - `.env`
4. **Migrations**: Runs pending migration scripts from `apps/start/migrations/`

**Migration Tracking:**
- Applied migrations marked in `.base-migrations/<name>.applied`
- Only runs pending (unapplied) migrations

**Usage:**
```bash
./dev.macos upgrade           # Install or update
./dev.macos upgrade --dry-run # Preview changes
./dev.macos upgrade --skip-fetch # Run local migrations only
```

## Process Management (process_unix.go / process_windows.go)

**Unix (`process_unix.go`):**
- **Port killing**: Uses `lsof -t -i :<port>` to find PIDs by port
- **Process groups**: Uses `syscall.SysProcAttr{Setpgid: true}` for group management
- **Signal handling**: Sends SIGTERM, waits 2s, then SIGKILL

**Windows (`process_windows.go`):**
- Platform-specific equivalents for Windows signal handling

## Binary Lifecycle

```bash
# 1. Build
cd apps/start && make dev.macos

# 2. Run
./dev.macos                    # Development (Bun + Encore)
./dev.macos upgrade           # Update template
./dev.macos version           # v1.0.0

# 3. Shutdown (Ctrl+C)
# - Sends SIGTERM to Bun and Encore
# - Waits up to 2s for graceful shutdown
# - Force kills if needed
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `FRONTEND_PORT` | Override frontend port (default: calculated) |
| `ENCORE_PORT` | Override API/Encore port (default: calculated) |
| `ENCORE_ENV` | Runtime environment (if not embedded) |
