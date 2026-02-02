VENV := .venv
PYTHON := $(VENV)/bin/python
PIP := $(VENV)/bin/pip
RUFF := $(VENV)/bin/ruff
MYPY := $(VENV)/bin/mypy
PYTEST := $(VENV)/bin/pytest

.PHONY: help venv install install-dev test test-cov lint format typecheck clean run all

help:
	@echo "LinodeMCP Python - Available targets:"
	@echo "  venv         - Create virtual environment"
	@echo "  install      - Install package"
	@echo "  install-dev  - Install package with dev dependencies"
	@echo "  test         - Run tests"
	@echo "  test-cov     - Run tests with coverage report"
	@echo "  lint         - Run ruff linter"
	@echo "  format       - Format code with ruff"
	@echo "  typecheck    - Run mypy type checker"
	@echo "  clean        - Remove build artifacts"
	@echo "  run          - Run the server"

venv:
	python3 -m venv $(VENV)
	$(PIP) install --upgrade pip

install: venv
	$(PIP) install .

install-dev: venv
	$(PIP) install -e ".[dev]"

test:
	$(PYTEST)

test-cov:
	$(PYTEST) --cov=linodemcp --cov-report=term-missing --cov-report=html

lint:
	$(RUFF) check src/ tests/

format:
	$(RUFF) format src/ tests/

typecheck:
	$(MYPY) src/linodemcp

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

run:
	$(VENV)/bin/linodemcp

all: clean install-dev lint typecheck test
