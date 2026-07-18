# LinodeMCP

**Note: This is an active weekend project. Tests and edge cases are currently being built out.**

An MCP (Model Context Protocol) server that gives AI assistants like Claude|Gemini|Copilot programmatic access to the Linode cloud platform API. Ships with both Go and Python implementations that share the same configuration format and tool interface.

## What It Does

LinodeMCP exposes Linode API operations as MCP tools. AI assistants can use these tools to query and manage your Linode infrastructure -- all through a standard protocol.

### Available tools

LinodeMCP aims for near-complete coverage of the [Linode API v4](https://techdocs.akamai.com/linode-api/reference/api): instances, volumes, object storage, networking, NodeBalancers, DNS, LKE, VPCs, databases, images, and account/profile. Each endpoint is exposed as an MCP tool named after it (e.g. `linode_instance_create`, `linode_volume_delete`, `linode_lke_cluster_create`).

To see exactly which tools your build registers, call the `version` tool (it reports the feature list) or the `linode_profile_list_tools` meta tool. Which of those an AI client can actually invoke is governed by the active [profile](docs/profiles.md).

Write, destroy, and admin tools require `confirm: true` and support `dry_run: true` previews; destructive calls are additionally gated (see [Dry-run & safety](#dry-run--safety)).

### Multi-Environment Support

Configure multiple Linode environments (production, staging, dev) in a single config file. Tools accept an optional `environment` parameter to target a specific one, falling back to `default` when omitted.

## Documentation

**For users & operators**, running, configuring, and trusting the server:

| Doc | What it covers |
|-----|----------------|
| [Profiles](docs/profiles.md) | Restrict which tools an AI client can see and use; token-scope validation; the built-in profiles |
| [Profile recipes](docs/profile-recipes.md) | Copy-paste profile starting points for common setups |
| [Dry-run & safety](docs/dry-run.md) | Preview any mutator, bypass-confirm on destructive calls, profile pre-check, and yolo |
| [Two-stage writes](docs/two-stage-writes.md) | Plan a destructive call, review it, then apply it by id; the server refuses if the resource drifted |
| [State drift](docs/state-drift.md) | What drift detection catches and how to read each refusal |
| [Audit log](docs/audit-log.md) | What the AI did: event schema, sinks, retention, redaction, and the query tools |
| [Audit reports](docs/audit-reports.md) | The custom-report filter grammar with worked examples |
| [Host integrations](docs/host-integrations/README.md) | Wiring Claude Desktop / Claude Code / Gemini / Copilot and their slash commands |
| [Verifying releases](docs/verifying-releases.md) | Checking checksums, cosign signatures, SBOMs, and SLSA provenance on anything we ship |

**For implementers & contributors**, working on the code:

- [Development](#development): build, test, and lint commands for both implementations
- [Adding a language](docs/adding-a-language.md): the checklist and gates for bringing up a new language implementation
- [Release process](docs/release-process.md): cutting a release, the artifact pipeline, failure handling
- [Project layout](#project-layout) and [key design decisions](#key-design-decisions)

The full docs index, including the machine-read gate files `make check` consumes, lives at [docs/README.md](docs/README.md). Agents can start from [llms.txt](llms.txt).

## Installation

### Prerequisites

**Go implementation:**

- Go 1.26+
- A Linode API token ([create one here](https://cloud.linode.com/profile/tokens))

**Python implementation:**

- Python 3.13+
- A Linode API token

### Configuration

Both implementations read from the same config file at `~/.config/linodemcp/config.yml`. The server creates a template on first run, or you can create one manually:

```yaml
server:
  name: "LinodeMCP"
  logLevel: "info"
  transport: "stdio"
  host: "127.0.0.1"
  port: 8080

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

resilience:
  rateLimitPerMinute: 700
  circuitBreakerThreshold: 5
  circuitBreakerTimeout: 30s
  maxRetries: 3
  baseRetryDelay: 1s
  maxRetryDelay: 30s

environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "${LINODEMCP_LINODE_TOKEN}"
```

You can also set configuration through environment variables:

| Variable | Description |
|----------|-------------|
| `LINODEMCP_CONFIG_PATH` | Custom config file path |
| `LINODEMCP_SERVER_NAME` | Override server name |
| `LINODEMCP_LOG_LEVEL` | Override log level |
| `LINODEMCP_LINODE_API_URL` | Linode API base URL |
| `LINODEMCP_LINODE_TOKEN` | Linode API token |

### Install a prebuilt binary (no toolchain needed)

Each release ships signed, prebuilt binaries for Linux, macOS, and Windows (amd64 and arm64), so you don't need a Go or Python toolchain to run the server or the CLI.

Download the archive for your platform from the [latest release](https://github.com/chadit/LinodeMCP/releases/latest), verify its checksum, and put the binary on your `PATH`:

```bash
ver=v0.2.0
base=https://github.com/chadit/LinodeMCP/releases/download/${ver}
curl -fsSL -O ${base}/linodemcp-linux-amd64.tar.gz
curl -fsSL -O ${base}/linodemcp-linux-amd64.tar.gz.sha256
sha256sum -c linodemcp-linux-amd64.tar.gz.sha256
tar -xzf linodemcp-linux-amd64.tar.gz
sudo install linodemcp /usr/local/bin/
linodemcp version
```

Swap `linux-amd64` for `darwin-arm64`, `windows-amd64` (a `.zip`), and so on. Releases are signed with cosign and carry SLSA provenance; see [Verifying releases](docs/verifying-releases.md).

### Install with the Go toolchain

```bash
go install github.com/chadit/LinodeMCP/go/cmd/linodemcp@latest
```

This builds and installs the `linodemcp` binary into `$(go env GOPATH)/bin`. Pin a version with `@v0.2.0` instead of `@latest` for reproducible installs.

### Build from source

#### Go

1. Clone the repo and change into the Go directory:

   ```bash
   git clone https://github.com/chadit/LinodeMCP.git
   cd LinodeMCP/go/
   ```

2. Install dev tooling:

   ```bash
   make install-tools
   ```

3. Build the binary:

   ```bash
   make build
   ```

   This puts the binary at `go/bin/linodemcp`. You'll need this absolute path for MCP client config below.

4. Quick test run:

   ```bash
   make run
   ```

#### Python

1. Clone the repo and change into the Python directory:

   ```bash
   git clone https://github.com/chadit/LinodeMCP.git
   cd LinodeMCP/python/
   ```

2. Install with dev dependencies (creates a venv automatically):

   ```bash
   make install-dev
   ```

   The binary lands at `python/.venv/bin/linodemcp`. You'll need this absolute path for MCP client config below.

3. Quick test run:

   ```bash
   make run
   ```

### Container

Pull the released multi-arch image (linux/amd64 + linux/arm64) from GHCR:

```bash
docker pull ghcr.io/chadit/linodemcp:latest
```

Pin a version tag (`ghcr.io/chadit/linodemcp:v0.2.0`) when you want reproducible setups; `latest` and the floating minor tag only ever point at stable releases. Images are signed with cosign and ship SBOMs and SLSA provenance, see [Verifying releases](docs/verifying-releases.md).

Or build a container image locally for either implementation:

```bash
make docker-build-go      # builds linodemcp:go
make docker-build-python  # builds linodemcp:python
```

To use Podman instead of Docker:

```bash
CONTAINER_ENGINE=podman make docker-build-go
```

See [Docker / Podman](#docker--podman) below for MCP client configuration with containers.

## MCP Client Setup

Each MCP client needs to know where the LinodeMCP binary lives and how to pass your Linode API token. Pick your client below -- all examples show both Go and Python variants.

### Claude Desktop

Add this to your Claude Desktop config on macOS at `~/Library/Application Support/Claude/claude_desktop_config.json`:

**Go:**

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "/absolute/path/to/LinodeMCP/go/bin/linodemcp",
      "env": {
        "LINODEMCP_LINODE_TOKEN": "your-token-here"
      }
    }
  }
}
```

**Python:**

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "/absolute/path/to/LinodeMCP/python/.venv/bin/linodemcp",
      "env": {
        "LINODEMCP_LINODE_TOKEN": "your-token-here"
      }
    }
  }
}
```

### Claude Code CLI

Add LinodeMCP to the current project with a one-liner:

**Go:**

```bash
claude mcp add linodemcp -- /absolute/path/to/LinodeMCP/go/bin/linodemcp
```

**Python:**

```bash
claude mcp add linodemcp -- /absolute/path/to/LinodeMCP/python/.venv/bin/linodemcp
```

To make it available across all your projects, add `--scope user`:

```bash
claude mcp add --scope user linodemcp -- /absolute/path/to/LinodeMCP/go/bin/linodemcp
```

For team sharing, drop a `.mcp.json` in the project root:

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "/absolute/path/to/LinodeMCP/go/bin/linodemcp",
      "env": {
        "LINODEMCP_LINODE_TOKEN": "${LINODEMCP_LINODE_TOKEN}"
      }
    }
  }
}
```

Set the `LINODEMCP_LINODE_TOKEN` env var in your shell, and Claude Code picks it up automatically.

### Gemini CLI

Add to `~/.gemini/settings.json`:

**Go:**

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "/absolute/path/to/LinodeMCP/go/bin/linodemcp",
      "env": {
        "LINODEMCP_LINODE_TOKEN": "$LINODEMCP_LINODE_TOKEN"
      }
    }
  }
}
```

**Python:**

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "/absolute/path/to/LinodeMCP/python/.venv/bin/linodemcp",
      "env": {
        "LINODEMCP_LINODE_TOKEN": "$LINODEMCP_LINODE_TOKEN"
      }
    }
  }
}
```

Note: Gemini CLI uses `$VAR` syntax (no curly braces) for environment variable references.

### GitHub Copilot (VS Code)

Create a `.vscode/mcp.json` in your workspace:

**Go:**

```json
{
  "servers": {
    "linodemcp": {
      "command": "/absolute/path/to/LinodeMCP/go/bin/linodemcp",
      "env": {
        "LINODEMCP_LINODE_TOKEN": "${input:linode-token}"
      }
    }
  }
}
```

**Python:**

```json
{
  "servers": {
    "linodemcp": {
      "command": "/absolute/path/to/LinodeMCP/python/.venv/bin/linodemcp",
      "env": {
        "LINODEMCP_LINODE_TOKEN": "${input:linode-token}"
      }
    }
  }
}
```

VS Code prompts you for the token value on first use through the `${input:linode-token}` pattern. The token gets cached for the session.

### Cursor / Windsurf

Both Cursor and Windsurf use the same `.mcp.json` format as Claude Code. See the [Claude Code CLI](#claude-code-cli) section and drop that same file in your project root.

### Docker / Podman

Running LinodeMCP in a container avoids installing Go or Python locally. MCP uses stdio transport, so the container needs `-i` (keep stdin open) and `-e` to forward your API token.

The examples below use locally built images; substitute `ghcr.io/chadit/linodemcp:latest` (or a pinned version tag) to use the released image instead.

**Go image:**

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "-e", "LINODEMCP_LINODE_TOKEN", "linodemcp:go"],
      "env": {
        "LINODEMCP_LINODE_TOKEN": "your-token-here"
      }
    }
  }
}
```

