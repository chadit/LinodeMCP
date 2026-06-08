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
	DefaultMaxRetries              = 3
	DefaultBaseRetryDelay          = 1 * time.Second
	DefaultMaxRetryDelay           = 30 * time.Second
	DefaultRateLimitPerMinute      = 700
	DefaultCircuitBreakerThreshold = 5
	DefaultCircuitBreakerTimeout   = 30 * time.Second
)

const (
	// DefaultAuditRetentionDays is the default rotated-log retention
	// window. Keep in sync with audit.DefaultAuditRetentionDays, which
	// is the sweeper's intrinsic default when no config is supplied
	// (the config package stays a leaf and does not import audit).
	DefaultAuditRetentionDays = 14

	// DefaultAuditSQLiteBusyTimeoutMS is the default SQLite busy_timeout
	// in milliseconds, applied when the SQLite sink is enabled but no
	// explicit timeout is configured. Consumed by the Phase 3b sink.
	DefaultAuditSQLiteBusyTimeoutMS = 5000

	// DefaultAuditRedactPII is the default for audit.redact_pii: PII
	// fields (tax_id, phone, address_1/2, city, state, zip) are redacted
	// alongside the always-on credential list. Operators who need raw
	// PII in audit (e.g. for accountability investigations) can opt out
	// by setting audit.redact_pii: false.
	DefaultAuditRedactPII = true
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

// ResilienceConfig holds retry, rate limit, and circuit breaker settings.
// RateLimitPerMinute caps outbound calls to stay under the Linode 700 req/min
// account limit. CircuitBreaker* gate the client when an upstream goes hard
// down so we stop hammering it.
type ResilienceConfig struct {
	MaxRetries              int           `json:"max_retries"               yaml:"maxRetries"`
	BaseRetryDelay          time.Duration `json:"base_retry_delay"          yaml:"baseRetryDelay"`
	MaxRetryDelay           time.Duration `json:"max_retry_delay"           yaml:"maxRetryDelay"`
	RateLimitPerMinute      int           `json:"rate_limit_per_minute"     yaml:"rateLimitPerMinute"`
	CircuitBreakerThreshold int           `json:"circuit_breaker_threshold" yaml:"circuitBreakerThreshold"`
	CircuitBreakerTimeout   time.Duration `json:"circuit_breaker_timeout"   yaml:"circuitBreakerTimeout"`
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
	Server                   ServerConfig                 `json:"server"                     yaml:"server"`
	Resilience               ResilienceConfig             `json:"resilience"                 yaml:"resilience"`
	Observability            ObservabilityConfig          `json:"observability"              yaml:"observability"`
	Environments             map[string]EnvironmentConfig `json:"environments"               yaml:"environments"`
	ActiveProfile            string                       `json:"active_profile"             yaml:"active_profile"`
	Profiles                 map[string]UserProfileConfig `json:"profiles"                   yaml:"profiles"`
	ProfilesBuiltinOverrides map[string]BuiltinOverride   `json:"profiles_builtin_overrides" yaml:"profiles_builtin_overrides"`
	Audit                    AuditConfig                  `json:"audit"                      yaml:"audit"`
}

// AuditConfig holds audit-log settings. The JSONL sink is always on
// (Phase 2); these fields tune retention, the optional SQLite sink
// (Phase 3b), the optional PII redaction tier (Phase 4c), and named
// custom reports (Phase 4a/b).
//
// RetentionDays and RedactPII are pointers so an explicit zero value
// ("never delete" / "log PII in cleartext") is distinguishable from
// "unset" (nil → defaults). After setDefaults runs they are always
// non-nil, so consumers can dereference safely.
type AuditConfig struct {
	RetentionDays *int                    `json:"retention_days" yaml:"retention_days"`
	RedactPII     *bool                   `json:"redact_pii"     yaml:"redact_pii"`
	SQLite        AuditSQLiteConfig       `json:"sqlite"         yaml:"sqlite"`
	Reports       map[string]ReportConfig `json:"reports"        yaml:"reports"`
}

// Report output modes. ReportOutputSummary aggregates into per-bucket
// counts (like linode_audit_summary); ReportOutputList returns the
// matching events (like linode_audit_recent), capped by Limit.
const (
	ReportOutputSummary = "summary"
	ReportOutputList    = "list"
)

// ReportConfig is the YAML/JSON shape for one named custom audit report
// under AuditConfig.Reports. The linode_audit_report tool (Phase 4b)
// resolves and runs it at call time, so editing the report file takes
// effect on the next call. An empty Output defaults to "summary".
type ReportConfig struct {
	Description string       `json:"description" yaml:"description"`
	Filter      ReportFilter `json:"filter"      yaml:"filter"`
	GroupBy     []string     `json:"group_by"    yaml:"group_by"`
	Output      string       `json:"output"      yaml:"output"`
	Limit       int          `json:"limit"       yaml:"limit"`
}

// ReportFilter types the small report filter grammar. Each field maps
// to an event field. Tool and Environment are globs; Capability and
// Status accept either a scalar or the *In list form (not both).
// SinceOffset is a duration relative to now (e.g. "24h"); Since and
// Until are absolute RFC 3339 timestamps. Compiled to a predicate by
// the Phase 4b tool; the grammar is intentionally small (no eval, no
// expression language).
type ReportFilter struct {
	Tool         string   `json:"tool"          yaml:"tool"`
	Capability   string   `json:"capability"    yaml:"capability"`
	CapabilityIn []string `json:"capability_in" yaml:"capability_in"`
	Status       string   `json:"status"        yaml:"status"`
	StatusIn     []string `json:"status_in"     yaml:"status_in"`
	Environment  string   `json:"environment"   yaml:"environment"`
	Profile      string   `json:"profile"       yaml:"profile"`
	SinceOffset  string   `json:"since_offset"  yaml:"since_offset"`
	Since        string   `json:"since"         yaml:"since"`
	Until        string   `json:"until"         yaml:"until"`
}

// AuditSQLiteConfig holds the optional SQLite audit sink settings.
// Disabled by default; when enabled, audit events dual-write to both
// JSONL and SQLite. An empty Path resolves to audit.db alongside the
// JSONL log. Consumed by the Phase 3b sink.
type AuditSQLiteConfig struct {
	Enabled       bool   `json:"enabled"         yaml:"enabled"`
	Path          string `json:"path"            yaml:"path"`
	BusyTimeoutMS int    `json:"busy_timeout_ms" yaml:"busy_timeout_ms"`
}

// UserProfileConfig is the YAML/JSON shape for a single user-defined profile
// entry under Config.Profiles. Built-in profiles live in code (see
// internal/profiles/builtin.go) and never appear in this map. Wildcard entries
// in AllowedTools and DeniedTools are expanded against the live tool registry
// during profile resolution in internal/profiles.ResolveActiveProfile.
type UserProfileConfig struct {
	Description         string   `json:"description"           yaml:"description"`
	AllowedTools        []string `json:"allowed_tools"         yaml:"allowed_tools"`
	DeniedTools         []string `json:"denied_tools"          yaml:"denied_tools"`
	AllowedEnvironments []string `json:"allowed_environments"  yaml:"allowed_environments"`
	RequiredTokenScopes []string `json:"required_token_scopes" yaml:"required_token_scopes"`
	AllowYolo           bool     `json:"allow_yolo"            yaml:"allow_yolo"`
}

// BuiltinOverride is the YAML/JSON shape for a per-built-in toggle under
// Config.ProfilesBuiltinOverrides. Only the Disabled field is honored today;
// future toggles (e.g. soft scope enforcement) would live alongside it.
type BuiltinOverride struct {
	Disabled bool `json:"disabled" yaml:"disabled"`
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
		return nil, fmt.Errorf("%w: %w", ErrConfigInvalid, err)
	}

	return &cfg, nil
}

