package audit_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/audit"
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
	assert.Equal(t, expected, got, "audit dir must be $XDG_STATE_HOME/linodemcp when set")
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

	require.True(
		t,
		got == systemPath || got == homePath,
		"audit dir must be system path or home-based path; got %q", got,
	)
}

// TestUserAuditDirRelativeConstantValue pins the constant value so a
// rename of the directory name (which would orphan existing logs on
// upgrade) is visible in code review.
func TestUserAuditDirRelativeConstantValue(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t, "linodemcp", audit.UserAuditDirRelative,
		"UserAuditDirRelative is part of the on-disk layout contract; "+
			"a rename here breaks existing deployments",
	)
}

// TestSystemAuditDirConstantValue pins the system path so a change
// from /var/log/linodemcp gets flagged in review.
func TestSystemAuditDirConstantValue(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t, "/var/log/linodemcp", audit.SystemAuditDir,
		"SystemAuditDir is part of the on-disk layout contract; "+
			"a change here breaks existing system-service deployments",
	)
}
