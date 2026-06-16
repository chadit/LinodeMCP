# syntax=docker/dockerfile:1

# Build stage. Pinned to the builder's native platform so the target
# platforms below are cross-compiled; the runtime stage only copies
# files, which keeps multi-arch builds free of QEMU emulation.
#
# hadolint ignore=DL3029
# BUILDPLATFORM is the documented buildx cross-compile pattern: the
# builder runs natively and GOOS/GOARCH select the target.
#
# digest-pin: skip the version tag tracks Go toolchain patches; the
# image rebuilds on every release tag, which is the patch cadence.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /src

# Dependency layer first so source-only changes reuse the module cache.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=0.1.0
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    "-tags=netgo,osusergo" \
    -ldflags="-s -w -buildid= \
      -X 'github.com/chadit/LinodeMCP/go/internal/appinfo.Version=${VERSION}' \
      -X 'github.com/chadit/LinodeMCP/go/internal/appinfo.commit=${COMMIT}' \
      -X 'github.com/chadit/LinodeMCP/go/internal/appinfo.buildDate=${BUILD_DATE}'" \
    -o /out/linodemcp ./cmd/linodemcp

# Runtime stage: distroless static (CA certs included), nonroot user.
#
# digest-pin: skip distroless rebuilds upstream for CVE fixes and this
# image rebuilds per release tag; that cadence is the security model.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/linodemcp /usr/local/bin/linodemcp

USER nonroot:nonroot

# The probe execs the profile CLI's usage path, which prints usage and
# exits 0. That proves the binary is present and runnable without
# spawning a second stdio server (a bare-invocation probe would start
# one); a stdio MCP process has no deeper liveness signal to offer.
HEALTHCHECK --interval=30s --timeout=5s --start-period=2s --retries=3 \
    CMD ["/usr/local/bin/linodemcp", "profile"]

# Bare invocation serves MCP over stdio; append "profile ..." for the
# profile management CLI.
ENTRYPOINT ["/usr/local/bin/linodemcp"]
