.PHONY: help build build-prod run test test-race test-all clean lint fmt install-tools coverage tidy gopls-check check _ensure-hooks update-hooks

# Auto-install pre-commit hooks if not already set up
_ensure-hooks:
	@if command -v pre-commit >/dev/null 2>&1; then \
		if [ ! -f ../.git/hooks/pre-commit ] || ! grep -q 'pre-commit' ../.git/hooks/pre-commit 2>/dev/null; then \
			echo "Installing pre-commit hooks..."; \
			cd .. && pre-commit install; \
		fi \
	fi

## help: Show this help message
help: _ensure-hooks
	@echo "LinodeMCP Go Development Commands:"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //' | awk -F': ' '{printf "  make %-18s %s\n", $$1, $$2}'

## update-hooks: Update pre-commit hooks to latest versions
update-hooks:
	@cd .. && pre-commit autoupdate

## build: Build development binary
build: _ensure-hooks
	@echo "Building linodemcp..."
	@go build -o bin/linodemcp cmd/linodemcp/main.go

APPINFO_PKG := github.com/chadit/LinodeMCP/internal/appinfo
LDFLAGS := -s -w -buildid= \
	-X '$(APPINFO_PKG).buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)'

## build-prod: Build security-hardened binary
build-prod: _ensure-hooks
	@echo "Building linodemcp (production)..."
	@go build \
		-ldflags="$(LDFLAGS)" \
		-trimpath \
		-buildmode=pie \
		-tags=netgo,osusergo \
		-o bin/linodemcp cmd/linodemcp/main.go

## run: Build and run
run: build
	@./bin/linodemcp

## test: Run unit tests with coverage
test: _ensure-hooks
	@echo "Running unit tests..."
	@go test -race -cover ./...

## test-race: Run tests with race detector
test-race: _ensure-hooks
	@go test -race ./...

## test-all: Run all tests verbose with race detector
test-all: _ensure-hooks
	@go test -race -v ./...

## clean: Clean build artifacts
clean:
	@rm -rf bin/ dist/ coverage.txt coverage.html *.coverprofile

## lint: Run golangci-lint, gosec, and cairnlint
lint: _ensure-hooks
	@echo "Running golangci-lint..."
	@golangci-lint run --allow-parallel-runners --max-issues-per-linter=0 --max-same-issues=0 ./...
	@echo "Running gosec..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec -quiet ./...; \
	else \
		echo "[warn] gosec not installed, skipping security scan"; \
	fi
	@echo "Running cairnlint..."
	@if command -v cairnlint >/dev/null 2>&1; then \
		cairnlint ./...; \
	elif command -v go >/dev/null 2>&1; then \
		go run github.com/chadit/cairnlint@latest ./...; \
	else \
		echo "[warn] cairnlint not installed, skipping custom rules check"; \
	fi

## fmt: Format with goimports and gofumpt
fmt: _ensure-hooks
	@goimports -w .
	@gofumpt -w .

## gopls-check: Run gopls diagnostics
gopls-check: _ensure-hooks
	@gopls check -severity=hint $$(find . -name '*.go' -not -path './vendor/*')

## tidy: Tidy and verify dependencies
tidy: _ensure-hooks
	@go mod tidy
	@go mod verify

## coverage: Generate HTML coverage report
coverage: _ensure-hooks
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## check: Run fmt, lint, and test
check: fmt lint test

## install-tools: Install dev tools (goimports, gofumpt, gopls, golangci-lint)
install-tools:
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install mvdan.cc/gofumpt@latest
	@go install golang.org/x/tools/gopls@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
