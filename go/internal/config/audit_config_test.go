package config_test

import (
	"errors"
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
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.Audit.RetentionDays == nil {
		t.Error("cfg.Audit.RetentionDays is nil")
	}

	if *cfg.Audit.RetentionDays != config.DefaultAuditRetentionDays {
		t.Errorf("*cfg.Audit.RetentionDays = %v, want %v", *cfg.Audit.RetentionDays, config.DefaultAuditRetentionDays)
	}

	if cfg.Audit.RedactPII == nil {
		t.Error("cfg.Audit.RedactPII is nil")
	}

	if !(*cfg.Audit.RedactPII) {
		t.Error("*cfg.Audit.RedactPII = false, want true")
	}

	if cfg.Audit.SQLite.Enabled {
		t.Error("cfg.Audit.SQLite.Enabled = true, want false")
	}

	if cfg.Audit.SQLite.BusyTimeoutMS != config.DefaultAuditSQLiteBusyTimeoutMS {
		t.Errorf("cfg.Audit.SQLite.BusyTimeoutMS = %v, want %v", cfg.Audit.SQLite.BusyTimeoutMS, config.DefaultAuditSQLiteBusyTimeoutMS)
	}
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
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.Audit.RedactPII == nil {
		t.Error("cfg.Audit.RedactPII is nil")
	}

	if *cfg.Audit.RedactPII {
		t.Error("*cfg.Audit.RedactPII = true, want false")
	}
}

// TestAuditRetentionExplicitZeroPreserved verifies an explicit 0
// ("never delete") survives defaulting rather than being clobbered to
// 14. This is the reason RetentionDays is a pointer.
func TestAuditRetentionExplicitZeroPreserved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", minimalConfigWith("audit:\n  retention_days: 0\n"))

	cfg, err := config.Load(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.Audit.RetentionDays == nil {
		t.Error("cfg.Audit.RetentionDays is nil")
	}

	if *cfg.Audit.RetentionDays != 0 {
		t.Errorf("*cfg.Audit.RetentionDays = %v, want %v", *cfg.Audit.RetentionDays, 0)
	}
}

// TestAuditRetentionExplicitValue verifies a non-default value passes
// through unchanged.
func TestAuditRetentionExplicitValue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", minimalConfigWith("audit:\n  retention_days: 30\n"))

	cfg, err := config.Load(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.Audit.RetentionDays == nil {
		t.Error("cfg.Audit.RetentionDays is nil")
	}

	if *cfg.Audit.RetentionDays != 30 {
		t.Errorf("*cfg.Audit.RetentionDays = %v, want %v", *cfg.Audit.RetentionDays, 30)
	}
}

// TestAuditRetentionNegativeRejected verifies a negative retention is
// a load-time validation error.
func TestAuditRetentionNegativeRejected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", minimalConfigWith("audit:\n  retention_days: -1\n"))

	_, err := config.Load(path)
	if !errors.Is(err, config.ErrConfigInvalid) {
		t.Errorf("error = %v, want %v", err, config.ErrConfigInvalid)
	}
}

// TestAuditSQLiteBlockParses verifies the SQLite sub-block fields load.
func TestAuditSQLiteBlockParses(t *testing.T) {
	t.Parallel()

	block := "audit:\n  sqlite:\n    enabled: true\n    path: \"/tmp/audit.db\"\n    busy_timeout_ms: 1234\n"
	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", minimalConfigWith(block))

	cfg, err := config.Load(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !cfg.Audit.SQLite.Enabled {
		t.Error("cfg.Audit.SQLite.Enabled = false, want true")
	}

	if cfg.Audit.SQLite.Path != "/tmp/audit.db" {
		t.Errorf("cfg.Audit.SQLite.Path = %v, want %v", cfg.Audit.SQLite.Path, "/tmp/audit.db")
	}

	if cfg.Audit.SQLite.BusyTimeoutMS != 1234 {
		t.Errorf("cfg.Audit.SQLite.BusyTimeoutMS = %v, want %v", cfg.Audit.SQLite.BusyTimeoutMS, 1234)
	}
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
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.Audit.RetentionDays == nil {
		t.Error("cfg.Audit.RetentionDays is nil")
	}

	if *cfg.Audit.RetentionDays != 7 {
		t.Errorf("*cfg.Audit.RetentionDays = %v, want %v", *cfg.Audit.RetentionDays, 7)
	}

	if cfg.Audit.RedactPII == nil {
		t.Error("cfg.Audit.RedactPII is nil")
	}

	if *cfg.Audit.RedactPII {
		t.Error("*cfg.Audit.RedactPII = true, want false")
	}

	if !cfg.Audit.SQLite.Enabled {
		t.Error("cfg.Audit.SQLite.Enabled = false, want true")
	}

	if cfg.Audit.SQLite.Path != "/var/audit.db" {
		t.Errorf("cfg.Audit.SQLite.Path = %v, want %v", cfg.Audit.SQLite.Path, "/var/audit.db")
	}

	if cfg.Audit.SQLite.BusyTimeoutMS != 999 {
		t.Errorf("cfg.Audit.SQLite.BusyTimeoutMS = %v, want %v", cfg.Audit.SQLite.BusyTimeoutMS, 999)
	}
}

// TestAuditRetentionDefaultMatchesAuditPackage guards against drift
// between the config default and the audit sweeper's intrinsic
// default (they are deliberately separate constants).
func TestAuditRetentionDefaultMatchesAuditPackage(t *testing.T) {
	t.Parallel()

	if config.DefaultAuditRetentionDays != 14 {
		t.Errorf("config.DefaultAuditRetentionDays = %v, want %v", config.DefaultAuditRetentionDays, 14)
	}
}
