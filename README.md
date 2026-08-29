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

The docs index at [docs/README.md](docs/README.md) maps every page: profiles, dry-run and safety, two-stage writes, auditing, host integrations, releases, and the contributor pages, plus the machine-read gate files `make check` consumes. Agents can start from [llms.txt](llms.txt).

## Installation

### Prerequisites

**Go implementation:**

- Go 1.26+
- A Linode API token ([create one here](https://cloud.linode.com/profile/tokens))

**Python implementation:**

- Python 3.14.6+
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
      token: "your-linode-api-token"
```

`apiUrl` carries the API version segment, and a handful of tools answer only
under `/v4beta`. Those tools swap the trailing `/v4` for themselves, per call,
so nothing else moves; the tools are listed in
[`docs/contracts/api-surfaces.txt`](docs/contracts/api-surfaces.txt) and each
one leads its description with `[v4beta]`. A base that does not end in `/v4`,
such as a proxy or a mock, is used exactly as written, so pointing `apiUrl`
somewhere else keeps every call there and the server says so once at startup.

Token values are literal: the config loader performs no `${VAR}` expansion.
Write the token into the file and keep the file's permissions tight, or
omit it and set `LINODEMCP_LINODE_TOKEN` in the environment, which
overrides the `default` environment's token.

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

Each MCP client needs to know where the LinodeMCP binary lives and how to pass your Linode API token. One worked example follows; the registration blocks for Claude Code, Gemini CLI, GitHub Copilot, and Cursor/Windsurf live in [docs/host-integrations/](docs/host-integrations/README.md), along with slash-command and shell wrappers per host.

### Claude Desktop

Add this to your Claude Desktop config on macOS at `~/Library/Application Support/Claude/claude_desktop_config.json`:

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

For the Python build, point `command` at `/absolute/path/to/LinodeMCP/python/.venv/bin/linodemcp` instead.

### Docker / Podman

Running LinodeMCP in a container avoids installing Go or Python locally. MCP uses stdio transport, so the container needs `-i` (keep stdin open) and `-e` to forward your API token:

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

The example uses a locally built image; substitute `ghcr.io/chadit/linodemcp:latest` (or a pinned version tag) to use the released image, and swap `"docker"` for `"podman"` in the command field for Podman. To use a config file instead of environment variables, add a `-v ~/.config/linodemcp:/home/linodemcp/.config/linodemcp:ro` mount to `args`.

### Context Forge (IBM MCP Gateway)

[Context Forge](https://github.com/IBM/mcp-context-forge) is an MCP gateway that acts as a central registry for MCP servers: register LinodeMCP once with the gateway and all your clients connect through it. Bridge the container's stdio to HTTP with `mcpgateway.translate`, register the bridged endpoint, and point clients at the gateway via `mcpgateway.wrapper`. The [Context Forge docs](https://ibm.github.io/mcp-context-forge/) carry the full setup guide.

## Profiles

Profiles control which tools the AI client can see. The server filters the tool list at registration time, so the model literally cannot invoke anything outside the active profile. Nine built-ins ship with the binary:

- `default` and `readonly-full`: read-only across every category. Safe default.
- `compute-admin`, `network-admin`, `kubernetes-admin`, `storage-admin`, `iam-admin`: read everywhere plus write + destroy on the named category.
- `full-access` and `emergency`: ship disabled. Enable them when you genuinely need them, then disable again.

Switch profiles via the CLI (`linodemcp profile list` / `show` / `use`); mutators write the config file atomically and the server hot-reloads without a restart. User-defined profiles live under `profiles:` in your config, and the AI can help compose them via the `linode_profile_*` builder tools; the user activates the saved profile separately.

A profile is not Linode IAM. The profile is local: it picks which tools this server offers the AI, and it can only take reach away. [Linode IAM](https://techdocs.akamai.com/cloud-computing/docs/identity-and-access-cm) is role-based access control the API runs on Akamai's side, per calling user, and it is what actually decides whether a call succeeds. A request has to clear the active profile, the token's OAuth scopes, and the user's IAM roles, so effective access is the intersection of all three. The `iam-admin` built-in manages the second system through this server; it deliberately leaves out the deprecated per-user grants tools, since Akamai warns against running grants and IAM on one account.

For the full reference (schema, capability tags, [profiles versus Linode IAM](docs/profiles.md#profiles-are-not-linode-iam) with setup for both, builder workflow, token-scope validation, security model) and copy-paste recipes, see [docs/profiles.md](docs/profiles.md). For host-specific wiring, see [docs/host-integrations/](docs/host-integrations/README.md).

## Dry-run & safety

Every mutating tool can be previewed before it runs, and destructive calls are gated so a resource can't be deleted or replaced without first previewing it (or explicitly opting out):

- **Dry-run**: pass `dry_run: true` to any write/destroy/admin tool to get back `would_execute` + `current_state` without mutating anything.
- **Bypass-confirm**: a `CapDestroy` call must either set `confirmed_dry_run: true` (it previewed first) or `confirm_bypass_dry_run: true` (explicitly skip the preview) alongside `confirm: true`, or it's rejected with guidance.
- **Pre-check**: `linode_profile_can_run` reports which calls in a planned sequence the active profile would permit, so the model can bail before partial execution.
- **Yolo**: a profile with `allow_yolo: true` (only the break-glass `emergency` built-in) lets `yolo: true` skip both the preview gate and confirm.

Each call's safety path is recorded in the audit log's `mode` field. Full reference: [docs/dry-run.md](docs/dry-run.md).

## Two-stage writes

A dry-run shows what a destructive call would do, but nothing ties that preview to the call you run next, so the resource can change in between. Two-stage writes close that gap: pass `mode: "plan"` to a delete tool to get back a `plan_id` plus the current state; pass `mode: "apply"` with that `plan_id` to run it. Before applying, the server re-reads the resource and refuses if it drifted, expired, or was already used.

Full reference, including drift refusals and recovery: [docs/two-stage-writes.md](docs/two-stage-writes.md).

## Auditing

Every tool call is recorded as a structured audit event, with sensitive values redacted before write. The default JSONL sink is always on; an opt-in SQLite sink dual-writes for fast indexed queries. The log lives at `/var/log/linodemcp/audit.log` for a system-service install and `$XDG_STATE_HOME/linodemcp/audit.log` (default `~/.local/state/linodemcp/audit.log`) otherwise. Five `CapMeta` MCP tools and the matching `linodemcp audit` CLI verbs query it from any profile.

Full reference (event schema, redaction model, query tools, sinks, retention, report grammar): [docs/audit.md](docs/audit.md). Host wiring: [docs/host-integrations/audit.md](docs/host-integrations/audit.md).

## Development

Each implementation carries its own Makefile; `make help` in `go/` or `python/` lists every target (build, test, lint, format, coverage). The repo root's `make check` runs both languages' suites plus every cross-language gate; it is the whole pre-push bar. See [docs/gates.md](docs/gates.md) for what each gate checks and [docs/parity.md](docs/parity.md) for the cross-language workflow.

### Key Design Decisions

- **Dual implementation**: Go for performance and single-binary deployment, Python for quick prototyping and the MCP Python ecosystem. Both share the same config format.
- **Proto contract**: The `proto/` directory is the single source of truth for both tool input schemas and tool output messages in both languages. `buf` generates the Go and Python types and the MCP input JSON Schema from those `.proto` files, so the two implementations cannot drift by construction. `make check` gates keep it honest, backed by a cross-language conformance corpus that feeds shared fixtures through both languages and asserts byte-identical output.
- **Stdio transport**: Communicates over stdin/stdout per the MCP spec. This is what Claude Desktop and similar clients expect.
- **Retry with backoff**: The Linode API client wraps all calls with configurable retry logic, exponential backoff, and circuit breaker protection.
- **Path validation**: Config file loading validates paths against a list of dangerous system directories and restricts access to the user's home, working directory, and temp paths.
- **Config caching**: Loaded configs are cached with mtime-based invalidation, so repeated loads don't re-read from disk unnecessarily.

## Status

This project is in active development (v0.2.0). Both implementations are pinned by [docs/contracts/tools-manifest.txt](docs/contracts/tools-manifest.txt), which lists 520 tools, and the surface is enforced by parity tests in each language; both languages serve the full set. Coverage spans compute, block storage, Object Storage, networking, DNS, LKE, VPCs, managed databases, images, placement groups, tags, support, Longview, Managed, Monitor, account, and profile operations. The trust-and-safety layer (profiles, dry-run previews, two-stage writes, audit log) is complete in both languages.

## License

MIT
