package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The tool-to-scope half of the contract. Every routed tool declares its
// documented OAuth scopes (or an explicit none) on its input message, and the
// wire spelling of each enum member lives here alone, so a scope string cannot
// drift between the languages that render it.

// scopeStrings is the wire spelling of each declarable scope member.
func scopeStrings() map[linodev1.ToolScope]string {
	return map[linodev1.ToolScope]string{
		linodev1.ToolScope_TOOL_SCOPE_ACCOUNT_READ_ONLY:         "account:read_only",
		linodev1.ToolScope_TOOL_SCOPE_ACCOUNT_READ_WRITE:        "account:read_write",
		linodev1.ToolScope_TOOL_SCOPE_DATABASES_READ_ONLY:       "databases:read_only",
		linodev1.ToolScope_TOOL_SCOPE_DATABASES_READ_WRITE:      "databases:read_write",
		linodev1.ToolScope_TOOL_SCOPE_DOMAINS_READ_ONLY:         "domains:read_only",
		linodev1.ToolScope_TOOL_SCOPE_DOMAINS_READ_WRITE:        "domains:read_write",
		linodev1.ToolScope_TOOL_SCOPE_EVENTS_READ_ONLY:          "events:read_only",
		linodev1.ToolScope_TOOL_SCOPE_FIREWALL_READ_ONLY:        "firewall:read_only",
		linodev1.ToolScope_TOOL_SCOPE_FIREWALL_READ_WRITE:       "firewall:read_write",
		linodev1.ToolScope_TOOL_SCOPE_IMAGES_READ_ONLY:          "images:read_only",
		linodev1.ToolScope_TOOL_SCOPE_IMAGES_READ_WRITE:         "images:read_write",
		linodev1.ToolScope_TOOL_SCOPE_IPS_READ_ONLY:             "ips:read_only",
		linodev1.ToolScope_TOOL_SCOPE_IPS_READ_WRITE:            "ips:read_write",
		linodev1.ToolScope_TOOL_SCOPE_LINODES_READ_ONLY:         "linodes:read_only",
		linodev1.ToolScope_TOOL_SCOPE_LINODES_READ_WRITE:        "linodes:read_write",
		linodev1.ToolScope_TOOL_SCOPE_LKE_READ_ONLY:             "lke:read_only",
		linodev1.ToolScope_TOOL_SCOPE_LKE_READ_WRITE:            "lke:read_write",
		linodev1.ToolScope_TOOL_SCOPE_LONGVIEW_READ_ONLY:        "longview:read_only",
		linodev1.ToolScope_TOOL_SCOPE_LONGVIEW_READ_WRITE:       "longview:read_write",
		linodev1.ToolScope_TOOL_SCOPE_MONITOR_READ_ONLY:         "monitor:read_only",
		linodev1.ToolScope_TOOL_SCOPE_MONITOR_READ_WRITE:        "monitor:read_write",
		linodev1.ToolScope_TOOL_SCOPE_NODEBALANCERS_READ_ONLY:   "nodebalancers:read_only",
		linodev1.ToolScope_TOOL_SCOPE_NODEBALANCERS_READ_WRITE:  "nodebalancers:read_write",
		linodev1.ToolScope_TOOL_SCOPE_OBJECT_STORAGE_READ_ONLY:  "object_storage:read_only",
		linodev1.ToolScope_TOOL_SCOPE_OBJECT_STORAGE_READ_WRITE: "object_storage:read_write",
		linodev1.ToolScope_TOOL_SCOPE_RESERVED_IPS_READ_ONLY:    "reserved-ips:read_only",
		linodev1.ToolScope_TOOL_SCOPE_RESERVED_IPS_READ_WRITE:   "reserved-ips:read_write",
		linodev1.ToolScope_TOOL_SCOPE_STACKSCRIPTS_READ_ONLY:    "stackscripts:read_only",
		linodev1.ToolScope_TOOL_SCOPE_STACKSCRIPTS_READ_WRITE:   "stackscripts:read_write",
		linodev1.ToolScope_TOOL_SCOPE_VOLUMES_READ_ONLY:         "volumes:read_only",
		linodev1.ToolScope_TOOL_SCOPE_VOLUMES_READ_WRITE:        "volumes:read_write",
		linodev1.ToolScope_TOOL_SCOPE_VPC_READ_WRITE:            "vpc:read_write",
	}
}

// readScopes resolves the declared tool_scopes into the scope strings the
// registries emit. It runs last in the contract build so a missing declaration
// is reported on a tool that is otherwise sound.
func (c *contract) readScopes(options protoreflect.ProtoMessage) error {
	declared, found := scopesOption(options)

	if c.Meta {
		if found {
			return fmt.Errorf("%w: %s", errScopesOnMeta, c.Name)
		}

		return nil
	}

	if !found || (!declared.GetNone() && len(declared.GetScope()) == 0) {
		return fmt.Errorf("%w: %s", errNoScopes, c.Name)
	}

	if declared.GetNone() && len(declared.GetScope()) > 0 {
		return fmt.Errorf("%w: %s", errScopesNoneBesideList, c.Name)
	}

	return c.resolveScopeMembers(declared.GetScope())
}

// resolveScopeMembers turns the declared members into wire spellings, refusing
// a repeat and a member with no spelling to render.
func (c *contract) resolveScopeMembers(members []linodev1.ToolScope) error {
	spellings := scopeStrings()
	seen := make(map[string]bool, len(members))

	for _, member := range members {
		text, mapped := spellings[member]
		if !mapped {
			return fmt.Errorf("%w: %s declares %s", errScopeUnrenderable, c.Name, member)
		}

		if seen[text] {
			return fmt.Errorf("%w: %s declares %s twice", errScopeRepeated, c.Name, text)
		}

		seen[text] = true
		c.Scopes = append(c.Scopes, text)
	}

	return nil
}

// scopesOption reads the declared tool_scopes, reporting whether the message
// declares one at all: presence is the fact the completeness refusal reads.
func scopesOption(options protoreflect.ProtoMessage) (*linodev1.ToolScopes, bool) {
	if !proto.HasExtension(options, linodev1.E_ToolScopes) {
		return nil, false
	}

	declared, ok := proto.GetExtension(options, linodev1.E_ToolScopes).(*linodev1.ToolScopes)

	return declared, ok
}

// scopedContracts is the cohort's scope-declaring tools, tool-name sorted so
// both registries render the table in one order. Tools declaring none are
// left out: an absent key already answers empty.
func scopedContracts(contracts []contract) []*contract {
	scoped := make([]*contract, 0, len(contracts))

	for i := range contracts {
		if len(contracts[i].Scopes) > 0 {
			scoped = append(scoped, &contracts[i])
		}
	}

	slices.SortFunc(scoped, func(left, right *contract) int {
		return strings.Compare(left.Name, right.Name)
	})

	return scoped
}

// quotedStrings is one declared list as a quoted, comma-joined literal body,
// which both registry renderings splice into their own collection syntax.
func quotedStrings(scopes []string) string {
	quoted := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		quoted = append(quoted, strconv.Quote(scope))
	}

	return strings.Join(quoted, ", ")
}
