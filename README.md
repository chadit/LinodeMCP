# LinodeMCP

An MCP (Model Context Protocol) server that gives AI assistants like Claude|Gemini|Copilot programmatic access to the Linode cloud platform API. Ships with both Go and Python implementations that share the same configuration format and tool interface.

## What It Does

LinodeMCP exposes Linode API operations as MCP tools. AI assistants can use these tools to query and manage your Linode infrastructure -- all through a standard protocol.

### Available Tools (63 total)

**Core Tools:**

| Tool | Description |
|------|-------------|
| `hello` | Smoke test -- returns a greeting to confirm the server is running |
| `version` | Returns build info: version, git commit, platform, feature flags |

**Account & Profile:**

| Tool | Description |
|------|-------------|
| `linode_profile` | Fetches your Linode account profile (username, email, 2FA status) |
| `linode_account` | Fetches account info (balance, billing, capabilities) |

**Compute:**

| Tool | Description |
|------|-------------|
| `linode_instances_list` | Lists your Linode instances with optional status filtering |
| `linode_instance_get` | Gets detailed info about a specific instance by ID |
| `linode_instance_create` | Creates a new Linode instance (confirm required) |
| `linode_instance_delete` | Deletes a Linode instance (confirm required) |
| `linode_instance_resize` | Resizes an instance to a different plan (confirm required) |
| `linode_instance_boot` | Boots a stopped instance |
| `linode_instance_reboot` | Reboots a running instance |
| `linode_instance_shutdown` | Shuts down a running instance |
| `linode_regions_list` | Lists available regions with country/capability filtering |
| `linode_types_list` | Lists instance types (plans) with class filtering |
| `linode_images_list` | Lists images with public/deprecated filtering |
| `linode_stackscripts_list` | Lists StackScripts with is_public/mine/label filtering |

**Storage:**

| Tool | Description |
|------|-------------|
| `linode_volumes_list` | Lists block storage volumes with region/label filtering |
| `linode_volume_create` | Creates a new block storage volume (confirm required) |
| `linode_volume_attach` | Attaches a volume to an instance |
| `linode_volume_detach` | Detaches a volume from an instance |
| `linode_volume_resize` | Resizes a volume, expand only (confirm required) |
| `linode_volume_delete` | Deletes a block storage volume (confirm required) |

**Object Storage:**

| Tool | Description |
|------|-------------|
| `linode_object_storage_buckets_list` | Lists all Object Storage buckets |
| `linode_object_storage_bucket_get` | Gets detailed info about a specific bucket |
| `linode_object_storage_bucket_contents` | Lists objects in a bucket with prefix/marker/delimiter filtering |
| `linode_object_storage_bucket_create` | Creates a new Object Storage bucket (confirm required) |
| `linode_object_storage_bucket_delete` | Deletes an Object Storage bucket (confirm required) |
| `linode_object_storage_bucket_access_get` | Gets bucket ACL and CORS settings |
| `linode_object_storage_bucket_access_update` | Updates bucket ACL and CORS settings (confirm required) |
| `linode_object_storage_clusters_list` | Lists Object Storage cluster endpoints |
| `linode_object_storage_type_list` | Lists Object Storage pricing types |
| `linode_object_storage_keys_list` | Lists Object Storage access keys |
| `linode_object_storage_key_get` | Gets detailed info about a specific access key |
| `linode_object_storage_key_create` | Creates a new access key (confirm required, secret shown once) |
| `linode_object_storage_key_update` | Updates access key label or bucket permissions (confirm required) |
| `linode_object_storage_key_delete` | Revokes an access key permanently (confirm required) |
| `linode_object_storage_transfer` | Gets Object Storage transfer usage |
| `linode_object_storage_presigned_url` | Generates a presigned URL for object download or upload |
| `linode_object_storage_object_acl_get` | Gets the ACL for a specific object |
| `linode_object_storage_object_acl_update` | Updates an object's ACL (confirm required) |
| `linode_object_storage_ssl_get` | Checks if a bucket has an SSL certificate |
| `linode_object_storage_ssl_delete` | Removes a bucket's SSL certificate (confirm required) |

