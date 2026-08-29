# Profiles

Profiles are LinodeMCP's permission model. A profile names a set of tools the connected AI client can see and call. The server filters tools at registration time, so the AI literally cannot invoke anything outside the active profile. Profiles also carry environment restrictions, token-scope requirements, and the `allow_yolo` opt-in for the dry-run bypass.

This doc covers the full surface: built-in catalog, [how profiles differ from Linode IAM](#profiles-are-not-linode-iam), config schema, capability tags, CLI commands, in-conversation builder, token-scope validation, hot-reload, the security model, and copy-paste [recipes](#recipes) for common postures. For copy-paste integration with specific MCP hosts, see [host-integrations/](./host-integrations/README.md).

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
| `CapAdmin` | Account-level mutations (payments, user management, account settings) |
| `CapMeta` | Tools that touch local config or session state, never the Linode API (profile builder, `hello`, `version`) |

`CapMeta` tools bypass every profile filter. The builder (`linode_profile_*` tools) and the smoke-test pair (`hello`, `version`) are always visible regardless of which profile is active. This is deliberate: the builder needs to work under the read-only default so users can compose their first elevated profile from inside a conversation.

The capability-and-confirm invariant is enforced in tests: any `CapWrite` or `CapDestroy` tool MUST declare `confirm` in its required-parameter list, and any `CapRead` or `CapMeta` tool MUST NOT. The invariant fires at server startup so a mistagged tool fails the build, not at request time.

## Built-in catalog

Nine profiles ship in the binary. They cover the common postures.

| Name | Reach | Default disabled? |
| --- | --- | --- |
| `default` | Read + Meta across every category | No (this is the literal default) |
| `readonly-full` | Same as default; explicit name for clarity in scripts | No |
| `compute-admin` | Read everywhere, plus Write + Destroy on compute (instances, regions, types, images, stackscripts, instance backups/disks/IPs/actions) | No |
| `network-admin` | Read everywhere, plus Write + Destroy on networking (firewalls, NodeBalancers, VLANs, IPv6 ranges) | No |
| `kubernetes-admin` | Read everywhere, plus Write + Destroy on LKE (clusters, node pools, ACL) | No |
| `storage-admin` | Read everywhere, plus Write + Destroy on volumes, object storage, and instance backups | No |
| `iam-admin` | Read everywhere, plus Write + Destroy on Linode IAM (role assignment, delegation, IdP/SSO configs) | No |
| `full-access` | Read + Write + Destroy across every category | **Yes** |
| `emergency` | Full access plus `allow_yolo: true` (skips dry-run gates) | **Yes** |

Two of them ship disabled: `full-access` and `emergency`. To use them, set `disabled: false` under `profiles_builtin_overrides` in your config. The shipped-disabled state exists so a default install can't accidentally hand a full admin surface to a fresh AI client.

Each built-in computes its `required_token_scopes` from the union of its tool list's scope tags at startup. There's no hand-maintained scope list per built-in; the tool tags drive it.

## Profiles are not Linode IAM

Two access-control systems sit in front of the same Linode account and neither one knows the other exists. A LinodeMCP profile lives on your machine and decides what this server offers the AI. Linode IAM lives on Akamai's side and decides what the calling user is allowed to do. Confusing the two is the fastest way to a surprising 403: the profile said yes, the API said no.

### What a LinodeMCP profile is

A profile is a registration-time filter over this server's tool list, written in your config file and read at server start.

- The server registers only the tools the active profile permits. mcp-go has no API for calling an unregistered tool, so the model cannot reach past the filter.
- A profile subtracts reach. It never adds any. Listing `linode_instance_delete` in a profile does not give anyone permission to delete a Linode; it only means this server will offer the tool.
- The filter is local. Nobody at Linode can see your profile, and switching profiles changes nothing about your account.

### What Linode IAM is

Linode IAM is role-based access control the API runs server side, per calling user. Akamai documents it under [Identity and Access Management](https://techdocs.akamai.com/cloud-computing/docs/identity-and-access-cm).

- A **permission** is the smallest unit and maps one to one onto an API operation: `delete_linode`, `update_user_permissions`, `list_entities`.
- A **role** bundles permissions. Account access roles cover account-level operations plus every entity you own, present and future. Entity access roles cover named existing entities only. The [role permissions reference](https://techdocs.akamai.com/linode-api/reference/get-role-permissions) lists the catalog.
- Roles are assigned to **Linode users**, not to tokens and not to this server. The API checks the caller's roles on every request, whatever client made it.
- An **entity** is any managed object with a life cycle: a Linode, a firewall, a NodeBalancer. Entity role assignment needs entity ids, which is what [`GET /entities`](https://techdocs.akamai.com/linode-api/reference/get-entities) is for.
- Only the `account_admin` role carries the permissions that read and write role assignments (`list_role_permissions`, `list_user_permissions`, `update_user_permissions`, `list_entities`, `list_all_child_accounts`). See [available roles](https://techdocs.akamai.com/cloud-computing/docs/identity-access-cm-available-roles).

### How the two layers stack

A call has to clear three independent fences, and effective access is their intersection:

| Fence | Who owns it | What it narrows |
| --- | --- | --- |
| Active profile | You, in this server's config | Which tools exist in the session at all |
| Token scopes | You, when you mint the personal access token | Which API operations that token may attempt |
| IAM role permissions | The account's `account_admin`, per Linode user | What the calling user may do, in any client |

None of the three can widen another. A token ["may not exceed the scopes"](https://techdocs.akamai.com/linode-api/reference/post-personal-access-token) of the user who created it, and no profile setting reaches Akamai's side at all.

The IAM routes are the odd case. Every `/iam/*` operation and `GET /entities` documents a personal access token and no OAuth scope, and their permission blocks name IAM permissions instead. For those calls the fence count drops to two: the active profile, and the calling user's role. Scope narrowing does nothing there, which is why `iam-admin` computes exactly the same `required_token_scopes` as `default` does. The IAM tools contribute none.

### Setting up a profile

Pick a built-in, or write your own under `profiles:`, then activate it:

```yaml
active_profile: "iam-admin"
```

```bash
linodemcp profile list          # every built-in and user-defined profile
linodemcp profile show iam-admin # its resolved tool list and scope union
linodemcp profile use iam-admin  # switch; the running server hot-reloads
```

The `iam-admin` built-in serves every Read tool plus Write and Destroy on the `iam` category: role assignment, delegation, and the SAML IdP configs. It is enabled out of the box, like its four sibling category admins.

### Setting up Linode IAM

Role assignment happens on Akamai's side, so it needs a user who already holds `account_admin`. Two ways in:

1. **Cloud Manager**, under Identity and Access. The point-and-click path, and the usual starting place.
2. **Through this server**, with a token whose user is an `account_admin` and a profile that serves the IAM mutators (`iam-admin` or a wildcard):

   ```text
   linode_iam_role_permission_list()               # the role catalog and what each role carries
   linode_entity_list()                            # entity ids for entity_access assignment
   linode_iam_user_role_permission_get(username=…) # what the user holds today
   linode_iam_user_role_permission_update(username=…, account_access=[…], entity_access=[…], confirm=true)
   ```

The update replaces the whole assignment rather than adding to it, so send the full list every time. An empty list blocks the user. Preview first with `dry_run: true`; the tool answers what it would send without sending it.

### Do not mix legacy grants with IAM

The older access-control model is per-user grants: `GET` and `PUT /account/users/{username}/grants`, shipped here as `linode_account_user_grants_get` and `linode_account_user_grants_update`. Both endpoints are deprecated in favor of IAM.

Akamai's [migration guide](https://techdocs.akamai.com/cloud-computing/docs/migration-grants-identity-and-access) says not to run both on one account: doing so "may expose your account to security risks, due to differences in access control between the two systems". Pick one model per account.

This server ships both, so the profile is where you draw the line:

- `iam-admin` cannot reach the grants mutator. `linode_account_user_grants_update` is an Admin-tier tool in the `account` category, and `iam-admin` elevates `iam` only. Its paired read is a plain Read, so every profile serves it, down to `default`.
- If your account still runs on grants, keep the IAM mutators out of whatever profile you activate. A user-defined profile with an explicit `allowed_tools` list is the clean way; `full-access` serves both systems at once and is exactly what the warning is about.
- For operations that document neither a permission nor a role, grants are still the control, so an account mid-migration is not free of them yet.

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

A profile is elevated when its allowed tools include any Write, Destroy, or Admin tool; the server derives an `Elevated` flag from the tool capabilities at load time. Scope suffixes cannot make the call: the API documents `:read_write` scopes on several read-only routes (kubeconfig, managed contacts, instance interfaces), so a read-only profile's scope union can legitimately contain write scopes without the profile being able to mutate anything. The threshold is per-environment; an unconfigured environment doesn't fail load for a read-only-everywhere profile.

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
- The audit log records the active profile name on every tool-call event, captured at call time so a switch mid-handler doesn't confuse the log.

What the profile system does NOT protect against:

- An attacker with memory-corruption capability against the server process can mutate the active profile pointer. That's out of threat model; the work targets "AI accidentally does dangerous things," not "attacker has memory-corruption capability."
- The MCP client itself (Claude Code, Claude Desktop, etc.) is trusted to relay only what the user typed. A malicious host could substitute requests after the model produces them; the profile filter doesn't see substitution at that layer.
- The Linode API itself enforces token scopes independently. If the profile permits a tool but the token doesn't carry the scope, the API call fails with a 403. The token-scope validator surfaces this at startup; without it, the failure happens mid-call.

## Recipes

Copy-paste starting points for common postures. Each recipe is a user-defined profile that drops into your config under `profiles:`. Adjust the names and tool lists to match your actual environments.

### Read-only oncall

The oncall person needs to debug a production incident. They can look at everything but cannot touch anything. Default `readonly-full` covers most of this, but the recipe below also restricts which environments the AI can hit, so a model that thinks it's helpful can't read from staging or dev when production was the actual question.

```yaml
profiles:
  oncall-read-prod-only:
    description: "Read-everything, restricted to the prod environment"
    allowed_tools:
      - "*"
    denied_tools:
      - "linode_*_create"
      - "linode_*_update"
      - "linode_*_delete"
      - "linode_instance_boot"
      - "linode_instance_reboot"
      - "linode_instance_shutdown"
      - "linode_instance_rebuild"
      - "linode_instance_rescue"
      - "linode_instance_resize"
      - "linode_instance_migrate"
      - "linode_instance_clone"
      - "linode_instance_password_reset"
    allowed_environments: ["prod"]
    required_token_scopes:
      - "linodes:read_only"
      - "domains:read_only"
      - "firewalls:read_only"
      - "nodebalancers:read_only"
    allow_yolo: false
```

`allowed_tools: ["*"]` plus a `denied_tools` list is the easiest way to express "everything except the destructive operations" without enumerating 200+ tools. Denial expands after allow, so explicit denies win.

### DNS administrator with read-everywhere

A DNS admin needs to manage records and zones. Everything else they should be able to look at, but not touch. The built-in `network-admin` is too broad: it covers firewalls and NodeBalancers as well. A scoped profile gives only what's needed.

```yaml
profiles:
  dns-admin:
    description: "DNS write access plus read-everywhere"
    allowed_tools:
      - "linode_domain_*"
      - "linode_domain_record_*"
    denied_tools: []
    allowed_environments: ["*"]
    required_token_scopes:
      - "domains:read_write"
    allow_yolo: false
```

This profile leans on the registration filter: the listed wildcards pick up `linode_domain_create`, `linode_domain_update`, `linode_domain_delete`, plus the per-record CRUD. The read tools (`linode_domain_list`, `linode_domain_get`) are also covered. Since the wildcards don't include anything outside the DNS surface, the AI cannot see compute or networking tools at all under this profile.

To add read-everywhere on top, the builder approach (next recipe) is cleaner than maintaining a hand-rolled list.

### Builder-composed: scoped admin with read-everything

A common shape: full admin on one category, read-only everywhere else. Compose it interactively with the builder rather than hand-writing the tool list.

In a conversation with the model:

```text
linode_profile_draft_new(name="lke-admin-readall", clone_from="readonly-full")
linode_profile_list_categories()
linode_profile_draft_add_tools(
    name="lke-admin-readall",
    tools=["linode_lke_*"])
linode_profile_draft_show(name="lke-admin-readall")
# Confirm the resolved tool list looks right.
linode_profile_draft_save(name="lke-admin-readall", confirm=true)
```

The clone-from-readonly-full seed gives every `CapRead` tool. The add-tools call layers in every LKE tool (read + write + destroy). The resulting profile has read everywhere plus full LKE control.

After `save`, run `linodemcp profile use lke-admin-readall` to activate. The save tool returns the diff but does NOT activate; activation is always an explicit step.

### Dev environment only

Restricting an AI session to a non-production environment is a strong guardrail. Even if the model invokes a destructive tool, the environment restriction means the call cannot land on the production Linode account.

The spec deliberately doesn't support "deny then re-allow" composition (denies apply after allow expansion), so express a strict subset with explicit `allowed_tools` wildcards:

```yaml
environments:
  default:
    label: "Production"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "your-production-scoped-token"
  dev:
    label: "Dev"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "your-dev-scoped-token"

profiles:
  dev-only-compute-admin:
    description: "Compute admin restricted to the dev environment"
    allowed_tools:
      - "linode_instance_*"
      - "linode_region_*"
      - "linode_type_*"
      - "linode_image_*"
      - "linode_stackscript_*"
    denied_tools: []
    allowed_environments: ["dev"]
    required_token_scopes:
      - "linodes:read_write"
    allow_yolo: false
```

### Token-isolated profile for an experiment

When you want to give the AI a separate Linode token (different scopes, different account, different rate-limit pool) for one experiment, declare a second environment and restrict the profile to it. The `required_token_scopes` field validates the token's scopes against the profile at server start.

```yaml
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "your-default-token"
  experiment:
    label: "Experiment"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "your-experiment-scoped-token"

profiles:
  experiment-read:
    description: "Read-only against the experiment environment"
    allowed_tools:
      - "*"
    denied_tools:
      - "linode_*_create"
      - "linode_*_update"
      - "linode_*_delete"
    allowed_environments: ["experiment"]
    required_token_scopes:
      - "linodes:read_only"
      - "domains:read_only"
    allow_yolo: false
```

If the experiment token has fewer scopes than the profile asks for, the server fails to start with a message listing the missing scope and which tools need it. If it has more, the server warns and continues.

### Emergency posture

The built-in `emergency` profile ships disabled. Enabling it gives the AI full reach plus `allow_yolo: true` (skipping dry-run gates). Use this only when you genuinely need to pop a cluster back up in the middle of an incident and you don't have time to type the confirmations.

Enable via CLI:

```bash
linodemcp profile enable emergency
linodemcp profile use emergency
```

Disable it back to default once the incident closes:

```bash
linodemcp profile use default
linodemcp profile disable emergency
```

The `emergency` built-in is unrestricted by design. A safer pattern for "I want to act fast but with limits" is to clone `full-access`, set `allow_yolo: true`, and add an `allowed_environments` restriction that scopes the blast radius:

```yaml
profiles:
  prod-emergency:
    description: "Emergency on production only. allow_yolo with env restriction"
    allowed_tools:
      - "*"
    denied_tools: []
    allowed_environments: ["prod"]
    required_token_scopes:
      - "linodes:read_write"
      - "domains:read_write"
      - "firewalls:read_write"
      - "nodebalancers:read_write"
    allow_yolo: true
```

## Related

- [host-integrations/](./host-integrations/README.md): wiring profiles into Claude Code, Claude Desktop, and other MCP hosts.
