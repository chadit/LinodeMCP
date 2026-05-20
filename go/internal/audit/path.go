package audit

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

// systemServiceUIDThreshold is the UID cutoff that distinguishes
// system daemons (typically UID < 1000) from interactive users. A
// server running under a UID below this threshold gets the
// /var/log/linodemcp directory; everyone else writes under their
// XDG state directory.
const systemServiceUIDThreshold = 1000

// SystemAuditDir is the path used when the server runs as a system
// daemon. Matches the spec wording verbatim.
const SystemAuditDir = "/var/log/linodemcp"

// UserAuditDirRelative is the path appended to XDG_STATE_HOME (or
// ~/.local/state) for non-system users.
const UserAuditDirRelative = "linodemcp"

// ResolveDefaultAuditDir picks the right directory for the audit
// log on the current host. Resolution order:
//
//  1. If $XDG_STATE_HOME is set, use $XDG_STATE_HOME/linodemcp.
//     Explicit user intent always wins. Daemons typically don't
//     set this so they fall through to step 2.
//  2. If the process appears to be running as a system service
//     (POSIX heuristic: UID below the system-service threshold),
//     use /var/log/linodemcp.
//  3. Otherwise use $HOME/.local/state/linodemcp.
//
// The function never returns an error. Path-creation failures
// surface later from NewJSONLSink, where the caller has enough
// context to decide whether to bail or to fall back to NoopSink.
func ResolveDefaultAuditDir() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, UserAuditDirRelative)
	}

	if isSystemService() {
		return SystemAuditDir
	}

	return userAuditDir()
}

// isSystemService is the POSIX heuristic from the spec: UID < 1000
// means we're running as a daemon. Windows always returns false
// because the UID concept doesn't apply; Windows hosts get the user
// path regardless.
func isSystemService() bool {
	current, err := user.Current()
	if err != nil {
		return false
	}

	uid, err := strconv.Atoi(current.Uid)
	if err != nil {
		// Non-numeric UID (Windows SID-style). Treat as non-system.
		return false
	}

	return uid < systemServiceUIDThreshold
}

// userAuditDir returns the XDG-style path for an interactive user.
// $XDG_STATE_HOME handling already happened in the caller, so this
// function only handles the home-directory fallback path. If the
// home directory can't be resolved (rare; happens in stripped-down
// containers), the function returns the working directory as a
// last resort so the path string isn't empty.
func userAuditDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// Last-resort fallback: current directory. Better than
		// returning "" since callers feed this into MkdirAll.
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return "." + string(filepath.Separator) + UserAuditDirRelative
		}

		return filepath.Join(cwd, UserAuditDirRelative)
	}

	return filepath.Join(home, ".local", "state", UserAuditDirRelative)
}
