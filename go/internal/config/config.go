// Package config provides configuration loading for LinodeMCP.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// dangerousPaths lists system directories that config files must never be loaded from,
// preventing path traversal attacks that could read sensitive OS files or overwrite system configs.
//
//nolint:gochecknoglobals // Package-level slice shared by multiple validation functions.
var dangerousPaths = []string{
	"/etc/", "/root/", "/proc/", "/sys/", "/dev/", "/bin/", "/sbin/",
	"/usr/bin/", "/usr/sbin/", "/boot/", "/var/log/", "/var/run/",
}

// Static configuration errors.
var (
	ErrConfigFileNotFound   = errors.New("configuration file not found")
	ErrConfigInvalid        = errors.New("configuration file is invalid")
	ErrConfigPermissions    = errors.New("insufficient permissions to access configuration file")
	ErrConfigMalformed      = errors.New("configuration file is malformed")
	ErrEnvironmentNotFound  = errors.New("environment not found in configuration")
	ErrConfigNil            = errors.New("configuration is nil")
	ErrNoEnvironments       = errors.New("no environments defined in configuration")
	ErrEmptyEnvironmentName = errors.New("environment name cannot be empty")
	ErrPathEmpty            = errors.New("path cannot be empty")
	ErrPathDangerous        = errors.New("path contains dangerous elements")
	ErrPathTraversal        = errors.New("path contains directory traversal elements")
	ErrPathOutsideAllowed   = errors.New("path is outside allowed directories")
)

// ServerConfig holds core server settings.
type ServerConfig struct {
	Name      string `json:"name"      yaml:"name"`
	LogLevel  string `json:"logLevel"  yaml:"logLevel"`
	Transport string `json:"transport" yaml:"transport"`
	Host      string `json:"host"      yaml:"host"`
	Port      int    `json:"port"      yaml:"port"`
}

// MetricsConfig holds Prometheus metrics settings.
type MetricsConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Port    int    `json:"port"    yaml:"port"`
	Path    string `json:"path"    yaml:"path"`
}

// TracingConfig holds OpenTelemetry tracing settings.
type TracingConfig struct {
	Enabled    bool    `json:"enabled"    yaml:"enabled"`
	Exporter   string  `json:"exporter"   yaml:"exporter"`
	Endpoint   string  `json:"endpoint"   yaml:"endpoint"`
	SampleRate float64 `json:"sampleRate" yaml:"sampleRate"`
}

// ResilienceConfig holds retry, rate limit, and circuit breaker settings.
type ResilienceConfig struct {
	RateLimitPerMinute      int           `json:"rateLimitPerMinute"      yaml:"rateLimitPerMinute"`
	CircuitBreakerThreshold int           `json:"circuitBreakerThreshold" yaml:"circuitBreakerThreshold"`
	CircuitBreakerTimeout   time.Duration `json:"circuitBreakerTimeout"   yaml:"circuitBreakerTimeout"`
	MaxRetries              int           `json:"maxRetries"              yaml:"maxRetries"`
	BaseRetryDelay          time.Duration `json:"baseRetryDelay"          yaml:"baseRetryDelay"`
	MaxRetryDelay           time.Duration `json:"maxRetryDelay"           yaml:"maxRetryDelay"`
}

// LinodeConfig holds Linode API settings for an environment.
type LinodeConfig struct {
	APIURL string `json:"apiUrl" yaml:"apiUrl"`
	Token  string `json:"token"  yaml:"token"`
}

// EnvironmentConfig holds settings for a named environment.
type EnvironmentConfig struct {
	Label  string       `json:"label"  yaml:"label"`
	Linode LinodeConfig `json:"linode" yaml:"linode"`
}

