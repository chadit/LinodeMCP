# CI-mirror image: run the identical `make check` gate CI runs, locally, on
# the same OS family, via `make check-container`. Provisioning is
# single-sourced: scripts/ci-setup.sh runs both here and in the CI workflow,
# so this image and the runner cannot drift. One-shot verification container:
# no HEALTHCHECK, not deployed, so digest pinning does not apply.

# Tracks latest Go on purpose: the repo's gate-tool policy is always-latest
# with local == CI, and the compile toolchain is governed by go.mod's
# toolchain directive, so build reproducibility does not depend on this tag.
# hadolint ignore=DL3007
FROM golang:latest

# Every RUN below is written against bash. pipefail is on so a pipe added
# later cannot pass on the strength of its last stage alone.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# Unpinned apt versions on purpose, same always-latest policy as the base
# tag. node backs `npx pyright@latest`; jq parses the betterleaks release
# metadata in ci-setup.sh.
# hadolint ignore=DL3008
RUN apt-get update \
    && apt-get install -y --no-install-recommends nodejs npm jq \
    && rm -rf /var/lib/apt/lists/*

# The same provisioning script the CI workflow runs, uv included: this image
# has no pip, so ci-setup.sh takes its checksum-verified release archive. GOBIN
# routes the go-installed tools into /usr/local/bin so they are on PATH for the
# non-root user.
COPY scripts/ci-setup.sh /tmp/ci-setup.sh
RUN GOBIN=/usr/local/bin bash /tmp/ci-setup.sh && rm /tmp/ci-setup.sh

# uv also supplies the Python 3.14 interpreter, because the Go base image ships
# no Python (CI gets its Python from actions/setup-python at the same version).
# The interpreter store lives outside /root so the non-root user can read it.
ENV UV_PYTHON_INSTALL_DIR=/opt/uv/python
RUN uv python install 3.14 \
    && ln -s "$(uv python find 3.14)" /usr/local/bin/python3.14-uv \
    && ln -s /usr/local/bin/python3.14-uv /usr/local/bin/python3

COPY --chmod=0755 scripts/ci-container-entry.sh /usr/local/bin/ci-container-entry

# Non-root, explicit UID. /work is the scratch checkout; go/npx/trivy runtime
# caches land under the user's HOME.
RUN useradd -m -u 1000 ci && mkdir -p /work && chown ci:ci /work
USER ci
ENV GOPATH=/home/ci/go
WORKDIR /work

CMD ["ci-container-entry"]
