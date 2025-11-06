.PHONY: help build run test clean lint fmt docker-build docker-run templ-gen

BINARY_NAME=s3-test-app
GO=go
GOFLAGS=-v
TEMPL=$(HOME)/go/bin/templ

help:
	@echo "Document Management System - Make Commands"
	@echo ""
	@echo "Development:"
	@echo "  make templ-gen     - Generate Templ components from .templ files"
	@echo "  make build         - Build the application binary"
	@echo "  make run           - Build and run the application"
	@echo "  make test          - Run tests"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make deps          - Download and tidy dependencies"
	@echo ""
	@echo "Code Quality:"
	@echo "  make lint          - Run linter (golangci-lint)"
	@echo "  make fmt           - Format code (go fmt)"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build  - Build Docker image"
	@echo "  make docker-run    - Build and run Docker container"
	@echo ""
	@echo "Docker Compose (recommended for development):"
	@echo "  docker-compose up   - Start MinIO and application"
	@echo "  docker-compose down - Stop all services"
	@echo ""
	@echo "Environment Variables Required:"
	@echo "  S3_ENDPOINT        - S3/MinIO endpoint (e.g., http://localhost:9000)"
	@echo "  S3_REGION          - AWS region (e.g., us-east-1)"
	@echo "  S3_BUCKET          - S3 bucket name (e.g., documents)"
	@echo "  S3_ACCESS_KEY      - S3 access key"
	@echo "  S3_SECRET_KEY      - S3 secret key"
	@echo "  AUTH_SECRET        - JWT signing secret"
	@echo "  SIGNUP_KEY         - Key required for user registration"
	@echo ""
	@echo "See README.md for detailed setup instructions"

templ-gen:
	@echo "Generating Templ components..."
	$(TEMPL) generate

build: templ-gen
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) ./cmd/server

run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

test:
	@echo "Running tests..."
	$(GO) test -v ./...

clean:
	@echo "Cleaning..."
	$(GO) clean
	rm -f $(BINARY_NAME)

lint:
	@echo "Running linter..."
	golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .

docker-run: docker-build
	@echo "Running Docker container..."
	@echo "Note: Set required environment variables:"
	@echo "  S3_ENDPOINT, S3_REGION, S3_BUCKET"
	@echo "  S3_ACCESS_KEY, S3_SECRET_KEY"
	@echo "  AUTH_SECRET, SIGNUP_KEY"
	docker run -p 8080:8080 \
		-v $(PWD)/data:/app/data \
		-e S3_ENDPOINT=${S3_ENDPOINT:-http://localhost:9000} \
		-e S3_REGION=${S3_REGION:-us-east-1} \
		-e S3_BUCKET=${S3_BUCKET:-documents} \
		-e S3_ACCESS_KEY=${S3_ACCESS_KEY} \
		-e S3_SECRET_KEY=${S3_SECRET_KEY} \
		-e AUTH_SECRET=${AUTH_SECRET:-your-secret-key} \
		-e SIGNUP_KEY=${SIGNUP_KEY:-your-signup-key} \
		$(BINARY_NAME):latest

deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy