.PHONY: help build test check check-container lint fmt-check go-fmt-check python-fmt-check scripts-fmt-check scripts-lint clean install-hooks check-hooks tool-parity profile-resolution tool-count dryrun pagination response-shapes list-envelope tool-routes api-surfaces field-location tool-capability tool-response route-evidence route-source generated-tools hand-validators hook-bodies system-params env-parity cli-surface docs-links metrics-surface coverage-floor coverage-report diff-coverage write-proto read-proto input-proto meta-proto behavior messages sync sync-enums sync-defaults sync-pagination sync-response-shapes sync-scopes sync-issues baseline-guard tool-float parity-todo \
	docker-build-go docker-build-python docker-build-all \
	docker-run-go docker-run-python docker-clean \
	go-build go-build-prod go-test go-lint go-fmt go-clean go-run go-check \
	python-build python-install-dev python-test python-lint python-fmt python-clean python-run python-check \
	betterleaks trivy actionlint proto generate

CONTAINER_ENGINE ?= docker
GO_IMAGE := linodemcp:go
PYTHON_IMAGE := linodemcp:python

## help: Show this help message
help:
	@echo "LinodeMCP - Root Makefile (use CONTAINER_ENGINE=podman to swap Docker)"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //' | awk -F': ' '{printf "  make %-22s %s\n", $$1, $$2}'
	@echo ""
	@echo "Every gate target stays invocable by name; docs/gates.md describes each one."

# --- Proto codegen ---
# Generated code is gitignored. Stamp-gated so build/test regenerate only when
# the proto sources change, which keeps offline builds working after one run.
PROTO_SRCS := $(shell find proto -name '*.proto') buf.yaml buf.gen.yaml buf.gen.protovalidate.yaml $(wildcard buf.lock) \
	docs/contracts/handwritten-tools.txt docs/contracts/languages.txt \
	scripts/gen_tool_registries.py \
	$(shell find go/cmd/toolgen -name '*.go' -not -name '*_test.go')
PROTO_STAMP := .make/proto-generated

## proto: Generate Go + Python types and MCP schemas from proto/ (needs buf)
proto: $(PROTO_STAMP)

generate: proto

