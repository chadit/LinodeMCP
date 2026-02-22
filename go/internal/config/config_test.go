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

func TestLoadFromFile_ValidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	config.ResetCaches()

	cfg, err := config.LoadFromFile(path)

	require.NoError(t, err)
	assert.Equal(t, "TestServer", cfg.Server.Name)
	assert.Equal(t, "debug", cfg.Server.LogLevel)
	assert.Equal(t, 9000, cfg.Server.Port)
	assert.Contains(t, cfg.Environments, "default")
	assert.Equal(t, "test-token-123", cfg.Environments["default"].Linode.Token)
}

func TestLoadFromFile_ValidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.json", validJSONConfig())

	config.ResetCaches()

	cfg, err := config.LoadFromFile(path)

	require.NoError(t, err)
	assert.Equal(t, "JSONServer", cfg.Server.Name)
	assert.Equal(t, "warn", cfg.Server.LogLevel)
}

func TestLoadFromFile_Defaults(t *testing.T) {
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

	config.ResetCaches()

	cfg, err := config.LoadFromFile(path)

	require.NoError(t, err)
	assert.Equal(t, config.DefaultServerName, cfg.Server.Name, "default server name.")
	assert.Equal(t, config.DefaultLogLevel, cfg.Server.LogLevel, "default log level.")
	assert.Equal(t, config.DefaultTransport, cfg.Server.Transport, "default transport.")
	assert.Equal(t, config.DefaultHost, cfg.Server.Host, "default host.")
	assert.Equal(t, config.DefaultServerPort, cfg.Server.Port, "default port.")
	assert.Equal(t, config.DefaultMetricsPort, cfg.Metrics.Port, "default metrics port.")
	assert.Equal(t, config.DefaultMetricsPath, cfg.Metrics.Path, "default metrics path.")
	assert.Equal(t, config.DefaultRateLimitPerMinute, cfg.Resilience.RateLimitPerMinute, "default rate limit.")
	assert.Equal(t, config.DefaultCircuitBreakerThreshold, cfg.Resilience.CircuitBreakerThreshold, "default CB threshold.")
	assert.Equal(t, config.DefaultCircuitBreakerTimeout, cfg.Resilience.CircuitBreakerTimeout, "default CB timeout.")
	assert.Equal(t, config.DefaultMaxRetries, cfg.Resilience.MaxRetries, "default max retries.")
	assert.Equal(t, config.DefaultBaseRetryDelay, cfg.Resilience.BaseRetryDelay, "default base retry delay.")
	assert.Equal(t, config.DefaultMaxRetryDelay, cfg.Resilience.MaxRetryDelay, "default max retry delay.")
	assert.InDelta(t, config.DefaultSampleRate, cfg.Tracing.SampleRate, 0.001, "default sample rate.")
}

func TestLoadFromFile_FileNotFound(t *testing.T) {
	t.Parallel()

	config.ResetCaches()

	_, err := config.LoadFromFile("/tmp/nonexistent-linodemcp-config-test.yml")
	require.Error(t, err)
}

func TestLoadFromFile_MalformedYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", `{{{invalid yaml`)

	config.ResetCaches()

	_, err := config.LoadFromFile(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrConfigMalformed)
}

func TestLoadFromFile_NoEnvironments(t *testing.T) {
	t.Parallel()

	yaml := `
server:
  name: "Test"
  logLevel: "info"
`
	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", yaml)

	config.ResetCaches()

	_, err := config.LoadFromFile(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrConfigInvalid)
}

func TestValidateConfig_NilConfig(t *testing.T) {
	t.Parallel()

	err := config.ExportedValidateConfig(nil)
	assert.ErrorIs(t, err, config.ErrConfigNil)
}

func TestValidateConfig_NoEnvironments(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{Name: "test", LogLevel: "info"},
	}
	err := config.ExportedValidateConfig(cfg)
	assert.ErrorIs(t, err, config.ErrNoEnvironments)
}

func TestValidateConfig_IncompleteLinodeConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{Name: "test", LogLevel: "info"},
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4"},
			},
		},
	}
	err := config.ExportedValidateConfig(cfg)
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrConfigInvalid)
}

func TestValidateConfig_ValidConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{Name: "test", LogLevel: "info"},
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "tok"},
			},
		},
	}
	err := config.ExportedValidateConfig(cfg)
	assert.NoError(t, err)
}

