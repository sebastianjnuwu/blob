## Help: lists all available commands
help:
	@echo "Available commands:"
	@echo "  make init     - Install all dependencies and prepare the environment"
	@echo "  make deps     - Download Go dependencies (go mod download)"
	@echo "  make tidy     - Clean up go.mod/go.sum (go mod tidy)"
	@echo "  make vendor   - Update vendor folder (go mod vendor)"
	@echo "  make lint     - Run linter (golangci-lint)"
	@echo "  make format   - Format code (go fmt)"
	@echo "  make test     - Run tests"
	@echo "  make build    - Build main binary"
	@echo "  make typos    - Check for common typos in the codebase (requires 'typos' tool)"
	@echo "  make run      - Run the main application"
	@echo "  make clean    - Remove binaries and cache"
	@echo "  make help     - Show this message"

## Install all dependencies and prepare the environment
init: tidy deps vendor

## Download Go dependencies
deps:
	go mod download

## Clean up go.mod/go.sum
tidy:
	go mod tidy

## Update vendor folder
vendor:
	go mod vendor

## Run linter
lint:
	golangci-lint run ./...

## Format code
format:
	go fmt ./...

## Run tests
test:
	go test ./...

## Build main binary
build:
	go build -o blob-server main.go

## Check for common typos in the codebase (requires 'typos' tool)
typos:
    typos --config ./typos.toml

## Run the main application
run:
	go run main.go

## Remove binaries and cache (cross-platform)
clean:
	del /f /q blob-server.exe blob-server 2>nul || true
	go clean -modcache
