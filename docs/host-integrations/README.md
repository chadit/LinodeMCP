# Host integrations

LinodeMCP runs as a stdio MCP server. Any MCP host can talk to it: there is no host-specific binary, and the profile system, write confirmations, and audit log all live inside the server itself. What changes per host is the *config glue*: how the host launches the server, where its config lives, and what convenience wrappers fit the host's workflow.

Three per-topic pages hold working command wrappers, each with a Claude Code section and a Claude Desktop section:

- [profiles.md](./profiles.md): a `/profile` slash command and shell aliases for switching profiles.
- [audit.md](./audit.md): `/audit` slash commands, ask-Claude patterns, and terminal queries.
- [two-stage.md](./two-stage.md): `/plan` and `/apply` wrappers and the conversation script for destructive calls.

They are not the only way to wire things up; they are a starting point that's been used and verified. On another host (Cline, Continue, etc.) the patterns transfer: the registration shape below is the same, and only the slash-command or alias mechanism differs.

## What the host needs to know

Three things, in order:

1. **Where the binary lives.** The host needs an absolute path to `linodemcp` (Go) or `python -m linodemcp` (Python). Put it on `$PATH` or hardcode the path in the host config.
2. **Where the config file lives.** Default is `~/.config/linodemcp/config.yml`. Override with `LINODEMCP_CONFIG_PATH` if you keep configs per-environment.
3. **Which profile is active.** The server reads the active profile from the config file at startup and applies it as a registration-time filter. Profile changes hot-reload. No restart needed.

The MCP host itself doesn't manage profiles. It just runs the server. Profile management is a separate CLI you invoke outside the host conversation:

```bash
linodemcp profile list                 # what's available
linodemcp profile show readonly-full   # what it lets the AI do
linodemcp profile use compute-admin    # switch
```

The host picks up the change on the next tool registration cycle (a few hundred ms).

## Registering the server

One registration per host, shown once here; the per-topic pages assume it is done. The examples use `/usr/local/bin/linodemcp`; substitute your build's absolute path (`.../LinodeMCP/go/bin/linodemcp` for the Go build, `.../LinodeMCP/python/.venv/bin/linodemcp` for the Python one). Pass the token via `LINODEMCP_LINODE_TOKEN`, or leave `env` empty and keep the token in the config file.

### Claude Code

The fastest way is the `claude mcp add` helper, which writes to your Claude Code settings:

```bash
claude mcp add linodemcp -- /usr/local/bin/linodemcp
```

Add `--scope user` to make it available across all your projects. For team sharing, drop a `.mcp.json` in the project root:

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "/usr/local/bin/linodemcp",
      "env": {
        "LINODEMCP_LINODE_TOKEN": "${LINODEMCP_LINODE_TOKEN}"
      }
    }
  }
}
```

Set the `LINODEMCP_LINODE_TOKEN` env var in your shell, and Claude Code picks it up. Restart Claude Code after registering; from here on, profile changes hot-reload and only the initial registration needs a host restart.

### Claude Desktop

Edit `claude_desktop_config.json`. On macOS the file is at `~/Library/Application Support/Claude/claude_desktop_config.json`; on Windows, `%APPDATA%\Claude\claude_desktop_config.json`. Add an entry under `mcpServers`:

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "/usr/local/bin/linodemcp",
      "args": [],
      "env": {
        "LINODEMCP_LINODE_TOKEN": "your-token-here"
      }
    }
  }
}
```

Quit Claude Desktop fully (Cmd+Q on macOS, not just close the window) and reopen it. The MCP server starts on launch. For a non-default config path, add `"LINODEMCP_CONFIG_PATH": "/path/to/config.yml"` to the `env` block.

### Gemini CLI

Add the same `mcpServers` entry to `~/.gemini/settings.json`. Gemini CLI uses `$VAR` syntax (no curly braces) for environment variable references:

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "/usr/local/bin/linodemcp",
      "env": {
        "LINODEMCP_LINODE_TOKEN": "$LINODEMCP_LINODE_TOKEN"
      }
    }
  }
}
```

### GitHub Copilot (VS Code)

Create a `.vscode/mcp.json` in your workspace. Note the top-level key is `servers`, not `mcpServers`:

```json
{
  "servers": {
    "linodemcp": {
      "command": "/usr/local/bin/linodemcp",
      "env": {
        "LINODEMCP_LINODE_TOKEN": "${input:linode-token}"
      }
    }
  }
}
```

VS Code prompts you for the token value on first use through the `${input:linode-token}` pattern and caches it for the session.

### Cursor / Windsurf

Both use the same `.mcp.json` format as Claude Code; drop the file from the [Claude Code](#claude-code) section in your project root.

## Security notes for the docs in this directory

The examples write nothing to your config except the active profile name (via `linodemcp profile use`) or user-defined profile entries (via `linodemcp profile clone`). No example exposes the Linode API token to the host UI. Tokens stay in environment variables or the config file with `0600` permissions.

Slash commands and shell wrappers in these examples never invoke `confirm: true` on a write tool. That decision belongs to the model and the user, not to a host-level wrapper.
