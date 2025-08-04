# Worker Service Makefile

.PHONY: proto build test clean run

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	@mkdir -p proto/gen
	@protoc --go_out=proto/gen --go_opt=paths=source_relative \
		--proto_path=proto \
		proto/workflow_events.proto \
		proto/task_events.proto

# Build the worker service
build: proto
	@echo "Building worker service..."
	@mkdir -p bin
	@go build -o bin/worker cmd/server/main.go

# Run tests
test:
	@echo "Running tests..."
	@go test ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -rf proto/gen/

# Run the worker service
run: build
	@echo "Starting worker service..."
	@./bin/worker

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod tidy
	@go mod download

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	@golangci-lint run

# Generate mocks for testing
mocks:
	@echo "Generating mocks..."
	@mockgen -source=internal/interfaces/interfaces.go -destination=internal/mocks/interfaces.go

# Docker build
docker-build:
	@echo "Building Docker image..."
	@docker build -t flunq-worker:latest .

# Docker run
docker-run:
	@echo "Running Docker container..."
	@docker run --rm -p 8082:8082 flunq-worker:latest

# Help
help:
	@echo "Available targets:"
	@echo "  proto        - Generate protobuf code"
	@echo "  build        - Build the worker service"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  run          - Run the worker service"
	@echo "  deps         - Install dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code"
	@echo "  mocks        - Generate mocks for testing"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo "  help         - Show this help"