**Networking:**

| Tool | Description |
|------|-------------|
| `linode_firewalls_list` | Lists Cloud Firewalls with status/label filtering |
| `linode_firewall_create` | Creates a new Cloud Firewall |
| `linode_firewall_update` | Updates firewall rules and settings |
| `linode_firewall_delete` | Deletes a Cloud Firewall (confirm required) |
| `linode_nodebalancers_list` | Lists NodeBalancers with region/label filtering |
| `linode_nodebalancer_get` | Gets detailed info about a specific NodeBalancer by ID |
| `linode_nodebalancer_create` | Creates a new NodeBalancer (confirm required) |
| `linode_nodebalancer_update` | Updates NodeBalancer settings (confirm required) |
| `linode_nodebalancer_delete` | Deletes a NodeBalancer (confirm required) |

**DNS:**

| Tool | Description |
|------|-------------|
| `linode_domains_list` | Lists DNS domains |
| `linode_domain_get` | Gets detailed info about a specific domain by ID |
| `linode_domain_create` | Creates a new DNS domain |
| `linode_domain_update` | Updates domain settings |
| `linode_domain_delete` | Deletes a DNS domain and all its records |
| `linode_domain_records_list` | Lists domain records with type/name filtering |
| `linode_domain_record_create` | Creates a new DNS record |
| `linode_domain_record_update` | Updates a DNS record |
| `linode_domain_record_delete` | Deletes a DNS record |

**Security:**

| Tool | Description |
|------|-------------|
| `linode_sshkeys_list` | Lists SSH keys with label filtering |
| `linode_sshkey_create` | Adds a new SSH key to your profile |
| `linode_sshkey_delete` | Removes an SSH key from your profile |

**Safety Note:** Tools marked with "(confirm required)" are destructive or incur billing charges. These operations require `confirm: true` to execute.

### Multi-Environment Support

Configure multiple Linode environments (production, staging, dev) in a single config file. Tools accept an optional `environment` parameter to target a specific one, falling back to `default` when omitted.

## Installation

### Prerequisites

**Go implementation:**

- Go 1.26+
- A Linode API token ([create one here](https://cloud.linode.com/profile/tokens))

**Python implementation:**

- Python 3.14+
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

metrics:
  enabled: true
  port: 9090
  path: "/metrics"

tracing:
  enabled: false
  exporter: "otlp"
  endpoint: "localhost:4317"
  sampleRate: 1.0

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

### Go

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

### Python

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
- **Stdio transport**: Communicates over stdin/stdout per the MCP spec. This is what Claude Desktop and similar clients expect.
- **Retry with backoff**: The Linode API client wraps all calls with configurable retry logic, exponential backoff, and circuit breaker protection.
- **Path validation**: Config file loading validates paths against a list of dangerous system directories and restricts access to the user's home, working directory, and temp paths.
- **Config caching**: Loaded configs are cached with mtime-based invalidation, so repeated loads don't re-read from disk unnecessarily.

### Dependencies

**Go:**

- [mcp-go](https://github.com/mark3labs/mcp-go) - MCP protocol implementation
- [yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) - YAML config parsing
- [testify](https://github.com/stretchr/testify) - Test assertions

**Python:**

- [mcp](https://pypi.org/project/mcp/) - MCP protocol SDK
- [httpx](https://www.python-httpx.org/) - Async HTTP client for Linode API
- [pyyaml](https://pyyaml.org/) - YAML config parsing
- [structlog](https://www.structlog.org/) - Structured logging

## Status

This project is in active development (v0.1.0). The foundation is complete with config management, API client, retry logic, and 63 tools covering compute, block storage, object storage, networking, DNS, and security resources. Both read and write operations are fully implemented across Go and Python.

## License

MIT