**Python image:**

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "docker",
      "args": ["run", "-i", "--rm", "-e", "LINODEMCP_LINODE_TOKEN", "linodemcp:python"],
      "env": {
        "LINODEMCP_LINODE_TOKEN": "your-token-here"
      }
    }
  }
}
```

For Podman, swap `"docker"` with `"podman"` in the command field.

To mount a config file instead of using environment variables:

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-v", "~/.config/linodemcp:/home/linodemcp/.config/linodemcp:ro",
        "-e", "LINODEMCP_LINODE_TOKEN",
        "linodemcp:go"
      ],
      "env": {
        "LINODEMCP_LINODE_TOKEN": "your-token-here"
      }
    }
  }
}
```

### Context Forge (IBM MCP Gateway)

[Context Forge](https://github.com/IBM/mcp-context-forge) is an MCP gateway that acts as a central registry for MCP servers. Instead of each client connecting to LinodeMCP directly, you register it once with Context Forge and all your clients connect through the gateway.

1. Bridge the container's stdio to HTTP. Context Forge communicates over HTTP, so use `mcpgateway.translate` to wrap the container's stdin/stdout:

   ```bash
   python3 -m mcpgateway.translate \
     --stdio "docker run --rm -i -e LINODEMCP_LINODE_TOKEN linodemcp:go" \
     --expose-sse \
     --port 8010 &
   ```

   For Podman, replace `docker` with `podman` in the command string.

2. Register the bridged server with Context Forge:

   ```bash
   export TOKEN=$(python3 -m mcpgateway.utils.create_jwt_token \
     --username admin@example.com --exp 10080 --secret your-jwt-secret)

   curl -X POST -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "name": "linodemcp",
       "url": "http://localhost:8010/sse"
     }' \
     http://localhost:4444/gateways
   ```

3. Add to a virtual server. In the Context Forge UI at `http://localhost:4444`, create or update a virtual server and add the LinodeMCP tools to it. Note the virtual server UUID.

4. Point your clients at Context Forge. Use the `mcpgateway.wrapper` module in your MCP client config. This works with Claude Desktop, Claude Code, Gemini CLI, and any other MCP-compatible client:

```json
{
  "mcpServers": {
    "context-forge": {
      "command": "python",
      "args": ["-m", "mcpgateway.wrapper"],
      "env": {
        "MCP_SERVER_URL": "http://localhost:4444/servers/YOUR_VIRTUAL_SERVER_UUID/mcp",
        "MCP_AUTH": "Bearer your-jwt-token",
        "MCP_TOOL_CALL_TIMEOUT": "120"
      }
    }
  }
}
```

This gives you a single gateway endpoint that bundles LinodeMCP with any other MCP servers you've registered. See the [Context Forge docs](https://ibm.github.io/mcp-context-forge/) for the full setup guide.

## Profiles

Profiles control which tools the AI client can see. The server filters the tool list at registration time, so the model literally cannot invoke anything outside the active profile. Eight built-ins ship with the binary:

- `default` and `readonly-full`: read-only across every category. Safe default.
- `compute-admin`, `network-admin`, `kubernetes-admin`, `storage-admin`: read everywhere plus write + destroy on the named category.
- `full-access` and `emergency`: ship disabled. Enable them when you genuinely need them, then disable again.

Switch profiles via the CLI:

```bash
linodemcp profile list                 # what's available, which is active
linodemcp profile show readonly-full   # full details for one
linodemcp profile use compute-admin    # switch the active profile
```

Mutators write the config file atomically and the server hot-reloads without a restart.

User-defined profiles live under `profiles:` in your config:

```yaml
active_profile: dns-admin

profiles:
  dns-admin:
    description: "DNS write access plus read-everywhere"
    allowed_tools:
      - "linode_domain_*"
      - "linode_domain_record_*"
      - "*"   # read-everything alongside the DNS writes
    denied_tools:
      - "linode_instance_*_create"
      - "linode_instance_*_delete"
    allowed_environments: ["prod"]
    required_token_scopes:
      - "domains:read_write"
      - "linodes:read_only"
```

The AI can also help compose profiles via the `linode_profile_*` builder tools. Those run inside the MCP conversation; the user activates the saved profile separately.

For the full reference (schema, capability tags, builder workflow, token-scope validation, security model), see [docs/profiles.md](docs/profiles.md). For copy-paste starting points, see [docs/profile-recipes.md](docs/profile-recipes.md). For host-specific wiring, see [docs/host-integrations/](docs/host-integrations/README.md).

## Dry-run & safety

Every mutating tool can be previewed before it runs, and destructive calls are gated so a resource can't be deleted or replaced without first previewing it (or explicitly opting out):

- **Dry-run**: pass `dry_run: true` to any write/destroy/admin tool to get back `would_execute` + `current_state` (plus dependency cascades, side effects, billing deltas, and warnings) without mutating anything. Coverage is build-enforced by a capability invariant test.
- **Bypass-confirm**: a `CapDestroy` call must either set `confirmed_dry_run: true` (it previewed first) or `confirm_bypass_dry_run: true` (explicitly skip the preview) alongside `confirm: true`, or it's rejected with guidance.
- **Pre-check**: `linode_profile_can_run` reports which calls in a planned sequence the active profile would permit, so the model can bail before partial execution.
- **Yolo**: a profile with `allow_yolo: true` (only the break-glass `emergency` built-in) lets `yolo: true` skip both the preview gate and confirm.

Each call's safety path is recorded in the audit log's `mode` field (`normal` / `dry_run` / `bypass_dry_run` / `yolo`). Full reference: [docs/dry-run.md](docs/dry-run.md).

## Two-stage writes

A dry-run shows what a destructive call would do, but nothing ties that preview to the call you run next, so the resource can change in between. Two-stage writes close that gap. Pass `mode: "plan"` to a delete tool to get back a `plan_id` plus the current state; pass `mode: "apply"` with that `plan_id` to run it. Before applying, the server re-reads the resource and refuses if it drifted since the plan, expired (five minutes by default), or was already used.

A plan carries the same preview a detailed dry-run would (dependencies, side effects, billing deltas, warnings) alongside the state hash. Plans are single-use and in-memory; a restart drops them. Destructive tools opt in by default, and the `two_stage` config block tunes lifetime and per-tool opt-in. Plan and apply each record a `mode` in the audit log.

Full reference: [docs/two-stage-writes.md](docs/two-stage-writes.md). Drift refusals and recovery: [docs/state-drift.md](docs/state-drift.md).

## Auditing

Every tool call is recorded as a structured audit event. The default JSONL sink is always on; an opt-in SQLite sink dual-writes for fast indexed queries.

Where the log lives:

- System service install (UID < 1000 or systemd-managed): `/var/log/linodemcp/audit.log`
- Otherwise: `$XDG_STATE_HOME/linodemcp/audit.log` (default `~/.local/state/linodemcp/audit.log`)

Five MCP tools query the log; all carry `CapMeta` so they are available in every profile, including read-only ones:

| Tool | Purpose |
| --- | --- |
| `linode_audit_recent` | Most recent events with filters (tool glob, status, capability, since/until) |
| `linode_audit_summary` | Counts grouped by tool, status, profile, environment |
| `linode_audit_health` | Audit subsystem state (paths, disk bytes, SQLite stats) |
| `linode_audit_export` | Dump a filtered range to a temp file in JSON / CSV / NDJSON |
| `linode_audit_report` | Run a named report from config |

Sensitive values are redacted before write. The credential list (API tokens, passwords, SSH keys, etc.) is always on; the PII list (postal address, phone, tax ID) is on by default and can be disabled with `audit.redact_pii: false` for operators investigating account-level activity. Both tiers redact by exact field name to keep results reviewable.

For copy-paste integration with specific MCP hosts, see [docs/host-integrations/claude-code/commands/audit.md](docs/host-integrations/claude-code/commands/audit.md) and [docs/host-integrations/claude-desktop/commands/audit.md](docs/host-integrations/claude-desktop/commands/audit.md). For the full reference (event schema, redaction model, query tools, investigative patterns), see [docs/audit-log.md](docs/audit-log.md). For sinks, retention, and recovery, see [docs/audit-operations.md](docs/audit-operations.md). For the custom-report filter grammar, see [docs/audit-reports.md](docs/audit-reports.md).

## Development

### Go Development

```bash
cd go/

make help              # Show all available targets
make test              # Run unit tests
make test-race         # Run tests with race detector
make test-all          # Run all tests with race detector + verbose
make coverage          # Generate HTML coverage report
make lint              # Run golangci-lint (strict mode, zero tolerance)
make fmt               # Format code (goimports + gofumpt)
make gopls-check       # Run gopls diagnostics
make tidy              # Tidy and verify go.mod
make build-prod        # Security-hardened production build
```

### Python Development

```bash
cd python/

make help              # Show all available targets
make test              # Run pytest with coverage
make test-cov          # Generate coverage report
make lint              # Run ruff linter
make format            # Format code with ruff
make typecheck         # Run mypy strict type checking
make all               # Clean + install-dev + lint + typecheck + test
```

### Project Layout

```text
LinodeMCP/
├── go/                              # Go implementation
│   ├── cmd/linodemcp/main.go        # Entry point
│   ├── internal/
│   │   ├── config/                  # YAML/JSON config loading, validation, caching
│   │   ├── linode/                  # API client with retry, error types
│   │   ├── server/                  # MCP server setup and tool registration
│   │   ├── tools/                   # Tool implementations (hello, version, profile, instances)
│   │   └── version/                 # Build-time version info
│   ├── pkg/contracts/               # Tool interface contract
│   ├── Makefile
│   ├── .golangci.yml                # Linter config (all linters enabled)
│   └── go.mod
├── python/                          # Python implementation
│   ├── src/linodemcp/
│   │   ├── main.py                  # Entry point
│   │   ├── config/                  # Config loading and validation
│   │   ├── linode/                  # Async API client (httpx)
│   │   ├── server/                  # MCP server
│   │   ├── tools/                   # Tool implementations
│   │   └── version.py               # Version info
│   ├── tests/                       # unit, integration, e2e test directories
│   ├── Makefile
│   ├── pyproject.toml               # Project config, ruff, mypy, pytest
│   └── ruff.toml
└── .gitignore
```

### Key Design Decisions

- **Dual implementation**: Go for performance and single-binary deployment, Python for quick prototyping and the MCP Python ecosystem. Both share the same config format.
- **Proto contract**: The `proto/` directory is the single source of truth for both tool input schemas and tool output messages in both languages. `buf` generates the Go and Python types and the MCP input JSON Schema from those `.proto` files, so the two implementations cannot drift by construction. Four ratchet gates keep it honest: `tool-parity` (matching input schemas), `input-proto` (input schemas are proto-generated), `read-proto` and `write-proto` (read and mutating output routed through proto), backed by a cross-language conformance corpus that feeds shared fixtures through both languages and asserts byte-identical output. `make check` runs all of them.
- **Stdio transport**: Communicates over stdin/stdout per the MCP spec. This is what Claude Desktop and similar clients expect.
- **Retry with backoff**: The Linode API client wraps all calls with configurable retry logic, exponential backoff, and circuit breaker protection.
- **Path validation**: Config file loading validates paths against a list of dangerous system directories and restricts access to the user's home, working directory, and temp paths.
- **Config caching**: Loaded configs are cached with mtime-based invalidation, so repeated loads don't re-read from disk unnecessarily.

### Dependencies

**Go:**

- [mcp-go](https://github.com/mark3labs/mcp-go) - MCP protocol implementation
- [yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) - YAML config parsing

**Python:**

- [mcp](https://pypi.org/project/mcp/) - MCP protocol SDK
- [httpx](https://www.python-httpx.org/) - Async HTTP client for Linode API
- [pyyaml](https://pyyaml.org/) - YAML config parsing
- [structlog](https://www.structlog.org/) - Structured logging

## Status

This project is in active development (v0.1.0). Both implementations are pinned by [docs/contracts/tools-manifest.txt](docs/contracts/tools-manifest.txt), which lists 460 tools, and the surface is enforced by parity tests in each language. Python implements the full set; Go implements all but a few routes that are tracked as accepted differences in [docs/contracts/tool-parity-baseline.txt](docs/contracts/tool-parity-baseline.txt). Coverage spans compute, block storage, Object Storage, networking, DNS, LKE, VPCs, managed databases, images, placement groups, tags, support, Longview, Managed, Monitor, account, and profile operations. The trust-and-safety layer (profiles, dry-run previews, two-stage writes, audit log) is complete in both languages, and the Python implementation is at full feature parity with Go.

## License

MIT
