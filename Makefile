.PHONY: help build run test clean install deploy dev

# Variables
BINARY_NAME=nms-agent
MAIN_PACKAGE=./cmd/agent
BINARY_PATH=./bin/$(BINARY_NAME)
VERSION=$(shell git describe --tags --always 2>/dev/null || echo "1.0.0")
BUILD_TIME=$(shell date -u '+%Y-%m-%d %H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS=-ldflags "-X 'main.AppVersion=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)' -X 'main.GitCommit=$(GIT_COMMIT)'"

help:
	@echo "nms-agent - Network Monitoring Agent"
	@echo ""
	@echo "Available commands:"
	@echo "  make build       - Build the binary"
	@echo "  make run         - Run the agent"
	@echo "  make dev         - Run with hot reload (requires air)"
	@echo "  make test        - Run unit tests"
	@echo "  make test-int    - Run integration tests"
	@echo "  make bench       - Run benchmarks"
	@echo "  make lint        - Run linter"
	@echo "  make fmt         - Format code"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make install     - Install as systemd service"
	@echo "  make deploy      - Build and deploy"
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-run  - Run with Docker Compose"

build:
	@echo "Building $(BINARY_NAME) v$(VERSION)..."
	@mkdir -p bin
	@go build $(LDFLAGS) -o $(BINARY_PATH) $(MAIN_PACKAGE)
	@echo "✓ Build successful: $(BINARY_PATH)"

run: build
	@echo "Running $(BINARY_NAME)..."
	@$(BINARY_PATH) -config config.yaml

dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	@air -c .air.toml

test:
	@echo "Running unit tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

test-int:
	@echo "Running integration tests..."
	@docker-compose -f docker-compose.test.yml up -d
	@go test -v -tags integration -race ./...
	@docker-compose -f docker-compose.test.yml down

bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

lint:
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w .

vet:
	@echo "Running go vet..."
	@go vet ./...

mod-tidy:
	@echo "Tidying modules..."
	@go mod tidy
	@go mod verify

mod-update:
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy

clean:
	@echo "Cleaning up..."
	@rm -rf bin/
	@rm -rf coverage.*
	@go clean -testcache
	@echo "✓ Cleanup complete"

install: build
	@echo "Installing nms-agent as systemd service..."
	@sudo groupadd -f nms-agent
	@sudo useradd -m -g nms-agent nms-agent || true
	@sudo mkdir -p /opt/nms-agent/bin
	@sudo mkdir -p /etc/nms-agent
	@sudo mkdir -p /var/log/nms-agent
	@sudo cp $(BINARY_PATH) /opt/nms-agent/bin/
	@sudo cp config.example.yaml /etc/nms-agent/config.yaml
	@sudo cp deployments/systemd/nms-agent.service /etc/systemd/system/
	@sudo cp deployments/systemd/nms-agent.default /etc/default/nms-agent
	@sudo chown -R nms-agent:nms-agent /opt/nms-agent
	@sudo chown -R nms-agent:nms-agent /var/log/nms-agent
	@sudo systemctl daemon-reload
	@sudo systemctl enable nms-agent
	@echo "✓ Installation complete"
	@echo "  Start service: sudo systemctl start nms-agent"
	@echo "  View status:   sudo systemctl status nms-agent"
	@echo "  View logs:     sudo journalctl -u nms-agent -f"

uninstall:
	@echo "Uninstalling nms-agent..."
	@sudo systemctl stop nms-agent || true
	@sudo systemctl disable nms-agent || true
	@sudo rm -f /etc/systemd/system/nms-agent.service
	@sudo rm -rf /opt/nms-agent
	@sudo systemctl daemon-reload
	@echo "✓ Uninstallation complete"

deploy: clean build install
	@echo "✓ Deployment complete"
	@sudo systemctl restart nms-agent

docker-build:
	@echo "Building Docker image..."
	@docker build -f deployments/docker/Dockerfile -t nms-agent:$(VERSION) .
	@echo "✓ Docker image built: nms-agent:$(VERSION)"

docker-run:
	@echo "Starting Docker Compose environment..."
	@docker-compose -f deployments/docker/docker-compose.yml up -d
	@echo "✓ Services started"
	@echo "  View logs: docker-compose logs -f"

docker-stop:
	@echo "Stopping Docker Compose environment..."
	@docker-compose -f deployments/docker/docker-compose.yml down
	@echo "✓ Services stopped"

.PHONY: all
all: clean build test lint

.PHONY: ci
ci: mod-tidy vet lint test build
	@echo "✓ CI pipeline complete"

