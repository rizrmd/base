# Encore Deployment

This project uses Docker for deployment with our own infrastructure, not Encore's cloud services.

## Docker Guidelines

- Use **Dockerfile** only - never create docker-compose.yml files
- Build single container images that include the compiled Encore binary
- Deploy directly to your own servers, Kubernetes, or container orchestration
- Do not use Encore's managed deployment or cloud platform

## Example Dockerfile

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY apps/ .
RUN go build -o main ./backend/cmd/your-entrypoint

# Runtime stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/main .
EXPOSE 4000
CMD ["./main"]
```

## Never Create

- docker-compose.yml
- docker-compose.prod.yml
- docker-compose.dev.yml
- Any Docker Compose configuration files

This project runs Encore apps directly using the compiled binaries (dev.macos, dev.linux, prod.linux), and deployment is handled via individual Dockerfile builds.
