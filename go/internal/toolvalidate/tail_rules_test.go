package toolvalidate_test

import (
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolvalidate"
)

// The rules that took over from the last format hooks, one test per family.
// Each case is one a client can make and the sentence it reads back, which is
// what the retired hooks were held to; a case expecting "" is one another part
// of the handler owns, either an argument's own reader or the value being
// accepted.

const (
	imageGetInput      = "linode.mcp.v1.ImageGetInput"
	byImageListInput   = "linode.mcp.v1.ImageShareGroupByImageListInput"
	engineGetInput     = "linode.mcp.v1.DatabaseEngineGetInput"
	quotaUsageInput    = "linode.mcp.v1.ObjectStorageQuotaUsageGetInput"
	tagObjectInput     = "linode.mcp.v1.TaggedObjectListInput"
	kernelGetInput     = "linode.mcp.v1.KernelGetInput"
	statsMonthInput    = "linode.mcp.v1.InstanceStatsMonthGetInput"
	transferMonthInput = "linode.mcp.v1.InstanceTransferMonthGetInput"
	memberTokenInput   = "linode.mcp.v1.ImageShareGroupMemberTokenGetInput"
	tokenCreateInput   = "linode.mcp.v1.ImageShareGroupTokenCreateInput"
	ipUpdateInput      = "linode.mcp.v1.IPAddressUpdateInput"
	thumbnailInput     = "linode.mcp.v1.AccountOAuthClientThumbnailUpdateInput"
	recordCreateInput  = "linode.mcp.v1.DomainRecordCreateInput"

	keyImageID        = "image_id"
	keyEngineID       = "engine_id"
	keyQuotaID        = "obj_quota_id"
	keyTagLabel       = "tag_label"
	keyKernelID       = "kernel_id"
	keyLinodeID       = "linode_id"
	keyYear           = "year"
	keyMonth          = "month"
	keyShareGroup     = "sharegroup_id"
	keyTokenUUID      = "token_uuid"
	keyShareUUID      = "valid_for_sharegroup_uuid"
	keyAddress        = "address"
	keyRDNS           = "rdns"
	keyClientID       = "client_id"
	keyThumbnail      = "thumbnail_png_base64"
	keyRecordDomainID = "domain_id"
	keyRecordType     = "type"
	keyTarget         = "target"

	canonicalUUID    = "123e4567-e89b-12d3-a456-426614174000"
	notAUUID         = "not-a-uuid"
	sampleClient     = "abc"
	sampleHostname   = "host.example.com"
	caseCarriesQuery = "carrying a query"
	caseTraversal    = "a traversal"

	imageNonEmpty = "image_id must be a non-empty string"
	imageEscape   = "image_id must not contain query separators, fragments, or traversal segments"
	yearRange     = "year must be an integer between 1970 and 9999"
	monthRange    = "month must be an integer between 1 and 12"
	tokenUUIDForm = "token_uuid must be a UUID"
	hostnameTail  = " record target must be a valid hostname: invalid DNS record name:" +
		" must contain only alphanumeric characters, hyphens, and dots"
)

// ruleCase is one call and the sentence the contract answers it with.
type ruleCase struct {
	arguments map[string]any
	name      string
	want      string
}

// runRuleCases drives one message's rules over the calls a client can make.
func runRuleCases(t *testing.T, message string, cases []ruleCase) {
	t.Helper()

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := toolvalidate.Check(message, test.arguments); got != test.want {
				t.Errorf("Check(%s) = %q, want %q", message, got, test.want)
			}
		})
	}
}

