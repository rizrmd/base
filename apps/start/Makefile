.PHONY: all dev.linux dev.macos dev.exe prod.linux clean

# Version - update this when releasing new versions
VERSION ?= 1.0.0

# Default: build all binaries
all: dev.linux dev.macos dev.exe prod.linux

# Linux dev binary
dev.linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.env=dev -X main.version=$(VERSION)" -o ../../dev.linux ./cmd

# macOS ARM64 dev binary
dev.macos:
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.env=dev -X main.version=$(VERSION)" -o ../../dev.macos ./cmd

# Windows dev binary
dev.exe:
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.env=dev -X main.version=$(VERSION)" -o ../../dev.exe ./cmd

# Linux prod binary
prod.linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.env=prod -X main.version=$(VERSION) -s -w" -o ../../prod.linux ./cmd

# Clean built binaries
clean:
	rm -f ../../dev.linux ../../dev.macos ../../dev.exe ../../prod.linux
