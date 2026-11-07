# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /services

# Configure Go toolchain
ENV GOTOOLCHAIN=local

# Copy dependency manifests first to leverage Docker layer caching
COPY go.mod go.sum ./

# Download dependencies using BuildKit cache mount
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the rest of the source code
COPY . .

# Build a statically linked production binary with compiler cache
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w" -o /blob .

# Production stage
FROM alpine:3.21

WORKDIR /services

# Install CA certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

# Copy only the optimized binary
COPY --from=builder /blob /services/blob

# Start the blob service
CMD ["/services/blob"]