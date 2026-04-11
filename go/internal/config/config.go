// Package config provides configuration loading for LinodeMCP.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Default server configuration values.
const (
	DefaultServerName = "LinodeMCP"
	DefaultLogLevel   = "info"
	DefaultTransport  = "stdio"
	DefaultHost       = "127.0.0.1"
	DefaultServerPort = 8080
)

// Default observability configuration values.
const (
	DefaultMetricsPort     = 8888
	DefaultMetricsPath     = "/metrics"
	DefaultHealthPort      = 8889
	DefaultHealthPath      = "/healthz"
	DefaultTracingSample   = 1.0
	DefaultTracingProtocol = "grpc"
)

// Default resilience configuration values.
const (
	DefaultMaxRetries     = 3
	DefaultBaseRetryDelay = 1 * time.Second
	DefaultMaxRetryDelay  = 30 * time.Second
)

const (
	appDirName     = "linodemcp"
	configDirName  = ".config"
	configFileJSON = "config.json"
	configFileYAML = "config.yml"

	defaultEnvironmentName  = "default"
	defaultEnvironmentLabel = "Default"

	// Boolean string constants for environment variable parsing.
	boolTrue  = "true"
	boolFalse = "false"
)

// ServerConfig holds core server settings.
type ServerConfig struct {
	Name      string `json:"name"      yaml:"name"`
	LogLevel  string `json:"log_level" yaml:"logLevel"`
	Transport string `json:"transport" yaml:"transport"`
	Host      string `json:"host"      yaml:"host"`
	Port      int    `json:"port"      yaml:"port"`
}

// ResilienceConfig holds retry settings.
type ResilienceConfig struct {
	MaxRetries     int           `json:"max_retries"      yaml:"maxRetries"`
	BaseRetryDelay time.Duration `json:"base_retry_delay" yaml:"baseRetryDelay"`
	MaxRetryDelay  time.Duration `json:"max_retry_delay"  yaml:"maxRetryDelay"`
}

// LinodeConfig holds Linode API settings for an environment.
type LinodeConfig struct {
	APIURL string `json:"api_url" yaml:"apiUrl"`
	Token  string `json:"token"   yaml:"token"`
}

// EnvironmentConfig holds settings for a named environment.
type EnvironmentConfig struct {
	Label  string       `json:"label"  yaml:"label"`
	Linode LinodeConfig `json:"linode" yaml:"linode"`
}

// Config holds the full LinodeMCP configuration.
type Config struct {
	Server        ServerConfig                 `json:"server"        yaml:"server"`
	Resilience    ResilienceConfig             `json:"resilience"    yaml:"resilience"`
	Observability ObservabilityConfig          `json:"observability" yaml:"observability"`
	Environments  map[string]EnvironmentConfig `json:"environments"  yaml:"environments"`
}

// ObservabilityConfig holds observability settings.
type ObservabilityConfig struct {
	Tracing TracingConfig `json:"tracing" yaml:"tracing"`
	Metrics MetricsConfig `json:"metrics" yaml:"metrics"`
	Logging LoggingConfig `json:"logging" yaml:"logging"`
	Health  HealthConfig  `json:"health"  yaml:"health"`
}

// TracingConfig holds OpenTelemetry tracing settings.
type TracingConfig struct {
	Enabled    bool              `json:"enabled"     yaml:"enabled"`
	Endpoint   string            `json:"endpoint"    yaml:"endpoint"`
	Protocol   string            `json:"protocol"    yaml:"protocol"`
	Insecure   bool              `json:"insecure"    yaml:"insecure"`
	SampleRate float64           `json:"sample_rate" yaml:"sampleRate"`
	Headers    map[string]string `json:"headers"     yaml:"headers"`
}

// MetricsConfig holds metrics settings.
type MetricsConfig struct {
	Enabled    bool              `json:"enabled"    yaml:"enabled"`
	Prometheus PrometheusConfig  `json:"prometheus" yaml:"prometheus"`
	OTLP       OTLPMetricsConfig `json:"otlp"       yaml:"otlp"`
	Runtime    bool              `json:"runtime"    yaml:"runtime"`
	Host       bool              `json:"host"       yaml:"host"`
}

// PrometheusConfig holds Prometheus-specific metrics settings.
type PrometheusConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Port    int    `json:"port"    yaml:"port"`
	Path    string `json:"path"    yaml:"path"`
}