func TestImageIDRulesReadInTheHandReadersOrder(t *testing.T) {
	t.Parallel()

	const (
		prefixed = "image_id must be a prefixed image ID such as linode/debian11, private/123, or shared/123"
		prefix   = "image_id prefix must be linode, private, or shared"
	)

	runRuleCases(t, imageGetInput, []ruleCase{
		{name: "naming no prefix", arguments: map[string]any{keyImageID: "debian11"}, want: prefixed},
		{name: "a blank name", arguments: map[string]any{keyImageID: "linode/ "}, want: prefixed},
		{name: "a traversal adding a part", arguments: map[string]any{keyImageID: "linode/../debian11"}, want: prefixed},
		{name: "an unknown prefix", arguments: map[string]any{keyImageID: "public/debian11"}, want: prefix},
		{name: caseCarriesQuery, arguments: map[string]any{keyImageID: "linode/debian11?x=1"}, want: imageEscape},
		{name: "a traversal inside the name", arguments: map[string]any{keyImageID: "linode/de..bian11"}, want: imageEscape},
		{name: "blank", arguments: map[string]any{keyImageID: " "}, want: imageNonEmpty},
		{name: "sent as a number reads as absent", arguments: map[string]any{keyImageID: float64(11)}, want: imageNonEmpty},
		{name: "a public image", arguments: map[string]any{keyImageID: "linode/debian11"}, want: ""},
		{name: "a shared image", arguments: map[string]any{keyImageID: "shared/123"}, want: ""},
	})
}

func TestShareGroupByImageRulesWantAPrivateImage(t *testing.T) {
	t.Parallel()

	runRuleCases(t, byImageListInput, []ruleCase{
		{
			name: "a public image", arguments: map[string]any{keyImageID: "linode/debian11"},
			want: "image_id must be a private image identifier like private/12345",
		},
		{name: caseCarriesQuery, arguments: map[string]any{keyImageID: "private/1?x=2"}, want: imageEscape},
		{name: "a private image", arguments: map[string]any{keyImageID: "private/12345"}, want: ""},
	})
}

func TestEngineIDRulesWantTheEngineVersionPair(t *testing.T) {
	t.Parallel()

	const (
		segment = "engine_id must not contain query, fragment, or traversal segments"
		pair    = "engine_id must use the engine/version format"
	)

	runRuleCases(t, engineGetInput, []ruleCase{
		{name: "padded", arguments: map[string]any{keyEngineID: " mysql/8"}, want: segment},
		{name: caseTraversal, arguments: map[string]any{keyEngineID: "mysql/../8"}, want: segment},
		{name: caseCarriesQuery, arguments: map[string]any{keyEngineID: "mysql/8?x=1"}, want: segment},
		{name: "naming no version", arguments: map[string]any{keyEngineID: "mysql"}, want: pair},
		{name: "a blank version", arguments: map[string]any{keyEngineID: "mysql/"}, want: pair},
		{name: "outside the character set", arguments: map[string]any{keyEngineID: "mysql/8$"}, want: pair},
		{name: "an engine and version", arguments: map[string]any{keyEngineID: "mysql/8"}, want: ""},
	})
}

func TestSegmentRulesOnTheQuotaAndTagReads(t *testing.T) {
	t.Parallel()

	const charset = "obj_quota_id must not contain path separators, query separators," +
		" traversal segments, or unsupported characters"

	runRuleCases(t, quotaUsageInput, []ruleCase{
		{name: caseTraversal, arguments: map[string]any{keyQuotaID: "quota-..-1"}, want: charset},
		{name: "outside the character set", arguments: map[string]any{keyQuotaID: "QUOTA-1"}, want: charset},
		{name: "absent", arguments: map[string]any{}, want: "obj_quota_id is required"},
		{name: "a quota id", arguments: map[string]any{keyQuotaID: "quota-1.2"}, want: ""},
	})

	runRuleCases(t, tagObjectInput, []ruleCase{
		{
			name: caseTraversal, arguments: map[string]any{keyTagLabel: "prod/../x"},
			want: "tag_label must not contain '?', '#', or '..'",
		},
		{name: "a label carrying a slash", arguments: map[string]any{keyTagLabel: "prod/web"}, want: ""},
		{name: "blank", arguments: map[string]any{keyTagLabel: " "}, want: "tag_label must be a non-empty string"},
	})
}

