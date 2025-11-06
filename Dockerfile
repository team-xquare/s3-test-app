# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install build dependencies for CGO (required for sqlite3)
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with CGO enabled for sqlite3
RUN CGO_ENABLED=1 GOOS=linux go build -a -o s3-test-app ./cmd/server

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS, curl for health check, and sqlite libraries
RUN apk --no-cache add ca-certificates curl sqlite-libs

# Copy the binary from builder
COPY --from=builder /build/s3-test-app .

# Create data directory for SQLite database
RUN mkdir -p /app/data

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

# Run the application
CMD ["./s3-test-app"]