#!/usr/bin/env bash
# ci-setup.sh installs the gate toolchain `make check` expects on PATH:
# gofumpt/goimports/golangci-lint/gopls (latest), buf (pinned), uv, droast,
# betterleaks (latest release binary), and trivy (latest). It is the single
# source of provisioning: the CI workflow and the CI-mirror container image
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

# latest_tag prints a GitHub repository's newest release tag, read from the
# redirect releases/latest answers with. The redirect rather than the REST API
# because the API's unauthenticated quota is shared across runner IPs.
# Returns non-zero via die when the redirect names no tag.
latest_tag() {
	local repo="$1"
	local resolved
	resolved="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/${repo}/releases/latest")"
	[[ "$resolved" == */tag/* ]] || die "could not resolve the latest ${repo} release tag"
	echo "${resolved##*/}"
}

# verify_sha256 checks a downloaded file against the digest its release
# publishes, so a corrupted or substituted download is refused before anything
# runs it. Returns non-zero via die when the two digests disagree.
verify_sha256() {
	local file="$1"
	local want="$2"
	local got
	local -a digest
	[[ -n "$want" ]] || die "no published checksum for $(basename -- "$file")"
	if command -v sha256sum >/dev/null 2>&1; then
		digest=(sha256sum)
	else
		digest=(shasum -a 256)
	fi
	got="$("${digest[@]}" -- "$file" | cut -d' ' -f1)"
	[[ "$got" == "$want" ]] || die "checksum mismatch for $(basename -- "$file"): published $want, downloaded $got"
	info "$(basename -- "$file") verified against the published SHA-256"
}

# install_go_tools installs the format/lint tools the make targets invoke
# from PATH. Everything floats at latest (repo policy: local == CI, always
# current) except buf, which is pinned because generated code must be
# byte-reproducible against the committed baselines.
install_go_tools() {
	command -v go >/dev/null 2>&1 || die "go is required (setup-go in CI, base image in the container)"
	info "installing gofumpt, goimports, golangci-lint, gopls (latest)"
	go install mvdan.cc/gofumpt@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install golang.org/x/tools/gopls@latest
	info "installing buf (pinned v1.71.0: codegen must be byte-reproducible)"
	go install github.com/bufbuild/buf/cmd/buf@v1.71.0
}

# install_uv provides uv for the lock-check gate and, in the CI-mirror image,
# the Python interpreter as well. pip when it is present (CI runner via
# setup-python); otherwise the release archive, checked against the SHA-256
# astral-sh publishes beside it. The one-line installer at astral.sh has no
# checksum of its own, so nothing about it could be verified before it ran.
install_uv() {
	if command -v uv >/dev/null 2>&1; then
		info "uv already present: $(uv --version)"
		return 0
	fi
	if command -v pip >/dev/null 2>&1; then
		info "installing uv via pip"
		pip install uv
		return 0
	fi
	local arch slug tag base archive tmpdir releases
	arch="$(uname -s)/$(uname -m)"
	case "$arch" in
	Linux/x86_64) slug="x86_64-unknown-linux-gnu" ;;
	Linux/aarch64 | Linux/arm64) slug="aarch64-unknown-linux-gnu" ;;
	Darwin/x86_64) slug="x86_64-apple-darwin" ;;
	Darwin/arm64) slug="aarch64-apple-darwin" ;;
	*) die "unsupported platform for uv: $arch" ;;
	esac
	releases="https://github.com/astral-sh/uv/releases"
	tag="$(latest_tag astral-sh/uv)"
	base="uv-${slug}"
	archive="${base}.tar.gz"
	tmpdir="$(mktemp -d)"
	CLEANUP_DIRS+=("$tmpdir")
	info "installing uv $tag ($archive)"
	curl -fsSL -o "$tmpdir/$archive" "$releases/download/$tag/$archive"
	verify_sha256 "$tmpdir/$archive" \
		"$(curl -fsSL "$releases/download/$tag/$archive.sha256" | cut -d' ' -f1)"
	tar -xzf "$tmpdir/$archive" -C "$tmpdir"
	install_file "$tmpdir/$base/uv" uv
	install_file "$tmpdir/$base/uvx" uvx
	uv --version
}

