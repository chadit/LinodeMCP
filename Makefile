.PHONY: help check lint clean \
	docker-build-go docker-build-python docker-build-all \
	docker-run-go docker-run-python docker-clean \
	go-build go-test go-lint go-fmt go-clean go-run go-check \
	python-build python-test python-lint python-fmt python-clean python-run python-check \
	betterleaks trivy

CONTAINER_ENGINE ?= docker
GO_IMAGE := linodemcp:go
PYTHON_IMAGE := linodemcp:python

## help: Show this help message
help:
	@echo "LinodeMCP - Root Makefile (use CONTAINER_ENGINE=podman to swap Docker)"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //' | awk -F': ' '{printf "  make %-22s %s\n", $$1, $$2}'

# --- Top-level targets ---

## check: Run all linters and tests (go-check + python-check)
check: go-check python-check

## lint: Run all linters (go-lint, python-lint, betterleaks, trivy)
lint: go-lint python-lint betterleaks trivy

# --- Container targets ---

## docker-build-go: Build Go container image
docker-build-go:
	$(CONTAINER_ENGINE) build -t $(GO_IMAGE) go/

## docker-build-python: Build Python container image
docker-build-python:
	$(CONTAINER_ENGINE) build -t $(PYTHON_IMAGE) python/

## docker-build-all: Build both container images
docker-build-all: docker-build-go docker-build-python

## docker-run-go: Run Go container (stdin open, token forwarded)
docker-run-go:
	$(CONTAINER_ENGINE) run -i --rm -e LINODEMCP_LINODE_TOKEN $(GO_IMAGE)

## docker-run-python: Run Python container (stdin open, token forwarded)
docker-run-python:
	$(CONTAINER_ENGINE) run -i --rm -e LINODEMCP_LINODE_TOKEN $(PYTHON_IMAGE)

# --- Go pass-through targets ---

## go-build: Build Go binary
go-build:
	$(MAKE) -C go build

## go-test: Run Go tests
go-test:
	$(MAKE) -C go test

## go-lint: Lint Go code
go-lint:
	$(MAKE) -C go lint

## go-fmt: Format Go code
go-fmt:
	$(MAKE) -C go fmt

## go-clean: Clean Go artifacts
go-clean:
	$(MAKE) -C go clean

## go-run: Run Go server
go-run:
	$(MAKE) -C go run

## go-check: Run Go fmt+lint+test
go-check:
	$(MAKE) -C go check

# --- Python pass-through targets ---

## python-build: Install Python dev deps
python-build:
	$(MAKE) -C python install-dev

## python-test: Run Python tests
python-test:
	$(MAKE) -C python test

## python-lint: Lint Python code
python-lint:
	$(MAKE) -C python lint

## python-fmt: Format Python code
python-fmt:
	$(MAKE) -C python format

## python-clean: Clean Python artifacts
python-clean:
	$(MAKE) -C python clean

## python-run: Run Python server
python-run:
	$(MAKE) -C python run

## python-check: Run Python lint+typecheck+test
python-check:
	$(MAKE) -C python check

# --- Shared linters ---

## betterleaks: Run betterleaks secrets scan
betterleaks:
	@if command -v betterleaks >/dev/null 2>&1; then \
		echo "Running betterleaks secrets scan..."; \
		betterleaks dir .; \
	else \
		echo "[warn] betterleaks not installed, skipping secrets scan"; \
	fi

## trivy: Run trivy security scan
trivy:
	@if command -v trivy >/dev/null 2>&1; then \
		echo "Running trivy security scan..."; \
		trivy fs --scanners vuln,misconfig --exit-code 1 .; \
	else \
		echo "[warn] trivy not installed, skipping security scan"; \
	fi

# --- Cleanup targets ---

## docker-clean: Remove container images only
docker-clean:
	-$(CONTAINER_ENGINE) rmi $(GO_IMAGE) 2>/dev/null
	-$(CONTAINER_ENGINE) rmi $(PYTHON_IMAGE) 2>/dev/null
	-$(CONTAINER_ENGINE) image prune -f --filter="label=io.buildah.version" 2>/dev/null

## clean: Clean all build artifacts and container images
clean: go-clean python-clean docker-clean