// Path returns the full path to the config file.
func Path() string {
	if customPath := os.Getenv("LINODEMCP_CONFIG_PATH"); customPath != "" {
		return customPath
	}

	return defaultConfigPath()
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
	setAuditDefaults(cfg)
}

func setAuditDefaults(cfg *Config) {
	if cfg.Audit.RetentionDays == nil {
		days := DefaultAuditRetentionDays
		cfg.Audit.RetentionDays = &days
	}

	if cfg.Audit.RedactPII == nil {
		redact := DefaultAuditRedactPII
		cfg.Audit.RedactPII = &redact
	}

	if cfg.Audit.SQLite.BusyTimeoutMS == 0 {
		cfg.Audit.SQLite.BusyTimeoutMS = DefaultAuditSQLiteBusyTimeoutMS
	}

	// Map values are not addressable, so reassign the whole struct after
	// filling the output default. Range over keys to avoid copying each
	// ReportConfig on every iteration.
	for name := range cfg.Audit.Reports {
		if cfg.Audit.Reports[name].Output == "" {
			report := cfg.Audit.Reports[name]
			report.Output = ReportOutputSummary
			cfg.Audit.Reports[name] = report
		}
	}
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

	if cfg.Resilience.RateLimitPerMinute == 0 {
		cfg.Resilience.RateLimitPerMinute = DefaultRateLimitPerMinute
	}

	if cfg.Resilience.CircuitBreakerThreshold == 0 {
		cfg.Resilience.CircuitBreakerThreshold = DefaultCircuitBreakerThreshold
	}

	if cfg.Resilience.CircuitBreakerTimeout == 0 {
		cfg.Resilience.CircuitBreakerTimeout = DefaultCircuitBreakerTimeout
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
	applyAuditOverrides(cfg)
}

func applyAuditOverrides(cfg *Config) {
	if v := os.Getenv("LINODEMCP_AUDIT_RETENTION_DAYS"); v != "" {
		if days, err := strconv.Atoi(v); err == nil {
			cfg.Audit.RetentionDays = &days
		}
	}

	if v := os.Getenv("LINODEMCP_AUDIT_REDACT_PII"); v != "" {
		redact := v == boolTrue || v == "1"
		cfg.Audit.RedactPII = &redact
	}

	if v := os.Getenv("LINODEMCP_AUDIT_SQLITE_ENABLED"); v != "" {
		cfg.Audit.SQLite.Enabled = v == boolTrue || v == "1"
	}

	if v := os.Getenv("LINODEMCP_AUDIT_SQLITE_PATH"); v != "" {
		cfg.Audit.SQLite.Path = v
	}

	if v := os.Getenv("LINODEMCP_AUDIT_SQLITE_BUSY_TIMEOUT_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil {
			cfg.Audit.SQLite.BusyTimeoutMS = ms
		}
	}
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

	if cfg.Audit.RetentionDays != nil && *cfg.Audit.RetentionDays < 0 {
		return ErrNegativeRetentionDays
	}

	return validateAuditReports(cfg.Audit.Reports)
}

