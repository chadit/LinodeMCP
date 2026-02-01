package config

// ValidateConfig exposes validateConfig for testing.
func ValidateConfig(cfg *Config) error {
	return validateConfig(cfg)
}

// ParseConfigData exposes parseConfigData for testing.
func ParseConfigData(data []byte, cfg *Config) error {
	return parseConfigData(data, cfg)
}

// ValidatePath exposes validatePath for testing.
func ValidatePath(path string) error {
	return validatePath(path)
}

// CacheManagerPathValidationCache returns the path validation cache for testing.
func (cm *CacheManager) PathValidationCache() map[string]error {
	return cm.pathValidationCache
}

// CacheManagerAllowedDirsCache returns the allowed dirs cache for testing.
func (cm *CacheManager) AllowedDirsCache() []string {
	return cm.allowedDirsCache
}

// CacheManagerAllowedDirsCached returns whether the allowed dirs cache is populated.
func (cm *CacheManager) AllowedDirsCached() bool {
	return cm.allowedDirsCached
}

// CacheManagerValidatePath exposes the instance method for testing.
func (cm *CacheManager) ValidatePath(path string) error {
	return cm.validatePath(path)
}