func TestKernelIDRuleAnswersOneSentence(t *testing.T) {
	t.Parallel()

	const form = "kernel_id must be a kernel identifier like linode/latest-64bit"

	runRuleCases(t, kernelGetInput, []ruleCase{
		{name: "naming no vendor", arguments: map[string]any{keyKernelID: "latest-64bit"}, want: form},
		{name: caseTraversal, arguments: map[string]any{keyKernelID: "linode/../x"}, want: form},
		{name: "a percent-encoded separator", arguments: map[string]any{keyKernelID: "linode/latest%2Fextra"}, want: form},
		{name: "padded still reads", arguments: map[string]any{keyKernelID: " linode/latest-64bit "}, want: ""},
		{name: "a kernel id", arguments: map[string]any{keyKernelID: "linode/latest-64bit"}, want: ""},
	})
}

func TestTheTwoMonthlyReadsShareOneWindow(t *testing.T) {
	t.Parallel()

	runRuleCases(t, statsMonthInput, []ruleCase{
		{
			name:      "a year below the epoch floor",
			arguments: map[string]any{keyLinodeID: float64(123), keyYear: float64(1969)}, want: yearRange,
		},
		{
			name:      "a year past four digits",
			arguments: map[string]any{keyLinodeID: float64(123), keyYear: float64(10000)}, want: yearRange,
		},
		{
			name:      "a month above the range",
			arguments: map[string]any{keyLinodeID: float64(123), keyYear: float64(2024), keyMonth: float64(13)},
			want:      monthRange,
		},
		{
			name:      "a bad year outranks the month",
			arguments: map[string]any{keyLinodeID: float64(123), keyYear: float64(1969), keyMonth: float64(13)},
			want:      yearRange,
		},
		{
			name:      "an unusable instance id outranks both",
			arguments: map[string]any{keyYear: float64(1969)}, want: "",
		},
		{
			name:      "a month of stats",
			arguments: map[string]any{keyLinodeID: float64(123), keyYear: float64(2024), keyMonth: float64(8)},
			want:      "",
		},
	})

	runRuleCases(t, transferMonthInput, []ruleCase{
		{
			name:      "the transfer route takes the same window",
			arguments: map[string]any{keyLinodeID: float64(1), keyYear: float64(1969)}, want: yearRange,
		},
		{
			name:      "a month above the range",
			arguments: map[string]any{keyLinodeID: float64(1), keyYear: float64(2024), keyMonth: float64(13)},
			want:      monthRange,
		},
		{
			name:      "a month of transfer",
			arguments: map[string]any{keyLinodeID: float64(1), keyYear: float64(2024), keyMonth: float64(5)},
			want:      "",
		},
	})
}

func TestShareGroupTokenRules(t *testing.T) {
	t.Parallel()

	runRuleCases(t, memberTokenInput, []ruleCase{
		{
			name:      "a path separator",
			arguments: map[string]any{keyShareGroup: float64(4), keyTokenUUID: "tokens/" + canonicalUUID},
			want: "token_uuid must not contain path separators, query separators," +
				" fragments, or traversal segments",
		},
		{
			name:      "not a uuid",
			arguments: map[string]any{keyShareGroup: float64(4), keyTokenUUID: notAUUID}, want: tokenUUIDForm,
		},
		{
			name:      "braced, which addresses nothing",
			arguments: map[string]any{keyShareGroup: float64(4), keyTokenUUID: "{" + canonicalUUID + "}"},
			want:      tokenUUIDForm,
		},
		{
			name:      "the token rules wait on the share group id",
			arguments: map[string]any{keyTokenUUID: notAUUID}, want: "",
		},
		{
			name:      "a member token",
			arguments: map[string]any{keyShareGroup: float64(4), keyTokenUUID: canonicalUUID}, want: "",
		},
	})

	const badUUID = "valid_for_sharegroup_uuid must be a valid UUID"

	// Derived rather than written out: a 32-character hex literal reads as a
	// credential to the secret scanner, and building it here also says what the
	// form is, which is the canonical spelling with its hyphens dropped.
	bareHex := strings.ReplaceAll(canonicalUUID, "-", "")
	wrongAlphabet := bareHex[:len(bareHex)-1] + "z"

	runRuleCases(t, tokenCreateInput, []ruleCase{
		{name: "not a uuid", arguments: map[string]any{keyShareUUID: notAUUID}, want: badUUID},
		{
			name:      "the wrong alphabet",
			arguments: map[string]any{keyShareUUID: wrongAlphabet}, want: badUUID,
		},
		{name: "the bare hex form", arguments: map[string]any{keyShareUUID: bareHex}, want: ""},
		{name: "the braced form", arguments: map[string]any{keyShareUUID: "{" + canonicalUUID + "}"}, want: ""},
		{name: "the urn form", arguments: map[string]any{keyShareUUID: "urn:uuid:" + canonicalUUID}, want: ""},
		{
			name:      "blank is refused before the form is judged",
			arguments: map[string]any{keyShareUUID: ""},
			want:      "valid_for_sharegroup_uuid must be a non-empty string",
		},
	})
}

