package linoderoute_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
)

// TestRetryDisabledReadsTheDeclaredPolicy pins the answer against real tools
// rather than a fixture: the routed write primitive picks its retry executor
// from this, so a policy read off the wrong message would replay a create.
func TestRetryDisabledReadsTheDeclaredPolicy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tool string
		want bool
	}{
		// POST /domains assigns the id, so a replay leaves a second zone.
		{tool: "linode_domain_create", want: true},
		{tool: "linode_domain_record_create", want: true},
		// PUT /domains/{id} addresses a resource the caller named, so a
		// replayed attempt lands on the same one.
		{tool: "linode_domain_update", want: false},
		{tool: "linode_domain_get", want: false},
	}

	for _, testCase := range cases {
		t.Run(testCase.tool, func(t *testing.T) {
			t.Parallel()

			if got := linoderoute.RetryDisabled(testCase.tool); got != testCase.want {
				t.Errorf("RetryDisabled(%q) = %t, want %t", testCase.tool, got, testCase.want)
			}
		})
	}
}

// TestRetryDisabledAnswersFalseForAnUnknownTool: the routed call resolves the
// same name a moment later and fails there, so this answer never reaches the
// API. Pinning it keeps a future reader from turning the miss into a panic.
func TestRetryDisabledAnswersFalseForAnUnknownTool(t *testing.T) {
	t.Parallel()

	if linoderoute.RetryDisabled("linode_not_a_tool") {
		t.Error("RetryDisabled reported a policy for a tool the contract does not declare")
	}
}