// Config holds the full LinodeMCP configuration.
type Config struct {
	Server       ServerConfig                 `json:"server"       yaml:"server"`
	Metrics      MetricsConfig                `json:"metrics"      yaml:"metrics"`
	Tracing      TracingConfig                `json:"tracing"      yaml:"tracing"`
	Resilience   ResilienceConfig             `json:"resilience"   yaml:"resilience"`
	Environments map[string]EnvironmentConfig `json:"environments" yaml:"environments"`
}

// CacheManager manages config caching.
type CacheManager struct {
	pathValidationCache map[string]error
	pathCacheMutex      sync.RWMutex
	allowedDirsCache    []string
	allowedDirsCached   bool
	allowedDirsMutex    sync.RWMutex
	configCache         map[string]*Config
	configCacheMutex    sync.RWMutex
	fileMtimeCache      map[string]time.Time
}

// NewCacheManager creates a new CacheManager with initialized maps.
func NewCacheManager() *CacheManager {
	return &CacheManager{
		pathValidationCache: make(map[string]error),
		configCache:         make(map[string]*Config),
		fileMtimeCache:      make(map[string]time.Time),
	}
}

// ResetCaches clears all path validation and allowed directory caches.
func (cm *CacheManager) ResetCaches() {
	cm.pathCacheMutex.Lock()
	cm.pathValidationCache = make(map[string]error)
	cm.pathCacheMutex.Unlock()

	cm.allowedDirsMutex.Lock()
	cm.allowedDirsCache = nil
	cm.allowedDirsCached = false
	cm.allowedDirsMutex.Unlock()
}

type packageCacheManager struct {
	once sync.Once
	cm   *CacheManager
}

func (p *packageCacheManager) get() *CacheManager {
	p.once.Do(func() {
		p.cm = NewCacheManager()
	})

	return p.cm
}

//nolint:gochecknoglobals // Singleton pattern for cache manager
var pkgCacheManager packageCacheManager

// ResetCaches clears all internal caches (primarily for testing).
func ResetCaches() {
	pkgCacheManager.get().ResetCaches()
}

func validatePath(path string) error {
	return pkgCacheManager.get().validatePath(path)
}

func (cm *CacheManager) validatePath(path string) error {
	if path == "" {
		return ErrPathEmpty
	}

	cm.pathCacheMutex.RLock()

	if err, exists := cm.pathValidationCache[path]; exists {
		cm.pathCacheMutex.RUnlock()

		return err
	}

	cm.pathCacheMutex.RUnlock()

	err := cm.performPathValidation(path)
	if err == nil {
		cm.pathCacheMutex.Lock()
		cm.pathValidationCache[path] = err
		cm.pathCacheMutex.Unlock()
	}

	return err
}

func (cm *CacheManager) performPathValidation(path string) error {
	if containsDangerousPathElements(path) {
		return ErrPathDangerous
	}

	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return ErrPathTraversal
	}

	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	dirPath := filepath.Dir(absPath)
	if !cm.isPathInAllowedDirectory(absPath) && !cm.isPathInAllowedDirectory(dirPath) {
		return fmt.Errorf("%w: %s", ErrPathOutsideAllowed, absPath)
	}

	return nil
}

func containsDangerousPathElements(path string) bool {
	for _, dangerous := range dangerousPaths {
		if strings.HasPrefix(path, dangerous) {
			return true
		}
	}

	return false
}

func (cm *CacheManager) isPathInAllowedDirectory(absPath string) bool {
	for _, dangerous := range dangerousPaths {
		if strings.HasPrefix(absPath, dangerous) {
			return false
		}
	}

	cm.allowedDirsMutex.RLock()

	if !cm.allowedDirsCached {
		cm.allowedDirsMutex.RUnlock()
		cm.allowedDirsMutex.Lock()

		if !cm.allowedDirsCached {
			cm.allowedDirsCache = buildAllowedDirectoriesCache()
			cm.allowedDirsCached = true
		}

		cm.allowedDirsMutex.Unlock()
		cm.allowedDirsMutex.RLock()
	}

	dirs := cm.allowedDirsCache
	cm.allowedDirsMutex.RUnlock()

	for _, prefix := range dirs {
		if strings.HasPrefix(absPath, prefix) {
			return true
		}
	}

	return false
}

