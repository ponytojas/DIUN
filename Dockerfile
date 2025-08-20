# Build stage
FROM golang:1.24.5-alpine AS builder

# Install git and ca-certificates (needed for go mod download)
RUN apk add --no-cache git ca-certificates tzdata

# Create appuser for security
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
ARG TARGETPLATFORM
RUN case "$TARGETPLATFORM" in \
    "linux/amd64") GOARCH=amd64 ;; \
    "linux/arm64") GOARCH=arm64 ;; \
    "linux/arm/v7") GOARCH=arm ;; \
    *) echo "Unsupported platform: $TARGETPLATFORM" && exit 1 ;; \
    esac && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$GOARCH go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o docker-notify ./cmd/main.go

# Final stage
FROM scratch

# Import from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/passwd /etc/passwd

# Copy binary
COPY --from=builder /build/docker-notify /docker-notify

# Copy default config (this will create the /app directory)
COPY --from=builder /build/configs/config.yaml /app/config.yaml

# Note: Container needs access to Docker socket
# Either run as root (less secure) or add user to docker group on host
# For production, consider: docker run --group-add $(getent group docker | cut -d: -f3)
USER appuser

# Set environment variables
ENV TZ=UTC
ENV CONFIG_PATH=/app/config.yaml

# Expose health check endpoint (if implemented)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/docker-notify", "-test"] || exit 1

# Set entrypoint
ENTRYPOINT ["/docker-notify"]

# Default command
CMD ["-config", "/app/config.yaml"]
