#!/bin/bash
# Migration: 002_add_env_file_support
# Description: Add .env file support for environment variables in child processes
# Version: 1.1.0
# Since: 1.1.0

set -e

echo "Adding .env file support..."

# Create .env.example if it doesn't exist
if [ ! -f ".env.example" ]; then
    cat > .env.example << 'EOF'
# Example environment variables for the base template
# Copy this file to .env and customize the values

# Encore API Configuration
ENCORE_ENV=development

# Clerk Authentication (if using Clerk auth)
CLERK_SECRET_KEY=pk_test_...
CLERK_PUBLISHABLE_KEY=pk_test_...

# External Services
# SMTP settings for email
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=your-email@example.com
SMTP_PASS=your-password

# API Keys
STRIPE_API_KEY=sk_test_...
SENDGRID_API_KEY=SG....

# Database (if using external database)
EXTERNAL_DB_HOST=localhost
EXTERNAL_DB_PORT=5432
EXTERNAL_DB_NAME=your_db
EXTERNAL_DB_USER=your_user
EXTERNAL_DB_PASSWORD=your_password
EOF
    echo "  Created .env.example file"
else
    echo "  .env.example already exists (skipping)"
fi

# Create .env from .env.example if .env doesn't exist
if [ ! -f ".env" ]; then
    cp .env.example .env
    echo "  Created .env from .env.example (customize as needed)"
else
    echo "  .env already exists (keeping existing values)"
fi

echo "Done! Environment variables from .env will now be passed to child processes."
