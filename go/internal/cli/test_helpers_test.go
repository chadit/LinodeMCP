package cli_test

import (
	"strings"
	"testing"
)

func wantContains(t *testing.T, label, got, want string) {
	t.Helper()

	if !strings.Contains(got, want) {
		t.Fatalf("%s does not contain %q", label, want)
	}
}

func wantNotContains(t *testing.T, label, got, unwanted string) {
	t.Helper()

	if strings.Contains(got, unwanted) {
		t.Fatalf("%s contains %q", label, unwanted)
	}
}