func buildAllowedDirectoriesCache() []string {
	var dirs []string

	if homeDir, err := os.UserHomeDir(); err == nil {
		if absHomeDir, err := filepath.Abs(homeDir); err == nil {
			dirs = append(dirs, absHomeDir)
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		if absCwd, err := filepath.Abs(cwd); err == nil {
			dirs = append(dirs, absCwd)
		}
	}

	if absTmpDir, err := filepath.Abs(os.TempDir()); err == nil {
		dirs = append(dirs, absTmpDir)
	}

	dirs = append(dirs, "/tmp")

	return dirs
}

func parseConfigData(data []byte, config *Config) error {
	if len(data) > 0 && data[0] == '{' {
		if err := json.Unmarshal(data, config); err == nil {
			return nil
		}
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return nil
}

// FileSystem abstracts filesystem operations for testing.
type FileSystem interface {
	ReadFile(filename string) ([]byte, error)
	Stat(name string) (os.FileInfo, error)
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(filename string, data []byte, perm os.FileMode) error
}

// OSFileSystem implements FileSystem using real OS operations.
type OSFileSystem struct{}

// Compile-time check that OSFileSystem implements FileSystem.
var _ FileSystem = (*OSFileSystem)(nil)

// ReadFile reads the named file and returns its contents.
func (fs *OSFileSystem) ReadFile(filename string) ([]byte, error) {
	if err := validatePath(filename); err != nil {
		return nil, fmt.Errorf("invalid file path %s: %w", filename, err)
	}

	// #nosec G304 -- Path validated by validatePath()
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	return data, nil
}

// Stat returns file info for the named file.
func (fs *OSFileSystem) Stat(name string) (os.FileInfo, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", name, err)
	}

	return info, nil
}

// MkdirAll creates a directory path and all parents that don't exist.
func (fs *OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	return nil
}

// WriteFile writes data to the named file, creating it if necessary.
func (fs *OSFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	if err := os.WriteFile(filename, data, perm); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filename, err)
	}

	return nil
}

// Loader provides configurable configuration loading.
type Loader struct {
	fs           FileSystem
	configPath   string
	cacheManager *CacheManager
}

// LoaderOption configures a Loader.
type LoaderOption func(*Loader)

// WithFileSystem sets a custom filesystem implementation on the Loader.
func WithFileSystem(fs FileSystem) LoaderOption {
	return func(l *Loader) { l.fs = fs }
}

// WithConfigPath sets a custom config file path on the Loader.
func WithConfigPath(path string) LoaderOption {
	return func(l *Loader) { l.configPath = path }
}

// WithCacheManager sets a custom CacheManager on the Loader.
func WithCacheManager(cm *CacheManager) LoaderOption {
	return func(l *Loader) { l.cacheManager = cm }
}

// NewLoader creates a new Loader with the given options applied.
func NewLoader(options ...LoaderOption) *Loader {
	l := &Loader{
		fs:           &OSFileSystem{},
		configPath:   GetConfigPath(),
		cacheManager: pkgCacheManager.get(),
	}

	for _, opt := range options {
		opt(l)
	}

	return l
}

// Load reads and returns the configuration from the configured path.
func (l *Loader) Load() (*Config, error) {
	return l.LoadFromFile(l.configPath)
}

// LoadFromFile reads and returns the configuration from the given file path.
func (l *Loader) LoadFromFile(path string) (*Config, error) {
	if cachedConfig := l.getCachedConfigIfValid(path); cachedConfig != nil {
		return cachedConfig, nil
	}

	return l.loadAndCacheFromFile(path)
}

// Exists returns true if the configured config file exists on disk.
func (l *Loader) Exists() bool {
	_, err := l.fs.Stat(l.configPath)

	return err == nil
}

