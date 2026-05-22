package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
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
	require.NoError(t, err)

	report, ok := cfg.Audit.Reports["daily-destroys"]
	require.True(t, ok, "report must be parsed")
	assert.Equal(t, "Destructive ops in the last 24h", report.Description)
	assert.Equal(t, "destroy", report.Filter.Capability)
	assert.Equal(t, "24h", report.Filter.SinceOffset)
	assert.Equal(t, []string{"tool", "environment"}, report.GroupBy)
	assert.Equal(t, config.ReportOutputSummary, report.Output)
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
	require.NoError(t, err)
	assert.Equal(t, config.ReportOutputSummary, cfg.Audit.Reports["no-output"].Output)
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
			require.Error(t, err)
			require.ErrorIs(t, err, config.ErrConfigInvalid)
			require.ErrorIs(t, err, tcase.wantErr)
		})
	}
}
