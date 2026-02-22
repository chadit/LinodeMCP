.PHONY: help build build-prod run test test-race test-all clean lint fmt install-tools coverage tidy gopls-check check _ensure-hooks update-hooks

# Auto-install pre-commit hooks if not already set up
_ensure-hooks:
	@if command -v pre-commit >/dev/null 2>&1; then \
		if [ ! -f ../.git/hooks/pre-commit ] || ! grep -q 'pre-commit' ../.git/hooks/pre-commit 2>/dev/null; then \
			echo "Installing pre-commit hooks..."; \
			cd .. && pre-commit install; \
		fi \
	fi

update-hooks:
	@cd .. && pre-commit autoupdate

help: _ensure-hooks
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
	@echo "  make fmt             - Format with goimports and gofumpt"
	@echo "  make gopls-check     - Run gopls diagnostics"
	@echo "  make tidy            - Tidy dependencies"
	@echo "  make clean           - Clean artifacts"
	@echo "  make check           - Run fmt, lint, and test (pre-commit)"
	@echo "  make install-tools   - Install dev tools"
	@echo "  make update-hooks    - Update pre-commit hooks to latest versions"

build: _ensure-hooks
	@echo "Building linodemcp..."
	@go build -o bin/linodemcp cmd/linodemcp/main.go

build-prod: _ensure-hooks
	@echo "Building linodemcp (production)..."
	@go build \
		-ldflags="-s -w -buildid=" \
		-trimpath \
		-buildmode=pie \
		-tags=netgo,osusergo \
		-o bin/linodemcp cmd/linodemcp/main.go

run: build
	@./bin/linodemcp

test: _ensure-hooks
	@echo "Running unit tests..."
	@go test -race -cover ./...

test-race: _ensure-hooks
	@go test -race ./...

test-all: _ensure-hooks
	@go test -race -v ./...

clean:
	@rm -rf bin/ dist/ coverage.txt coverage.html *.coverprofile

lint: _ensure-hooks
	@golangci-lint run --max-issues-per-linter=0 --max-same-issues=0
	@gopls check -severity=hint $$(find . -name '*.go' -not -path './vendor/*')

fmt: _ensure-hooks
	@goimports -w .
	@gofumpt -w .

gopls-check: _ensure-hooks
	@gopls check -severity=hint $$(find . -name '*.go' -not -path './vendor/*')

tidy: _ensure-hooks
	@go mod tidy
	@go mod verify

coverage: _ensure-hooks
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

check: fmt lint test

install-tools:
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install mvdan.cc/gofumpt@latest
	@go install golang.org/x/tools/gopls@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