func TestApplyEnvironmentOverrides(t *testing.T) {
	t.Setenv("LINODEMCP_SERVER_NAME", "EnvServer")
	t.Setenv("LINODEMCP_LOG_LEVEL", "error")
	t.Setenv("LINODEMCP_LINODE_API_URL", "https://override.api.com")
	t.Setenv("LINODEMCP_LINODE_TOKEN", "env-token")

	loader := config.NewLoader()
	cfg, err := loader.Load()
	// The loader may fail to find a config file, but the env overrides should still apply.
	// We need a file for the loader, so use a minimal config.
	_ = err
	_ = cfg

	// Test indirectly via a full load with a minimal file.
	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", `
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
`)

	config.ResetCaches()

	cfg, err = config.LoadFromFile(path)
	require.NoError(t, err)

	assert.Equal(t, "EnvServer", cfg.Server.Name)
	assert.Equal(t, "error", cfg.Server.LogLevel)
}

func TestSelectEnvironment_ExactMatch(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"prod":    {Label: "Production"},
			"staging": {Label: "Staging"},
		},
	}
	env, err := cfg.SelectEnvironment("prod")
	require.NoError(t, err)
	assert.Equal(t, "Production", env.Label)
}

func TestSelectEnvironment_CaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"Production": {Label: "Prod"},
		},
	}
	env, err := cfg.SelectEnvironment("production")
	require.NoError(t, err)
	assert.Equal(t, "Prod", env.Label)
}

func TestSelectEnvironment_FallsBackToDefault(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default Env"},
			"other":   {Label: "Other"},
		},
	}
	env, err := cfg.SelectEnvironment("nonexistent")
	require.NoError(t, err)
	assert.Equal(t, "Default Env", env.Label)
}

func TestSelectEnvironment_EmptyInput(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default"},
		},
	}
	_, err := cfg.SelectEnvironment("")
	require.Error(t, err)
}

func TestSelectEnvironment_NoEnvironments(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, err := cfg.SelectEnvironment("anything")
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrEnvironmentNotFound)
}

func TestGetLinodeEnvironment_Found(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"prod": {
				Label:  "Production",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "tok"},
			},
		},
	}
	lc, err := cfg.GetLinodeEnvironment("prod")
	require.NoError(t, err)
	assert.Equal(t, "https://api.linode.com/v4", lc.APIURL)
}

func TestGetLinodeEnvironment_NotFound(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"prod": {Label: "Production"},
		},
	}
	_, err := cfg.GetLinodeEnvironment("staging")
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrEnvironmentNotFound)
}

func TestGetLinodeEnvironment_NoEnvironments(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, err := cfg.GetLinodeEnvironment("any")
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrEnvironmentNotFound)
}

func TestValidatePath_EmptyPath(t *testing.T) {
	t.Parallel()

	config.ResetCaches()

	err := config.ExportedValidatePath("")
	assert.ErrorIs(t, err, config.ErrPathEmpty)
}

func TestValidatePath_DangerousPath(t *testing.T) {
	t.Parallel()

	config.ResetCaches()

	err := config.ExportedValidatePath("/etc/passwd")
	assert.ErrorIs(t, err, config.ErrPathDangerous)
}

func TestValidatePath_TraversalPath(t *testing.T) {
	t.Parallel()

	config.ResetCaches()

	err := config.ExportedValidatePath("/tmp/foo/../../etc/passwd")
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrPathOutsideAllowed)
}

func TestValidatePath_ValidTempPath(t *testing.T) {
	t.Parallel()

	config.ResetCaches()

	err := config.ExportedValidatePath("/tmp/linodemcp-test-config.yml")
	assert.NoError(t, err)
}

func TestCacheManager_PathValidationCaching(t *testing.T) {
	t.Parallel()

	cacheManager := config.NewCacheManager()
	path := "/tmp/cache-test-path"

	// First call performs validation.
	err1 := cacheManager.ExportedValidatePath(path)
	require.NoError(t, err1)

	// Second call should hit the cache.
	err2 := cacheManager.ExportedValidatePath(path)
	assert.NoError(t, err2)
}

func TestCacheManager_ResetCaches(t *testing.T) {
	t.Parallel()

	cacheManager := config.NewCacheManager()
	cacheManager.PathValidationCache()["test"] = nil

	cacheManager.ResetCaches()

	assert.Empty(t, cacheManager.PathValidationCache())
	assert.Nil(t, cacheManager.AllowedDirsCache())
	assert.False(t, cacheManager.AllowedDirsCached())
}

func TestParseConfigData_JSON(t *testing.T) {
	t.Parallel()

	var cfg config.Config

	err := config.ExportedParseConfigData([]byte(validJSONConfig()), &cfg)
	require.NoError(t, err)
	assert.Equal(t, "JSONServer", cfg.Server.Name)
}

func TestParseConfigData_YAML(t *testing.T) {
	t.Parallel()

	var cfg config.Config

	err := config.ExportedParseConfigData([]byte(validYAMLConfig()), &cfg)
	require.NoError(t, err)
	assert.Equal(t, "TestServer", cfg.Server.Name)
}

func TestParseConfigData_Invalid(t *testing.T) {
	t.Parallel()

	var cfg config.Config

	err := config.ExportedParseConfigData([]byte(`not: [valid: yaml: {{`), &cfg)
	require.Error(t, err)
}

func TestLoader_Exists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	loader := config.NewLoader(config.WithConfigPath(path))
	assert.True(t, loader.Exists())

	loaderMissing := config.NewLoader(config.WithConfigPath(filepath.Join(dir, "nope.yml")))
	assert.False(t, loaderMissing.Exists())
}

func TestLoader_ConfigCaching(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := writeConfigFile(t, dir, "config.yml", validYAMLConfig())

	config.ResetCaches()

	cm := config.NewCacheManager()
	loader := config.NewLoader(config.WithConfigPath(path), config.WithCacheManager(cm))

	cfg1, err := loader.LoadFromFile(path)
	require.NoError(t, err)

	// Second load should come from cache.
	cfg2, err := loader.LoadFromFile(path)
	require.NoError(t, err)

	assert.Equal(t, cfg1.Server.Name, cfg2.Server.Name)
}

func TestGetConfigPath_WithEnvOverride(t *testing.T) {
	dir := t.TempDir()
	customPath := filepath.Join(dir, "custom-config.yml")
	require.NoError(t, os.WriteFile(customPath, []byte(""), 0o600))

	config.ResetCaches()
	t.Setenv("LINODEMCP_CONFIG_PATH", customPath)

	result := config.GetConfigPath()
	assert.Equal(t, customPath, result)
}

func TestGetConfigDir_WithEnvOverride(t *testing.T) {
	dir := t.TempDir()
	customPath := filepath.Join(dir, "custom-config.yml")

	config.ResetCaches()
	t.Setenv("LINODEMCP_CONFIG_PATH", customPath)

	result := config.GetConfigDir()
	assert.Equal(t, dir, result)
}
