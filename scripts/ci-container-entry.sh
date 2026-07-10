#!/usr/bin/env bash
# Container entrypoint for `make check-container`: copy the repository from
# the read-only host mount into a scratch tree in fresh-checkout state (no
# venv, no generated code, no host caches), then run the full gate. This is
# the local rehearsal of exactly what CI does: same OS, same toolchain
# provisioning (ci/Dockerfile runs scripts/ci-setup.sh), same single command.
set -euo pipefail

trap 'echo "E: error on line $LINENO" >&2' ERR

# info logs a progress line to stderr so stdout stays free for make output.
info() { echo "I: $1" >&2; }

# die logs an error to stderr and exits 1.
die() {
	echo "E: $1" >&2
	exit 1
}

[[ -d /src ]] || die "expected the repository mounted read-only at /src (make check-container does this)"

info "copying repository into a clean work tree (host venv, generated code, and caches excluded)"
archive="$(mktemp)"
trap 'rm -f "$archive"' EXIT
# tar exits 1 ("file changed as we read it") when the host mutates a file
# mid-read; the host repo sits under a live file-sync tool, so mtime jitter
# through the mount is normal. Exit 1 still produces a complete archive, so
# absorb it; exit 2+ is a real failure.
tar_status=0
tar -C /src \
	--exclude=./.git \
	--exclude=./.make \
	--exclude=./python/.venv \
	--exclude=./go/internal/genpb \
	--exclude=./python/src/linodemcp/genpb \
	--exclude=./go/internal/toolschemas/data \
	--exclude=./go/bin \
	--exclude=./python/bin \
	--exclude=./python/htmlcov \
	--exclude='__pycache__' \
	--exclude='.pytest_cache' \
	--exclude='.ruff_cache' \
	--exclude='.mypy_cache' \
	-cf "$archive" . || tar_status=$?
[[ "$tar_status" -le 1 ]] || die "copying the repository failed (tar exit $tar_status)"
tar -C /work -xf "$archive"
rm -f "$archive"

cd /work || die "cd /work failed"

info "running make check (the full gate) in the container"
make check
