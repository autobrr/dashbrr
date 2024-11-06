# Binary name
BINARY_NAME=dashbrr

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Frontend parameters
PNPM=pnpm

# Docker parameters
DOCKER_COMPOSE=docker compose

# Build directory
BUILD_DIR=dist

# Main Go file
MAIN_GO=./backend/main.go

.PHONY: all clean frontend backend deps-go deps-frontend dev docker-dev docker-dev-quick docker-build help redis-dev redis-stop docker-clean test-integration test-integration-db test-integration-db-stop run

# Default target
all: clean deps-frontend deps-go frontend backend

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf backend/web/dist
	rm -f $(BINARY_NAME)

# Install Go dependencies
deps-go:
	@echo "Installing Go dependencies..."
	$(GOMOD) tidy
	$(GOGET) github.com/gin-gonic/gin

# Install frontend dependencies
deps-frontend:
	@echo "Installing frontend dependencies..."
	$(PNPM) install

# Build frontend
frontend: deps-frontend
	@echo "Building frontend..."
	$(PNPM) build
	@echo "Moving frontend build to backend directory for embedding..."
	mkdir -p backend/web
	cp -r $(BUILD_DIR) backend/web/

# Build backend and create final binary
backend: deps-go
	@echo "Building backend..."
	$(GOBUILD) -o $(BINARY_NAME) $(MAIN_GO)

# Start Redis for development
redis-dev:
	@if ! command -v redis-server > /dev/null; then \
		echo "Redis is not installed. Please install Redis first."; \
		exit 1; \
	fi
	@if ! pgrep redis-server > /dev/null; then \
		redis-server --daemonize yes; \
		echo "Redis server started"; \
	else \
		echo "Redis server is already running"; \
	fi

# Stop Redis development server
redis-stop:
	@if pgrep redis-server > /dev/null; then \
		redis-cli shutdown; \
		echo "Redis server stopped"; \
	else \
		echo "Redis server is not running"; \
	fi

# Wait for backend to be ready
wait-backend:
	@echo "Waiting for backend to be ready..."
	@for i in $$(seq 1 30); do \
		if curl -s http://localhost:8080/health > /dev/null; then \
			echo "Backend is ready!"; \
			exit 0; \
		fi; \
		echo "Waiting for backend... ($$i/30)"; \
		sleep 1; \
	done; \
	echo "Backend failed to start within 30 seconds"; \
	exit 1

# Development mode - run frontend and backend with SQLite
dev: redis-dev
	@echo "Starting development servers..."
	@echo "Redis is running on localhost:6379"
	@echo "Starting backend server with SQLite..."
	DASHBRR__DB_TYPE=sqlite $(GOCMD) run $(MAIN_GO) --db ./data/dashbrr.db & \
	backend_pid=$$!; \
	echo "Waiting for backend to be ready..."; \
	$(MAKE) wait-backend; \
	echo "Starting frontend server..."; \
	$(PNPM) dev & \
	frontend_pid=$$!; \
	trap 'kill $$backend_pid $$frontend_pid 2>/dev/null; make redis-stop' EXIT; \
	wait

# Docker development mode - run with PostgreSQL (with rebuild)
docker-dev:
	@echo "Starting Docker development environment with PostgreSQL (rebuilding containers)..."
	$(DOCKER_COMPOSE) down
	$(DOCKER_COMPOSE) build
	$(DOCKER_COMPOSE) up

# Docker development mode - run with PostgreSQL (quick start, no rebuild)
docker-dev-quick:
	@echo "Starting Docker development environment with PostgreSQL (quick start, no rebuild)..."
	$(DOCKER_COMPOSE) up

# Clean Docker development environment (including volumes)
docker-clean:
	@echo "Cleaning Docker development environment (including volumes)..."
	$(DOCKER_COMPOSE) down -v

# Docker commands
docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .

# Start PostgreSQL for integration tests
test-integration-db:
	@echo "Starting PostgreSQL for integration tests..."
	$(DOCKER_COMPOSE) -f docker-compose.integration.yml up -d
	@echo "Waiting for PostgreSQL to be ready..."
	@for i in $$(seq 1 30); do \
		if docker compose -f docker-compose.integration.yml exec -T postgres pg_isready -U dashbrr > /dev/null 2>&1; then \
			echo "PostgreSQL is ready!"; \
			exit 0; \
		fi; \
		echo "Waiting for PostgreSQL... ($$i/30)"; \
		sleep 1; \
	done; \
	echo "PostgreSQL failed to start within 30 seconds"; \
	exit 1

# Stop PostgreSQL for integration tests
test-integration-db-stop:
	@echo "Stopping PostgreSQL for integration tests..."
	$(DOCKER_COMPOSE) -f docker-compose.integration.yml down -v

# Run integration tests
test-integration: test-integration-db
	@echo "Running integration tests..."
	DASHBRR__DB_HOST=localhost \
	DASHBRR__DB_PORT=5432 \
	DASHBRR__DB_USER=dashbrr \
	DASHBRR__DB_PASSWORD=dashbrr \
	DASHBRR__DB_NAME=dashbrr_test \
	$(GOCMD) test -v -tags=integration ./... || (make test-integration-db-stop && exit 1)
	@echo "Stopping test database..."
	@make test-integration-db-stop

# Run the application
run: all
	@echo "Starting $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Help target
help:
	@echo "Available targets:"
	@echo "  all                      - Build everything (clean, install dependencies, build frontend and backend)"
	@echo "  clean                    - Remove build artifacts and clean Go workspace"
	@echo "  deps-go                  - Install Go dependencies"
	@echo "  deps-frontend            - Install frontend dependencies using pnpm"
	@echo "  frontend                 - Build the frontend application"
	@echo "  backend                  - Build the backend Go binary"
	@echo "  dev                      - Start development environment with SQLite and Redis"
	@echo "  redis-dev                - Start Redis server for development"
	@echo "  redis-stop               - Stop Redis development server"
	@echo "  docker-dev               - Start Docker development environment with PostgreSQL (rebuilds containers)"
	@echo "  docker-dev-quick         - Start Docker development environment without rebuilding"
	@echo "  docker-clean             - Clean Docker environment including volumes"
	@echo "  docker-build             - Build Docker image"
	@echo "  test-integration         - Run integration tests with PostgreSQL"
	@echo "  test-integration-db      - Start PostgreSQL for integration tests"
	@echo "  test-integration-db-stop - Stop PostgreSQL integration test database"
	@echo "  run                      - Build and run the application"
