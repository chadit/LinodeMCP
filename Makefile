.PHONY: help build test check check-container lint fmt-check go-fmt-check python-fmt-check scripts-fmt-check scripts-lint clean install-hooks check-hooks tool-parity tool-count dryrun pagination response-shapes list-envelope tool-routes route-evidence system-params env-parity cli-surface docs-links metrics-surface coverage-floor coverage-report diff-coverage write-proto read-proto input-proto meta-proto behavior messages sync sync-enums sync-defaults sync-pagination sync-response-shapes sync-scopes sync-issues baseline-guard tool-float parity-todo \
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
# Generated code is gitignored; `make proto` regenerates it from proto/ via buf.
# Stamp-gated so build/test only regenerate when the proto sources change, which
# keeps offline builds working once the code has been generated once.
PROTO_SRCS := $(shell find proto -name '*.proto') buf.yaml buf.gen.yaml $(wildcard buf.lock)
PROTO_STAMP := .make/proto-generated

## proto: Generate Go + Python types and MCP schemas from proto/ (needs buf)
proto: $(PROTO_STAMP)

## generate: Alias for proto
generate: proto

$(PROTO_STAMP): $(PROTO_SRCS)
	@command -v buf >/dev/null 2>&1 || { echo "buf is required: https://buf.build/docs/installation"; exit 1; }
	buf generate
	@# protoc-gen-python emits absolute cross-proto imports (from linode.mcp.v1 import X)
	@# based on the proto package path. Rewrite them to the package-qualified path so the
	@# generated tree imports as one module tree under linodemcp.genpb (no top-level `linode`
	@# on sys.path, no duplicate descriptor registration).
	perl -pi -e 's{^from linode\.mcp\.v1 import }{from linodemcp.genpb.linode.mcp.v1 import }' python/src/linodemcp/genpb/linode/mcp/v1/*_pb2.py python/src/linodemcp/genpb/linode/mcp/v1/*_pb2.pyi
	@# protoc emits no __init__.py, which leaves genpb a namespace package while every
	@# other subpackage under linodemcp is a regular one. mypy derives a module name by
	@# walking up only while __init__.py exists, so without these it names audit_pb2 as a
	@# top-level module and then reports linodemcp.genpb.linode.mcp.v1 has no such
	@# attribute. `make check` hides it by passing src/ and tests/ together from python/;
	@# any narrower invocation (a shared lint script, an editor, one file) hits it. buf
	@# runs with clean: true and wipes the tree, so this has to be regenerated here.
	find python/src/linodemcp/genpb -type d -exec touch {}/__init__.py \;
	@# proto enums carry an `unspecified = 0` zero-value sentinel (proto3 requires one);
	@# strip it from the generated JSON Schema enum arrays so clients see only real API
	@# values. Runs over both schema dirs to keep Go and Python schemas byte-identical.
	python3 scripts/strip_enum_sentinel.py
	@mkdir -p $(dir $@)
	@touch $@

# --- Top-level targets ---

## build: Build all language binaries (Go + Python) into each language's bin/
build: proto go-build python-build

## check: THE gate. Everything, one target (fmt, full lint incl. security scans, all tests, all cross-language gates, both builds)
# check is the single definition of done: CI's one job runs exactly `make check`
# and the pre-push hook runs exactly `make check`, so local green, hook green,
# and CI green are the same fact. Nothing quality-gating lives outside this
# target (only the network-dependent sync-* live checks stay scheduled-only).
# python-install-dev runs first because check provisions its own venv: half the
# targets below need python/.venv (ruff, mypy, pytest, every gate script), and
# a fresh checkout (CI, new clone) has none. Self-provisioning also means the
# venv is refreshed whenever pyproject changes, so a stale local venv can't
# pass a check a fresh CI venv fails. Ordering after that is cheap-fails-first:
# format/lint/workflow checks, the two language suites, gates, security scans,
# then builds.
check: proto python-install-dev fmt-check scripts-lint actionlint baseline-guard tool-float go-check python-check coverage-floor diff-coverage tool-parity tool-count dryrun pagination response-shapes list-envelope tool-routes route-evidence system-params env-parity cli-surface docs-links metrics-surface write-proto read-proto input-proto meta-proto behavior messages betterleaks trivy build go-build-prod

## check-container: Run the full `make check` gate inside the CI-mirror Linux container
# The local rehearsal of CI itself: same OS family, same toolchain (the image
# runs scripts/ci-setup.sh, the identical provisioning script the CI job
# runs), same single command, against a fresh-checkout copy of the tree (the
# entrypoint excludes the host venv, generated code, and caches). Run this
# before pushing when a change touches the gate chain, CI config, or
# provisioning; it catches what a dirty local workspace structurally cannot.
check-container:
	$(CONTAINER_ENGINE) build -t linodemcp:ci -f ci/Dockerfile .
	$(CONTAINER_ENGINE) run --rm -v "$(CURDIR)":/src:ro linodemcp:ci

## fmt-check: Verify Go + Python + scripts formatting, read-only (generated code excluded). Shared by check, lint, and CI.
# Read-only on purpose: it must mirror what CI checks, never auto-fix (an
# auto-fixing check hides drift that CI's read-only gate would fail on). Run
# `make fmt` / `make -C python format` to apply formatting. Generated genpb is
# excluded (Go via GO_FMT_SRC, Python via the ruff config) so a fresh regen is
# never format-gated.
fmt-check: go-fmt-check python-fmt-check scripts-fmt-check

go-fmt-check:
	$(MAKE) -C go fmt-check

python-fmt-check:
	$(MAKE) -C python fmt-check

## scripts-fmt-check: Verify formatting of the repo gate/verify scripts (scripts/)
# The scripts/ tree is linted with its own scripts/ruff.toml (extends
# python/pyproject.toml, ignores the rules that are legit for CLI gate scripts).
# ruff auto-discovers that config when run from the repo root over scripts/.
scripts-fmt-check:
	@echo "Running ruff format --check on scripts/..."
	@python/.venv/bin/ruff format --check scripts/

## scripts-lint: Lint the repo gate/verify scripts (scripts/) with ruff
# Same scripts/ruff.toml as scripts-fmt-check. Folded into `lint` so a ruff
# violation in a gate script fails the same gate every other tree runs through.
scripts-lint:
	@echo "Running ruff check on scripts/..."
	@python/.venv/bin/ruff check scripts/

## tool-parity: Verify Go/Python tool-surface parity (capability, params, required, scopes)
# Runs the Go dumper (go run) and imports the Python registry (needs the venv),
# then diffs the two against docs/contracts/tool-parity-baseline.txt. Fails on any new
# divergence or any baseline entry that is now fixed (the baseline only shrinks).
tool-parity:
	@python/.venv/bin/python scripts/verify_tool_parity.py

## write-proto: Verify mutating handlers route success output through proto
# Statically classifies every Write/Destroy/Admin tool on both sides as
# proto-routed or legacy (Go: go run ./cmd/write-proto-dump; Python: the
# _write_proto_classifier module, needs the venv), then ratchets the straggler
# set and the missing-conformance-fixture set down against their baselines in
# docs/contracts/. Fails on any new straggler or any baseline entry that is now fixed.
write-proto:
	@python/.venv/bin/python scripts/verify_write_proto.py

## read-proto: Verify read handlers route output through proto
# The read-surface sibling of write-proto: statically classifies every Read
# tool on both sides (Go: go run ./cmd/write-proto-dump -surface read; Python:
# the _write_proto_classifier module in read mode, needs the venv), then
# ratchets the straggler set down against docs/contracts/read-proto-baseline.txt. That
# baseline doubles as the remaining-work list for the read-surface conversion.
read-proto:
	@python/.venv/bin/python scripts/verify_read_proto.py

## input-proto: Verify tool input schemas are proto-generated
# The input-schema sibling of write-proto/read-proto: statically classifies
# every tool's factory on both sides (Go: go run ./cmd/write-proto-dump
# -surface input; Python: the _write_proto_classifier module in input mode,
# needs the venv) as proto-generated or hand-built, then ratchets the straggler
# set down against docs/contracts/input-proto-baseline.txt. That baseline doubles as the
# remaining-work list for the input-surface conversion.
input-proto:
	@python/.venv/bin/python scripts/verify_input_proto.py

## meta-proto: Verify meta tool handlers route output through proto
# The Meta-capability sibling of write-proto/read-proto: statically classifies
# every Meta tool on both sides (Go: go run ./cmd/write-proto-dump -surface
# meta; Python: the _write_proto_classifier module in meta mode, needs the
# venv), then ratchets the straggler set down against
# docs/contracts/meta-proto-baseline.txt.
meta-proto:
	@python/.venv/bin/python scripts/verify_meta_proto.py

## behavior: Verify behavior-fixture coverage of the tool surface
# The handler-semantics gate: the shared fixtures in testdata/behavior/ replay
# identical cases through both languages' real dispatch paths (the two test
# runners enforce correctness); this target ratchets fixture COVERAGE against
# docs/contracts/behavior-baseline.txt so new tools need fixtures and covered tools
# cannot lose them.
behavior:
	@python/.venv/bin/python scripts/verify_behavior.py

## messages: Verify cross-language confirm-message parity
# Diffs every extractable confirm-gate message across both languages
# (heuristic extractors promoted from the P1 sweep) and ratchets against
# docs/contracts/message-parity-baseline.txt, so text drift on branches no fixture
# exercises still fails.
messages:
	@python/.venv/bin/python scripts/verify_messages.py

## sync-enums: LIVE-check proto enums against the Linode API spec (scheduled agent; needs network)
# Deliberately NOT part of `check`: it fetches the live OpenAPI spec + changelog,
# so it is non-deterministic and offline-hostile. The inner gates prove Go and
# Python emit identical proto-generated enums; this proves those enums still match
# the current API. Run on a cron / by the sync agent. --update-baseline records a
# reviewed drift set after a human reconciles a real API change.
sync-enums:
	@python3 scripts/verify_sync_enums.py

## sync-defaults: LIVE-check wire-body defaults against the Linode API spec (scheduled agent; needs network)
sync-defaults:
	@python3 scripts/verify_sync_defaults.py

## sync-scopes: LIVE-check per-tool OAuth scopes against the Linode API spec (scheduled agent; needs network + venv)
# The tool-parity gate pins every language's scope mapping equal to Python's;
# this proves Python's mapping still matches the spec's per-operation security
# blocks, so all languages transitively track the docs. Routes come from the
# proto contract's tool_route options; accepted deviations live annotated in
# docs/contracts/scope-sync-baseline.txt. A route the spec documents no
# operation for is named and skipped rather than failed: the spec lags
# techdocs, and route-evidence proves offline that the route is real. Unlike
# the other sync gates this one needs the venv, because the tool dump comes
# from the Python registry.
sync-scopes: python-install-dev
	@python3 scripts/verify_sync_scopes.py

## sync: Run all live API-drift checks (scheduled agent; needs network)
sync: sync-enums sync-defaults sync-pagination sync-response-shapes sync-scopes sync-issues

## sync-issues: Verify every baseline acceptance still cites an open tracking issue
# Network (resolves each cited issue through gh), so scheduled-only like the
# other sync gates. The offline baseline guard only checks that an annotation
# names something shaped like an issue URL, which a closed issue satisfies
# forever: issue 1038 closed while 20 baseline lines across 8 files pointed at
# it, and 6 of those annotations were written the day after. Skips loudly when
# gh is unavailable rather than passing an unchecked promise.
sync-issues:
	@python3 scripts/verify_tracking_issues.py

## baseline-guard: Verify baseline growth vs BASE (default origin/main) carries issue-linked annotations
# Diff-aware but cheap (git show plus file parses, no artifacts), so it rides
# EARLY in `check` (cheap-fails-first: a bad annotation fails in seconds, not
# after ten minutes of tests) and therefore in the pre-push hook. Same layout
# as diff-coverage: BASE defaults to origin/main, which is right locally and
# on PRs; an unreachable rev skips loudly. CI additionally runs the script
# with the event's true base (.github/workflows/baseline-guard.yml), which
# matters on pushes to main where origin/main already equals HEAD.
BASE ?= origin/main
baseline-guard:
	@python3 scripts/verify_baseline_direction.py "$(BASE)"

## tool-float: Verify gate tooling floats at latest (app deps pin; tools do not)
# Cheap line scans over pyproject's dev group, the Makefiles, ci-setup.sh, and
# workflow run commands, so it rides early in `check` beside baseline-guard.
# A capped or pinned gate tool fails unless its module carries a reasoned
# entry in the script's deliberate-pin allowlist (currently only buf).
tool-float:
	@python3 scripts/verify_tool_float.py

## parity-todo: Report per-language remaining work from the parity baselines
# Read-only aggregation of docs/contracts/languages.txt plus every ratchet baseline:
# what each language is missing, what is accepted-and-tracked, and what a
# newly registered language still owes. Needs no venv.
parity-todo:
	@python3 scripts/parity_todo.py

## tool-count: Verify README's tool count matches docs/contracts/tools-manifest.txt
# Offline single-file check, so it rides in `check`: the manifest is the source
# of truth and this fails when the README prose count drifts from it.
tool-count:
	@python3 scripts/verify_docs_tool_count.py

## dryrun: Verify dry_run is advertised per capability tier across the surface
# Offline and hard (no baseline): every Write/Admin/Destroy input carries
# dry_run, no Read/Meta input does, and every tool maps to its proto input.
# The fixture half (a pinned preview case) ratchets in the behavior gate.
dryrun:
	@python3 scripts/verify_dryrun.py

## response-shapes: Verify behavior fixtures serve each route's spec response shape
# Offline; judges fixture bodies against the reviewed snapshot in
# docs/contracts/api-response-shapes-baseline.txt (sync-response-shapes owns
# that). A wrong-shaped fixture proves every language conforms to a contract
# the API never had, which is how a cross-language decode divergence ships.
# Known gaps ratchet down in docs/contracts/response-shape-baseline.txt.
response-shapes:
	@python3 scripts/verify_response_shapes.py

## list-envelope: Verify no Python list handler collapses a falsey member with `or []`
# Offline source scan, scoped by docs/contracts/languages.txt rather than a
# path in the script. `raw.get(key) or []` reads as a null guard but swallows
# {}, "", 0, and false into an empty list, so a malformed response ships as a
# successful empty result in Python while Go rejects it. A registered language
# with no scanner and no stated exemption fails the gate by name. Known gaps
# ratchet down in docs/contracts/list-envelope-baseline.txt.
list-envelope:
	@python3 scripts/verify_list_envelope.py

## tool-routes: Verify every non-meta tool declares its Linode route in the proto
# Offline and hard (no baseline): each tool's proto input message carries a
# `linode.mcp.v1.tool_route` option naming the tool, the method, and the path
# template, which is what makes the descriptors the single source for
# tool-to-route. The gate pins that from both sides against
# docs/contracts/tools-manifest.txt: a non-meta tool with no option fails, a
# Meta tool carrying one fails, and an option naming an unregistered tool or
# sitting on the wrong input message fails.
tool-routes:
	@python3 scripts/verify_tool_routes.py

## route-evidence: Verify every declared route is one a client can build
# Offline source scan, scoped by docs/contracts/languages.txt rather than a
# path in the script. Go resolves through go/cmd/route-dump, Python through
# scripts/_routescan.py; both follow the call graph outward from the request
# primitive, so a path assembled from a base constant and a format verb counts
# as evidence where a text search finds nothing. That false negative is what
# sends a catalog scan chasing a route the client has had all along. Known
# gaps ratchet down in docs/contracts/route-evidence-baseline.txt.
route-evidence:
	@python3 scripts/verify_route_evidence.py

## system-params: Verify every server-injected proto input field is marked
# Offline source scan of proto/linode/mcp/v1/. The system params (environment,
# confirm, dry_run, and the two-stage mode/plan_id) are the server's own
# plumbing, indistinguishable in the proto from the Linode API params beside
# them. The gate pins them to a trailing `// system param` marker, which stays
# out of the generated JSON Schema, so the descriptions MCP clients see do not
# move. The name-and-type set lives in docs/contracts/system-params.txt, and
# both directions are hard failures: an unmarked system param, and a marker on
# a field the contract does not name.
system-params:
	@python3 scripts/verify_system_params.py

## pagination: Verify list tools paginate when their spec route paginates
# Offline: judges the tool surface against the reviewed snapshot in
# docs/contracts/api-pagination-baseline.txt (sync-pagination owns that).
# Known gaps ratchet down in docs/contracts/pagination-baseline.txt.
pagination:
	@python3 scripts/verify_pagination.py

## env-parity: Verify every language reads exactly the contracted env vars
# Offline and hard: docs/contracts/env-vars.txt pins the whole env surface,
# and a variable read by one language but not another fails the gate. This
# is what keeps one-sided env overrides from drifting back in.
env-parity:
	@python3 scripts/verify_env_parity.py

## cli-surface: Verify the CLI verbs and flags match across languages
# Offline and hard: extracts each language's verb set and per-verb flag
# surface from source and diffs them, so a flag added to one CLI cannot
# land without its twin.
cli-surface:
	@python3 scripts/verify_cli_surface.py

## docs-links: Verify every internal link in README and docs/ resolves
# Offline single-pass walk; a moved or deleted doc fails the gate instead
# of leaving a dead link for the next reader to find.
docs-links:
	@python3 scripts/verify_docs_links.py

## metrics-surface: Verify instrument names and attribute keys match across languages
# Offline and hard: dashboards and alerts key on these names, so a
# one-sided rename forks every consumer. Bucket boundaries are pinned
# separately by testdata/observability/duration_buckets.json.
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
# Offline, rides in `check` right after the two language suites: go-check's
# test run writes go/coverage.out and this parses it (hand-written code only;
# generated genpb and the cmd/ mains are excluded). Python's floor is enforced
# at test time by pytest --cov-fail-under, so here the contract and pyproject
# are checked for agreement. Floors live in docs/contracts/coverage-floors.txt
# and only rise. Per-line enforcement is the diff-coverage target below.
coverage-floor:
	@python3 scripts/verify_coverage_floor.py

## diff-coverage: Verify source lines added since BASE (default origin/main) are covered by tests
# In `check`, and so in the pre-push hook and CI: reads the artifacts the
# test targets just wrote (go/coverage.out, python/coverage.json) and fails
# on any added or untracked source line no test executed. BASE defaults to
# origin/main, which locally means "everything not yet pushed"; when the rev
# is unreachable (tarball checkout, shallow clone) the script skips loudly
# rather than failing unrelated work. CI re-runs it after `make check` with
# the event's true base (PR merge parent / push predecessor), which matters
# on pushes to main where origin/main already equals HEAD and the in-check
# run sees an empty diff.
diff-coverage:
	@python3 scripts/verify_diff_coverage.py "$(BASE)"

## lint: Run all linters (fmt-check, go-lint, python-lint, scripts-lint, betterleaks, trivy, actionlint)
lint: proto fmt-check go-lint python-lint scripts-lint betterleaks trivy actionlint

## test: Run all tests (go-test + python-test)
test: proto go-test python-test coverage-report

## coverage-report: Print one coverage line per registered language
# Each language's suite reports in its own format and neither leaves a single
# readable number, so this collapses them into one block at the end. Scope
# comes from docs/contracts/languages.txt, so a newly registered language
# shows up here without touching this target. Reporting only: the
# coverage-floor gate in `make check` owns pass/fail.
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
# Hard requirement, not skip-if-missing: a warn-skip here meant machines
# without the binary passed a scan CI ran (the gosec false-green trap).
# --verbose lists each finding (file, line, rule) instead of only the tally, so
# a failure is actionable without a second manual run. --redact masks the secret
# value: a real leak's location is what you need, and echoing the raw value into
# the terminal or CI logs would just copy the secret somewhere new.
# --regex-engine=stdlib pins one engine everywhere: CI already forced stdlib
# (the WASM engine trips betterleaks#74 there), and scanning with different
# engines locally vs CI can produce different findings.
# The scan target is the file set git reports as not ignored (tracked plus
# untracked-unignored), not the whole directory: betterleaks has no gitignore
# awareness, and gitignored content (virtualenvs, build output, scratch dirs)
# can never reach a commit, so a hit there fails the gate on debris that
# cannot ship. Deriving the list from git honors every ignore source
# (.gitignore, .git/info/exclude, the global excludes file) with no
# hand-maintained mirror to drift. The existence filter drops index entries
# deleted from the working tree, which betterleaks aborts on. --config is
# explicit because auto-discovery keys off a directory target and does not
# fire for a file list, and losing the repo config would silently drop the
# fixture allowlists. CI sees no change: a fresh checkout has no ignored
# debris beyond generated code.
betterleaks:
	@command -v betterleaks >/dev/null 2>&1 || { echo "[error] betterleaks required (release binary: https://github.com/betterleaks/betterleaks/releases)" >&2; exit 1; }
	@echo "Running betterleaks secrets scan..."
	@git ls-files --cached --others --exclude-standard | \
		while IFS= read -r f; do if [ -e "$$f" ]; then printf '%s\n' "$$f"; fi; done | \
		tr '\n' '\0' | xargs -0 betterleaks dir --config .betterleaks.toml --verbose --redact --regex-engine=stdlib

## trivy: Run trivy security scan
# Hard requirement, not skip-if-missing (same false-green trap as betterleaks).
# All severities on purpose: accepted findings live as annotated entries in
# .trivyignore.yaml, not behind a severity filter, and dotai's lint.sh runs
# trivy unfiltered, so filtering here let the two scans disagree on the same
# tree. CI runs this exact target, so local and CI fail on the same findings.
# Trivy has no gitignore awareness, so gitignored paths are passed as skip
# flags derived from git's own ignore computation at scan time (covers
# .gitignore, .git/info/exclude, and the global excludes file with no
# hand-maintained mirror to drift). Gitignored content never ships, so a
# finding there blocks pushes over debris that cannot reach a commit. The
# flags use the =-attached form so each stays a single word through shell
# word splitting. CI sees no change: a fresh checkout has no ignored debris
# beyond generated code.
trivy:
	@command -v trivy >/dev/null 2>&1 || { echo "[error] trivy required (install: https://trivy.dev/latest/getting-started/installation/)" >&2; exit 1; }
	@echo "Running trivy security scan..."
	@trivy fs --scanners vuln,misconfig --exit-code 1 \
		$$(git ls-files --others --ignored --exclude-standard --directory | awk '{ print (sub(/\/$$/, "") ? "--skip-dirs=" : "--skip-files=") $$0 }') .

## actionlint: Lint GitHub Actions workflow files
# Unconditional `go run @latest`, same pattern as gosec/cairnlint/pyright: a
# prefer-local-binary fallback is a stale-version channel (local binary ages,
# CI fetches latest, and the two diverge exactly when a new check lands).
# Workflow files are passed explicitly: bare `actionlint` discovers the
# project by looking for .git, which breaks in any git-less checkout
# (tarball, clean-room verification copy).
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