func (l *Loader) getCachedConfigIfValid(path string) *Config {
	l.cacheManager.configCacheMutex.RLock()
	defer l.cacheManager.configCacheMutex.RUnlock()

	cachedConfig, exists := l.cacheManager.configCache[path]
	if !exists {
		return nil
	}

	cachedMtime, mtimeExists := l.cacheManager.fileMtimeCache[path]
	if !mtimeExists {
		return nil
	}

	if info, err := os.Stat(path); err == nil {
		if !info.ModTime().After(cachedMtime) {
			configCopy := *cachedConfig

			return &configCopy
		}
	}

	return nil
}

func (l *Loader) loadAndCacheFromFile(path string) (*Config, error) {
	var fileModTime time.Time

	if info, err := os.Stat(path); err == nil {
		fileModTime = info.ModTime()
	}

	data, err := l.fs.ReadFile(path)
	if err != nil {
		unwrappedErr := err
		for unwrappedErr != nil {
			if os.IsPermission(unwrappedErr) {
				return nil, fmt.Errorf("%w: %s", ErrConfigPermissions, path)
			}

			if os.IsNotExist(unwrappedErr) {
				return nil, fmt.Errorf("%w: %s", ErrConfigFileNotFound, path)
			}

			unwrappedErr = errors.Unwrap(unwrappedErr)
		}

		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var config Config
	if err := parseConfigData(data, &config); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrConfigMalformed, err.Error())
	}

	l.setDefaults(&config)
	l.applyEnvironmentOverrides(&config)

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrConfigInvalid, err.Error())
	}

	l.cacheConfig(path, &config, fileModTime)

	return &config, nil
}

func (l *Loader) cacheConfig(path string, config *Config, modTime time.Time) {
	l.cacheManager.configCacheMutex.Lock()
	defer l.cacheManager.configCacheMutex.Unlock()

	configCopy := *config
	l.cacheManager.configCache[path] = &configCopy
	l.cacheManager.fileMtimeCache[path] = modTime
}

func (l *Loader) setDefaults(config *Config) {
	if config.Server.Name == "" {
		config.Server.Name = DefaultServerName
	}

	if config.Server.LogLevel == "" {
		config.Server.LogLevel = DefaultLogLevel
	}

	if config.Server.Transport == "" {
		config.Server.Transport = DefaultTransport
	}

	if config.Server.Host == "" {
		config.Server.Host = DefaultHost
	}

	if config.Server.Port == 0 {
		config.Server.Port = DefaultServerPort
	}

	if config.Metrics.Port == 0 {
		config.Metrics.Port = DefaultMetricsPort
	}

	if config.Metrics.Path == "" {
		config.Metrics.Path = DefaultMetricsPath
	}

	if config.Resilience.RateLimitPerMinute == 0 {
		config.Resilience.RateLimitPerMinute = DefaultRateLimitPerMinute
	}

	if config.Resilience.CircuitBreakerThreshold == 0 {
		config.Resilience.CircuitBreakerThreshold = DefaultCircuitBreakerThreshold
	}

	if config.Resilience.CircuitBreakerTimeout == 0 {
		config.Resilience.CircuitBreakerTimeout = DefaultCircuitBreakerTimeout
	}

	if config.Resilience.MaxRetries == 0 {
		config.Resilience.MaxRetries = DefaultMaxRetries
	}

	if config.Resilience.BaseRetryDelay == 0 {
		config.Resilience.BaseRetryDelay = DefaultBaseRetryDelay
	}

	if config.Resilience.MaxRetryDelay == 0 {
		config.Resilience.MaxRetryDelay = DefaultMaxRetryDelay
	}

	if config.Tracing.SampleRate == 0 {
		config.Tracing.SampleRate = DefaultSampleRate
	}
}

