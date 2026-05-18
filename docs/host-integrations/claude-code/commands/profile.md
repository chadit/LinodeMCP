# Claude Code: `/profile` slash command

This guide wires LinodeMCP into Claude Code and adds a `/profile` slash command for managing the active profile.

## 1. Register the MCP server

The fastest way is the `claude mcp add` helper, which writes to your Claude Code settings:

```bash
claude mcp add linodemcp /usr/local/bin/linodemcp
```

If you prefer hand-editing, the equivalent config lives under your Claude Code settings (`~/.claude/settings.json` on macOS/Linux, `%APPDATA%\Claude\settings.json` on Windows):

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "/usr/local/bin/linodemcp",
      "args": []
    }
  }
}
```

The server reads `~/.config/linodemcp/config.yml` by default. Override the path with `LINODEMCP_CONFIG_PATH=/some/path/config.yml` in the `env` block if you keep configs per-environment.

Restart Claude Code after registering. From here on, profile changes hot-reload; only the initial registration needs a host restart.

## 2. Verify the server is up

In a Claude Code session:

```text
List the MCP servers you have access to. For each, list its tool count.
```

If `linodemcp` is listed and the tool count is non-zero, the server is up. If the tool count is zero, the active profile is filtering everything out: check `linodemcp profile show <active-name>` in a terminal.

## 3. Drop in the `/profile` slash command

Claude Code slash commands are markdown files under `~/.claude/commands/` (user-level) or `.claude/commands/` (project-level). Copy the block below into `~/.claude/commands/profile.md`:

````markdown
---
description: Manage LinodeMCP profiles from inside a Claude Code session.
allowed-tools: Bash
---

# `/profile`

Read `$ARGUMENTS` and run the matching `linodemcp profile` subcommand. The CLI
is read-only for `list` and `show`; the rest write the active config file
atomically.

If `$ARGUMENTS` is empty, run `linodemcp profile list` and stop there.

Otherwise, treat the first word as the subcommand and the rest as positional
arguments. Pass them through verbatim:

```bash
linodemcp profile $ARGUMENTS
```

Supported subcommands (built into the CLI, no extra wiring needed):

- `list` reports built-in and user-defined profiles plus the active one
- `show <name>` prints one profile's full details
- `use <name>` switches the active profile (atomic config write)
- `enable <name>` clears the disabled flag on a built-in profile
- `disable <name>` sets the disabled flag on a built-in profile
- `clone <src> <dst>` copies any profile into a new user-defined entry
- `delete <name>` removes a user-defined profile

After a successful mutation, mention to the user that the tool list will refresh on the next MCP registration cycle (typically under a second). The user does not need to restart Claude Code.

If the CLI exits non-zero, surface stderr verbatim. Do not retry. Do not
guess at corrective arguments.
````

The `allowed-tools: Bash` line means Claude Code can execute the `linodemcp profile` call without prompting for tool approval each time. If you'd rather approve every invocation, drop that line.

## 4. Try it

Open a Claude Code session and run:

```text
/profile list
/profile show readonly
/profile use readonly
```

The first two should print the CLI's stdout inline. The third writes the active profile back to your config file. Within about a second the MCP tool list shrinks to the read subset; you can confirm by asking Claude what tools it has access to.

## Gotchas

- **Slash command not found.** Claude Code reloads slash commands on session start, not on file save. Restart Claude Code after adding `profile.md`.
- **`linodemcp: command not found`.** The shell that runs the slash command may have a different `$PATH` than the one you used to install. Use the absolute path in the `Bash` invocation, or symlink the binary into `/usr/local/bin`.
- **Config file rejected by `WriteAtomic`.** Mutators round-trip the rewritten file through the config loader before renaming. If the rejection persists, run `linodemcp profile list` from a terminal to see the same validation error: the CLI prints it; the slash command may not surface it cleanly.
- **No hot-reload after `profile use`.** The file watcher reacts to renames. If your editor saves in-place rather than atomically, the watcher misses the change. Run `linodemcp profile use <name>` (which writes atomically) rather than editing the file by hand.

## Why a slash command rather than letting the model call a tool?

Profile management lives outside the MCP tool surface on purpose. A profile change rewrites the config file, which is a higher-trust operation than any individual tool call. Keeping it in a slash command means: the model can suggest a profile switch, the user runs it explicitly, and the change is captured in shell history rather than a tool-call log. The MCP `linode_profile_*` builder tools (Phase 8) handle the in-conversation builder workflow; they create *user-defined* profiles in a draft state and require explicit `_save` with `confirm: true` to write. Switching the active profile stays a CLI operation.
