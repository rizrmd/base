.PHONY: all dev.linux dev.macos dev.exe prod.linux clean clean-logs run

# Version - update this when releasing new versions
VERSION ?= 1.0.0

# Default: build all binaries
all: dev.linux dev.macos dev.exe prod.linux

# Linux dev binary
dev.linux:
	cd apps/start && GOOS=linux GOARCH=amd64 go build -ldflags "-X main.env=dev -X main.version=$(VERSION)" -o ../../dev.linux ./cmd

# macOS ARM64 dev binary
dev.macos:
	cd apps/start && GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.env=dev -X main.version=$(VERSION)" -o ../../dev.macos ./cmd

# Windows dev binary
dev.exe:
	cd apps/start && GOOS=windows GOARCH=amd64 go build -ldflags "-X main.env=dev -X main.version=$(VERSION)" -o ../../dev.exe ./cmd

# Linux prod binary
prod.linux:
	cd apps/start && GOOS=linux GOARCH=amd64 go build -ldflags "-X main.env=prod -X main.version=$(VERSION) -s -w" -o ../../prod.linux ./cmd

# Clean built binaries
clean:
	rm -f ./dev.linux ./dev.macos ./dev.exe ./prod.linux

# Clean log files
clean-logs:
	rm -rf ./logs

# Run the development server (requires dev.macos to be built first)
run: dev.macos
	./dev.macos
