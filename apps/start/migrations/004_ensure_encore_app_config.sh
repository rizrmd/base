#!/bin/bash
# Migration: Ensure encore.app has proper configuration
# This migration ensures:
# 1. id field is always empty (no Encore Cloud app ID)
# 2. metadata.name matches the root folder name

set -e

ENCORE_APP_FILE="apps/encore.app"

# Get root folder name
ROOT_FOLDER=$(basename "$(pwd)")

echo "Ensuring encore.app configuration..."

# Check if file exists
if [ ! -f "$ENCORE_APP_FILE" ]; then
    echo "Creating $ENCORE_APP_FILE"
    cat > "$ENCORE_APP_FILE" <<EOF
{
  "id": "",
  "metadata": {
    "name": "$ROOT_FOLDER"
  }
}
EOF
    exit 0
fi

# Read current content
CURRENT_CONTENT=$(cat "$ENCORE_APP_FILE")

# Validate and fix the configuration
# Use Python for reliable JSON manipulation
if command -v python3 &> /dev/null; then
    python3 <<PYTHON_SCRIPT
import json
import sys

file_path = "$ENCORE_APP_FILE"
root_folder = "$ROOT_FOLDER"

try:
    with open(file_path, 'r') as f:
        config = json.load(f)
except json.JSONDecodeError:
    print(f"Error: {file_path} contains invalid JSON")
    sys.exit(1)

# Ensure id is empty
config['id'] = ''

# Ensure metadata exists with correct name
if 'metadata' not in config:
    config['metadata'] = {}
config['metadata']['name'] = root_folder

# Write back
with open(file_path, 'w') as f:
    json.dump(config, f, indent=2)
    f.write('\n')

print(f"Updated {file_path}")
print(f"  id: {config['id']}")
print(f"  metadata.name: {config['metadata']['name']}")
PYTHON_SCRIPT
else
    # Fallback: just rewrite the file with correct values
    echo "Python3 not found, rewriting $ENCORE_APP_FILE"
    cat > "$ENCORE_APP_FILE" <<EOF
{
  "id": "",
  "metadata": {
    "name": "$ROOT_FOLDER"
  }
}
EOF
fi

echo "✓ encore.app configuration updated"
