# Zuri Backend Makefile

.PHONY: all build test test-all test-coverage clean docker-up docker-down run-user-service run-gateway migrate-up migrate-down migrate-create deps fmt lint check-syntax help

# Default target
all: build

# Build all Go microservices
build:
	@echo "Building Go microservices..."
	@mkdir -p bin
	go build -o bin/user-service ./services/user/cmd/main.go
	go build -o bin/gateway ./services/gateway/cmd/main.go
	go build -o bin/content-service ./services/content/cmd/main.go
	go build -o bin/analytics-service ./services/analytics/cmd/main.go
	go build -o bin/notification-service ./services/notification-service/main.go
	go build -o bin/sync-service ./services/sync-service/main.go
	@echo "All Go binaries built in bin/"

# Run Go unit tests across all services
test:
	@echo "Running Go unit tests..."
	go test -v ./services/...

# Run all tests (Go tests + Python syntax checks)
test-all: test check-syntax

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./services/...
	go tool cover -html=coverage.out -o coverage.html

# Run Python AST syntax verification
check-syntax:
	@echo "Checking Python services syntax..."
	python3 scripts/check_syntax.py

# Run Python integration suite
test-integration:
	@echo "Running Python integration suite..."
	python3 tests/integration/integration_test_phase1_phase2.py

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Start infrastructure with Docker Compose
docker-up:
	@echo "Starting infrastructure..."
	docker-compose -f infra/docker-compose.yml up -d

# Stop infrastructure
docker-down:
	@echo "Stopping infrastructure..."
	docker-compose -f infra/docker-compose.yml down

# View logs
docker-logs:
	docker-compose -f infra/docker-compose.yml logs -f

# Run the user service locally (requires local postgres and redis)
run-user-service:
	@echo "Running User Service..."
	go run ./services/user/cmd/main.go

# Run the gateway locally
run-gateway:
	@echo "Running Gateway Service..."
	go run ./services/gateway/cmd/main.go

# Database migrations (requires golang-migrate installed)
migrate-up:
	@echo "Running database migrations..."
	migrate -path infra/migrations -database "${DATABASE_URL}" up

migrate-down:
	@echo "Rolling back database migrations..."
	migrate -path infra/migrations -database "${DATABASE_URL}" down

# Create a new migration
migrate-create:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir infra/migrations -seq $$name

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Generate mocks (if using mockgen)
mocks:
	@echo "Generating mocks..."
	# go generate ./...

# Help
help:
	@echo "Available targets:"
	@echo "  build              - Build all Go services into bin/"
	@echo "  test               - Run unit tests across all Go services"
	@echo "  test-all           - Run Go tests and Python syntax verification"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo "  check-syntax       - Check Python service syntax"
	@echo "  test-integration   - Run Python integration test suite"
	@echo "  clean              - Clean build artifacts"
	@echo "  docker-up          - Start infrastructure with Docker Compose"
	@echo "  docker-down        - Stop infrastructure"
	@echo "  docker-logs        - View Docker logs"
	@echo "  run-user-service   - Run User Service locally"
	@echo "  run-gateway        - Run Gateway locally"
	@echo "  migrate-up         - Run database migrations"
	@echo "  migrate-down       - Rollback database migrations"
	@echo "  migrate-create     - Create a new migration"
	@echo "  deps               - Install dependencies"
	@echo "  fmt                - Format code"
	@echo "  lint               - Run linter"