// validateAuditReports checks each custom report's structural grammar:
// a known output mode, a parseable since_offset duration, parseable
// since/until timestamps, and that capability/status use either the
// scalar or the list form but not both. Value semantics (is this a real
// capability name) are checked when the report runs.
func validateAuditReports(reports map[string]ReportConfig) error {
	for name := range reports {
		report := reports[name]

		if report.Output != ReportOutputSummary && report.Output != ReportOutputList {
			return fmt.Errorf("%w: report %q has output %q", ErrInvalidReportOutput, name, report.Output)
		}

		if err := validateReportFilter(name, &report.Filter); err != nil {
			return err
		}
	}

	return nil
}

// validateReportFilter checks one report's filter for a parseable
// duration/timestamps and the scalar-xor-list rule on capability and
// status.
func validateReportFilter(name string, filter *ReportFilter) error {
	if filter.Capability != "" && len(filter.CapabilityIn) > 0 {
		return fmt.Errorf("%w: report %q sets both capability and capability_in", ErrReportScalarAndList, name)
	}

	if filter.Status != "" && len(filter.StatusIn) > 0 {
		return fmt.Errorf("%w: report %q sets both status and status_in", ErrReportScalarAndList, name)
	}

	if filter.SinceOffset != "" {
		if _, err := time.ParseDuration(filter.SinceOffset); err != nil {
			return fmt.Errorf("%w: report %q since_offset %q: %w", ErrInvalidReportDuration, name, filter.SinceOffset, err)
		}
	}

	for _, bound := range []struct{ label, value string }{
		{"since", filter.Since},
		{"until", filter.Until},
	} {
		if bound.value == "" {
			continue
		}

		if _, err := time.Parse(time.RFC3339, bound.value); err != nil {
			return fmt.Errorf("%w: report %q %s %q: %w", ErrInvalidReportTimestamp, name, bound.label, bound.value, err)
		}
	}

	return nil
}

func defaultConfigPath() string {
	configDir := defaultConfigDir()

	jsonPath := filepath.Join(configDir, configFileJSON)
	if _, err := os.Stat(jsonPath); err == nil {
		return jsonPath
	}

	return filepath.Join(configDir, configFileYAML)
}

func defaultConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), appDirName)
	}

	return filepath.Join(homeDir, configDirName, appDirName)
}
