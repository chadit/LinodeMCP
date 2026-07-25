# CLI and server modes

Each LinodeMCP binary is two things: an MCP stdio server for AI hosts, and a
CLI for humans in a shell. Which one you get depends only on the arguments.
Both implementations dispatch the same way (Go in `go/cmd/linodemcp/main.go`,
Python in `main()` via `_CLI_SUBCOMMANDS`), so everything below applies to
either binary.

## Server mode

Bare invocation, or the explicit alias:

```bash
linodemcp            # starts the MCP stdio server
linodemcp serve      # same thing; keeps existing host configs working
```

In server mode, stdout belongs to the MCP protocol: it carries JSON-RPC and
nothing else. All logging goes to stderr in both languages. If you ever see a
log line on stdout in server mode, that's a bug; hosts parse stdout and a
stray line corrupts the channel.

The server reads its config from `~/.config/linodemcp/config.yml` (override
the path with `LINODEMCP_CONFIG_PATH`) and applies the active
[profile](./profiles.md) as a registration-time tool filter. Host-specific
wiring lives in [host-integrations/](./host-integrations/README.md).

## CLI mode

Any of the recognized verbs runs a command and exits without starting the
server:

| Verb | What it does |
| --- | --- |
| `profile` | List, show, switch, enable/disable, clone, and delete profiles. Full reference in [profiles.md](./profiles.md). |
| `call` | Invoke a single tool from the shell. The tool name is validated against the catalog before anything runs. |
| `tools` | Inspect the tool surface: what exists, with what capability. |
| `audit` | Query the [audit log](./audit-log.md) from the shell, no MCP host needed. |
| `tui` | Interactive terminal UI over the same surface. |
| `version` | Build and version information. |

The CLI verbs don't require a config file: when none exists they fall back to
built-in defaults (`loadConfigOrDefault` on the Go side), with the
`LINODEMCP_*` environment overrides applied on top. Supplying
`LINODEMCP_LINODE_API_URL` and `LINODEMCP_LINODE_TOKEN` is therefore enough to
run API-backed calls with nothing on disk, which is the shape CI runners want.
`linodemcp version` and `linodemcp tools` are safe first commands on a fresh
install either way.

### Paging through a list

List tools whose API route paginates accept `page` and `page_size`:

```bash
linodemcp call linode_instance_list --arg page=2 --arg page_size=50
```

`page` starts at 1 and `page_size` runs from 25 through 500, the bounds the
Linode API applies to every paginated collection. Omit either one and the
API's own default applies rather than a value the CLI invents. Both are
rejected before any request when they are not integers or fall outside the
range, so a typo fails locally instead of returning a surprising page.

Not every list tool takes them yet;
[contracts/pagination-baseline.txt](./contracts/pagination-baseline.txt) is
the current list of the ones that don't, and `linodemcp tools` shows the real
input schema for any tool.

## Why profile switching is CLI-only

Switching the active profile changes what the AI is allowed to do, so it's
deliberately not an MCP tool: the model can suggest a switch, but a human runs
it in a shell, and shell history is the audit trail. The in-conversation
builder (`linode_profile_draft_*`) can compose and save profile definitions,
but activation always goes through `linodemcp profile use`. The reasoning is
laid out in [profiles.md](./profiles.md).

## Related

- [Host integrations](./host-integrations/README.md): registering the server
  binary with Claude Code, Claude Desktop, and other MCP hosts.
- [Profiles](./profiles.md): the permission model both modes enforce.
- [Audit log](./audit-log.md): what the `audit` verb queries.
