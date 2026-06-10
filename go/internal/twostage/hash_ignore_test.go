package twostage_test

import (
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/internal/twostage"
)

// TestHashIgnoreFields guards the per-resource-type cosmetic-field lists. A
// regression here (a dropped field, a renamed key) would otherwise only surface
// as a false drift refusal at apply time, so assert the lists directly.
func TestHashIgnoreFields(t *testing.T) {
	t.Parallel()

	const (
		fieldUpdated = "updated"
		fieldCreated = "created"
	)

	cases := []struct {
		resourceType string
		mustContain  []string
	}{
		{"Instance", []string{fieldUpdated, "last_seen_ipv4", "watchdog_enabled", "host_uuid"}},
		{"Volume", []string{fieldUpdated, "last_seen_ipv4"}},
		{"LKECluster", []string{fieldUpdated, fieldCreated}},
		{"Firewall", []string{fieldUpdated}},
		{"NodeBalancer", []string{fieldUpdated, "transfer"}},
		{"VPC", []string{fieldUpdated}},
		{"Domain", []string{fieldUpdated}},
		{"StackScript", []string{fieldUpdated, "deployments_total"}},
		{"Disk", []string{fieldUpdated}},
		{"VPCSubnet", []string{fieldUpdated}},
		{"DomainRecord", []string{fieldUpdated}},
		{"LKENodePool", []string{"nodes"}},
		{"DatabaseInstance", []string{fieldUpdated}},
		{"ImageShareGroup", []string{fieldUpdated}},
		{"ImageShareGroupToken", []string{fieldUpdated}},
		{"FirewallDevice", []string{fieldUpdated}},
		{"LKEKubeconfig", []string{fieldUpdated, fieldCreated}},
		{"LKEServiceToken", []string{fieldUpdated, fieldCreated}},
	}

	for _, testCase := range cases {
		t.Run(testCase.resourceType, func(t *testing.T) {
			t.Parallel()

			got := twostage.HashIgnoreFields(testCase.resourceType)
			for _, field := range testCase.mustContain {
				if !slices.Contains(got, field) {
					t.Errorf("HashIgnoreFields(%q) = %v, missing %q", testCase.resourceType, got, field)
				}
			}
		})
	}
}

// TestHashIgnoreFieldsUnknownTypeReturnsNil confirms an unmapped resource type
// hashes its whole state (no fields stripped), the safe conservative default.
func TestHashIgnoreFieldsUnknownTypeReturnsNil(t *testing.T) {
	t.Parallel()

	if got := twostage.HashIgnoreFields("NoSuchResource"); got != nil {
		t.Errorf("HashIgnoreFields(unknown) = %v, want nil", got)
	}
}
