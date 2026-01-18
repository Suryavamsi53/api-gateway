BUILD=bin/gateway
DOWNSTREAM_BUILD=bin/downstream

.PHONY: all test build docker tidy bench coverage clean run run-downstream dev help lint docker-compose

all: test build

help:
	@echo "API Gateway - Makefile targets:"
	@echo "  make all              - Run tests and build"
	@echo "  make test             - Run unit tests"
	@echo "  make coverage         - Run tests with coverage report"
	@echo "  make bench            - Run benchmarks"
	@echo "  make build            - Build gateway binary"
	@echo "  make build-downstream - Build downstream test service"
	@echo "  make docker           - Build Docker images"
	@echo "  make docker-compose   - Start services with docker-compose"
	@echo "  make run              - Run gateway locally (in-memory store)"
	@echo "  make run-downstream   - Run downstream service locally"
	@echo "  make dev              - Start full dev stack (Redis + downstream + gateway)"
	@echo "  make clean            - Remove build artifacts"
	@echo "  make tidy             - Run go mod tidy"
	@echo "  make lint             - Run linter (if installed)"

test:
	go test ./...

coverage:
	go test ./... -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

bench:
	go test -bench=. -benchmem ./internal/service

build: 
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o $(BUILD) ./cmd/gateway
	@echo "✓ Gateway built: $(BUILD)"

build-downstream:
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o $(DOWNSTREAM_BUILD) ./cmd/downstream
	@echo "✓ Downstream built: $(DOWNSTREAM_BUILD)"

run: build
	./$(BUILD)

run-downstream: build-downstream
	./$(DOWNSTREAM_BUILD)

docker: build build-downstream
	@if command -v docker >/dev/null 2>&1; then \
		docker build -t api-gateway:latest .; \
		docker build -f Dockerfile.downstream -t api-gateway-downstream:latest .; \
		echo "✓ Docker images built successfully"; \
	elif command -v podman >/dev/null 2>&1; then \
		podman build -t api-gateway:latest .; \
		podman build -f Dockerfile.downstream -t api-gateway-downstream:latest .; \
		echo "✓ Podman images built successfully"; \
	else \
		echo "ERROR: docker or podman required"; exit 1; \
	fi

docker-compose:
	@if command -v docker-compose >/dev/null 2>&1; then \
		docker-compose up -d; \
	elif command -v docker >/dev/null 2>&1; then \
		docker compose up -d; \
	else \
		echo "ERROR: docker-compose required"; exit 1; \
	fi
	@echo "✓ Services started. Gateway: http://localhost:8080"

dev:
	@echo "Starting development stack..."
	@make docker-compose
	@sleep 2
	@echo "Services ready:"
	@echo "  - Redis:     localhost:6379"
	@echo "  - Gateway:   http://localhost:8080"
	@echo "  - Downstream: http://localhost:8081"
	@echo "  - Metrics:   http://localhost:8080/metrics"
	@echo "  - Health:    http://localhost:8080/health"

clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "✓ Build artifacts removed"

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; }
	golangci-lint run ./...

tidy:
	go mod tidy
	@echo "✓ Dependencies tidied"
