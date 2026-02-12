.PHONY: all dev.linux dev.macos dev.exe prod.linux clean

# Default: build all binaries
all: dev.linux dev.macos dev.exe prod.linux

# Linux dev binary
dev.linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.env=dev" -o ../../dev.linux .

# macOS ARM64 dev binary
dev.macos:
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.env=dev" -o ../../dev.macos .

# Windows dev binary
dev.exe:
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.env=dev" -o ../../dev.exe .

# Linux prod binary
prod.linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.env=prod -s -w" -o ../../prod.linux .

# Clean built binaries
clean:
	rm -f ../../dev.linux ../../dev.macos ../../dev.exe ../../prod.linux