func TestReverseDNSUpdateAnswersTheAddressFirst(t *testing.T) {
	t.Parallel()

	const badAddress = "address must be a valid IP address"

	runRuleCases(t, ipUpdateInput, []ruleCase{
		{
			name:      "a malformed address",
			arguments: map[string]any{keyAddress: "not-an-ip", keyRDNS: sampleHostname}, want: badAddress,
		},
		{
			name:      "a zoned address",
			arguments: map[string]any{keyAddress: "fe80::1%eth0", keyRDNS: sampleHostname}, want: badAddress,
		},
		{name: "no replacement", arguments: map[string]any{keyAddress: "203.0.113.5"}, want: "rdns is required"},
		{name: "neither sent", arguments: map[string]any{}, want: "address must be a non-empty string"},
		{
			name:      "an rdns update",
			arguments: map[string]any{keyAddress: "203.0.113.5", keyRDNS: sampleHostname}, want: "",
		},
	})
}

func TestThumbnailRuleReadsStandardBase64(t *testing.T) {
	t.Parallel()

	runRuleCases(t, thumbnailInput, []ruleCase{
		{
			name:      "outside the alphabet",
			arguments: map[string]any{keyClientID: sampleClient, keyThumbnail: "not!!base64"},
			want:      "thumbnail_png_base64 must be valid standard base64",
		},
		{
			name:      "blank is left to its reader",
			arguments: map[string]any{keyClientID: sampleClient, keyThumbnail: "   "}, want: "",
		},
		{
			name:      "a decodable image",
			arguments: map[string]any{keyClientID: sampleClient, keyThumbnail: "aGVsbG8="}, want: "",
		},
	})
}

func TestDNSRecordTargetRulesReadPerRecordType(t *testing.T) {
	t.Parallel()

	record := func(recordType string, target any) map[string]any {
		arguments := map[string]any{keyRecordDomainID: float64(5), keyRecordType: recordType}
		if target != nil {
			arguments[keyTarget] = target
		}

		return arguments
	}

	runRuleCases(t, recordCreateInput, []ruleCase{
		{
			name: "an A record wants an address", arguments: record("A", "example.com"),
			want: "a record target must be a valid IPv4 address",
		},
		{
			name: "an A record cannot point at reserved space", arguments: record("A", "192.0.2.1"),
			want: "a record target cannot be a private IP address",
		},
		{
			name: "an A record cannot point at RFC 1918 space", arguments: record("A", "172.20.0.1"),
			want: "a record target cannot be a private IP address",
		},
		{
			name: "an AAAA record refuses a 4-in-6 address", arguments: record("AAAA", "::ffff:8.8.8.8"),
			want: "aaaa record target must be a valid IPv6 address",
		},
		{
			name: "a CNAME record wants a hostname", arguments: record("CNAME", "not a host"),
			want: "CNAME" + hostnameTail,
		},
		{
			name: "an MX record wants a hostname", arguments: record("MX", "not a host"),
			want: "MX" + hostnameTail,
		},
		{name: "an omitted target", arguments: record("A", nil), want: "target is required"},
		{name: "a public A record", arguments: record("A", "8.8.8.8"), want: ""},
		{name: "an AAAA record", arguments: record("AAAA", "2001:db8::1"), want: ""},
		{
			name: "a CAA record leaves its tag to the membership reader",
			arguments: map[string]any{
				keyRecordDomainID: float64(5), keyRecordType: "CAA", keyTarget: "ca.example.com", "tag": "issuance",
			},
			want: "",
		},
	})
}
