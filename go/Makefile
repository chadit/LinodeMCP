.PHONY: help build build-prod run test test-race test-all clean lint lint-fast fmt fmt-check install-tools coverage tidy gopls-check check _ensure-hooks update-hooks

# Auto-install commit and push hooks if not already set up
_ensure-hooks:
	@$(MAKE) -C .. install-hooks

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
	@go build -o bin/linodemcp ./cmd/linodemcp

APPINFO_PKG := github.com/chadit/LinodeMCP/go/internal/appinfo
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
		-o bin/linodemcp ./cmd/linodemcp

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
	@# go run @latest, unconditionally, byte-matching CI. A PATH-guarded gosec
	@# silently SKIPS the scan on any machine without it installed (local passes,
	@# CI fails), and a stale PATH binary misses what CI's latest catches.
	@# -exclude-generated skips the protoc-gen-go tree (genpb).
	@go run github.com/securego/gosec/v2/cmd/gosec@latest -quiet -exclude-generated ./...
	@echo "Running cairnlint..."
	@go run github.com/chadit/cairnlint@latest ./...

## lint-fast: Run golangci-lint only (CI fast-lane, skips gosec/cairnlint)
lint-fast: _ensure-hooks
	@echo "Running golangci-lint..."
	@golangci-lint run --allow-parallel-runners --max-issues-per-linter=0 --max-same-issues=0 ./...

# Hand-written Go sources, excluding the generated genpb tree. protoc-gen-go
# emits a single import group that goimports regroups into two (stdlib split
# out), so format-gating generated code makes a fresh `buf generate` (one group)
# diverge from a formatted on-disk copy (two groups) -- the exact local-vs-CI
# drift that broke CI. Generated code is never format-gated; its source is the
# proto contract, and a clean regen must reproduce it byte-for-byte.
GO_FMT_SRC := $(shell find . -name '*.go' -not -path './internal/genpb/*')

## fmt: Format hand-written Go with goimports and gofumpt (never the generated tree)
fmt: _ensure-hooks
	@goimports -w $(GO_FMT_SRC)
	@gofumpt -w $(GO_FMT_SRC)

## fmt-check: Verify hand-written code is formatted (CI gate, no writes, excludes genpb)
fmt-check:
	@unformatted=$$(gofumpt -l $(GO_FMT_SRC) 2>/dev/null); \
	if [ -n "$$unformatted" ]; then \
		echo "[error] gofumpt would reformat:"; echo "$$unformatted"; exit 1; \
	fi
	@unimports=$$(goimports -l $(GO_FMT_SRC) 2>/dev/null); \
	if [ -n "$$unimports" ]; then \
		echo "[error] goimports would rewrite:"; echo "$$unimports"; exit 1; \
	fi

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

## check: Run lint and test (formatting is gated by the root `fmt-check` target)
check: lint test

## install-tools: Install dev tools (goimports, gofumpt, gopls, golangci-lint)
install-tools:
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install mvdan.cc/gofumpt@latest
	@go install golang.org/x/tools/gopls@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
