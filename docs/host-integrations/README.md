# Host integrations

LinodeMCP runs as a stdio MCP server. Any MCP host can talk to it: there is no host-specific binary, and the profile system, write confirmations, and audit log all live inside the server itself. What changes per host is the *config glue*: how the host launches the server, where its config lives, and what convenience wrappers fit the host's workflow.

This directory holds working examples per host. They are not the only way to wire things up; they are a starting point that's been used and verified.

## Layout

```text
docs/host-integrations/
├── README.md            # this file
├── claude-code/
│   └── commands/
│       ├── profile.md   # slash command for profile management
│       └── audit.md     # slash commands for audit queries
└── claude-desktop/
    └── commands/
        ├── profile.md   # shell wrappers for profiles (no native slash commands)
        └── audit.md     # jq-based audit queries + ask-Claude patterns
```

Each host directory is self-contained. Pick the one that matches your environment and read its `commands/profile.md` for the integration walk-through; `commands/audit.md` then adds the audit-log query shortcuts (depends on the server already being registered).

## What the host needs to know

Three things, in order:

1. **Where the binary lives.** The host needs an absolute path to `linodemcp` (Go) or `python -m linodemcp` (Python). Put it on `$PATH` or hardcode the path in the host config.
2. **Where the config file lives.** Default is `~/.config/linodemcp/config.yml`. Override with `LINODEMCP_CONFIG_PATH` if you keep configs per-environment.
3. **Which profile is active.** The server reads the active profile from the config file at startup and applies it as a registration-time filter. Profile changes hot-reload. No restart needed.

The MCP host itself doesn't manage profiles. It just runs the server. Profile management is a separate CLI you invoke outside the host conversation:

```bash
linodemcp profile list                 # what's available
linodemcp profile show readonly        # what it lets the AI do
linodemcp profile use compute-admin    # switch
```

The host picks up the change on the next tool registration cycle (a few hundred ms).

## Profile lifecycle in plain terms

The server holds three pieces of state that interact:

- **Built-in profiles** ship in the binary. Eight of them, covering read-only through full admin. Built-ins can be disabled via config but never deleted.
- **User-defined profiles** live in the config file under `profiles:`. They can shadow built-ins by name.
- **Active profile** is one name set under `active_profile:` in the config, resolved against built-ins plus user-defined at startup and on every config-file change.

A successful `linodemcp profile use <name>` writes the active name back to the config file atomically. The file watcher sees the rename and triggers `Server.ReloadProfile`, which diffs the old and new allowed tool sets and patches the MCP tool registry in place. The host's tool list updates without restart.

If the config file ends up malformed, the server keeps running with its previous profile and logs the failure. The user sees a stale tool list, not a crash.

## When to look at the per-host docs

- You want a `/profile` slash command in Claude Code: read `claude-code/commands/profile.md`.
- You want `/audit` slash commands in Claude Code: read `claude-code/commands/audit.md`.
- You're on Claude Desktop and want shell aliases or a wrapper script: read `claude-desktop/commands/profile.md`.
- You're on Claude Desktop and want audit-log queries from the terminal: read `claude-desktop/commands/audit.md`.
- You're on another host (Cline, Continue, etc.): these patterns transfer. The MCP server registration shape is the same; only the slash-command or alias mechanism differs.

## Security notes for the docs in this directory

The examples write nothing to your config except the active profile name (via `linodemcp profile use`) or user-defined profile entries (via `linodemcp profile clone`). No example exposes the Linode API token to the host UI. Tokens stay in environment variables or the config file with `0600` permissions.

Slash commands and shell wrappers in these examples never invoke `confirm: true` on a write tool. That decision belongs to the model and the user, not to a host-level wrapper.
