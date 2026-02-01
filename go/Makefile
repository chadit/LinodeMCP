.PHONY: help build build-prod run test test-race test-all clean lint fmt install-tools coverage tidy

help:
	@echo "LinodeMCP Development Commands:"
	@echo ""
	@echo "Build:"
	@echo "  make build           - Build development binary"
	@echo "  make build-prod      - Build security-hardened binary"
	@echo "  make run             - Build and run"
	@echo ""
	@echo "Test:"
	@echo "  make test            - Run unit tests"
	@echo "  make test-race       - Run tests with race detector"
	@echo "  make test-all        - Run all tests"
	@echo "  make coverage        - Generate coverage report"
	@echo ""
	@echo "Tools:"
	@echo "  make lint            - Run golangci-lint"
	@echo "  make fmt             - Format with gofumpt"
	@echo "  make tidy            - Tidy dependencies"
	@echo "  make clean           - Clean artifacts"
	@echo "  make install-tools   - Install dev tools"

build:
	@echo "Building linodemcp..."
	@go build -o bin/linodemcp cmd/linodemcp/main.go

build-prod:
	@echo "Building linodemcp (production)..."
	@go build \
		-ldflags="-s -w -buildid=" \
		-trimpath \
		-buildmode=pie \
		-tags=netgo,osusergo \
		-o bin/linodemcp cmd/linodemcp/main.go

run: build
	@./bin/linodemcp

test:
	@echo "Running unit tests..."
	@go test -v ./... -short

test-race:
	@go test -race ./...

test-all:
	@go test -race -v ./...

clean:
	@rm -rf bin/ dist/ coverage.txt coverage.html *.coverprofile

lint:
	@golangci-lint run

fmt:
	@gofumpt -w .

tidy:
	@go mod tidy
	@go mod verify

coverage:
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

install-tools:
	@go install mvdan.cc/gofumpt@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
