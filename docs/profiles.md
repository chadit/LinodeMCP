# Profiles

Profiles are LinodeMCP's permission model. A profile names a set of tools the connected AI client can see and call. The server filters tools at registration time, so the AI literally cannot invoke anything outside the active profile. Profiles also carry environment restrictions, token-scope requirements, and the `allow_yolo` opt-in for the dry-run bypass.

This doc covers the full surface: built-in catalog, config schema, capability tags, CLI commands, in-conversation builder, token-scope validation, hot-reload, and the security model. For copy-paste integration with specific MCP hosts, see [host-integrations/](./host-integrations/README.md). For recipe-style examples, see [profile-recipes.md](./profile-recipes.md).

## Why profiles exist

A general-purpose MCP server is dangerous by default. If the AI can call any tool, a misread instruction or a prompt injection can rebuild your production cluster. Profiles narrow what the AI sees down to what you actually want it to do *right now*, with a fast way to switch postures.

You pick a profile at startup. The model only sees tools that profile permits. If a tool isn't registered, the AI cannot call it. No bypass exists at the MCP layer.

## Capability tags

Every tool the server ships carries a capability tag. The tag is what profiles match against.

| Tag | Meaning |
| --- | --- |
| `CapRead` | GET endpoints, no state change |
| `CapWrite` | POST/PUT operations that create or update resources |
| `CapDestroy` | DELETE endpoints and explicitly destructive POSTs (delete instance, rebuild, password reset) |
| `CapAdmin` | Account-level mutations (payments, user management). No tool carries this today |
| `CapMeta` | Tools that touch local config or session state, never the Linode API (profile builder, `hello`, `version`) |

`CapMeta` tools bypass every profile filter. The builder (`linode_profile_*` tools) and the smoke-test pair (`hello`, `version`) are always visible regardless of which profile is active. This is deliberate: the builder needs to work under the read-only default so users can compose their first elevated profile from inside a conversation.

The capability-and-confirm invariant is enforced in tests: any `CapWrite` or `CapDestroy` tool MUST declare `confirm` in its required-parameter list, and any `CapRead` or `CapMeta` tool MUST NOT. The invariant fires at server startup so a mistagged tool fails the build, not at request time.

## Built-in catalog

Eight profiles ship in the binary. They cover the common postures.

| Name | Reach | Default disabled? |
| --- | --- | --- |
| `default` | Read + Meta across every category | No (this is the literal default) |
| `readonly-full` | Same as default; explicit name for clarity in scripts | No |
| `compute-admin` | Read everywhere, plus Write + Destroy on compute (instances, regions, types, images, stackscripts, instance backups/disks/IPs/actions) | No |
| `network-admin` | Read everywhere, plus Write + Destroy on networking (firewalls, NodeBalancers, VLANs, IPv6 ranges) | No |
| `kubernetes-admin` | Read everywhere, plus Write + Destroy on LKE (clusters, node pools, ACL) | No |
| `storage-admin` | Read everywhere, plus Write + Destroy on volumes, object storage, and instance backups | No |
| `full-access` | Read + Write + Destroy across every category | **Yes** |
| `emergency` | Full access plus `allow_yolo: true` (skips dry-run gates) | **Yes** |

Two of them ship disabled: `full-access` and `emergency`. To use them, set `disabled: false` under `profiles_builtin_overrides` in your config. The shipped-disabled state exists so a default install can't accidentally hand a full admin surface to a fresh AI client.

Each built-in computes its `required_token_scopes` from the union of its tool list's scope tags at startup. There's no hand-maintained scope list per built-in; the tool tags drive it.

## Config schema

Profiles live in `~/.config/linodemcp/config.yml` (or `.json`).

```yaml
# Which profile to activate at server start. Falls back to "default"
# when this key is missing or empty.
active_profile: compute-admin

# Override toggles for built-ins. Today only `disabled` is settable.
profiles_builtin_overrides:
  full-access:
    disabled: false
  emergency:
    disabled: true   # noop; emergency is already disabled by default

# Your own profile entries. Keys are profile names. Wildcards in
# allowed_tools/denied_tools expand against the live tool catalog
# at server start.
profiles:
  dns-admin-readall:
    description: "DNS write access plus read-everything"
    allowed_tools:
      - "linode_domain_*"
      - "linode_domain_record_*"
    denied_tools: []
    allowed_environments: ["prod", "staging"]
    required_token_scopes:
      - "domains:read_write"
    allow_yolo: false
```

