# syntax=docker/dockerfile:1
# hadolint ignore=DL3007
FROM python:3.13 AS builder

# hadolint ignore=DL3008
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential \
        libffi-dev \
        pkg-config && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /parent/build

COPY pyproject.toml .
COPY src/ src/

RUN echo "# LinodeMCP" > /parent/README.md && \
    pip wheel --no-cache-dir --wheel-dir=/wheels .

FROM python:3.13-slim

RUN --mount=type=bind,from=builder,source=/wheels,target=/tmp/wheels \
    pip install --no-cache-dir --no-index --find-links=/tmp/wheels linodemcp

RUN groupadd -r linodemcp && \
    useradd -r -g linodemcp -d /home/linodemcp -m -s /sbin/nologin linodemcp

USER linodemcp

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["linodemcp", "--help"] || exit 1

ENTRYPOINT ["linodemcp"]
