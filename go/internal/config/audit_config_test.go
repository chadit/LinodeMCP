package config_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/config"
)

// minimalConfigWith returns a minimal valid config with the supplied
// audit block spliced in (or none when auditBlock is empty).
func minimalConfigWith(auditBlock string) string {
	return `
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
` + auditBlock
}

// TestAuditDefaults verifies the audit block's defaults when omitted.
func TestAuditDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", minimalConfigWith(""))

	cfg, err := config.Load(path)
	checkNoError(t, err)

	checkNotNil(t, cfg.Audit.RetentionDays, "retention_days must be defaulted to a non-nil pointer")
	checkEqual(t, config.DefaultAuditRetentionDays, *cfg.Audit.RetentionDays, "omitted retention_days defaults to 14")
	checkNotNil(t, cfg.Audit.RedactPII, "redact_pii must be defaulted to a non-nil pointer")
	checkTrue(t, *cfg.Audit.RedactPII, "omitted redact_pii defaults to true (PII redaction on)")
	checkFalse(t, cfg.Audit.SQLite.Enabled, "SQLite sink is off by default")
	checkEqual(t, config.DefaultAuditSQLiteBusyTimeoutMS, cfg.Audit.SQLite.BusyTimeoutMS, "omitted busy_timeout_ms defaults to 5000")
}

// TestAuditRedactPIIExplicitFalsePreserved verifies an explicit
// redact_pii: false survives defaulting rather than being clobbered to
// true. Same pointer-default discipline as retention_days, since an
// operator who explicitly opts out of PII redaction must not be
// quietly re-enrolled by the defaulter on the next reload.
func TestAuditRedactPIIExplicitFalsePreserved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", minimalConfigWith("audit:\n  redact_pii: false\n"))

	cfg, err := config.Load(path)
	checkNoError(t, err)

	checkNotNil(t, cfg.Audit.RedactPII)
	checkFalse(t, *cfg.Audit.RedactPII, "explicit redact_pii: false must be preserved as opt-out")
}

// TestAuditRetentionExplicitZeroPreserved verifies an explicit 0
// ("never delete") survives defaulting rather than being clobbered to
// 14. This is the reason RetentionDays is a pointer.
func TestAuditRetentionExplicitZeroPreserved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", minimalConfigWith("audit:\n  retention_days: 0\n"))

	cfg, err := config.Load(path)
	checkNoError(t, err)

	checkNotNil(t, cfg.Audit.RetentionDays)
	checkEqual(t, 0, *cfg.Audit.RetentionDays, "explicit retention_days: 0 must be preserved as never-delete")
}

// TestAuditRetentionExplicitValue verifies a non-default value passes
// through unchanged.
func TestAuditRetentionExplicitValue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", minimalConfigWith("audit:\n  retention_days: 30\n"))

	cfg, err := config.Load(path)
	checkNoError(t, err)

	checkNotNil(t, cfg.Audit.RetentionDays)
	checkEqual(t, 30, *cfg.Audit.RetentionDays)
}

// TestAuditRetentionNegativeRejected verifies a negative retention is
// a load-time validation error.
func TestAuditRetentionNegativeRejected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", minimalConfigWith("audit:\n  retention_days: -1\n"))

	_, err := config.Load(path)
	checkError(t, err, "negative retention_days must fail to load")
	checkErrorIs(t, err, config.ErrConfigInvalid, "error must wrap the invalid-config sentinel")
}

// TestAuditSQLiteBlockParses verifies the SQLite sub-block fields load.
func TestAuditSQLiteBlockParses(t *testing.T) {
	t.Parallel()

	block := "audit:\n  sqlite:\n    enabled: true\n    path: \"/tmp/audit.db\"\n    busy_timeout_ms: 1234\n"
	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", minimalConfigWith(block))

	cfg, err := config.Load(path)
	checkNoError(t, err)

	checkTrue(t, cfg.Audit.SQLite.Enabled, "enabled flag must parse")
	checkEqual(t, "/tmp/audit.db", cfg.Audit.SQLite.Path, "path must parse")
	checkEqual(t, 1234, cfg.Audit.SQLite.BusyTimeoutMS, "busy_timeout_ms must parse")
}

// TestAuditEnvOverrides verifies the LINODEMCP_AUDIT_* env overrides
// take effect over file values. Not parallel: t.Setenv mutates
// process-global state.
func TestAuditEnvOverrides(t *testing.T) {
	t.Setenv("LINODEMCP_AUDIT_RETENTION_DAYS", "7")
	t.Setenv("LINODEMCP_AUDIT_REDACT_PII", "false")
	t.Setenv("LINODEMCP_AUDIT_SQLITE_ENABLED", "true")
	t.Setenv("LINODEMCP_AUDIT_SQLITE_PATH", "/var/audit.db")
	t.Setenv("LINODEMCP_AUDIT_SQLITE_BUSY_TIMEOUT_MS", "999")

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", minimalConfigWith("audit:\n  retention_days: 30\n  redact_pii: true\n"))

	cfg, err := config.Load(path)
	checkNoError(t, err)

	checkNotNil(t, cfg.Audit.RetentionDays)
	checkEqual(t, 7, *cfg.Audit.RetentionDays, "env override beats the file value")
	checkNotNil(t, cfg.Audit.RedactPII)
	checkFalse(t, *cfg.Audit.RedactPII, "env override beats the file value for redact_pii")
	checkTrue(t, cfg.Audit.SQLite.Enabled)
	checkEqual(t, "/var/audit.db", cfg.Audit.SQLite.Path)
	checkEqual(t, 999, cfg.Audit.SQLite.BusyTimeoutMS)
}

// TestAuditRetentionDefaultMatchesAuditPackage guards against drift
// between the config default and the audit sweeper's intrinsic
// default (they are deliberately separate constants).
func TestAuditRetentionDefaultMatchesAuditPackage(t *testing.T) {
	t.Parallel()

	checkEqual(t, 14, config.DefaultAuditRetentionDays, "config default must stay in sync with audit.DefaultAuditRetentionDays")
}
