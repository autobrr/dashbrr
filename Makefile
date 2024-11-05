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

# Build directory
BUILD_DIR=dist

# Main Go file
MAIN_GO=./backend/main.go

.PHONY: all build clean frontend backend deps-go deps-frontend dev redis-dev redis-stop wait-backend

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

# Development mode - run frontend and backend separately
dev: redis-dev
	@echo "Starting development servers..."
	@echo "Redis is running on localhost:6379"
	@echo "Starting backend server..."
	$(GOCMD) run $(MAIN_GO) --db ./data/dashbrr.db & \
	backend_pid=$$!; \
	echo "Waiting for backend to be ready..."; \
	$(MAKE) wait-backend; \
	echo "Starting frontend server..."; \
	$(PNPM) dev & \
	frontend_pid=$$!; \
	trap 'kill $$backend_pid $$frontend_pid 2>/dev/null; make redis-stop' EXIT; \
	wait

# Run the application
run: all
	@echo "Starting $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Help target
help:
	@echo "Available targets:"
	@echo "  all           - Build everything (default)"
	@echo "  clean         - Remove build artifacts"
	@echo "  deps-go       - Install Go dependencies"
	@echo "  deps-frontend - Install frontend dependencies"
	@echo "  frontend      - Build only the frontend"
	@echo "  backend       - Build only the backend"
	@echo "  run          - Build and run the application"
	@echo "  dev          - Run in development mode with Redis"
	@echo "  redis-dev    - Start Redis server"
	@echo "  redis-stop   - Stop Redis server"
