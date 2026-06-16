package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
)

// Shared test constants. Pulled out so goconst stops flagging the
// same string recurring in config_test.go and write_test.go.
const (
	envLabelProduction = "Production"
	envKeyDefault      = "default"
	envLabelDefault    = "Default"
	apiURLLinodeV4     = "https://api.linode.com/v4"
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
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	return path
}

func TestLoadFromFileValidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	cfg, err := config.Load(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.Server.Name != tcTestServer {
		t.Errorf("cfg.Server.Name = %v, want %v", cfg.Server.Name, tcTestServer)
	}

	if cfg.Server.LogLevel != "debug" {
		t.Errorf("cfg.Server.LogLevel = %v, want %v", cfg.Server.LogLevel, "debug")
	}

	if cfg.Server.Port != 9000 {
		t.Errorf("cfg.Server.Port = %v, want %v", cfg.Server.Port, 9000)
	}

	if cfg.Environments["default"].Linode.Token != "test-token-123" {
		t.Errorf("got %v, want %v", cfg.Environments["default"].Linode.Token, "test-token-123")
	}
}

func TestLoadFromFileValidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.json", validJSONConfig())

	cfg, err := config.Load(path)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.Server.Name != "JSONServer" {
		t.Errorf("cfg.Server.Name = %v, want %v", cfg.Server.Name, "JSONServer")
	}

	if cfg.Server.LogLevel != "warn" {
		t.Errorf("cfg.Server.LogLevel = %v, want %v", cfg.Server.LogLevel, "warn")
	}
}

func TestLoadFromFileDefaults(t *testing.T) {
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
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.Server.Name != config.DefaultServerName {
		t.Errorf("cfg.Server.Name = %v, want %v", cfg.Server.Name, config.DefaultServerName)
	}

	if cfg.Server.LogLevel != config.DefaultLogLevel {
		t.Errorf("cfg.Server.LogLevel = %v, want %v", cfg.Server.LogLevel, config.DefaultLogLevel)
	}

	if cfg.Server.Transport != config.DefaultTransport {
		t.Errorf("cfg.Server.Transport = %v, want %v", cfg.Server.Transport, config.DefaultTransport)
	}

	if cfg.Server.Host != config.DefaultHost {
		t.Errorf("cfg.Server.Host = %v, want %v", cfg.Server.Host, config.DefaultHost)
	}

	if cfg.Server.Port != config.DefaultServerPort {
		t.Errorf("cfg.Server.Port = %v, want %v", cfg.Server.Port, config.DefaultServerPort)
	}

	if cfg.Resilience.MaxRetries != config.DefaultMaxRetries {
		t.Errorf("cfg.Resilience.MaxRetries = %v, want %v", cfg.Resilience.MaxRetries, config.DefaultMaxRetries)
	}

	if cfg.Resilience.BaseRetryDelay != config.DefaultBaseRetryDelay {
		t.Errorf("cfg.Resilience.BaseRetryDelay = %v, want %v", cfg.Resilience.BaseRetryDelay, config.DefaultBaseRetryDelay)
	}

	if cfg.Resilience.MaxRetryDelay != config.DefaultMaxRetryDelay {
		t.Errorf("cfg.Resilience.MaxRetryDelay = %v, want %v", cfg.Resilience.MaxRetryDelay, config.DefaultMaxRetryDelay)
	}
}

func TestLoadFromFileFileNotFound(t *testing.T) {
	t.Parallel()

	_, err := config.Load("/tmp/nonexistent-linodemcp-config-test.yml")
	if err == nil {
		t.Error("expected an error, got nil")
	}
}

func TestLoadFromFileMalformedYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", `{{{invalid yaml`)

	_, err := config.Load(path)
	if !errors.Is(err, config.ErrConfigMalformed) {
		t.Errorf("error = %v, want %v", err, config.ErrConfigMalformed)
	}
}

func TestLoadFromFileNoEnvironments(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", `
server:
  name: "Test"
  logLevel: "info"
`)

	_, err := config.Load(path)
	if !errors.Is(err, config.ErrConfigInvalid) {
		t.Errorf("error = %v, want %v", err, config.ErrConfigInvalid)
	}
}

func TestLoadFromFileIncompleteLinodeConfig(t *testing.T) {
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
	if !errors.Is(err, config.ErrConfigInvalid) {
		t.Errorf("error = %v, want %v", err, config.ErrConfigInvalid)
	}
}

func TestLoadFromFileUnknownFieldsIgnored(t *testing.T) {
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
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.Server.Name != tcTestServer {
		t.Errorf("cfg.Server.Name = %v, want %v", cfg.Server.Name, tcTestServer)
	}
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
					"prod":    {Label: envLabelProduction},
					"staging": {Label: "Staging"},
				},
			},
			input:     "prod",
			wantLabel: envLabelProduction,
		},
		{
			name: "case insensitive",
			cfg: &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					envLabelProduction: {Label: "Prod"},
				},
			},
			input:     "production",
			wantLabel: "Prod",
		},
		{
			name: "falls back to default",
			cfg: &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {Label: "Default Env"},
					"other":       {Label: "Other"},
				},
			},
			input:     "nonexistent",
			wantLabel: "Default Env",
		},
		{
			name: "empty input",
			cfg: &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {Label: envLabelDefault},
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
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if env.Label != tt.wantLabel {
				t.Errorf("env.Label = %v, want %v", env.Label, tt.wantLabel)
			}
		})
	}
}

func TestPathWithEnvOverride(t *testing.T) {
	dir := t.TempDir()

	customPath := filepath.Join(dir, "custom-config.yml")
	if err := os.WriteFile(customPath, []byte(""), 0o600); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	t.Setenv("LINODEMCP_CONFIG_PATH", customPath)

	if config.Path() != customPath {
		t.Errorf("config.Path() = %v, want %v", config.Path(), customPath)
	}
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
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if cfg.Server.Name != "EnvServer" {
		t.Errorf("cfg.Server.Name = %v, want %v", cfg.Server.Name, "EnvServer")
	}

	if cfg.Server.LogLevel != "error" {
		t.Errorf("cfg.Server.LogLevel = %v, want %v", cfg.Server.LogLevel, "error")
	}

	if cfg.Environments["default"].Linode.APIURL != "https://override.api.com" {
		t.Errorf("got %v, want %v", cfg.Environments["default"].Linode.APIURL, "https://override.api.com")
	}

	if cfg.Environments["default"].Linode.Token != "env-token" {
		t.Errorf("got %v, want %v", cfg.Environments["default"].Linode.Token, "env-token")
	}
}
