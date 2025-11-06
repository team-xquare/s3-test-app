# Syntax: docker/dockerfile:1
# Multi-stage build for minimal image size

# ============================================
# Stage 1: Dependencies Cache
# ============================================
FROM golang:1.25-alpine AS deps-builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

# ============================================
# Stage 2: Builder
# ============================================
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build

# Copy cached dependencies from stage 1
COPY --from=deps-builder /go/pkg /go/pkg

COPY go.mod go.sum ./

COPY . .

# Build with optimizations:
# -ldflags: Strip debug symbols and remove DWARF info (~40% size reduction)
# -s: Disable symbol table
# -w: Disable DWARF debugging information
# -trimpath: Strip build path from binary
RUN CGO_ENABLED=1 GOOS=linux CGO_CFLAGS="-O3" LDFLAGS="-s -w -X main.Version=1.0" \
    go build -a -trimpath \
    -ldflags="-s -w" \
    -o s3-test-app ./cmd/server && \
    chmod +x s3-test-app && \
    ls -lh s3-test-app

# ============================================
# Stage 3: Runtime (Minimal Image)
# ============================================
FROM alpine:3.20

# Install only essential runtime dependencies
RUN apk add --no-cache --virtual .runtime ca-certificates sqlite-libs libc6-compat && \
    addgroup -g 1000 app && \
    adduser -D -u 1000 -G app app && \
    mkdir -p /app/data && \
    chown -R app:app /app

WORKDIR /app

# Copy binary from builder
COPY --from=builder --chown=app:app /build/s3-test-app ./

# Copy health check script (minimal)
RUN echo '#!/bin/sh' > /healthcheck.sh && \
    echo 'curl -f http://localhost:8080/health || exit 1' >> /healthcheck.sh && \
    chmod +x /healthcheck.sh

# Non-root user
USER app

# Expose port
EXPOSE 8080

# Health check (use lightweight check)
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD /healthcheck.sh

# Run the application
ENTRYPOINT ["./s3-test-app"]