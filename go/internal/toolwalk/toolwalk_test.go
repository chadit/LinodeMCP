package toolwalk_test

import (
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolwalk"
)

// The walks the surface declares, driven through the same entry point a
// generated handler calls. Each case is one a client can make and the sentence
// it reads back; a case expecting "" is one the walk accepts and another part of
// the handler owns.

const (
	configCreateInput   = "linode.mcp.v1.InstanceConfigCreateInput"
	configUpdateInput   = "linode.mcp.v1.InstanceConfigUpdateInput"
	contactCreateInput  = "linode.mcp.v1.ManagedContactCreateInput"
	settingsUpdateInput = "linode.mcp.v1.ManagedLinodeSettingsUpdateInput"
	firewallInput       = "linode.mcp.v1.FirewallSettingsUpdateInput"
	grantsInput         = "linode.mcp.v1.AccountUserGrantsUpdateInput"

	keyDevices  = "devices"
	keyHelpers  = "helpers"
	keySlotSDA  = "sda"
	keyDiskID   = "disk_id"
	keyVolumeID = "volume_id"
	keySSH      = "ssh"
	keyPhone    = "phone"
	keyGlobal   = "global"
	keyLinode   = "linode"
	keyIDs      = "default_firewall_ids"

	errSlotNotObject = "device sda must be an object"
	errDiskInteger   = "invalid devices JSON: sda.disk_id must be an integer"
	errDiskPositive  = "device sda disk_id must be greater than 0"
	errSSHPort       = "ssh.port must be an integer from 1 to 65535 or null"
	errSSHUser       = "ssh.user must be a string up to 32 characters or null"
	errDevicesNeeded = "devices is required"

	keySSHPort       = "port"
	keySSHUser       = "user"
	grantRead        = "read_only"
	notAKind         = "yes"
	caseNoObject     = "not an object"
	errSectionShape  = "linode must be an array of objects"
	keyGrantID       = "id"
	keyPermissions   = "permissions"
	grantUnknown     = "admin"
	keyAccountAccess = "account_access"
	keyPurpose       = "purpose"
	keySSHAccess     = "access"
	errPurposeChoice = "interfaces[0].purpose must be one of: public, vlan, vpc"
	caseEmpty        = "empty"
)

// walkCase is one call and the sentence the contract answers it with.
type walkCase struct {
	arguments map[string]any
	name      string
	want      string
}

// runCases drives one message's walks over the calls a client can make.
func runCases(t *testing.T, message string, cases []walkCase) {
	t.Helper()

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := toolwalk.Check(message, test.arguments); got != test.want {
				t.Errorf("Check = %q, want %q", got, test.want)
			}
		})
	}
}

// device is one config device slot map carrying the named members.
func device(members map[string]any) map[string]any {
	return map[string]any{keyDevices: map[string]any{keySlotSDA: members}}
}

// TestMessageWithNoWalkIsAccepted: the seam is written only for a tool that
// declares a walk, but a name that resolves to no declaration must answer
// nothing rather than refusing every call.
func TestMessageWithNoWalkIsAccepted(t *testing.T) {
	t.Parallel()

	for _, message := range []string{
		"linode.mcp.v1.DomainGetInput",
		"linode.mcp.v1.NoSuchInput",
		"linode.mcp.v1.ValueKind",
		"",
	} {
		if got := toolwalk.Check(message, map[string]any{keyDevices: 7}); got != "" {
			t.Errorf("Check(%q) = %q, want no answer", message, got)
		}
	}
}

