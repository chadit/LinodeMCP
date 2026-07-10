#!/usr/bin/env bash
# ci-setup.sh installs the gate toolchain `make check` expects on PATH:
# gofumpt/goimports/golangci-lint (latest), buf (pinned), uv, betterleaks
# (latest release binary), and trivy (latest). It is the single source of
# provisioning: the CI workflow and the CI-mirror container image
# (ci/Dockerfile) both run this exact script, so the two environments cannot
# drift. Go and Python themselves come from the caller (actions/setup-go and
# actions/setup-python in CI; the container base image locally).
set -euo pipefail

trap 'echo "E: error on line $LINENO" >&2' ERR

# Temp dirs registered by any function, removed by the single EXIT trap
# (per-function EXIT traps would overwrite each other).
CLEANUP_DIRS=()

# cleanup removes every registered temp dir; runs on any exit.
cleanup() {
	local dir
	for dir in "${CLEANUP_DIRS[@]:-}"; do
		[[ -n "$dir" ]] && rm -rf -- "$dir"
	done
}
trap cleanup EXIT

# info logs a progress line to stderr so stdout stays free for tool output.
info() { echo "I: $1" >&2; }

# die logs an error to stderr and exits 1.
die() {
	echo "E: $1" >&2
	exit 1
}

# install_file installs a binary into /usr/local/bin, escalating with sudo
# only when the directory is not writable by the current user (CI runner).
# Returns non-zero via die when neither write access nor sudo is available.
install_file() {
	local src="$1"
	local name="$2"
	local dest="/usr/local/bin"
	if [[ -w "$dest" ]]; then
		install -m 0755 -- "$src" "$dest/$name"
	elif command -v sudo >/dev/null 2>&1; then
		sudo install -m 0755 -- "$src" "$dest/$name"
	else
		die "cannot write to $dest and sudo is unavailable"
	fi
}

# install_go_tools installs the format/lint tools the make targets invoke
# from PATH. Everything floats at latest (repo policy: local == CI, always
# current) except buf, which is pinned because generated code must be
# byte-reproducible against the committed baselines.
install_go_tools() {
	command -v go >/dev/null 2>&1 || die "go is required (setup-go in CI, base image in the container)"
	info "installing gofumpt, goimports, golangci-lint (latest)"
	go install mvdan.cc/gofumpt@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	info "installing buf (pinned v1.71.0: codegen must be byte-reproducible)"
	go install github.com/bufbuild/buf/cmd/buf@v1.71.0
}

# install_uv provides uv for the lock-check gate. pip is used when present
# (CI runner via setup-python); otherwise the official installer.
install_uv() {
	if command -v uv >/dev/null 2>&1; then
		info "uv already present: $(uv --version)"
		return 0
	fi
	if command -v pip >/dev/null 2>&1; then
		info "installing uv via pip"
		pip install uv
	else
		info "installing uv via the official installer"
		curl -LsSf https://astral.sh/uv/install.sh | UV_INSTALL_DIR=/usr/local/bin sh
	fi
}

# install_betterleaks fetches the latest release binary for this
# architecture. A release binary because betterleaks carries a `replace`
# directive in its go.mod, which `go install pkg@latest` refuses. GH_TOKEN,
# when set, raises the GitHub API rate limit (shared CI runner IPs exhaust
# the unauthenticated quota).
install_betterleaks() {
	if command -v betterleaks >/dev/null 2>&1; then
		info "betterleaks already present: $(betterleaks version)"
		return 0
	fi
	command -v jq >/dev/null 2>&1 || die "jq is required to parse the betterleaks release metadata"
	local arch bl_arch
	arch="$(uname -m)"
	case "$arch" in
	x86_64) bl_arch="x64" ;;
	aarch64 | arm64) bl_arch="arm64" ;;
	*) die "unsupported architecture for betterleaks: $arch" ;;
	esac
	local auth=()
	if [[ -n "${GH_TOKEN:-}" ]]; then
		auth=(-H "Authorization: Bearer ${GH_TOKEN}")
	fi
	local api="https://api.github.com/repos/betterleaks/betterleaks/releases/latest"
	local tag version asset tmpdir
	tag="$(curl -fsSL "${auth[@]}" "$api" | jq -r .tag_name)"
	[[ -n "$tag" && "$tag" != "null" ]] || die "could not resolve the latest betterleaks release tag"
	version="${tag#v}"
	asset="betterleaks_${version}_linux_${bl_arch}.tar.gz"
	tmpdir="$(mktemp -d)"
	CLEANUP_DIRS+=("$tmpdir")
	info "installing betterleaks $tag ($asset)"
	curl -fsSL "${auth[@]}" -o "$tmpdir/$asset" \
		"https://github.com/betterleaks/betterleaks/releases/download/$tag/$asset"
	tar -xzf "$tmpdir/$asset" -C "$tmpdir" betterleaks
	install_file "$tmpdir/betterleaks" betterleaks
	betterleaks version
}

# install_trivy fetches the latest trivy via the official install script,
# staged through a temp dir so install_file centralizes the sudo decision.
install_trivy() {
	if command -v trivy >/dev/null 2>&1; then
		info "trivy already present: $(trivy --version | head -1)"
		return 0
	fi
	local tmpdir
	tmpdir="$(mktemp -d)"
	CLEANUP_DIRS+=("$tmpdir")
	info "installing trivy (latest)"
	curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh |
		sh -s -- -b "$tmpdir"
	install_file "$tmpdir/trivy" trivy
	trivy --version
}

# main provisions the full gate toolchain in dependency-free order.
main() {
	install_go_tools
	install_uv
	install_betterleaks
	install_trivy
	info "gate toolchain ready"
}

main "$@"
