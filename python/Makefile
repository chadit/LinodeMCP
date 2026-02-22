VENV := .venv
PYTHON := $(VENV)/bin/python
PIP := $(VENV)/bin/pip
RUFF := $(VENV)/bin/ruff
MYPY := $(VENV)/bin/mypy
PYTEST := $(VENV)/bin/pytest

.PHONY: help venv install install-dev test test-cov lint format typecheck vulture deptry clean run all check _ensure-hooks update-hooks

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
	@echo "LinodeMCP Python - Available targets:"
	@echo "  venv         - Create virtual environment"
	@echo "  install      - Install package"
	@echo "  install-dev  - Install package with dev dependencies"
	@echo "  test         - Run tests"
	@echo "  test-cov     - Run tests with coverage report"
	@echo "  lint         - Run ruff linter"
	@echo "  format       - Format code with ruff"
	@echo "  typecheck    - Run mypy type checker"
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
	$(PIP) install -e ".[dev]"

test: _ensure-hooks
	$(PYTEST)

test-cov: _ensure-hooks
	$(PYTEST) --cov=linodemcp --cov-report=term-missing --cov-report=html

lint: _ensure-hooks
	$(RUFF) check src/ tests/

format: _ensure-hooks
	$(RUFF) format src/ tests/

typecheck: _ensure-hooks
	$(MYPY) src/linodemcp

vulture: _ensure-hooks
	vulture src/linodemcp --min-confidence 80

deptry: _ensure-hooks
	deptry src/linodemcp

clean:
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

check: lint typecheck test

all: clean install-dev lint typecheck test
