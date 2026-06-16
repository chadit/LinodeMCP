package config_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
)

// reportBaseYAML is a minimal valid config with an audit.reports block
// spliced in. The base supplies the environment the loader requires.
func reportBaseYAML(reportsBlock string) string {
	return `
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
audit:
  reports:
` + reportsBlock
}

// TestLoadReportsParse confirms a custom report round-trips from YAML
// into the typed config, including the filter grammar fields.
func TestLoadReportsParse(t *testing.T) {
	t.Parallel()

	yaml := reportBaseYAML(`    daily-destroys:
      description: "Destructive ops in the last 24h"
      filter:
        capability: "destroy"
        since_offset: "24h"
      group_by: ["tool", "environment"]
      output: "summary"
`)

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", yaml)

	cfg, err := config.Load(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	report, ok := cfg.Audit.Reports["daily-destroys"]
	if !ok {
		t.Error("ok = false, want true")
	}

	if report.Description != "Destructive ops in the last 24h" {
		t.Errorf("report.Description = %v, want %v", report.Description, "Destructive ops in the last 24h")
	}

	if report.Filter.Capability != "destroy" {
		t.Errorf("report.Filter.Capability = %v, want %v", report.Filter.Capability, "destroy")
	}

	if report.Filter.SinceOffset != "24h" {
		t.Errorf("report.Filter.SinceOffset = %v, want %v", report.Filter.SinceOffset, "24h")
	}

	if !reflect.DeepEqual(report.GroupBy, []string{"tool", "environment"}) {
		t.Errorf("report.GroupBy = %v, want %v", report.GroupBy, []string{"tool", "environment"})
	}

	if report.Output != config.ReportOutputSummary {
		t.Errorf("report.Output = %v, want %v", report.Output, config.ReportOutputSummary)
	}
}

// TestLoadReportsDefaultOutput confirms an omitted output defaults to
// summary.
func TestLoadReportsDefaultOutput(t *testing.T) {
	t.Parallel()

	yaml := reportBaseYAML(`    no-output:
      filter:
        capability: "read"
`)

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", yaml)

	cfg, err := config.Load(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.Audit.Reports["no-output"].Output != config.ReportOutputSummary {
		t.Errorf("got %v, want %v", cfg.Audit.Reports["no-output"].Output, config.ReportOutputSummary)
	}
}

// TestLoadReportsValidation rejects malformed report grammar with the
// expected reason in the error message.
func TestLoadReportsValidation(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		reportsBlock string
		wantErr      error
	}{
		"bad output": {
			reportsBlock: `    bad-out:
      output: "csv"
`,
			wantErr: config.ErrInvalidReportOutput,
		},
		"scalar and list": {
			reportsBlock: `    both-cap:
      output: "list"
      filter:
        capability: "destroy"
        capability_in: ["read", "write"]
`,
			wantErr: config.ErrReportScalarAndList,
		},
		"bad duration": {
			reportsBlock: `    bad-dur:
      output: "summary"
      filter:
        since_offset: "yesterday"
`,
			wantErr: config.ErrInvalidReportDuration,
		},
		"bad timestamp": {
			reportsBlock: `    bad-ts:
      output: "summary"
      filter:
        since: "not-a-date"
`,
			wantErr: config.ErrInvalidReportTimestamp,
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := writeConfigFile(t, dir, "config.yml", reportBaseYAML(tcase.reportsBlock))

			_, err := config.Load(path)
			if !errors.Is(err, config.ErrConfigInvalid) {
				t.Fatalf("error = %v, want %v", err, config.ErrConfigInvalid)
			}

			if !errors.Is(err, tcase.wantErr) {
				t.Errorf("error = %v, want %v", err, tcase.wantErr)
			}
		})
	}
}
