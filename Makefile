.PHONY: all fmt lint test test-coverage build run clean generate help security

# Default target
all: fmt lint test build

## fmt: Format code with goimports and gofmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "goimports not found. Install with: go install golang.org/x/tools/cmd/goimports@latest"; \
	fi

## lint: Run golangci-lint
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install from: https://golangci-lint.run/usage/install/"; \
	fi

## test: Run all tests with race detector
test:
	@echo "Running tests..."
	@go test -v -race ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## test-unit: Run only unit tests (skip integration)
test-unit:
	@echo "Running unit tests..."
	@go test -v -race -short ./...

## test-integration: Run only integration tests
test-integration:
	@echo "Running integration tests..."
	@go test -v -race ./tests/integration/...

## build: Build the kore binary
build:
	@echo "Building kore..."
	@go build -o bin/kore cmd/kore/main.go
	@echo "Binary built: bin/kore"

## build-all: Build for all platforms
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p bin
	@GOOS=linux GOARCH=amd64 go build -o bin/kore-linux-amd64 cmd/kore/main.go
	@GOOS=darwin GOARCH=amd64 go build -o bin/kore-darwin-amd64 cmd/kore/main.go
	@GOOS=darwin GOARCH=arm64 go build -o bin/kore-darwin-arm64 cmd/kore/main.go
	@GOOS=windows GOARCH=amd64 go build -o bin/kore-windows-amd64.exe cmd/kore/main.go
	@echo "Binaries built in bin/"

## run: Run the application
run:
	@echo "Running Kore..."
	@go run cmd/kore/main.go

## clean: Clean build artifacts and test cache
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@go clean -testcache
	@echo "Clean complete"

## generate: Generate mock code and wire dependencies
generate:
	@echo "Generating code..."
	@go generate ./...

## security: Run security scan with gosec
security:
	@echo "Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not found. Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
	fi

## deps: Download and tidy dependencies
deps:
	@echo "Tidying dependencies..."
	@go mod download
	@go mod tidy

## verify: Verify dependencies
verify:
	@echo "Verifying dependencies..."
	@go mod verify

## update-deps: Update dependencies
update-deps:
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy

## install: Install kore to GOPATH/bin
install:
	@echo "Installing kore..."
	@go install cmd/kore/main.go
	@echo "kore installed to $(go env GOPATH)/bin"

## dev: Run development workflow (fmt -> lint -> test)
dev: fmt lint test

## ci: Run CI pipeline locally
ci: lint security test-coverage build

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
