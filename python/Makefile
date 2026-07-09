VENV := .venv
PYTHON := $(VENV)/bin/python
PIP := $(VENV)/bin/pip
RUFF := $(VENV)/bin/ruff
MYPY := $(VENV)/bin/mypy
PYTEST := $(VENV)/bin/pytest
PYRIGHT := pyright

.PHONY: help venv install install-dev build test test-cov lint format typecheck lock-check vulture deptry clean run all check _ensure-hooks update-hooks

# Auto-install commit and push hooks if not already set up
_ensure-hooks:
	@$(MAKE) -C .. install-hooks

update-hooks:
	@cd .. && pre-commit autoupdate

help: _ensure-hooks
	@echo "LinodeMCP Python - Available targets:"
	@echo "  venv         - Create virtual environment"
	@echo "  install      - Install package"
	@echo "  install-dev  - Install package with dev dependencies"
	@echo "  build        - Build wheel and sdist into bin/"
	@echo "  test         - Run tests"
	@echo "  test-cov     - Run tests with coverage report"
	@echo "  lint         - Run ruff linter"
	@echo "  format       - Format code with ruff"
	@echo "  typecheck    - Run mypy type checker"
	@echo "  lock-check   - Verify uv.lock is in sync with pyproject.toml"
	@echo "  vulture      - Find dead code"
	@echo "  deptry       - Check for unused dependencies"
	@echo "  clean        - Remove build artifacts"
	@echo "  run          - Run the server"
	@echo "  check        - Run lint, typecheck, and test (pre-commit)"
	@echo "  update-hooks - Update pre-commit hooks to latest versions"

venv: _ensure-hooks
	python3 -m venv $(VENV)
	$(PIP) install --upgrade pip

install: venv
	$(PIP) install .

install-dev: venv
	$(PIP) install -e ".[dev,observability]"

build: venv
	@echo "Building linodemcp wheel + sdist into bin/..."
	@$(PIP) install --quiet --upgrade build
	@rm -rf bin/
	@$(PYTHON) -m build --outdir bin/

test: _ensure-hooks
	$(PYTEST)

test-cov: _ensure-hooks
	$(PYTEST) --cov=linodemcp --cov-report=term-missing --cov-report=html

lint: _ensure-hooks
	@echo "Running ruff check..."
	@$(RUFF) check src/ tests/
	@echo "Running ruff format --check..."
	@$(RUFF) format --check src/ tests/
	@echo "Running mypy type check..."
	@if [ -x "$(MYPY)" ]; then \
		$(MYPY) src/ tests/; \
	else \
		echo "[warn] mypy not installed, skipping type check"; \
	fi
	@echo "Running pyright strict mode..."
	@if command -v $(PYRIGHT) >/dev/null 2>&1; then \
		cd .. && $(PYRIGHT) python/; \
	else \
		echo "[warn] pyright not installed, skipping strict type check"; \
		echo "       install: npm install -g pyright"; \
	fi

format: _ensure-hooks
	$(RUFF) format src/ tests/

typecheck: _ensure-hooks
	$(MYPY) src/linodemcp

# Catch pyproject.toml <-> uv.lock drift in the normal check chain, not just at
# pre-push. A dep floor bump that skips regenerating the lock passes lint/test
# but fails `uv lock --check` on push; running it here surfaces it early. Skips
# with a warning when uv is absent, matching the pre-commit hook and the
# betterleaks/trivy skip pattern.
lock-check: _ensure-hooks
	@echo "Checking uv.lock is in sync with pyproject.toml..."
	@if command -v uv >/dev/null 2>&1; then \
		uv lock --check; \
	else \
		echo "[warn] uv not installed, skipping lock-sync check"; \
	fi

vulture: _ensure-hooks
	vulture src/linodemcp --min-confidence 80

deptry: _ensure-hooks
	deptry src/linodemcp

clean:
	rm -rf bin/
	rm -rf build/
	rm -rf dist/
	rm -rf *.egg-info
	rm -rf .pytest_cache/
	rm -rf .mypy_cache/
	rm -rf .ruff_cache/
	rm -rf htmlcov/
	rm -rf .coverage
	find . -type d -name __pycache__ -exec rm -rf {} +
	find . -type f -name "*.pyc" -delete

run: _ensure-hooks
	$(VENV)/bin/linodemcp

check: lock-check lint typecheck test

all: clean install-dev lock-check lint typecheck test
