package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
)

func validYAMLConfig() string {
	return `
server:
  name: "TestServer"
  logLevel: "debug"
  transport: "stdio"
  host: "0.0.0.0"
  port: 9000
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "test-token-123"
`
}

func validJSONConfig() string {
	return `{
  "server": {"name": "JSONServer", "log_level": "warn"},
  "environments": {
    "default": {
      "label": "Default",
      "linode": {"api_url": "https://api.linode.com/v4", "token": "json-token"}
    }
  }
}`
}

func writeConfigFile(t *testing.T, dir, filename, content string) string {
	t.Helper()

	path := filepath.Join(dir, filename)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestLoadFromFile(t *testing.T) {
	t.Parallel()

	t.Run("valid YAML", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

		cfg, err := config.Load(path)

		require.NoError(t, err)
		assert.Equal(t, "TestServer", cfg.Server.Name)
		assert.Equal(t, "debug", cfg.Server.LogLevel)
		assert.Equal(t, 9000, cfg.Server.Port)
		assert.Equal(t, "test-token-123", cfg.Environments["default"].Linode.Token)
	})

	t.Run("valid JSON", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeConfigFile(t, dir, "config.json", validJSONConfig())

		cfg, err := config.Load(path)

		require.NoError(t, err)
		assert.Equal(t, "JSONServer", cfg.Server.Name)
		assert.Equal(t, "warn", cfg.Server.LogLevel)
	})

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()

		minimalYAML := `
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
`
		dir := t.TempDir()
		path := writeConfigFile(t, dir, "config.yml", minimalYAML)

		cfg, err := config.Load(path)

		require.NoError(t, err)
		assert.Equal(t, config.DefaultServerName, cfg.Server.Name)
		assert.Equal(t, config.DefaultLogLevel, cfg.Server.LogLevel)
		assert.Equal(t, config.DefaultTransport, cfg.Server.Transport)
		assert.Equal(t, config.DefaultHost, cfg.Server.Host)
		assert.Equal(t, config.DefaultServerPort, cfg.Server.Port)
		assert.Equal(t, config.DefaultMaxRetries, cfg.Resilience.MaxRetries)
		assert.Equal(t, config.DefaultBaseRetryDelay, cfg.Resilience.BaseRetryDelay)
		assert.Equal(t, config.DefaultMaxRetryDelay, cfg.Resilience.MaxRetryDelay)
	})

	t.Run("file not found", func(t *testing.T) {
		t.Parallel()

		_, err := config.Load("/tmp/nonexistent-linodemcp-config-test.yml")
		require.Error(t, err)
	})

	t.Run("malformed YAML", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeConfigFile(t, dir, "config.yml", `{{{invalid yaml`)

		_, err := config.Load(path)
		require.Error(t, err)
		assert.ErrorIs(t, err, config.ErrConfigMalformed)
	})

	t.Run("no environments", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeConfigFile(t, dir, "config.yml", `
server:
  name: "Test"
  logLevel: "info"
`)

		_, err := config.Load(path)
		require.Error(t, err)
		assert.ErrorIs(t, err, config.ErrConfigInvalid)
	})

	t.Run("incomplete linode config", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeConfigFile(t, dir, "config.yml", `
server:
  name: "Test"
  logLevel: "info"
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
`)

		_, err := config.Load(path)
		require.Error(t, err)
		assert.ErrorIs(t, err, config.ErrConfigInvalid)
	})

	t.Run("unknown fields ignored", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := writeConfigFile(t, dir, "config.yml", `
unknownField: "foo"
server:
  name: "TestServer"
  logLevel: "debug"
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
`)

		cfg, err := config.Load(path)

		require.NoError(t, err)
		assert.Equal(t, "TestServer", cfg.Server.Name)
	})
}

func TestSelectEnvironment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       *config.Config
		input     string
		wantLabel string
		wantErr   error
	}{
		{
			name: "exact match",
			cfg: &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					"prod":    {Label: "Production"},
					"staging": {Label: "Staging"},
				},
			},
			input:     "prod",
			wantLabel: "Production",
		},
		{
			name: "case insensitive",
			cfg: &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					"Production": {Label: "Prod"},
				},
			},
			input:     "production",
			wantLabel: "Prod",
		},
		{
			name: "falls back to default",
			cfg: &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					"default": {Label: "Default Env"},
					"other":   {Label: "Other"},
				},
			},
			input:     "nonexistent",
			wantLabel: "Default Env",
		},
		{
			name: "empty input",
			cfg: &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					"default": {Label: "Default"},
				},
			},
			input:   "",
			wantErr: config.ErrEmptyEnvironmentName,
		},
		{
			name:    "no environments",
			cfg:     &config.Config{},
			input:   "anything",
			wantErr: config.ErrEnvironmentNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env, err := tt.cfg.SelectEnvironment(tt.input)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantLabel, env.Label)
		})
	}
}

func TestGetConfigPathWithEnvOverride(t *testing.T) {
	dir := t.TempDir()
	customPath := filepath.Join(dir, "custom-config.yml")
	require.NoError(t, os.WriteFile(customPath, []byte(""), 0o600))

	t.Setenv("LINODEMCP_CONFIG_PATH", customPath)

	assert.Equal(t, customPath, config.GetConfigPath())
}

func TestApplyEnvironmentOverrides(t *testing.T) {
	t.Setenv("LINODEMCP_SERVER_NAME", "EnvServer")
	t.Setenv("LINODEMCP_LOG_LEVEL", "error")
	t.Setenv("LINODEMCP_LINODE_API_URL", "https://override.api.com")
	t.Setenv("LINODEMCP_LINODE_TOKEN", "env-token")

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", `
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
`)

	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, "EnvServer", cfg.Server.Name)
	assert.Equal(t, "error", cfg.Server.LogLevel)
	assert.Equal(t, "https://override.api.com", cfg.Environments["default"].Linode.APIURL)
	assert.Equal(t, "env-token", cfg.Environments["default"].Linode.Token)
}
