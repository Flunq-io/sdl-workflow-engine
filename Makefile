# Flunq.io Development Makefile

.PHONY: help build start stop clean test lint deps

# Default target
help:
	@echo "Flunq.io Development Commands:"
	@echo ""
	@echo "  make build     - Build all services"
	@echo "  make start     - Start all services"
	@echo "  make stop      - Stop all services"
	@echo "  make clean     - Clean up containers and volumes"
	@echo "  make test      - Run tests for all services"
	@echo "  make lint      - Run linting for all services"
	@echo "  make deps      - Install dependencies for all services"
	@echo ""
	@echo "Individual service commands:"
	@echo "  make api       - Start only API service"
	@echo "  make events    - Start only Events service"
	@echo "  make worker    - Start only Worker service"
	@echo "  make tasks     - Start only Tasks service"
	@echo "  make ui        - Start only UI service"

# Build all services
build:
	docker-compose build

# Start all services
start:
	docker-compose up -d

# Stop all services
stop:
	docker-compose down

# Clean up everything
clean:
	docker-compose down -v --remove-orphans
	docker system prune -f

# Run tests
test:
	@echo "Running API tests..."
	cd api && go test ./...
	@echo "Running Events tests..."
	cd events && go test ./...
	@echo "Running Worker tests..."
	cd worker && go test ./...
	@echo "Running Tasks tests..."
	cd tasks && go test ./...
	@echo "Running UI tests..."
	cd ui && npm test

# Run linting
lint:
	@echo "Linting API..."
	cd api && go vet ./...
	@echo "Linting Events..."
	cd events && go vet ./...
	@echo "Linting Worker..."
	cd worker && go vet ./...
	@echo "Linting Tasks..."
	cd tasks && go vet ./...
	@echo "Linting UI..."
	cd ui && npm run lint

# Install dependencies
deps:
	@echo "Installing API dependencies..."
	cd api && go mod tidy
	@echo "Installing Events dependencies..."
	cd events && go mod tidy
	@echo "Installing Worker dependencies..."
	cd worker && go mod tidy
	@echo "Installing Tasks dependencies..."
	cd tasks && go mod tidy
	@echo "Installing UI dependencies..."
	cd ui && npm install

# Individual service targets
api:
	docker-compose up -d redis
	cd api && go run cmd/server/main.go

events:
	docker-compose up -d redis
	cd events && go run cmd/server/main.go

worker:
	docker-compose up -d redis events
	cd worker && go run cmd/worker/main.go

tasks:
	docker-compose up -d redis
	cd tasks && go run cmd/server/main.go

ui:
	docker-compose up -d api
	cd ui && npm run dev

# Development helpers
logs:
	docker-compose logs -f

status:
	docker-compose ps

restart:
	docker-compose restart

# Database operations
redis-cli:
	docker-compose exec redis redis-cli

# Monitoring
monitor:
	@echo "Service Status:"
	@curl -s http://localhost:8080/health | jq . || echo "API: DOWN"
	@curl -s http://localhost:8081/health | jq . || echo "Events: DOWN"
	@curl -s http://localhost:8083/health | jq . || echo "Tasks: DOWN"
	@echo "UI: http://localhost:3000"