// TestDeviceSlotsWalkTwoLevels covers the one walk on the surface that reads a
// second level: the slot vocabulary, then the members inside one slot, then the
// pair a slot cannot hold together.
func TestDeviceSlotsWalkTwoLevels(t *testing.T) {
	t.Parallel()

	runCases(t, configCreateInput, []walkCase{
		{name: "absent", arguments: map[string]any{}, want: errDevicesNeeded},
		{
			name:      "explicit null reads as absent",
			arguments: map[string]any{keyDevices: nil},
			want:      errDevicesNeeded,
		},
		{
			name:      caseNoObject,
			arguments: map[string]any{keyDevices: "sda"},
			want:      "devices must be a JSON object",
		},
		{
			name:      caseEmpty,
			arguments: map[string]any{keyDevices: map[string]any{}},
			want:      "devices must include at least one device slot",
		},
		{
			name:      "slot outside the vocabulary",
			arguments: map[string]any{keyDevices: map[string]any{"sdz": map[string]any{}}},
			want:      "device slot sdz must be one of sda through sdh",
		},
		{
			name:      "slot that is not an object",
			arguments: map[string]any{keyDevices: map[string]any{keySlotSDA: 111}},
			want:      errSlotNotObject,
		},
		{
			name:      "slot sent as null",
			arguments: map[string]any{keyDevices: map[string]any{keySlotSDA: nil}},
			want:      errSlotNotObject,
		},
		{
			name:      "member outside the vocabulary",
			arguments: device(map[string]any{"cdrom": 1}),
			want:      `invalid devices JSON: unknown field "cdrom"`,
		},
		{
			name:      "member of the wrong kind",
			arguments: device(map[string]any{keyDiskID: "111"}),
			want:      errDiskInteger,
		},
		{
			name:      "member carrying a fraction",
			arguments: device(map[string]any{keyDiskID: 1.5}),
			want:      errDiskInteger,
		},
		{
			name:      "member below its bound",
			arguments: device(map[string]any{keyDiskID: 0}),
			want:      errDiskPositive,
		},
		{
			name:      "slot naming neither member",
			arguments: device(map[string]any{}),
			want:      "device sda requires disk_id or volume_id",
		},
		{
			name:      "slot naming both members",
			arguments: device(map[string]any{keyDiskID: 111, keyVolumeID: 222}),
			want:      "device sda can use disk_id or volume_id, not both",
		},
		{name: "one whole disk id", arguments: device(map[string]any{keyDiskID: 111})},
		{name: "a JSON float id", arguments: device(map[string]any{keyDiskID: float64(111)})},
		{name: "an int64 id", arguments: device(map[string]any{keyVolumeID: int64(222)})},
	})
}

// TestUpdateAcceptsAnAbsentMapWhereTheCreateRequiresOne: the two config writes
// walk the same shapes and differ only in whether the caller has to name one.
func TestUpdateAcceptsAnAbsentMapWhereTheCreateRequiresOne(t *testing.T) {
	t.Parallel()

	runCases(t, configUpdateInput, []walkCase{
		{name: "absent devices", arguments: map[string]any{}},
		{
			name:      "empty devices",
			arguments: map[string]any{keyDevices: map[string]any{}},
			want:      "devices must include at least one device slot",
		},
	})
}

// TestHelpersCheckTheVocabularyAndNothingElse: the route reads every helper as
// a flag, so the walk holds the names and never judges a value.
func TestHelpersCheckTheVocabularyAndNothingElse(t *testing.T) {
	t.Parallel()

	runCases(t, configCreateInput, []walkCase{
		{
			name:      "unknown helper",
			arguments: map[string]any{keyDevices: nil, keyHelpers: map[string]any{"turbo": true}},
			want:      errDevicesNeeded,
		},
		{
			name: "helper of any kind",
			arguments: map[string]any{
				keyDevices: map[string]any{keySlotSDA: map[string]any{keyDiskID: 111}},
				keyHelpers: map[string]any{"distro": notAKind},
			},
		},
	})
}

// TestInterfacesWalkALegacyList: the members block that words no unknown key,
// so an entry may carry anything beside the purpose it has to name.
func TestInterfacesWalkALegacyList(t *testing.T) {
	t.Parallel()

	withInterfaces := func(entries ...any) map[string]any {
		return map[string]any{
			keyDevices:   map[string]any{keySlotSDA: map[string]any{keyDiskID: 111}},
			"interfaces": entries,
		}
	}

	runCases(t, configCreateInput, []walkCase{
		{
			name:      "not a list",
			arguments: map[string]any{keyDevices: nil, "interfaces": "public"},
			want:      errDevicesNeeded,
		},
		{
			name:      "entry that is not an object",
			arguments: withInterfaces("public"),
			want:      "interfaces must be an array of objects",
		},
		{
			name:      "entry naming no purpose",
			arguments: withInterfaces(map[string]any{}),
			want:      errPurposeChoice,
		},
		{
			name:      "purpose outside the vocabulary",
			arguments: withInterfaces(map[string]any{keyPurpose: "carrier-pigeon"}),
			want:      errPurposeChoice,
		},
		{
			name:      "purpose of the wrong kind",
			arguments: withInterfaces(map[string]any{keyPurpose: 7}),
			want:      errPurposeChoice,
		},
		{name: "an empty list", arguments: withInterfaces()},
		{
			name:      "a member the walk does not name",
			arguments: withInterfaces(map[string]any{keyPurpose: "vlan", "label": "vlan-1"}),
		},
	})
}

