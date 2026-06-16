package audit_test

import (
	"path/filepath"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/audit"
)

// TestResolveDefaultAuditDirHonorsXDGStateHome verifies that when
// XDG_STATE_HOME is set, the resolver appends linodemcp under it.
// Interactive users typically run with a UID >= 1000 so the
// "system service" branch shouldn't fire on the dev box this test
// runs on.
func TestResolveDefaultAuditDirHonorsXDGStateHome(t *testing.T) {
	customState := filepath.Join(t.TempDir(), "state")
	t.Setenv("XDG_STATE_HOME", customState)

	got := audit.ResolveDefaultAuditDir()

	expected := filepath.Join(customState, audit.UserAuditDirRelative)
	if got != expected {
		t.Errorf("got = %v, want %v", got, expected)
	}
}

// TestResolveDefaultAuditDirFallsBackToHomeDir verifies the
// home-dir fallback: when XDG_STATE_HOME is empty, the resolver
// uses $HOME/.local/state/linodemcp. The system-service branch is
// gated by UID and out of scope for this test (test box is
// interactive).
func TestResolveDefaultAuditDirFallsBackToHomeDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", fakeHome)

	got := audit.ResolveDefaultAuditDir()

	// Resolver may pick the system path on hosts with low UID, so
	// the assertion is permissive on absolute prefix: it must either
	// be the system path, or the home-relative path. Anything else
	// is a bug.
	systemPath := audit.SystemAuditDir
	homePath := filepath.Join(fakeHome, ".local", "state", audit.UserAuditDirRelative)

	if got != systemPath && got != homePath {
		t.Fatal("expected condition to be true")
	}
}

// TestUserAuditDirRelativeConstantValue pins the constant value so a
// rename of the directory name (which would orphan existing logs on
// upgrade) is visible in code review.
func TestUserAuditDirRelativeConstantValue(t *testing.T) {
	t.Parallel()

	if audit.UserAuditDirRelative != "linodemcp" {
		t.Errorf("audit.UserAuditDirRelative = %v, want %v", audit.UserAuditDirRelative, "linodemcp")
	}
}

// TestSystemAuditDirConstantValue pins the system path so a change
// from /var/log/linodemcp gets flagged in review.
func TestSystemAuditDirConstantValue(t *testing.T) {
	t.Parallel()

	if audit.SystemAuditDir != "/var/log/linodemcp" {
		t.Errorf("audit.SystemAuditDir = %v, want %v", audit.SystemAuditDir, "/var/log/linodemcp")
	}
}