$(PROTO_STAMP): $(PROTO_SRCS)
	@command -v buf >/dev/null 2>&1 || { echo "buf is required: https://buf.build/docs/installation"; exit 1; }
	buf generate
	@# Second pass, after the first because that one wipes its output: the Python
	@# messages for the buf.validate options the contract carries. protovalidate's
	@# wheel does not ship them and its runtime imports them by name, so buf writes
	@# them here the same way protovalidate's own build writes its copy.
	buf generate buf.build/bufbuild/protovalidate --template buf.gen.protovalidate.yaml
	@# protoc-gen-python emits absolute cross-proto imports (from linode.mcp.v1 import X),
	@# which would put a top-level `linode` on sys.path and register descriptors twice.
	perl -pi -e 's{^from linode\.mcp\.v1 import }{from linodemcp.genpb.linode.mcp.v1 import }' python/src/linodemcp/genpb/linode/mcp/v1/*_pb2.py python/src/linodemcp/genpb/linode/mcp/v1/*_pb2.pyi
	@# Same reason for the validation options: a bare `from buf.validate import ...`
	@# would load a second copy of validate_pb2 under another module name, and the
	@# descriptor pool refuses the same file twice.
	perl -pi -e 's{^from buf\.validate import }{from linodemcp.genpb.buf.validate import }' python/src/linodemcp/genpb/linode/mcp/v1/*_pb2.py python/src/linodemcp/genpb/linode/mcp/v1/*_pb2.pyi
	@# protoc emits no __init__.py. mypy derives a module name by walking up only while
	@# __init__.py exists, so without these it names audit_pb2 a top-level module and then
	@# reports linodemcp.genpb.linode.mcp.v1 has no such attribute. `make check` hides that
	@# by passing src/ and tests/ together from python/; any narrower invocation (a shared
	@# lint script, an editor, one file) hits it. buf runs clean: true and wipes the tree.
	find python/src/linodemcp/genpb -type d -exec touch {}/__init__.py \;
	@# proto3 requires an `unspecified = 0` sentinel. Dropping it from both schema dirs
	@# leaves clients only real API values and keeps the two schemas byte-identical.
	python3 scripts/strip_enum_sentinel.py
	@# The tool surface and each tool's tier are declarations the descriptors carry, so the
	@# two registries that used to restate them are written from the descriptors instead.
	@# Both land gitignored beside the generated code, which is what makes adding a tool
	@# one proto message and nothing else.
	python3 scripts/gen_tool_registries.py
	@# The emitter runs last because it reads the schemas above as well as the
	@# descriptors. One run writes a tree per language languages.txt registers, from
	@# one contract model, so a tool cannot reach one language and miss another.
	@# Every declared tool is generated except the ones handwritten-tools.txt still
	@# claims, so new surface is born generated. Both contracts and the emitter's own
	@# sources join PROTO_SRCS, so editing any of them regenerates. The Python arm
	@# renders through the repo's ruff, so the venv has to be installed first.
	go -C go run ./cmd/toolgen \
		-languages ../docs/contracts/languages.txt \
		-handwritten ../docs/contracts/handwritten-tools.txt \
		-schemas internal/toolschemas/data \
		-out internal/gentools \
		-python-out ../python/src/linodemcp/gentools \
		-ruff ../python/.venv/bin/ruff
	@mkdir -p $(dir $@)
	@touch $@

# --- Top-level targets ---

## build: Build all language binaries (Go + Python) into each language's bin/
build: proto go-build python-build

# THE gate order, cheap fails first, venv install before everything that needs
# it. CI's one job and the pre-push hook run exactly this list (docs/gates.md).
CHECK_GATES := proto python-install-dev fmt-check scripts-lint actionlint \
	baseline-guard tool-float go-check python-check coverage-floor \
	diff-coverage tool-parity profile-resolution tool-count dryrun \
	pagination response-shapes list-envelope tool-routes api-surfaces \
	field-location tool-capability tool-response route-evidence route-source \
	generated-tools hand-validators hook-bodies system-params env-parity cli-surface \
	docs-links metrics-surface write-proto read-proto input-proto meta-proto \
	behavior messages betterleaks trivy build go-build-prod

## check: THE gate. Everything, one target (fmt, full lint incl. security scans, all tests, all cross-language gates, both builds)
check: $(CHECK_GATES)

## check-container: Run the full `make check` gate inside the CI-mirror Linux container
# Same provisioning as the CI job (scripts/ci-setup.sh) against a copy of the tree.
check-container:
	$(CONTAINER_ENGINE) build -t linodemcp:ci -f ci/Dockerfile .
	$(CONTAINER_ENGINE) run --rm -v "$(CURDIR)":/src:ro linodemcp:ci

## fmt-check: Verify Go + Python + scripts formatting, read-only (generated code excluded). Shared by check, lint, and CI.
# Read-only on purpose: auto-fixing here would hide drift CI still fails on.
fmt-check: go-fmt-check python-fmt-check scripts-fmt-check

go-fmt-check:
	$(MAKE) -C go fmt-check

python-fmt-check:
	$(MAKE) -C python fmt-check

# ruff discovers scripts/ruff.toml only when run from the repo root over scripts/.
scripts-fmt-check:
	@echo "Running ruff format --check on scripts/..."
	@python/.venv/bin/ruff format --check scripts/

scripts-lint:
	@echo "Running ruff check on scripts/..."
	@python/.venv/bin/ruff check scripts/

## lint: Run all linters (fmt-check, go-lint, python-lint, scripts-lint, betterleaks, trivy, actionlint)
lint: proto fmt-check go-lint python-lint scripts-lint betterleaks trivy actionlint

## test: Run all tests (go-test + python-test)
test: proto go-test python-test coverage-report

# --- Cross-language gates (each documented in docs/gates.md) ---

# A gate named x-y runs scripts/verify_x_y.py. This list runs on the project
# venv because the scripts import the Python registry.
VENV_GATES := tool-parity write-proto read-proto input-proto meta-proto \
	behavior messages

$(VENV_GATES):
	@python/.venv/bin/python scripts/verify_$(subst -,_,$@).py

# Same name-to-script mapping for the scripts that need no venv.
PLAIN_GATES := profile-resolution dryrun pagination response-shapes \
	list-envelope api-surfaces tool-routes field-location tool-capability \
	tool-response route-evidence route-source generated-tools \
	hand-validators hook-bodies system-params env-parity cli-surface docs-links \
	metrics-surface coverage-floor tool-float sync-enums sync-defaults \
	sync-scopes sync-pagination sync-response-shapes

$(PLAIN_GATES):
	@python3 scripts/verify_$(subst -,_,$@).py

# sync-scopes imports the Python registry, unlike the other sync gates.
sync-scopes: python-install-dev

# The rest take an argument or run a script outside the verify_x_y mapping.
BASE ?= origin/main

baseline-guard:
	@python3 scripts/verify_baseline_direction.py "$(BASE)"

diff-coverage:
	@python3 scripts/verify_diff_coverage.py "$(BASE)"

tool-count:
	@python3 scripts/verify_docs_tool_count.py

sync-issues:
	@python3 scripts/verify_tracking_issues.py

parity-todo:
	@python3 scripts/parity_todo.py

# Reporting only, never fails: coverage-floor owns pass/fail.
coverage-report:
	@python3 scripts/report_coverage.py

## sync: Run all live API-drift checks (scheduled agent; needs network)
sync: sync-enums sync-defaults sync-pagination sync-response-shapes sync-scopes sync-issues

install-hooks:
	@./scripts/git-hooks.sh install

check-hooks:
	@./scripts/git-hooks.sh check

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

# In check because the hardened build has link constraints the dev build lacks.
go-build-prod:
	$(MAKE) -C go build-prod

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

## go-check: Run Go fmt+lint+test
go-check:
	$(MAKE) -C go check

# --- Python pass-through targets ---

python-build:
	$(MAKE) -C python build

python-install-dev:
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

## python-check: Run Python lint+typecheck+test
python-check:
	$(MAKE) -C python check

# --- Shared linters ---

# Hard requirement on purpose: a warn-skip let machines without the binary
# pass a scan CI ran. Flag rationale lives in docs/gates.md.
betterleaks:
	@command -v betterleaks >/dev/null 2>&1 || { echo "[error] betterleaks required (release binary: https://github.com/betterleaks/betterleaks/releases)" >&2; exit 1; }
	@echo "Running betterleaks secrets scan..."
	@git ls-files --cached --others --exclude-standard | \
		while IFS= read -r f; do if [ -e "$$f" ]; then printf '%s\n' "$$f"; fi; done | \
		tr '\n' '\0' | xargs -0 betterleaks dir --config .betterleaks.toml --verbose --redact --regex-engine=stdlib

# Hard requirement, all severities; accepted findings live in .trivyignore.yaml.
trivy:
	@command -v trivy >/dev/null 2>&1 || { echo "[error] trivy required (install: https://trivy.dev/latest/getting-started/installation/)" >&2; exit 1; }
	@echo "Running trivy security scan..."
	@trivy fs --scanners vuln,misconfig --exit-code 1 \
		$$(git ls-files --others --ignored --exclude-standard --directory | awk '{ print (sub(/\/$$/, "") ? "--skip-dirs=" : "--skip-files=") $$0 }') .

# Unconditional @latest so the local run matches what CI fetches.
WORKFLOW_FILES := $(wildcard .github/workflows/*.yml .github/workflows/*.yaml)
actionlint:
	@echo "Running actionlint..."
	@go run github.com/rhysd/actionlint/cmd/actionlint@latest $(WORKFLOW_FILES)

# --- Cleanup targets ---

docker-clean:
	-$(CONTAINER_ENGINE) rmi $(GO_IMAGE) 2>/dev/null
	-$(CONTAINER_ENGINE) rmi $(PYTHON_IMAGE) 2>/dev/null
	-$(CONTAINER_ENGINE) image prune -f --filter="label=io.buildah.version" 2>/dev/null

## clean: Clean all build artifacts and container images
clean: go-clean python-clean docker-clean
	-rm -rf .make go/internal/genpb python/src/linodemcp/genpb go/internal/toolschemas/data
	-rm -rf go/internal/gentools python/src/linodemcp/gentools
