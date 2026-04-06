# syntax=docker/dockerfile:1
# hadolint ignore=DL3007
FROM golang:1.23 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -buildid=" \
    -trimpath \
    -tags=netgo,osusergo \
    -o linodemcp cmd/linodemcp/main.go

# hadolint ignore=DL3007
FROM alpine:3.21

# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates && \
    addgroup -S linodemcp && \
    adduser -S -G linodemcp -h /home/linodemcp -s /sbin/nologin linodemcp

COPY --from=builder /build/linodemcp /usr/local/bin/linodemcp

USER linodemcp

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/linodemcp", "--help"] || exit 1

ENTRYPOINT ["linodemcp"]