// OTLPMetricsConfig holds OTLP metrics settings.
type OTLPMetricsConfig struct {
	Enabled  bool              `json:"enabled"  yaml:"enabled"`
	Endpoint string            `json:"endpoint" yaml:"endpoint"`
	Protocol string            `json:"protocol" yaml:"protocol"`
	Insecure bool              `json:"insecure" yaml:"insecure"`
	Headers  map[string]string `json:"headers"  yaml:"headers"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `json:"level"  yaml:"level"`
	Format string `json:"format" yaml:"format"`
}

// HealthConfig holds health check settings.
type HealthConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Port    int    `json:"port"    yaml:"port"`
	Path    string `json:"path"    yaml:"path"`
}

// Load reads and returns the configuration from the given file path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from operator config or env var
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("%w: %s", ErrConfigPermissions, path)
		}

		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrConfigFileNotFound, path)
		}

		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg Config
	if err := parseConfigData(data, &cfg); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrConfigMalformed, err.Error())
	}

	setDefaults(&cfg)
	applyEnvironmentOverrides(&cfg)

	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrConfigInvalid, err.Error())
	}

	return &cfg, nil
}

// GetConfigPath returns the full path to the config file.
func GetConfigPath() string {
	if customPath := os.Getenv("LINODEMCP_CONFIG_PATH"); customPath != "" {
		return customPath
	}

	return getDefaultConfigPath()
}

// SelectEnvironment picks a Linode environment from the config.
func (c *Config) SelectEnvironment(userInput string) (*EnvironmentConfig, error) {
	trimmed := strings.TrimSpace(userInput)
	if trimmed == "" {
		return nil, ErrEmptyEnvironmentName
	}

	if len(c.Environments) == 0 {
		return nil, fmt.Errorf("%w: no provider environments configured", ErrEnvironmentNotFound)
	}

	for envName, env := range c.Environments {
		if strings.EqualFold(envName, trimmed) {
			return &env, nil
		}
	}

	if defaultEnv, exists := c.Environments[defaultEnvironmentName]; exists {
		return &defaultEnv, nil
	}

	for _, env := range c.Environments {
		return &env, nil
	}

	return nil, fmt.Errorf("%w: no matching environment found for input: %s", ErrEnvironmentNotFound, userInput)
}

func parseConfigData(data []byte, cfg *Config) error {
	if len(data) > 0 && data[0] == '{' {
		if err := json.Unmarshal(data, cfg); err == nil {
			return nil
		}
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return nil
}

func setDefaults(cfg *Config) {
	setServerDefaults(cfg)
	setResilienceDefaults(cfg)
	setObservabilityDefaults(cfg)
}

func setServerDefaults(cfg *Config) {
	if cfg.Server.Name == "" {
		cfg.Server.Name = DefaultServerName
	}

	if cfg.Server.LogLevel == "" {
		cfg.Server.LogLevel = DefaultLogLevel
	}

	if cfg.Server.Transport == "" {
		cfg.Server.Transport = DefaultTransport
	}

	if cfg.Server.Host == "" {
		cfg.Server.Host = DefaultHost
	}

	if cfg.Server.Port == 0 {
		cfg.Server.Port = DefaultServerPort
	}
}

func setResilienceDefaults(cfg *Config) {
	if cfg.Resilience.MaxRetries == 0 {
		cfg.Resilience.MaxRetries = DefaultMaxRetries
	}

	if cfg.Resilience.BaseRetryDelay == 0 {
		cfg.Resilience.BaseRetryDelay = DefaultBaseRetryDelay
	}

	if cfg.Resilience.MaxRetryDelay == 0 {
		cfg.Resilience.MaxRetryDelay = DefaultMaxRetryDelay
	}
}

func setObservabilityDefaults(cfg *Config) {
	// Metrics defaults
	if cfg.Observability.Metrics.Prometheus.Port == 0 {
		cfg.Observability.Metrics.Prometheus.Port = DefaultMetricsPort
	}

	if cfg.Observability.Metrics.Prometheus.Path == "" {
		cfg.Observability.Metrics.Prometheus.Path = DefaultMetricsPath
	}

	// Health defaults
	if cfg.Observability.Health.Port == 0 {
		cfg.Observability.Health.Port = DefaultHealthPort
	}

	if cfg.Observability.Health.Path == "" {
		cfg.Observability.Health.Path = DefaultHealthPath
	}

	// Tracing defaults
	if cfg.Observability.Tracing.SampleRate == 0 {
		cfg.Observability.Tracing.SampleRate = DefaultTracingSample
	}

	if cfg.Observability.Tracing.Protocol == "" {
		cfg.Observability.Tracing.Protocol = DefaultTracingProtocol
	}

	// Enable runtime and host metrics by default
	if !cfg.Observability.Metrics.Runtime && !cfg.Observability.Metrics.Host {
		cfg.Observability.Metrics.Runtime = true
		cfg.Observability.Metrics.Host = true
	}

	// Enable Prometheus by default if metrics enabled
	if cfg.Observability.Metrics.Enabled && !cfg.Observability.Metrics.Prometheus.Enabled {
		cfg.Observability.Metrics.Prometheus.Enabled = true
	}

	// Enable health checks by default
	if !cfg.Observability.Health.Enabled {
		cfg.Observability.Health.Enabled = true
	}

	// Default logging format
	if cfg.Observability.Logging.Format == "" {
		cfg.Observability.Logging.Format = "json"
	}
}

func applyEnvironmentOverrides(cfg *Config) {
	applyServerOverrides(cfg)
	applyObservabilityOverrides(cfg)
	applyOTELOverrides(cfg)
	applyLinodeOverrides(cfg)
}

func applyServerOverrides(cfg *Config) {
	if v := os.Getenv("LINODEMCP_SERVER_NAME"); v != "" {
		cfg.Server.Name = v
	}

	if v := os.Getenv("LINODEMCP_LOG_LEVEL"); v != "" {
		cfg.Server.LogLevel = v
		cfg.Observability.Logging.Level = v
	}
}

func applyObservabilityOverrides(cfg *Config) {
	// Metrics overrides
	if v := os.Getenv("LINODEMCP_METRICS_ENABLED"); v != "" {
		cfg.Observability.Metrics.Enabled = v == boolTrue || v == "1"
	}

	if v := os.Getenv("LINODEMCP_METRICS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Observability.Metrics.Prometheus.Port = port
		}
	}

	// Tracing overrides
	if v := os.Getenv("LINODEMCP_TRACING_ENABLED"); v != "" {
		cfg.Observability.Tracing.Enabled = v == boolTrue || v == "1"
	}

	if v := os.Getenv("LINODEMCP_TRACING_ENDPOINT"); v != "" {
		cfg.Observability.Tracing.Endpoint = v
	}

	if v := os.Getenv("LINODEMCP_TRACING_SAMPLE_RATE"); v != "" {
		if rate, err := strconv.ParseFloat(v, 64); err == nil && rate >= 0 && rate <= 1 {
			cfg.Observability.Tracing.SampleRate = rate
		}
	}

	// Health overrides
	if v := os.Getenv("LINODEMCP_HEALTH_ENABLED"); v != "" {
		cfg.Observability.Health.Enabled = v == boolTrue || v == "1"
	}
}

func applyOTELOverrides(cfg *Config) {
	if v := os.Getenv("OTEL_TRACES_EXPORTER"); v != "" {
		cfg.Observability.Tracing.Enabled = v != "none" && v != "console"
	}

	if v := os.Getenv("OTEL_METRICS_EXPORTER"); v != "" {
		cfg.Observability.Metrics.Enabled = v != "none" && v != "console"
	}

	// OTEL_SERVICE_NAME is handled directly by the OTEL SDK

	if envVar := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); envVar != "" {
		if cfg.Observability.Tracing.Endpoint == "" {
			cfg.Observability.Tracing.Endpoint = envVar
		}

		if cfg.Observability.Metrics.OTLP.Endpoint == "" {
			cfg.Observability.Metrics.OTLP.Endpoint = envVar
		}
	}

	if v := os.Getenv("OTEL_TRACES_SAMPLER_ARG"); v != "" {
		if rate, err := strconv.ParseFloat(v, 64); err == nil && rate >= 0 && rate <= 1 {
			cfg.Observability.Tracing.SampleRate = rate
		}
	}
}

func applyLinodeOverrides(cfg *Config) {
	if cfg.Environments == nil {
		cfg.Environments = make(map[string]EnvironmentConfig)
	}

	defaultEnv := cfg.Environments[defaultEnvironmentName]

	var linodeEnvVarsSet bool

	if v := os.Getenv("LINODEMCP_LINODE_API_URL"); v != "" {
		defaultEnv.Linode.APIURL = v
		linodeEnvVarsSet = true
	}

	if v := os.Getenv("LINODEMCP_LINODE_TOKEN"); v != "" {
		defaultEnv.Linode.Token = v
		linodeEnvVarsSet = true
	}

	if linodeEnvVarsSet {
		if defaultEnv.Label == "" {
			defaultEnv.Label = defaultEnvironmentLabel
		}

		cfg.Environments[defaultEnvironmentName] = defaultEnv
	}
}

func validateConfig(cfg *Config) error {
	if cfg.Server.Name == "" {
		return ErrEmptyServerName
	}

	if cfg.Server.LogLevel == "" {
		return ErrEmptyLogLevel
	}

	if len(cfg.Environments) == 0 {
		return ErrNoEnvironments
	}

	for envName, env := range cfg.Environments {
		if envName == "" {
			return ErrEmptyEnvironmentName
		}

		if env.Linode.APIURL != "" || env.Linode.Token != "" {
			if env.Linode.APIURL == "" {
				return fmt.Errorf("%w: environment '%s'", ErrMissingAPIURL, envName)
			}

			if env.Linode.Token == "" {
				return fmt.Errorf("%w: environment '%s'", ErrMissingToken, envName)
			}
		}
	}

	return nil
}

func getDefaultConfigPath() string {
	configDir := getDefaultConfigDir()

	jsonPath := filepath.Join(configDir, configFileJSON)
	if _, err := os.Stat(jsonPath); err == nil {
		return jsonPath
	}

	return filepath.Join(configDir, configFileYAML)
}

func getDefaultConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), appDirName)
	}

	return filepath.Join(homeDir, configDirName, appDirName)
}
