.PHONY: help clean \
	docker-build-go docker-build-python docker-build-all \
	docker-run-go docker-run-python docker-clean \
	go-build go-test go-lint go-fmt go-clean go-run go-check \
	python-build python-test python-lint python-fmt python-clean python-run python-check

CONTAINER_ENGINE ?= docker
GO_IMAGE := linodemcp:go
PYTHON_IMAGE := linodemcp:python

help:
	@echo "LinodeMCP - Root Makefile"
	@echo ""
	@echo "Container targets:"
	@echo "  docker-build-go       Build Go container image"
	@echo "  docker-build-python   Build Python container image"
	@echo "  docker-build-all      Build both container images"
	@echo "  docker-run-go         Run Go container (stdin open, token forwarded)"
	@echo "  docker-run-python     Run Python container (stdin open, token forwarded)"
	@echo ""
	@echo "Go pass-through targets:"
	@echo "  go-build              Build Go binary (make -C go build)"
	@echo "  go-test               Run Go tests (make -C go test)"
	@echo "  go-lint               Lint Go code (make -C go lint)"
	@echo "  go-fmt                Format Go code (make -C go fmt)"
	@echo "  go-clean              Clean Go artifacts (make -C go clean)"
	@echo "  go-run                Run Go server (make -C go run)"
	@echo "  go-check              Run Go fmt+lint+test (make -C go check)"
	@echo ""
	@echo "Python pass-through targets:"
	@echo "  python-build          Install Python dev deps (make -C python install-dev)"
	@echo "  python-test           Run Python tests (make -C python test)"
	@echo "  python-lint           Lint Python code (make -C python lint)"
	@echo "  python-fmt            Format Python code (make -C python format)"
	@echo "  python-clean          Clean Python artifacts (make -C python clean)"
	@echo "  python-run            Run Python server (make -C python run)"
	@echo "  python-check          Run Python lint+typecheck+test (make -C python check)"
	@echo ""
	@echo "Cleanup targets:"
	@echo "  clean                 Clean all build artifacts and container images"
	@echo "  docker-clean          Remove container images only"
	@echo ""
	@echo "Use CONTAINER_ENGINE=podman to swap Docker for Podman."

# --- Container targets ---

docker-build-go:
	$(CONTAINER_ENGINE) build -t $(GO_IMAGE) go/

docker-build-python:
	$(CONTAINER_ENGINE) build -t $(PYTHON_IMAGE) python/

docker-build-all: docker-build-go docker-build-python

docker-run-go:
	$(CONTAINER_ENGINE) run -i --rm -e LINODEMCP_LINODE_TOKEN $(GO_IMAGE)

docker-run-python:
	$(CONTAINER_ENGINE) run -i --rm -e LINODEMCP_LINODE_TOKEN $(PYTHON_IMAGE)

# --- Go pass-through targets ---

go-build:
	$(MAKE) -C go build

go-test:
	$(MAKE) -C go test

go-lint:
	$(MAKE) -C go lint

go-fmt:
	$(MAKE) -C go fmt

go-clean:
	$(MAKE) -C go clean

go-run:
	$(MAKE) -C go run

go-check:
	$(MAKE) -C go check

# --- Python pass-through targets ---

python-build:
	$(MAKE) -C python install-dev

python-test:
	$(MAKE) -C python test

python-lint:
	$(MAKE) -C python lint

python-fmt:
	$(MAKE) -C python format

python-clean:
	$(MAKE) -C python clean

python-run:
	$(MAKE) -C python run

python-check:
	$(MAKE) -C python check

# --- Cleanup targets ---

docker-clean:
	-$(CONTAINER_ENGINE) rmi $(GO_IMAGE) 2>/dev/null
	-$(CONTAINER_ENGINE) rmi $(PYTHON_IMAGE) 2>/dev/null
	-$(CONTAINER_ENGINE) image prune -f --filter="label=io.buildah.version" 2>/dev/null

clean: go-clean python-clean docker-clean
