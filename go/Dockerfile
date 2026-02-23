# syntax=docker/dockerfile:1
# hadolint ignore=DL3007
FROM golang:latest AS builder

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
FROM alpine:latest

# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates && \
    addgroup -S linodemcp && \
    adduser -S -G linodemcp -h /home/linodemcp -s /sbin/nologin linodemcp

COPY --from=builder /build/linodemcp /usr/local/bin/linodemcp

USER linodemcp

ENTRYPOINT ["linodemcp"]
