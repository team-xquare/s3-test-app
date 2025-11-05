.PHONY: help build run test clean lint fmt docker-build docker-run templ-gen

BINARY_NAME=s3-test-app
GO=go
GOFLAGS=-v
TEMPL=$(HOME)/go/bin/templ

help:
	@echo "Available commands:"
	@echo "  make templ-gen     - Generate Templ components"
	@echo "  make build         - Build the application"
	@echo "  make run           - Run the application (requires S3_ACCESS_KEY and S3_SECRET_KEY env vars)"
	@echo "  make test          - Run tests"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make lint          - Run linter"
	@echo "  make fmt           - Format code"
	@echo "  make docker-build  - Build Docker image"
	@echo "  make docker-run    - Run Docker container"

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
	docker run -p 8080:8080 \
		-e S3_ACCESS_KEY=minioadmin \
		-e S3_SECRET_KEY=minioadmin \
		$(BINARY_NAME):latest

deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy