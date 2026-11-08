# syntax=docker/dockerfile:1

# Build stage: standard Go toolchain.
FROM golang:1.27 AS builder

WORKDIR /services

# Copy and download Go modules.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy application source.
COPY . .

# Build a static, minimal binary.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w" \
    -o /blob \
    .

# Runtime stage: scratch with CA bundle.
FROM scratch

# Runtime environment.
ENV HOST=0.0.0.0 PORT=3000

# Keep a CA bundle for TLS validation (postgres/redis/S3 upstreams).
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

WORKDIR /services

# Copy only the executable.
COPY --from=builder /blob ./blob

CMD ["./blob"]