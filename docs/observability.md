# Observability

Metrics, tracing, and health endpoints for running LinodeMCP as something you
monitor rather than something you hope about. Both implementations expose the
same metric names and the same endpoints. This is the OpenTelemetry layer;
the [audit log](./audit-log.md) is a separate, more structured stream for
tool-call accountability, with its own [operations page](./audit-operations.md).

## Configuration

The `observability` block in `~/.config/linodemcp/config.yml`:

```yaml
observability:
  metrics:
    enabled: true
    prometheus:
      enabled: true
      host: "127.0.0.1"
      port: 8888
      path: "/metrics"
  tracing:
    enabled: false
    endpoint: "localhost:4317"
    protocol: "grpc"      # or "http" for OTLP over HTTP
    insecure: false       # true skips TLS (local collectors)
    sampleRate: 1.0
  health:
    enabled: true
    host: "127.0.0.1"
    port: 8889
    path: "/healthz"
```

Defaults: metrics on `127.0.0.1:8888/metrics`, health on
`127.0.0.1:8889/healthz`, tracing off. Everything binds to loopback by
default; expose it deliberately if a scraper lives elsewhere.

## Metrics

Instruments are registered through OpenTelemetry and exported in Prometheus
text format on the configured port. The names below are the Prometheus
rendering (OTel's dotted names with underscores):

| Metric | Type | Labels | Counts |
| --- | --- | --- | --- |
| `linodemcp_requests_total` | counter | `tool`, `method`, `status` | MCP tool calls |
| `linodemcp_request_duration_seconds` | histogram | `tool`, `method` | MCP tool-call latency |
| `linodemcp_errors_total` | counter | `tool`, `error_type` | MCP errors |
| `linodemcp_api_requests_total` | counter | `endpoint`, `method`, `status_code` | Outgoing Linode API requests |
| `linodemcp_api_request_duration_seconds` | histogram | `endpoint`, `method` | Linode API latency |

The split between `requests_total` and `api_requests_total` matters for
debugging: the first counts what the AI asked the server to do, the second
counts what the server asked Linode to do. A dry-run or a refused destroy
shows up in the first and (mostly) not in the second.

The Go endpoint also carries the standard runtime collector series
(`go_goroutines` and friends), pinned by a scrape test
(`go/internal/observability/metrics_scrape_test.go`) so a registry refactor
can't silently drop them.

Quick check that the exporter is alive:

```bash
curl -s http://127.0.0.1:8888/metrics | grep linodemcp_requests_total
```

## Health endpoints

Three routes hang off the configured health `path` (default `/healthz` on
port 8889):

| Route | Question it answers | Response |
| --- | --- | --- |
| `<path>/live` | Is the process up? | 200 whenever the listener is serving |
| `<path>/ready` | Are all registered dependency checks passing? | 200 with a JSON body when healthy, 503 otherwise |
| `<path>/healthz` | Same as `/ready` | Same |

With the default path that means `/healthz/live`, `/healthz/ready`, and
`/healthz/healthz`. Point liveness probes at `/live` and readiness probes at
`/ready`.

## Tracing

Tracing is off by default. When enabled, spans export over OTLP to the
configured `endpoint`, using `protocol: grpc` (default, collector port 4317)
or `protocol: http`. With no `endpoint` configured the tracer is a noop in
every language; an export target is always an explicit choice, and no
environment variable can supply one (observability has no env overrides, see
[contracts/env-vars.txt](./contracts/env-vars.txt)). `insecure: true` skips
TLS for local collectors, and `sampleRate` sets the head-sampling fraction
(1.0 traces everything).

## Related

- [Audit log](./audit-log.md): the accountability stream layered on top of
  this; its `mode` field and query tools answer "what did the AI do", which
  metrics deliberately don't.
- [Audit operations](./audit-operations.md): sinks and retention for that
  stream.
