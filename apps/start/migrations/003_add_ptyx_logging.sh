#!/bin/bash
# Migration: 003_add_ptyx_logging
# Description: Add PTY-based logging system with ANSI code stripping
# Version: 1.2.0
# Since: 1.2.0

set -e

echo "Adding PTY-based logging system..."

# Create logs directory if it doesn't exist
if [ ! -d "logs" ]; then
    mkdir -p logs
    echo "  Created logs/ directory"
else
    echo "  logs/ directory already exists (keeping existing logs)"
fi

# Ensure go.mod has ptyx dependency
if ! grep -q "github.com/KennethanCeyer/ptyx" apps/start/go.mod 2>/dev/null; then
    echo "  Adding ptyx dependency to go.mod..."
    cd apps/start
    go get github.com/KennethanCeyer/ptyx
    cd ../..
    echo "  Added github.com/KennethanCeyer/ptyx to go.mod"
else
    echo "  ptyx dependency already exists in go.mod"
fi

# Ensure CLAUDE.md mentions ptyx-logging skill
if ! grep -q "ptyx-logging" .claude/CLAUDE.md 2>/dev/null; then
    echo "  Adding ptyx-logging skill to CLAUDE.md..."
    # This will be handled by the upgrade script itself
    echo "  Note: CLAUDE.md will be updated by upgrade script"
else
    echo "  CLAUDE.md already references ptyx-logging"
fi

echo ""
echo "Migration complete! Changes:"
echo "  - Startup binary now captures all output (Encore + frontend) using PTY"
echo "  - Logs are written to logs/backend.log and logs/frontend.log"
echo "  - ANSI escape codes are stripped from log files for readability"
echo "  - Console output still displays with full colors"
echo ""
echo "Next steps:"
echo "  1. Rebuild startup binary: make dev.macos (or make all)"
echo "  2. Run as usual: ./dev.macos or make run"
echo "  3. Check logs in logs/ directory"
echo ""