func (l *Loader) applyEnvironmentOverrides(config *Config) {
	originallyNilEnvironments := config.Environments == nil

	anyEnvVarsSet := false

	if v := os.Getenv("LINODEMCP_SERVER_NAME"); v != "" {
		config.Server.Name = v
		anyEnvVarsSet = true
	}

	if v := os.Getenv("LINODEMCP_LOG_LEVEL"); v != "" {
		config.Server.LogLevel = v
		anyEnvVarsSet = true
	}

	if config.Environments == nil {
		config.Environments = make(map[string]EnvironmentConfig)
	}

	defaultEnv := config.Environments[defaultEnvironmentName]
	linodeEnvVarsSet := false

	if v := os.Getenv("LINODEMCP_LINODE_API_URL"); v != "" {
		defaultEnv.Linode.APIURL = v
		linodeEnvVarsSet = true
		anyEnvVarsSet = true
	}

	if v := os.Getenv("LINODEMCP_LINODE_TOKEN"); v != "" {
		defaultEnv.Linode.Token = v
		linodeEnvVarsSet = true
		anyEnvVarsSet = true
	}

	if linodeEnvVarsSet || (originallyNilEnvironments && anyEnvVarsSet) {
		if defaultEnv.Label == "" {
			defaultEnv.Label = defaultEnvironmentLabel
		}

		config.Environments[defaultEnvironmentName] = defaultEnv
	}
}

const (
	appDirName     = "linodemcp"
	configDirName  = ".config"
	configFileJSON = "config.json"
	configFileYAML = "config.yml"

	defaultEnvironmentName  = "default"
	defaultEnvironmentLabel = "Default"
)

// Default server configuration values.
const (
	DefaultServerName  = "LinodeMCP"
	DefaultLogLevel    = "info"
	DefaultTransport   = "stdio"
	DefaultHost        = "127.0.0.1"
	DefaultServerPort  = 8080
	DefaultMetricsPort = 9090
	DefaultMetricsPath = "/metrics"
)

// Default resilience configuration values.
const (
	DefaultRateLimitPerMinute      = 700
	DefaultCircuitBreakerThreshold = 5
	DefaultCircuitBreakerTimeout   = 30 * time.Second
	DefaultMaxRetries              = 3
	DefaultBaseRetryDelay          = 1 * time.Second
	DefaultMaxRetryDelay           = 30 * time.Second
)

// DefaultSampleRate is the default tracing sample rate.
const DefaultSampleRate = 1.0

// DirPermissions is the default permission for config directories.
const DirPermissions os.FileMode = 0o755

// FilePermissions is the default permission for config files.
const FilePermissions os.FileMode = 0o600

// Load reads and returns the configuration from the default path.
func Load() (*Config, error) {
	return NewLoader().Load()
}

// LoadFromFile reads and returns the configuration from the given file path.
func LoadFromFile(path string) (*Config, error) {
	return NewLoader(WithConfigPath(path)).LoadFromFile(path)
}

// GetConfigDir returns the directory containing the config file.
func GetConfigDir() string {
	if customPath := os.Getenv("LINODEMCP_CONFIG_PATH"); customPath != "" {
		if err := validatePath(customPath); err != nil {
			return getDefaultConfigDir()
		}

		return filepath.Dir(customPath)
	}

	return getDefaultConfigDir()
}

func getDefaultConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), appDirName)
	}

	return filepath.Join(homeDir, configDirName, appDirName)
}

// GetConfigPath returns the full path to the config file.
func GetConfigPath() string {
	if customPath := os.Getenv("LINODEMCP_CONFIG_PATH"); customPath != "" {
		if err := validatePath(customPath); err != nil {
			return getDefaultConfigPath()
		}

		return customPath
	}

	return getDefaultConfigPath()
}

func getDefaultConfigPath() string {
	configDir := GetConfigDir()

	jsonPath := filepath.Join(configDir, configFileJSON)

	if _, err := os.Stat(jsonPath); err == nil {
		return jsonPath
	}

	return filepath.Join(configDir, configFileYAML)
}