// TestSSHMembersEachCarryTheirOwnKind: the one walk whose members differ from
// each other, including the two a null clears.
func TestSSHMembersEachCarryTheirOwnKind(t *testing.T) {
	t.Parallel()

	ssh := func(members map[string]any) map[string]any {
		return map[string]any{keySSH: members}
	}

	runCases(t, settingsUpdateInput, []walkCase{
		{name: "absent ssh", arguments: map[string]any{}},
		{name: "ssh that is not an object", arguments: map[string]any{keySSH: "on"}},
		{
			name:      "two unknown members are both named",
			arguments: ssh(map[string]any{"shell": "zsh", "agent": true}),
			want:      "ssh contains unsupported fields: agent, shell",
		},
		{
			name:      "access of the wrong kind",
			arguments: ssh(map[string]any{keySSHAccess: notAKind}),
			want:      "ssh.access must be a boolean",
		},
		{
			name:      "access sent as null",
			arguments: ssh(map[string]any{keySSHAccess: nil}),
			want:      "ssh.access must be a boolean",
		},
		{
			name:      "blank ip",
			arguments: ssh(map[string]any{"ip": "   "}),
			want:      "ssh.ip must be a non-empty string",
		},
		{
			name:      "ip of the wrong kind",
			arguments: ssh(map[string]any{"ip": 7}),
			want:      "ssh.ip must be a non-empty string",
		},
		{name: "port below its range", arguments: ssh(map[string]any{keySSHPort: 0}), want: errSSHPort},
		{name: "port above its range", arguments: ssh(map[string]any{keySSHPort: 65536}), want: errSSHPort},
		{name: "port carrying a fraction", arguments: ssh(map[string]any{keySSHPort: 22.5}), want: errSSHPort},
		{name: "port sent as a bool", arguments: ssh(map[string]any{keySSHPort: true}), want: errSSHPort},
		{name: "null port clears it", arguments: ssh(map[string]any{keySSHPort: nil})},
		{name: "a boolean access", arguments: ssh(map[string]any{keySSHAccess: true})},
		{name: "port inside its range", arguments: ssh(map[string]any{keySSHPort: 22})},
		{
			name:      "user past the cap",
			arguments: ssh(map[string]any{keySSHUser: strings.Repeat("u", 33)}),
			want:      errSSHUser,
		},
		{name: "user at the cap", arguments: ssh(map[string]any{keySSHUser: strings.Repeat("u", 32)})},
		{name: "null user clears it", arguments: ssh(map[string]any{keySSHUser: nil})},
	})
}

// TestPhoneNeedsOneNumber: the at-least-one arm at the top level of a walk,
// where no key sits above it.
func TestPhoneNeedsOneNumber(t *testing.T) {
	t.Parallel()

	runCases(t, contactCreateInput, []walkCase{
		{name: "absent phone", arguments: map[string]any{}},
		{name: "phone that is not an object", arguments: map[string]any{keyPhone: "5551234"}},
		{
			name:      "phone naming neither number",
			arguments: map[string]any{keyPhone: map[string]any{}},
			want:      "phone must include primary or secondary",
		},
		{
			name:      "blank number",
			arguments: map[string]any{keyPhone: map[string]any{"primary": " "}},
			want:      "phone.primary must be a non-empty string or null",
		},
		{name: "null number clears it", arguments: map[string]any{keyPhone: map[string]any{"secondary": nil}}},
		{name: "one number", arguments: map[string]any{keyPhone: map[string]any{"primary": "5551234"}}},
	})
}

