# Profile management per host

How to drive LinodeMCP [profiles](../profiles.md) from inside each MCP host: a `/profile` slash command for Claude Code, and shell aliases for Claude Desktop, which has no user-defined slash command system.

Assumes the server is already registered with your host per the [directory README](./README.md).

## Verify the server is up

In a session with your host:

```text
List the MCP servers you have access to. For each, list its tool count.
```

If `linodemcp` is listed and the tool count is non-zero, the server is up. If the tool count is zero, the active profile is filtering everything out. Check from a terminal:

```bash
linodemcp profile show "$(yq '.active_profile' ~/.config/linodemcp/config.yml)"
```

## Claude Code: `/profile` slash command

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

### Try it

Open a Claude Code session and run:

```text
/profile list
/profile show readonly-full
/profile use readonly-full
```

The first two should print the CLI's stdout inline. The third writes the active profile back to your config file. Within about a second the MCP tool list shrinks to the read subset; you can confirm by asking Claude what tools it has access to.

### Claude Code gotchas

- **Slash command not found.** Claude Code reloads slash commands on session start, not on file save. Restart Claude Code after adding `profile.md`.
- **`linodemcp: command not found`.** The shell that runs the slash command may have a different `$PATH` than the one you used to install. Use the absolute path in the `Bash` invocation, or symlink the binary into `/usr/local/bin`.
- **Config file rejected by `WriteAtomic`.** Mutators round-trip the rewritten file through the config loader before renaming. If the rejection persists, run `linodemcp profile list` from a terminal to see the same validation error: the CLI prints it; the slash command may not surface it cleanly.
- **No hot-reload after `profile use`.** The file watcher reacts to renames. If your editor saves in-place rather than atomically, the watcher misses the change. Run `linodemcp profile use <name>` (which writes atomically) rather than editing the file by hand.

## Claude Desktop: shell aliases

Drop these into `~/.zshrc` or `~/.bashrc`. They give you the same ergonomics as the Claude Code slash command, just from your terminal:

```bash
# LinodeMCP profile shortcuts
alias lmp='linodemcp profile'
alias lmpl='linodemcp profile list'
alias lmps='linodemcp profile show'
alias lmpu='linodemcp profile use'
```

Reload your shell (`source ~/.zshrc`). Now:

```bash
lmpl                       # list profiles
lmps readonly-full         # show one profile's details
lmpu compute-admin         # switch the active profile
```

The MCP tool list inside Claude Desktop refreshes within about a second after `lmpu`. You don't need to switch focus back to the app first; the watcher fires on the config file rename.

### Optional: status-line helper

If you keep multiple Linode environments and switch often, add a one-liner that prints the active profile name. Handy for shell prompts:

```bash
lmp_active() {
  linodemcp profile list 2>/dev/null \
    | awk '/^\* / { sub(/^\* /, ""); print $1; exit }'
}
```

Then in your prompt config:

```bash
PS1='[lmp:$(lmp_active)] %~ %# '
```

The leading `*` is what `profile list` prints next to the currently-active profile on the matching line.

### Claude Desktop gotchas

- **Active profile didn't switch.** Claude Desktop caches the tool list between restarts in some versions. If the new tool set isn't visible after `lmpu`, fully quit (Cmd+Q) and relaunch. The CLI itself succeeded; the host just isn't re-querying yet.
- **Profile-changed banner missing.** Claude Desktop has no built-in indicator for MCP tool-list changes. The user-visible signal is the tool list itself; ask the model to list its tools when you're verifying a switch.
- **Multiple Linode tokens.** One environment per `environments:` block in the config. Each `mcpServers` entry talks to one LinodeMCP process; that process can serve any environment your config defines. Tools take an `environment:` parameter at call time.
- **Symlinks in the config path.** `LINODEMCP_CONFIG_PATH` is followed once at server startup, but the file watcher tracks the resolved target. If you replace the symlink to switch environments, the watcher continues watching the old target; restart Claude Desktop to repoint.

## Why profile switching stays outside the conversation

Profile management lives outside the MCP tool surface on purpose. A profile change rewrites the config file, which is a higher-trust operation than any individual tool call. Keeping it in a slash command or shell alias means: the model can suggest a profile switch, the user runs it explicitly, and the change is captured in shell history rather than a tool-call log.

Claude Desktop could in principle ship profile management as MCP "prompts" (text templates rendered as `/server-name/prompt-name`), but writing config from a prompt handler crosses the same trust line as writing config from a tool handler: the user gets a one-click escalation surface inside the AI conversation. Shell aliases keep the boundary clear.

The MCP `linode_profile_*` builder tools handle the in-conversation builder workflow; they create *user-defined* profiles in a draft state and require explicit `_save` with `confirm: true` to write. Switching the active profile stays a CLI operation.
