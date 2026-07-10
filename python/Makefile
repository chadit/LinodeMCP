VENV := .venv
PYTHON := $(VENV)/bin/python
PIP := $(VENV)/bin/pip
RUFF := $(VENV)/bin/ruff
MYPY := $(VENV)/bin/mypy
PYTEST := $(VENV)/bin/pytest
# npx @latest, not a globally installed pyright: a global install is whatever
# version was installed last (stale-tool drift vs CI's npm latest), and a
# `command -v pyright || skip` guard silently skipped the check on machines
# without it. Same rationale as gosec via `go run @latest` in go/Makefile.
PYRIGHT := npx --yes pyright@latest

.PHONY: help venv install install-dev build test test-cov lint fmt-check format typecheck lock-check vulture deptry clean run all check _ensure-hooks update-hooks

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
	@echo "Running mypy type check..."
	@# Unconditional: a skip-if-missing guard here let machines without the
	@# tool pass a check CI fails. mypy is guaranteed by `make install-dev`.
	@$(MYPY) src/ tests/
	@echo "Running pyright strict mode..."
	@cd .. && $(PYRIGHT) python/

format: _ensure-hooks
	$(RUFF) format src/ tests/

## fmt-check: Verify Python formatting (read-only; generated genpb excluded via ruff config)
fmt-check: _ensure-hooks
	@echo "Running ruff format --check..."
	@$(RUFF) format --check src/ tests/

typecheck: _ensure-hooks
	$(MYPY) src/linodemcp

# Catch pyproject.toml <-> uv.lock drift in the normal check chain, not just at
# pre-push. A dep floor bump that skips regenerating the lock passes lint/test
# but fails `uv lock --check` on push; running it here surfaces it early. Skips
# with a warning when uv is absent, matching the pre-commit hook and the
# betterleaks/trivy skip pattern.
lock-check: _ensure-hooks
	@echo "Checking uv.lock is in sync with pyproject.toml..."
	@# Hard requirement, not skip-if-missing: the old warn-skip meant CI's
	@# make-check job (no uv installed) silently never ran this check.
	@command -v uv >/dev/null 2>&1 || { echo "[error] uv required for lock-check (install: pip install uv)" >&2; exit 1; }
	@uv lock --check

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