// TestFirewallAssignmentsShareOneMemberSpec: every key carries the same value,
// which is what the shared-value form of a walk declares.
func TestFirewallAssignmentsShareOneMemberSpec(t *testing.T) {
	t.Parallel()

	runCases(t, firewallInput, []walkCase{
		{name: "absent", arguments: map[string]any{}, want: "default_firewall_ids is required"},
		{
			name:      caseNoObject,
			arguments: map[string]any{keyIDs: 5},
			want:      "default_firewall_ids must be a non-empty object",
		},
		{
			name:      caseEmpty,
			arguments: map[string]any{keyIDs: map[string]any{}},
			want:      "default_firewall_ids must be a non-empty object",
		},
		{
			name:      "key outside the vocabulary",
			arguments: map[string]any{keyIDs: map[string]any{"volume": 5}},
			want:      "default_firewall_ids contains unsupported key: volume",
		},
		{
			name:      "non-positive id",
			arguments: map[string]any{keyIDs: map[string]any{"nodebalancer": 0}},
			want:      "default_firewall_ids.nodebalancer must be a positive integer",
		},
		{
			name:      "id sent as a bool",
			arguments: map[string]any{keyIDs: map[string]any{keyLinode: true}},
			want:      "default_firewall_ids.linode must be a positive integer",
		},
		{name: "a whole id", arguments: map[string]any{keyIDs: map[string]any{keyLinode: 9}}},
		{name: "a JSON float id", arguments: map[string]any{keyIDs: map[string]any{keyLinode: float64(9)}}},
	})
}

// TestGrantSectionsWalkEachEntry: the list form of a walk, where the index
// stands where a map walk's key does.
func TestGrantSectionsWalkEachEntry(t *testing.T) {
	t.Parallel()

	section := func(entries ...any) map[string]any {
		return map[string]any{keyLinode: entries}
	}

	runCases(t, grantsInput, []walkCase{
		{
			name:      "not a list",
			arguments: map[string]any{keyLinode: grantRead},
			want:      errSectionShape,
		},
		{
			name:      "entry that is not an object",
			arguments: section(grantRead),
			want:      errSectionShape,
		},
		{
			name:      "entry naming no id",
			arguments: section(map[string]any{keyPermissions: grantRead}),
			want:      errSectionShape,
		},
		{
			name:      "entry carrying a member the API does not read",
			arguments: section(map[string]any{keyGrantID: 1, keyPermissions: grantRead, "label": "web"}),
			want:      errSectionShape,
		},
		{
			name:      "permission outside the vocabulary",
			arguments: section(map[string]any{keyGrantID: 1, keyPermissions: grantUnknown}),
			want:      "linode[0].permissions must be one of: read_only, read_write",
		},
		{
			name:      "the index names the entry it is about",
			arguments: section(map[string]any{keyGrantID: 1, keyPermissions: grantRead}, map[string]any{keyGrantID: 2, keyPermissions: grantUnknown}),
			want:      "linode[1].permissions must be one of: read_only, read_write",
		},
		{name: "null permission revokes the grant", arguments: section(map[string]any{keyGrantID: 1, keyPermissions: nil})},
		{name: "a readable grant", arguments: section(map[string]any{keyGrantID: 1, keyPermissions: "read_write"})},
	})
}

// TestGlobalGrantsMixSharedAndOwnMembers: one block declaring both a shared
// value for twelve keys and two members with a value of their own.
func TestGlobalGrantsMixSharedAndOwnMembers(t *testing.T) {
	t.Parallel()

	global := func(members map[string]any) map[string]any {
		return map[string]any{keyGlobal: members}
	}

	runCases(t, grantsInput, []walkCase{
		{
			name:      caseNoObject,
			arguments: global(nil),
			want:      "global must be an object matching the grants schema",
		},
		{
			name:      caseEmpty,
			arguments: map[string]any{keyGlobal: map[string]any{}},
			want:      "global must be an object matching the grants schema",
		},
		{
			name:      "member outside the vocabulary",
			arguments: global(map[string]any{"add_pigeons": true}),
			want:      "global has unknown fields: add_pigeons",
		},
		{
			name:      "shared member of the wrong kind",
			arguments: global(map[string]any{"add_linodes": notAKind}),
			want:      "global.add_linodes must be a boolean",
		},
		{
			name:      "account_access outside its vocabulary",
			arguments: global(map[string]any{keyAccountAccess: grantUnknown}),
			want:      "global.account_access must be one of: read_only, read_write",
		},
		{
			name:      "child_account_access of the wrong kind",
			arguments: global(map[string]any{"child_account_access": notAKind}),
			want:      "global.child_account_access must be a boolean or null",
		},
		{name: "a boolean flag", arguments: global(map[string]any{"add_linodes": true})},
		{name: "null clears child_account_access", arguments: global(map[string]any{"child_account_access": nil})},
		{name: "null clears account_access", arguments: global(map[string]any{keyAccountAccess: nil})},
		{name: "a readable global grant", arguments: global(map[string]any{keyAccountAccess: grantRead})},
	})
}
