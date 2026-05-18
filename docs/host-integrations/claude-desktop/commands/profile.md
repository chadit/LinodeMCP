# Claude Desktop: profile management

Claude Desktop has no user-defined slash command system, so profile management lives in your shell rather than inside the Claude UI. This guide registers LinodeMCP with Claude Desktop, then sets up shell aliases that make profile operations one-line ergonomics.

## 1. Register the MCP server

Edit `claude_desktop_config.json`. On macOS the file is at:

```text
~/Library/Application Support/Claude/claude_desktop_config.json
```

On Windows:

```text
%APPDATA%\Claude\claude_desktop_config.json
```

Add an entry under `mcpServers`:

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "/usr/local/bin/linodemcp",
      "args": [],
      "env": {}
    }
  }
}
```

Quit Claude Desktop (Cmd+Q on macOS, not just close the window) and reopen it. The MCP server starts on launch. Profile changes hot-reload from here on; only the initial registration needs a host restart.

If you want a non-default config path, set it via env:

```json
{
  "mcpServers": {
    "linodemcp": {
      "command": "/usr/local/bin/linodemcp",
      "args": [],
      "env": {
        "LINODEMCP_CONFIG_PATH": "/Users/you/work/linodemcp.yml"
      }
    }
  }
}
```

The active profile is whatever the config file's `active_profile:` key says, both at startup and after every config change. Use the CLI (`linodemcp profile use <name>`) to switch.

## 2. Verify the server is up

In a Claude Desktop session:

```text
List the MCP servers you have access to. For each, list its tool count.
```

If `linodemcp` shows up with a non-zero tool count, the registration worked. Zero tools means the active profile is filtering everything out; check from a terminal:

```bash
linodemcp profile show "$(yq '.active_profile' ~/.config/linodemcp/config.yml)"
```

## 3. Add shell aliases

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
lmps readonly              # show one profile's details
lmpu compute-admin         # switch the active profile
```

The MCP tool list inside Claude Desktop refreshes within about a second after `lmpu`. You don't need to switch focus back to the app first; the watcher fires on the config file rename.

## 4. Optional: status-line helper

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

## Gotchas

- **Active profile didn't switch.** Claude Desktop caches the tool list between restarts in some versions. If the new tool set isn't visible after `lmpu`, fully quit (Cmd+Q) and relaunch. The CLI itself succeeded; the host just isn't re-querying yet.
- **Profile-changed banner missing.** Claude Desktop has no built-in indicator for MCP tool-list changes. The user-visible signal is the tool list itself; ask the model to list its tools when you're verifying a switch.
- **Multiple Linode tokens.** One environment per `environments:` block in the config. Each `mcpServers` entry talks to one LinodeMCP process; that process can serve any environment your config defines. Tools take an `environment:` parameter at call time.
- **Symlinks in the config path.** `LINODEMCP_CONFIG_PATH` is followed once at server startup, but the file watcher tracks the resolved target. If you replace the symlink to switch environments, the watcher continues watching the old target; restart Claude Desktop to repoint.

## Why no in-app slash command?

Claude Desktop doesn't currently expose a user-defined slash command system. The equivalent functionality in MCP is "prompts": text templates the server exposes that the host renders as `/server-name/prompt-name`. We could ship profile management as MCP prompts, but writing config from a prompt handler crosses the same trust line as writing config from a tool handler: the user gets a one-click escalation surface inside the AI conversation. Shell aliases keep the boundary clear. The model can suggest a profile switch; the user runs it.