// Exists returns true if a config file exists at the default path.
func Exists() bool {
	_, err := os.Stat(GetConfigPath())

	return err == nil
}

func validateConfig(config *Config) error {
	if config == nil {
		return ErrConfigNil
	}

	if config.Server.Name == "" {
		return fmt.Errorf("%w: server name cannot be empty", ErrConfigInvalid)
	}

	if config.Server.LogLevel == "" {
		return fmt.Errorf("%w: log level cannot be empty", ErrConfigInvalid)
	}

	if len(config.Environments) == 0 {
		return ErrNoEnvironments
	}

	for envName, env := range config.Environments {
		if envName == "" {
			return ErrEmptyEnvironmentName
		}

		if env.Linode.APIURL != "" || env.Linode.Token != "" {
			if env.Linode.APIURL == "" {
				return fmt.Errorf("%w: environment '%s': Linode API URL is required when token is provided", ErrConfigInvalid, envName)
			}

			if env.Linode.Token == "" {
				return fmt.Errorf("%w: environment '%s': Linode token is required when API URL is provided", ErrConfigInvalid, envName)
			}
		}
	}

	return nil
}

// SelectEnvironment picks a Linode environment from the config.
func (c *Config) SelectEnvironment(userInput string) (*EnvironmentConfig, error) {
	if strings.TrimSpace(userInput) == "" {
		return nil, ErrEmptyEnvironmentName
	}

	if len(c.Environments) == 0 {
		return nil, fmt.Errorf("%w: no provider environments configured", ErrEnvironmentNotFound)
	}

	userInputLower := strings.ToLower(strings.TrimSpace(userInput))

	for envName, env := range c.Environments {
		if strings.ToLower(envName) == userInputLower {
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

// GetLinodeEnvironment returns the LinodeConfig for a named environment.
func (c *Config) GetLinodeEnvironment(environmentName string) (*LinodeConfig, error) {
	if len(c.Environments) == 0 {
		return nil, fmt.Errorf("%w: no provider environments configured", ErrEnvironmentNotFound)
	}

	env, exists := c.Environments[environmentName]
	if !exists {
		return nil, fmt.Errorf("%w: environment '%s' not found", ErrEnvironmentNotFound, environmentName)
	}

	return &env.Linode, nil
}

// EnsureConfigDir creates the config directory if it does not exist.
func EnsureConfigDir() error {
	configDir := GetConfigDir()

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, DirPermissions); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	return nil
}

// CreateTemplateConfig writes a template config file if one does not exist.
func CreateTemplateConfig() error {
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	configPath := GetConfigPath()

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		return nil
	}

	return createYAMLTemplate(configPath)
}

func createYAMLTemplate(configPath string) error {
	template := `# LinodeMCP Configuration
server:
  name: "LinodeMCP"
  logLevel: "info"
  transport: "stdio"
  host: "127.0.0.1"
  port: 8080

metrics:
  enabled: true
  port: 9090
  path: "/metrics"

tracing:
  enabled: false
  exporter: "otlp"
  endpoint: "localhost:4317"
  sampleRate: 1.0

resilience:
  rateLimitPerMinute: 700
  circuitBreakerThreshold: 5
  circuitBreakerTimeout: 30s
  maxRetries: 3
  baseRetryDelay: 1s
  maxRetryDelay: 30s

environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "${LINODEMCP_LINODE_TOKEN}"
`

	if err := os.WriteFile(configPath, []byte(template), FilePermissions); err != nil {
		return fmt.Errorf("failed to create template config: %w", err)
	}

	return nil
}

// InitializeConfig ensures the config directory and template file exist.
func InitializeConfig() error {
	if err := EnsureConfigDir(); err != nil {
		return fmt.Errorf("failed to initialize config directory: %w", err)
	}

	if !Exists() {
		if err := CreateTemplateConfig(); err != nil {
			return fmt.Errorf("failed to create template config: %w", err)
		}
	}

	return nil
}
