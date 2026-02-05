# LinodeMCP

An MCP (Model Context Protocol) server that gives AI assistants like Claude programmatic access to the Linode cloud platform API. Ships with both Go and Python implementations that share the same configuration format and tool interface.

## What It Does

LinodeMCP exposes Linode API operations as MCP tools. AI assistants can use these tools to query and manage your Linode infrastructure -- all through a standard protocol.

### Available Tools (43 total)

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

## Quick Start

### Prerequisites

**Go implementation:**
- Go 1.24+
- A Linode API token ([create one here](https://cloud.linode.com/profile/tokens))

**Python implementation:**
- Python 3.11+
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

```bash
cd go/

# Install dev tools
make install-tools

# Build and run
make build
./bin/linodemcp

# Or build + run in one step
make run
```

### Python

```bash
cd python/

# Install with dev dependencies
make install-dev

# Run the server
make run
```

### Claude Desktop Integration

Add this to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "/path/to/linodemcp/go/bin/linodemcp",
      "env": {
        "LINODEMCP_LINODE_TOKEN": "your-token-here"
      }
    }
  }
}
```

For the Python version:

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "python",
      "args": ["-m", "linodemcp"],
      "env": {
        "LINODEMCP_LINODE_TOKEN": "your-token-here"
      }
    }
  }
}
```

## Development

### Go

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

### Python

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

```
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

This project is in active development (v0.1.0). The foundation is complete with config management, API client, retry logic, and 43 tools covering compute, storage, networking, DNS, and security resources. Both read and write operations are fully implemented across Go and Python.

## License

MIT
