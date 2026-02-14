# PTY Logging System

This application uses the `ptyx` library to intercept and log all output from Encore (backend) and the frontend dev server.

## How It Works

The startup binary creates PTY (pseudo-terminal) sessions for both Encore and the frontend, capturing all output including:
- Progress indicators
- Error messages
- Debug traces

All output is:
1. **Displayed in real-time on your terminal** - With full ANSI colors and formatting for readability
2. **Written to timestamped log files** - With all ANSI escape codes stripped for clean, readable logs

## Log Files

- **`logs/backend.log`** - All Encore backend output with timestamps (clean, no ANSI codes)
- **`logs/frontend.log`** - All frontend dev server output with timestamps (clean, no ANSI codes)

Logs are written in ISO 8601 format with millisecond precision: `[2026-02-14T08:47:15.783+07:00]`

## Building

To build the development binaries:

```bash
# From repository root
make dev.macos    # macOS ARM64
make dev.linux     # Linux
make dev.exe       # Windows
make all           # All platforms
```

Or build manually:
```bash
cd apps/start
go build -ldflags "-X main.env=dev -X main.version=1.0.0" -o ../../dev.macos ./cmd
```

## Running

```bash
./dev.macos
```

Or use make:
```bash
make run
```

## Cleaning

```bash
make clean        # Remove built binaries
make clean-logs   # Remove log files
```

## Implementation Details

### ANSI Code Stripping

The logger automatically strips all ANSI escape sequences from log output using a comprehensive regex that matches:
- CSI sequences (ESC [ ... )
- OSC sequences (ESC ] ... )
- Other single-character escape sequences

This ensures logs remain human-readable while preserving full colors in the terminal.

### Code Location

- Logger implementation: `apps/start/cmd/main.go` (Logger struct, stripANSI function)
- PTY integration: Uses `ptyx.Spawn()` to create pseudo-terminal sessions

## Library Used

- **[ptyx](https://github.com/KennethanCeyer/ptyx)** - Cross-platform PTY/T TY toolkit for Go
