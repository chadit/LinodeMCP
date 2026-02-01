.PHONY: help install install-dev test test-cov lint format typecheck clean run

help:
	@echo "LinodeMCP Python - Available targets:"
	@echo "  install      - Install package"
	@echo "  install-dev  - Install package with dev dependencies"
	@echo "  test         - Run tests"
	@echo "  test-cov     - Run tests with coverage report"
	@echo "  lint         - Run ruff linter"
	@echo "  format       - Format code with ruff"
	@echo "  typecheck    - Run mypy type checker"
	@echo "  clean        - Remove build artifacts"
	@echo "  run          - Run the server"

install:
	pip install .

install-dev:
	pip install -e ".[dev]"

test:
	pytest

test-cov:
	pytest --cov=linodemcp --cov-report=term-missing --cov-report=html

lint:
	ruff check src/ tests/

format:
	ruff format src/ tests/

typecheck:
	mypy src/linodemcp

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
	linodemcp

all: clean install-dev lint typecheck test
