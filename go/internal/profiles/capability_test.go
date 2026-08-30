package profiles_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// TestCapabilityStringNamesEveryMember pins the diagnostic name of each
// capability. Error messages and the registration invariant report print
// these, so a renamed or reordered constant would change what an operator
// reads without any compile error.
func TestCapabilityStringNamesEveryMember(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		want       string
		capability profiles.Capability
	}{
		{name: "zero value is the untagged marker", capability: profiles.CapUnknown, want: "CapUnknown"},
		{name: "read", capability: profiles.CapRead, want: "CapRead"},
		{name: "write", capability: profiles.CapWrite, want: "CapWrite"},
		{name: "destroy", capability: profiles.CapDestroy, want: "CapDestroy"},
		{name: "admin", capability: profiles.CapAdmin, want: "CapAdmin"},
		{name: "meta", capability: profiles.CapMeta, want: "CapMeta"},
		{name: "value past the last member reports invalid", capability: profiles.Capability(99), want: "Capability(invalid)"},
		{name: "negative value reports invalid", capability: profiles.Capability(-1), want: "Capability(invalid)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.capability.String(); got != tt.want {
				t.Errorf("Capability(%d).String() = %q, want %q", int(tt.capability), got, tt.want)
			}
		})
	}
}