### Field reference

- **`active_profile`** (string, optional): name of the profile to activate at server start. Defaults to `default`.
- **`profiles_builtin_overrides`** (map): toggles applied to built-ins. Currently `disabled: bool` is the only knob.
- **`profiles`** (map): user-defined entries. Names are case-sensitive and shadow built-ins by name.
- Per user-defined profile:
  - `description` (string): one-line summary surfaced by `profile show`.
  - `allowed_tools` ([]string): list of literal tool names and wildcards. `*` is the only glob character. Empty list means "no tools."
  - `denied_tools` ([]string): subtracted from `allowed_tools` after expansion. An explicit deny always beats a wildcard allow.
  - `allowed_environments` ([]string): restricts which Linode environments tools may target. Empty or `["*"]` means "any environment."
  - `required_token_scopes` ([]string): Linode OAuth/PAT scopes the profile assumes. Validated against the active token at server start; missing scopes fail to load, extra scopes warn.
  - `allow_yolo` (bool): opts the profile into the yolo execution path (skips dry-run gates for the two-stage-writes flow). Default `false`.

User-defined entries shadow built-ins by name. If you name a profile `compute-admin` in your config, it replaces the built-in for resolution purposes.

## CLI commands

Profile management lives in the `linodemcp profile` subcommand tree. These are CLI operations, not MCP tools. Switching the active profile is a higher-trust action than any single tool call, and keeping it in shell history is the audit trail.

```text
linodemcp profile list                 # all profiles + which is active
linodemcp profile show <name>          # full details for one profile
linodemcp profile use <name>           # switch the active profile
linodemcp profile enable <name>        # clear `disabled` on a built-in
linodemcp profile disable <name>       # set `disabled` on a built-in
linodemcp profile clone <src> <dst>    # copy any profile into a new user-defined entry
linodemcp profile delete <name>        # remove a user-defined profile
```

All mutators write the config file atomically (`config.WriteAtomic` in Go, `write_atomic` in Python). The active profile cannot be disabled or deleted; switch first, then mutate.

After a successful `profile use`, the server's file watcher sees the rename, calls `Server.ReloadProfile` (Go) or `Server.reload_profile` (Python), and the MCP tool list updates without a server restart. The change is visible to the model on the next `tools/list` request.

Comments and key ordering in your config file are **not** preserved through the rewrite. The atomic-write path round-trips through the schema validator before renaming, which strips formatting. Use a separate file (managed via your editor or version control) if you need to keep notes alongside profile entries.

## Builder workflow (`linode_profile_*` MCP tools)

The builder lets the model help you compose a profile in conversation. The tools all carry `CapMeta` so they're always available, even under `default`. The draft state lives in server-process memory only; drafts do not persist across restarts.

| Tool | Purpose |
| --- | --- |
| `linode_profile_list_tools` | Enumerate the full registerable tool surface with capability and categories. Optional `category` and `capability` filters. |
| `linode_profile_list_categories` | Deduplicated category list with tool counts. Discover what categories exist. |
| `linode_profile_draft_new` | Start a new draft. Optional `clone_from` to seed from an existing profile. |
| `linode_profile_draft_show` | Read a draft's current state. |
| `linode_profile_draft_add_tools` | Add literal-or-wildcard tool patterns to the draft. Wildcards expand against the live catalog at call time. |
| `linode_profile_draft_remove_tools` | Remove tools. Patterns match the draft's current state, not the live catalog. |
| `linode_profile_draft_set` | Set scalar/list fields on the draft (`allowed_environments`, `required_token_scopes`, `allow_yolo`). |
| `linode_profile_draft_discard` | Drop a draft. Idempotent. |
| `linode_profile_draft_save` | Write the draft to the config file. Requires `confirm: true`. Returns a diff against the prior state (or empty for new). Does NOT change the active profile. |

### Example conversation

