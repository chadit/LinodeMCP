.PHONY: help build test check check-container lint fmt-check go-fmt-check python-fmt-check scripts-fmt-check scripts-lint clean install-hooks check-hooks tool-parity tool-count dryrun pagination response-shapes list-envelope tool-routes field-location tool-capability tool-response route-evidence route-source generated-tools system-params env-parity cli-surface docs-links metrics-surface coverage-floor coverage-report diff-coverage write-proto read-proto input-proto meta-proto behavior messages sync sync-enums sync-defaults sync-pagination sync-response-shapes sync-scopes sync-issues baseline-guard tool-float parity-todo \
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

# --- Proto codegen ---
# Generated code is gitignored. Stamp-gated so build/test regenerate only when
# the proto sources change, which keeps offline builds working after one run.
PROTO_SRCS := $(shell find proto -name '*.proto') buf.yaml buf.gen.yaml $(wildcard buf.lock) \
	docs/contracts/generated-tools.txt docs/contracts/tool-hooks.txt \
	docs/contracts/languages.txt scripts/toolgen_py.py \
	$(shell find go/cmd/toolgen -name '*.go' -not -name '*_test.go')
PROTO_STAMP := .make/proto-generated

## proto: Generate Go + Python types and MCP schemas from proto/ (needs buf)
proto: $(PROTO_STAMP)

## generate: Alias for proto
generate: proto