# install_droast fetches the release binary for this platform, verified against
# the repository's published sha256sums.txt. droast backs the `make dockerfiles`
# gate.
install_droast() {
	if command -v droast >/dev/null 2>&1; then
		info "droast already present: $(droast --version)"
		return 0
	fi
	local arch asset tag tmpdir releases
	arch="$(uname -s)/$(uname -m)"
	case "$arch" in
	Linux/x86_64) asset="droast-linux-x86_64" ;;
	Linux/aarch64 | Linux/arm64) asset="droast-linux-arm64" ;;
	Darwin/x86_64) asset="droast-macos-x86_64" ;;
	Darwin/arm64) asset="droast-macos-arm64" ;;
	*) die "unsupported platform for droast: $arch" ;;
	esac
	releases="https://github.com/immanuwell/dockerfile-roast/releases"
	tag="$(latest_tag immanuwell/dockerfile-roast)"
	tmpdir="$(mktemp -d)"
	CLEANUP_DIRS+=("$tmpdir")
	info "installing droast $tag ($asset)"
	curl -fsSL -o "$tmpdir/$asset" "$releases/download/$tag/$asset"
	curl -fsSL -o "$tmpdir/sha256sums.txt" "$releases/download/$tag/sha256sums.txt"
	verify_sha256 "$tmpdir/$asset" \
		"$(awk -v want="$asset" '$2 == want || $2 == "*" want { print $1 }' "$tmpdir/sha256sums.txt")"
	install_file "$tmpdir/$asset" droast
	droast --version
}

# install_betterleaks fetches the latest release archive for this architecture,
# verified against the checksums file the release publishes. A release binary
# because betterleaks carries a `replace` directive in its go.mod, which
# `go install pkg@latest` refuses. It reads its tag from the GitHub API, not
# the redirect the others read, so GH_TOKEN can raise the rate limit shared CI
# runner IPs exhaust.
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
	# The ${arr[@]+...} guard below keeps an unset token safe under set -u on
	# bash 3.2, which a macOS /bin/bash still is: there an empty array
	# expansion is an unbound variable rather than nothing.
	local auth=()
	if [[ -n "${GH_TOKEN:-}" ]]; then
		auth=(-H "Authorization: Bearer ${GH_TOKEN}")
	fi
	local api="https://api.github.com/repos/betterleaks/betterleaks/releases/latest"
	local tag version asset tmpdir
	tag="$(curl -fsSL ${auth[@]+"${auth[@]}"} "$api" | jq -r .tag_name)"
	[[ -n "$tag" && "$tag" != "null" ]] || die "could not resolve the latest betterleaks release tag"
	version="${tag#v}"
	asset="betterleaks_${version}_linux_${bl_arch}.tar.gz"
	tmpdir="$(mktemp -d)"
	CLEANUP_DIRS+=("$tmpdir")
	local releases="https://github.com/betterleaks/betterleaks/releases"
	info "installing betterleaks $tag ($asset)"
	curl -fsSL ${auth[@]+"${auth[@]}"} -o "$tmpdir/$asset" "$releases/download/$tag/$asset"
	curl -fsSL ${auth[@]+"${auth[@]}"} -o "$tmpdir/checksums.txt" "$releases/download/$tag/checksums.txt"
	verify_sha256 "$tmpdir/$asset" \
		"$(awk -v want="$asset" '$2 == want { print $1 }' "$tmpdir/checksums.txt")"
	tar -xzf "$tmpdir/$asset" -C "$tmpdir" betterleaks
	install_file "$tmpdir/betterleaks" betterleaks
	betterleaks version
}

# install_trivy fetches the latest release archive, verified against the
# checksums file the release publishes. The archive rather than the official
# install script, which is a shell script with no checksum of its own.
install_trivy() {
	if command -v trivy >/dev/null 2>&1; then
		info "trivy already present: $(trivy --version | head -1)"
		return 0
	fi
	local arch slug tag version archive tmpdir releases
	arch="$(uname -s)/$(uname -m)"
	case "$arch" in
	Linux/x86_64) slug="Linux-64bit" ;;
	Linux/aarch64 | Linux/arm64) slug="Linux-ARM64" ;;
	Darwin/x86_64) slug="macOS-64bit" ;;
	Darwin/arm64) slug="macOS-ARM64" ;;
	*) die "unsupported platform for trivy: $arch" ;;
	esac
	releases="https://github.com/aquasecurity/trivy/releases"
	tag="$(latest_tag aquasecurity/trivy)"
	version="${tag#v}"
	archive="trivy_${version}_${slug}.tar.gz"
	tmpdir="$(mktemp -d)"
	CLEANUP_DIRS+=("$tmpdir")
	info "installing trivy $tag ($archive)"
	curl -fsSL -o "$tmpdir/$archive" "$releases/download/$tag/$archive"
	curl -fsSL -o "$tmpdir/checksums.txt" "$releases/download/$tag/trivy_${version}_checksums.txt"
	verify_sha256 "$tmpdir/$archive" \
		"$(awk -v want="$archive" '$2 == want { print $1 }' "$tmpdir/checksums.txt")"
	tar -xzf "$tmpdir/$archive" -C "$tmpdir" trivy
	install_file "$tmpdir/trivy" trivy
	trivy --version
}

# main provisions the full gate toolchain in dependency-free order.
main() {
	install_go_tools
	install_uv
	install_droast
	install_betterleaks
	install_trivy
	info "gate toolchain ready"
}

main "$@"
