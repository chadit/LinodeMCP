package config

// ExportedValidateConfig exposes validateConfig for testing.
func ExportedValidateConfig(cfg *Config) error {
	return validateConfig(cfg)
}

// ExportedParseConfigData exposes parseConfigData for testing.
func ExportedParseConfigData(data []byte, cfg *Config) error {
	return parseConfigData(data, cfg)
}

// ExportedValidatePath exposes validatePath for testing.
func ExportedValidatePath(path string) error {
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

// ExportedCacheManagerValidatePath exposes the instance method for testing.
func (cm *CacheManager) ExportedValidatePath(path string) error {
	return cm.validatePath(path)
}