$(PROTO_STAMP): $(PROTO_SRCS)
	@command -v buf >/dev/null 2>&1 || { echo "buf is required: https://buf.build/docs/installation"; exit 1; }
	buf generate
	@# protoc-gen-python emits absolute cross-proto imports (from linode.mcp.v1 import X),
	@# which would put a top-level `linode` on sys.path and register descriptors twice.
	perl -pi -e 's{^from linode\.mcp\.v1 import }{from linodemcp.genpb.linode.mcp.v1 import }' python/src/linodemcp/genpb/linode/mcp/v1/*_pb2.py python/src/linodemcp/genpb/linode/mcp/v1/*_pb2.pyi
	@# protoc emits no __init__.py. mypy derives a module name by walking up only while
	@# __init__.py exists, so without these it names audit_pb2 a top-level module and then
	@# reports linodemcp.genpb.linode.mcp.v1 has no such attribute. `make check` hides that
	@# by passing src/ and tests/ together from python/; any narrower invocation (a shared
	@# lint script, an editor, one file) hits it. buf runs clean: true and wipes the tree.
	find python/src/linodemcp/genpb -type d -exec touch {}/__init__.py \;
	@# proto3 requires an `unspecified = 0` sentinel. Dropping it from both schema dirs
	@# leaves clients only real API values and keeps the two schemas byte-identical.
	python3 scripts/strip_enum_sentinel.py
	@# Runs last because it reads the schemas above as well as the descriptors. The cohort
	@# file and the emitter's own sources join PROTO_SRCS, so editing either regenerates.
	go -C go run ./cmd/toolgen \
		-cohort ../docs/contracts/generated-tools.txt \
		-out internal/gentools \
		-schemas internal/toolschemas/data \
		-hooks ../docs/contracts/tool-hooks.txt
	@# Falls back to the venv when the plain interpreter cannot import the descriptors. It
	@# checks the whole hook manifest, including the lines the Go emitter reads, so a
	@# language column naming nothing registered fails here instead of sitting unread.
	python3 scripts/toolgen_py.py \
		-cohort docs/contracts/generated-tools.txt \
		-out python/src/linodemcp/gentools \
		-hooks docs/contracts/tool-hooks.txt \
		-languages docs/contracts/languages.txt
	@mkdir -p $(dir $@)
	@touch $@

# --- Top-level targets ---

## build: Build all language binaries (Go + Python) into each language's bin/
build: proto go-build python-build

## check: THE gate. Everything, one target (fmt, full lint incl. security scans, all tests, all cross-language gates, both builds)
# CI's one job and the pre-push hook both run exactly this, so local green, hook
# green, and CI green are the same fact; only the network sync-* checks sit
# outside it. python-install-dev runs first because everything below needs the
# venv it builds, and it refreshes when pyproject changes so a stale local venv
# cannot pass what a fresh CI venv fails. The rest is ordered cheap-fails-first.
check: proto python-install-dev fmt-check scripts-lint actionlint baseline-guard tool-float go-check python-check coverage-floor diff-coverage tool-parity tool-count dryrun pagination response-shapes list-envelope tool-routes field-location tool-capability tool-response route-evidence route-source generated-tools system-params env-parity cli-surface docs-links metrics-surface write-proto read-proto input-proto meta-proto behavior messages betterleaks trivy build go-build-prod

## check-container: Run the full `make check` gate inside the CI-mirror Linux container
# Mirrors CI: same provisioning (the image runs scripts/ci-setup.sh, the script
# the CI job runs) against a copy of the tree with the host venv, generated
# code, and caches excluded. Run it when a change touches the gate chain or CI.
check-container:
	$(CONTAINER_ENGINE) build -t linodemcp:ci -f ci/Dockerfile .
	$(CONTAINER_ENGINE) run --rm -v "$(CURDIR)":/src:ro linodemcp:ci

## fmt-check: Verify Go + Python + scripts formatting, read-only (generated code excluded). Shared by check, lint, and CI.
# Read-only on purpose: auto-fixing here would hide drift CI still fails on.
# Generated genpb is excluded (Go via GO_FMT_SRC, Python via the ruff config),
# so a fresh regen is never format-gated.
fmt-check: go-fmt-check python-fmt-check scripts-fmt-check

go-fmt-check:
	$(MAKE) -C go fmt-check

python-fmt-check:
	$(MAKE) -C python fmt-check

## scripts-fmt-check: Verify formatting of the repo gate/verify scripts (scripts/)
# scripts/ has its own scripts/ruff.toml (extends python/pyproject.toml), which
# ruff auto-discovers only when run from the repo root over scripts/.
scripts-fmt-check:
	@echo "Running ruff format --check on scripts/..."
	@python/.venv/bin/ruff format --check scripts/

## scripts-lint: Lint the repo gate/verify scripts (scripts/) with ruff
# Same scripts/ruff.toml as scripts-fmt-check.
scripts-lint:
	@echo "Running ruff check on scripts/..."
	@python/.venv/bin/ruff check scripts/

## tool-parity: Verify Go/Python tool-surface parity (capability, params, required, scopes)
# Needs the venv (it imports the Python registry). Baseline
# docs/contracts/tool-parity-baseline.txt only shrinks: a fixed entry also fails.
tool-parity:
	@python/.venv/bin/python scripts/verify_tool_parity.py

## write-proto: Verify mutating handlers route success output through proto
# Static classification of every Write/Destroy/Admin tool, needs the venv. Two
# baselines under docs/contracts/ (stragglers, missing conformance fixtures),
# both only shrink: a new straggler fails, so does a fixed entry left in place.
write-proto:
	@python/.venv/bin/python scripts/verify_write_proto.py

## read-proto: Verify read handlers route output through proto
# Read-surface sibling of write-proto, needs the venv. Baseline
# docs/contracts/read-proto-baseline.txt is the remaining conversion work.
read-proto:
	@python/.venv/bin/python scripts/verify_read_proto.py

## input-proto: Verify tool input schemas are proto-generated
# Input-schema sibling of write-proto, needs the venv. Baseline
# docs/contracts/input-proto-baseline.txt is the remaining conversion work.
input-proto:
	@python/.venv/bin/python scripts/verify_input_proto.py

## meta-proto: Verify meta tool handlers route output through proto
# Meta-capability sibling of write-proto, needs the venv. Ratchets against
# docs/contracts/meta-proto-baseline.txt.
meta-proto:
	@python/.venv/bin/python scripts/verify_meta_proto.py

## behavior: Verify behavior-fixture coverage of the tool surface
# Correctness lives in the two test runners replaying testdata/behavior/; this
# target only ratchets fixture COVERAGE against docs/contracts/behavior-baseline.txt,
# so a new tool needs a fixture and a covered tool cannot lose one.
behavior:
	@python/.venv/bin/python scripts/verify_behavior.py

## messages: Verify cross-language confirm-message parity
# Heuristic extractors, so it catches confirm-text drift on branches no fixture
# exercises. Ratchets against docs/contracts/message-parity-baseline.txt.
messages:
	@python/.venv/bin/python scripts/verify_messages.py

## sync-enums: LIVE-check proto enums against the Linode API spec (scheduled agent; needs network)
# Network (live spec + changelog), so it stays out of `check`. The offline gates
# prove both languages emit the same enums; this proves those enums still match
# the API. --update-baseline records a drift set a human has reconciled.
sync-enums:
	@python3 scripts/verify_sync_enums.py

## sync-defaults: LIVE-check wire-body defaults against the Linode API spec (scheduled agent; needs network)
sync-defaults:
	@python3 scripts/verify_sync_defaults.py

## sync-scopes: LIVE-check per-tool OAuth scopes against the Linode API spec (scheduled agent; needs network + venv)
# Needs the venv, unlike the other sync gates; deviations live annotated in
# docs/contracts/scope-sync-baseline.txt. A route the spec documents no operation
# for is skipped, not failed: the spec lags techdocs, and route-evidence already
# proves offline that the route is real.
sync-scopes: python-install-dev
	@python3 scripts/verify_sync_scopes.py

## sync: Run all live API-drift checks (scheduled agent; needs network)
sync: sync-enums sync-defaults sync-pagination sync-response-shapes sync-scopes sync-issues

## sync-issues: Verify every baseline acceptance still cites an open tracking issue
# Network (gh), so scheduled-only. baseline-guard only checks that an annotation
# looks like an issue URL, which a closed issue satisfies forever; this resolves
# each one. Skips loudly without gh rather than passing an unchecked promise.
sync-issues:
	@python3 scripts/verify_tracking_issues.py

## baseline-guard: Verify baseline growth vs BASE (default origin/main) carries issue-linked annotations
# Diff-aware but needs no build artifacts, so it rides early in `check`. BASE
# defaults to origin/main and an unreachable rev skips loudly; CI re-runs it
# (baseline-guard.yml) with the event's true base, since on main BASE == HEAD.
BASE ?= origin/main
baseline-guard:
	@python3 scripts/verify_baseline_direction.py "$(BASE)"

## tool-float: Verify gate tooling floats at latest (app deps pin; tools do not)
# Offline line scans over pyproject's dev group, the Makefiles, ci-setup.sh, and
# workflow runs. A pinned gate tool fails unless it carries a reasoned entry in
# the script's deliberate-pin allowlist (only buf today).
tool-float:
	@python3 scripts/verify_tool_float.py

## parity-todo: Report per-language remaining work from the parity baselines
# Read-only report over docs/contracts/languages.txt and every ratchet baseline.
# Reports only, never fails; needs no venv.
parity-todo:
	@python3 scripts/parity_todo.py

## tool-count: Verify README's tool count matches docs/contracts/tools-manifest.txt
# Offline. The manifest is the source of truth; the README prose is what drifts.
tool-count:
	@python3 scripts/verify_docs_tool_count.py

## dryrun: Verify dry_run is advertised per capability tier across the surface
# Offline, no baseline: every Write/Admin/Destroy input carries dry_run and no
# Read/Meta one does. The preview-fixture half ratchets in the behavior gate.
dryrun:
	@python3 scripts/verify_dryrun.py

## response-shapes: Verify behavior fixtures serve each route's spec response shape
# Offline. Fixtures are judged against the snapshot sync-response-shapes owns in
# docs/contracts/api-response-shapes-baseline.txt; gaps ratchet down in
# docs/contracts/response-shape-baseline.txt. A wrong-shaped fixture makes every
# language agree on a contract the API never had.
response-shapes:
	@python3 scripts/verify_response_shapes.py

## list-envelope: Verify no Python list handler collapses a falsey member with `or []`
# Offline; scope from docs/contracts/languages.txt (a registered language with no
# scanner fails by name), gaps in docs/contracts/list-envelope-baseline.txt.
# `or []` folds {}, "", 0, and false into an empty list, so a malformed response
# ships as a successful empty result in Python while Go rejects it.
list-envelope:
	@python3 scripts/verify_list_envelope.py

## tool-routes: Verify every non-meta tool declares its Linode route in the proto
# Offline, no baseline. The tool_route options are the single source for
# tool-to-route; this pins them against docs/contracts/tools-manifest.txt in both
# directions, so neither the proto nor the manifest can drift alone.
tool-routes:
	@python3 scripts/verify_tool_routes.py

## field-location: Verify every routed input field declares where it goes
# Offline, no baseline: a field with no location fails, PATH fields must line up
# with the route template both ways, and LOCAL must agree with the `// system
# param` marker system-params pins. That last one keeps dry_run off the wire.
field-location:
	@python3 scripts/verify_field_location.py

## tool-capability: Verify the proto's tier for every tool matches the manifest
# Offline, no baseline. Holds docs/contracts/tools-capabilities.txt to being a
# mirror of the proto's tool_capability, plus exactly one of tool_route (reaches
# the API) or tool_meta (local state). A capability value with no manifest tier
# fails first, since it would otherwise drop out of the comparison unnoticed.
tool-capability:
	@python3 scripts/verify_tool_capability.py

## tool-response: Verify the proto says what every tool answers with
# Offline, no baseline: response message, confirm prose, success-text
# placeholders, and a Destroy's two-stage resource type, all failing in both
# directions. Hash-ignore keys must agree across every language in
# docs/contracts/languages.txt, because an unknown type and a typo look alike
# there and both silently hash the whole state.
tool-response:
	@python3 scripts/verify_tool_response.py

## route-evidence: Verify every declared route is one a client can build
# Offline; scope from docs/contracts/languages.txt, gaps in
# docs/contracts/route-evidence-baseline.txt. The resolvers (go/cmd/route-dump,
# scripts/_routescan.py) walk the call graph out from the request primitive, so a
# path built from a base constant and a format verb still counts as evidence.
route-evidence:
	@python3 scripts/verify_route_evidence.py

## route-source: Count request call sites that still build their endpoint by hand
# Offline; scope from docs/contracts/languages.txt. Counts in
# docs/contracts/route-source-counts.txt only fall: a new hand-built call site
# fails, and so does a removed one whose line was not lowered, which keeps the
# file honest about the remaining migration work.
route-source:
	@python3 scripts/verify_route_source.py

## generated-tools: Verify the generator owns its cohort, and count what is still hand-written
# Offline; scope from docs/contracts/languages.txt. Requires `make proto`, since
# the trees it reads are the ones the emitters write. A hand-written factory left
# behind for a cohort tool fails: both languages register by scanning, so the
# leftover stages the tool twice. generated-tools-counts.txt only falls.
generated-tools:
	@python3 scripts/verify_generated_tools.py

## system-params: Verify every server-injected proto input field is marked
# Offline scan of proto/linode/mcp/v1/ against docs/contracts/system-params.txt,
# failing both ways: an unmarked system param, and a marker on a field the
# contract does not name. The marker is a trailing comment so it never reaches
# the generated JSON Schema, leaving the descriptions MCP clients see unchanged.
system-params:
	@python3 scripts/verify_system_params.py

## pagination: Verify list tools paginate when their spec route paginates
# Offline against the snapshot sync-pagination owns in
# docs/contracts/api-pagination-baseline.txt; gaps ratchet down in
# docs/contracts/pagination-baseline.txt.
pagination:
	@python3 scripts/verify_pagination.py

## env-parity: Verify every language reads exactly the contracted env vars
# Offline, no baseline: docs/contracts/env-vars.txt pins the whole env surface,
# so a variable one language reads and another does not fails here.
env-parity:
	@python3 scripts/verify_env_parity.py

## cli-surface: Verify the CLI verbs and flags match across languages
# Offline, no baseline: verbs and per-verb flags are extracted from source and
# diffed, so a flag cannot land on one CLI without its twin.
cli-surface:
	@python3 scripts/verify_cli_surface.py

## docs-links: Verify every internal link in README and docs/ resolves
# Offline: internal targets only, nothing is fetched.
docs-links:
	@python3 scripts/verify_docs_links.py

## metrics-surface: Verify instrument names and attribute keys match across languages
# Offline, no baseline: dashboards and alerts key on these names, so a one-sided
# rename forks every consumer. Bucket boundaries pin separately, in
# testdata/observability/duration_buckets.json.
metrics-surface:
	@python3 scripts/verify_metrics_surface.py

## sync-pagination: Diff the live spec's paginated-route set and bounds vs the snapshot
# Network (live spec fetch), so scheduled-only like the other sync gates.
sync-pagination:
	@python3 scripts/verify_sync_pagination.py

## sync-response-shapes: Diff the live spec's route response shapes vs the snapshot
# Network (live spec fetch), so scheduled-only like the other sync gates.
sync-response-shapes:
	@python3 scripts/verify_sync_response_shapes.py

## coverage-floor: Verify each language's total unit-test coverage meets its contracted floor
# Must follow the two language suites: it parses the go/coverage.out go-check
# writes (genpb and the cmd/ mains excluded). Python's floor is enforced by
# pytest --cov-fail-under, so here pyproject and the contract are only checked
# to agree. Floors live in docs/contracts/coverage-floors.txt and only rise.
coverage-floor:
	@python3 scripts/verify_coverage_floor.py

## diff-coverage: Verify source lines added since BASE (default origin/main) are covered by tests
# Reads what the test targets just wrote (go/coverage.out, python/coverage.json),
# so it has to follow them. BASE defaults to origin/main, locally meaning
# everything not yet pushed; an unreachable rev skips loudly. CI re-runs it with
# the event's true base, since on pushes to main origin/main already is HEAD.
diff-coverage:
	@python3 scripts/verify_diff_coverage.py "$(BASE)"

## lint: Run all linters (fmt-check, go-lint, python-lint, scripts-lint, betterleaks, trivy, actionlint)
lint: proto fmt-check go-lint python-lint scripts-lint betterleaks trivy actionlint

## test: Run all tests (go-test + python-test)
test: proto go-test python-test coverage-report

## coverage-report: Print one coverage line per registered language
# Reporting only, never fails: coverage-floor owns pass/fail. Scope comes from
# docs/contracts/languages.txt, so a new language shows up without editing this.
coverage-report:
	@python3 scripts/report_coverage.py

## install-hooks: Install commit and push hooks from .pre-commit-config.yaml
install-hooks:
	@./scripts/git-hooks.sh install

## check-hooks: Verify commit and push hooks are installed
check-hooks:
	@./scripts/git-hooks.sh check

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

## go-build-prod: Build security-hardened Go binary (PIE, trimpath, stripped, static)
# Part of check: the hardened build has different link constraints than the dev
# build, so only building dev locally lets a prod-only link failure reach CI.
go-build-prod:
	$(MAKE) -C go build-prod

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

## python-build: Build Python wheel + sdist into python/bin/
python-build:
	$(MAKE) -C python build

## python-install-dev: Install Python package with dev dependencies (editable)
python-install-dev:
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
# Hard requirement, not skip-if-missing: a warn-skip meant machines without the
# binary passed a scan CI ran. --redact keeps the secret value out of terminals
# and CI logs; --regex-engine=stdlib matches what CI forces (the WASM engine
# trips betterleaks#74 there). The file list comes from git because betterleaks
# has no gitignore awareness, so a hit in ignored debris would fail the gate over
# something that cannot ship; the existence filter drops index entries deleted
# from the working tree, which betterleaks aborts on. --config is explicit
# because auto-discovery keys off a directory target and never fires for a file
# list, which would silently drop the fixture allowlists.
betterleaks:
	@command -v betterleaks >/dev/null 2>&1 || { echo "[error] betterleaks required (release binary: https://github.com/betterleaks/betterleaks/releases)" >&2; exit 1; }
	@echo "Running betterleaks secrets scan..."
	@git ls-files --cached --others --exclude-standard | \
		while IFS= read -r f; do if [ -e "$$f" ]; then printf '%s\n' "$$f"; fi; done | \
		tr '\n' '\0' | xargs -0 betterleaks dir --config .betterleaks.toml --verbose --redact --regex-engine=stdlib

## trivy: Run trivy security scan
# Hard requirement, same false-green trap as betterleaks. All severities on
# purpose: accepted findings live annotated in .trivyignore.yaml, and the outside
# lint.sh scans unfiltered too, so a severity filter here would let the two
# disagree on the same tree. Skip flags are derived from git's ignore computation
# because trivy has no gitignore awareness; they use the =-attached form so each
# survives shell word splitting as one word.
trivy:
	@command -v trivy >/dev/null 2>&1 || { echo "[error] trivy required (install: https://trivy.dev/latest/getting-started/installation/)" >&2; exit 1; }
	@echo "Running trivy security scan..."
	@trivy fs --scanners vuln,misconfig --exit-code 1 \
		$$(git ls-files --others --ignored --exclude-standard --directory | awk '{ print (sub(/\/$$/, "") ? "--skip-dirs=" : "--skip-files=") $$0 }') .

## actionlint: Lint GitHub Actions workflow files
# Unconditional `go run @latest`: a prefer-local-binary fallback ages out of sync
# with what CI fetches, exactly when a new check lands. Workflow files are passed
# explicitly because bare actionlint finds the project by looking for .git, which
# a tarball or clean-room checkout does not have.
WORKFLOW_FILES := $(wildcard .github/workflows/*.yml .github/workflows/*.yaml)
actionlint:
	@echo "Running actionlint..."
	@go run github.com/rhysd/actionlint/cmd/actionlint@latest $(WORKFLOW_FILES)

# --- Cleanup targets ---

## docker-clean: Remove container images only
docker-clean:
	-$(CONTAINER_ENGINE) rmi $(GO_IMAGE) 2>/dev/null
	-$(CONTAINER_ENGINE) rmi $(PYTHON_IMAGE) 2>/dev/null
	-$(CONTAINER_ENGINE) image prune -f --filter="label=io.buildah.version" 2>/dev/null

## clean: Clean all build artifacts and container images
clean: go-clean python-clean docker-clean
	-rm -rf .make go/internal/genpb python/src/linodemcp/genpb go/internal/toolschemas/data
	-rm -rf go/internal/gentools python/src/linodemcp/gentools
