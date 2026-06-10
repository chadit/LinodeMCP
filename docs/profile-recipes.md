# Profile recipes

Copy-paste starting points for common postures. Each recipe is a user-defined profile that drops into your config under `profiles:`. Adjust the names and tool lists to match your actual environments.

For the full reference (schema, capability tags, builder workflow), see [profiles.md](./profiles.md).

## Read-only oncall

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

## DNS administrator with read-everywhere

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

## Builder-composed: scoped admin with read-everything

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

## Dev environment only

Restricting an AI session to a non-production environment is a strong guardrail. Even if the model invokes a destructive tool, the environment restriction means the call cannot land on the production Linode account.

```yaml
environments:
  default:
    label: "Production"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "${LINODEMCP_LINODE_TOKEN_PROD}"
  dev:
    label: "Dev"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "${LINODEMCP_LINODE_TOKEN_DEV}"

profiles:
  dev-only-compute-admin:
    description: "Compute admin restricted to the dev environment"
    allowed_tools:
      - "*"
    denied_tools:
      - "linode_*"  # Strip everything…
    # …then re-allow only compute (the resolver applies denies after
    # allow expansion, so a denied wildcard followed by a more
    # specific allow doesn't work directly. The cleaner path is to
    # list compute prefixes in allowed_tools without the deny.)
    allowed_environments: ["dev"]
    required_token_scopes:
      - "linodes:read_write"
    allow_yolo: false
```

The pattern above is intentionally awkward because the spec deliberately doesn't support "deny then re-allow" composition. Use explicit allowed_tools wildcards (or build via the conversation flow above) when you want a strict subset.

Cleaner variant:

```yaml
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

## Token-isolated profile for an experiment

When you want to give the AI a separate Linode token (different scopes, different account, different rate-limit pool) for one experiment, declare a second environment and restrict the profile to it. The `required_token_scopes` field validates the token's scopes against the profile at server start.

```yaml
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "${LINODEMCP_LINODE_TOKEN_DEFAULT}"
  experiment:
    label: "Experiment"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "${LINODEMCP_LINODE_TOKEN_EXPERIMENT}"

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

## Emergency posture

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

- [profiles.md](./profiles.md): full reference.
- [host-integrations/](./host-integrations/README.md): wiring profiles into MCP hosts.