```text
User:  "Build me a profile that can manage DNS and read everything else."
Model: linode_profile_draft_new(name="dns-admin-readall")
       linode_profile_list_tools(category="dns")
       linode_profile_draft_add_tools(
           name="dns-admin-readall",
           tools=["linode_domain_*", "linode_domain_record_*"])
       linode_profile_list_tools(capability="read")
       linode_profile_draft_add_tools(
           name="dns-admin-readall",
           tools=[<all CapRead names>])
       linode_profile_draft_show(name="dns-admin-readall")
Model: "Draft 'dns-admin-readall' has 56 tools. Save?"
User:  "Yes."
Model: linode_profile_draft_save(name="dns-admin-readall", confirm=true)
       returns the diff: "+12 DNS write/destroy tools, +44 read tools"
Model: "Saved. Run `linodemcp profile use dns-admin-readall` to activate."
```

The save tool never changes the active profile. Activating the new profile is a separate, explicit CLI step. This keeps the trust boundary clear: the model can suggest, the user activates.

Built-in profile names (`default`, `compute-admin`, etc.) are refused as save targets to prevent shadowing.

## Token-scope validation

At server start (and on every hot-reload), the server calls `GET /profile` and `GET /profile/grants` against the configured Linode token, then compares the returned scopes against the active profile's `required_token_scopes`.

- **Missing required scope**: server fails to start. The error message lists the missing scope and which tools needed it.
- **Excess scope on the token**: server warns at startup and continues. The token has more reach than the profile asks for; this is a least-privilege nudge, not an error. Set `strict_token_scope: true` per-environment to upgrade to a fail.

If the active environment has no token configured at all, behavior depends on whether the profile is "elevated":

- **Read-only profiles** (default, readonly-full): warn but continue. Tools that hit the API will fail at call time with an auth error; the server still starts so users can browse the tool list or inspect audit history.
- **Elevated profiles** (compute-admin, full-access, etc.): fail to start. A server with no valid token has no business registering write tools.

The elevation classification is `:read_write` scope presence in the profile's `required_token_scopes`. The threshold is per-environment; an unconfigured environment doesn't fail load for a read-only-everywhere profile.

## Hot-reload

The config-file watcher polls every `DefaultWatchInterval` (currently 2 seconds). On a detected rename or content change, it calls the server's reload callback:

1. The new config loads through the same path the initial server start uses.
2. The resolver builds the new active profile.
3. The server diffs old vs new `AllowedTools`, calls `DeleteTools` for removed names and `AddTool` for added ones.
4. mcp-go's `notifications/tools/list_changed` fires automatically (Go) / the next `tools/list` returns the new set via mutable handler state (Python).

A failed reload (malformed config, unknown active profile name) is a no-op. The server keeps running with its previous profile and logs the failure. The user sees a stale tool list, not a crash.

## Security model

The boundary the profile system enforces is "the AI under this MCP session cannot call tools outside the active profile's allow list." Concretely:

- The registration filter at startup never registers a filtered-out tool. mcp-go has no API to call an unregistered tool, so the model cannot bypass.
- The Python dispatch path also gates on the allow list before invoking the handler. Belt-and-suspenders.
- Profile switching is a CLI operation. The model can suggest a switch but cannot execute one. The builder's `_draft_save` writes the *definition*, not the activation.
- Built-in profiles are immutable as catalog entries. Overrides only toggle `disabled`.
- Built-in profile names refuse user-defined shadowing in the save and clone paths.
- Audit (Phase 6 deferred) records the active profile name on every tool call event, captured at call time so a switch mid-handler doesn't confuse the audit log.

What the profile system does NOT protect against:

- An attacker with memory-corruption capability against the server process can mutate the active profile pointer. That's out of threat model; the work targets "AI accidentally does dangerous things," not "attacker has memory-corruption capability."
- The MCP client itself (Claude Code, Claude Desktop, etc.) is trusted to relay only what the user typed. A malicious host could substitute requests after the model produces them; the profile filter doesn't see substitution at that layer.
- The Linode API itself enforces token scopes independently. If the profile permits a tool but the token doesn't carry the scope, the API call fails with a 403. The token-scope validator surfaces this at startup; without it, the failure happens mid-call.

For the long-form decision record on process-per-profile alternatives that were considered and rejected, see the "Considered alternatives" section in `.claude/specs/profiles/requirements.md`.

## Related

- [host-integrations/](./host-integrations/README.md): wiring profiles into Claude Code, Claude Desktop, and other MCP hosts.
- [profile-recipes.md](./profile-recipes.md): example user-defined profiles for common scenarios.
