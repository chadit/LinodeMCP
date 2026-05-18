package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// configFormat identifies the on-disk serialization the writer should
// use. Detected from the target file's extension so the round-trip
// preserves whatever the operator originally chose.
type configFormat int

const (
	formatYAML configFormat = iota
	formatJSON
)

// writeAtomicMode is the POSIX permission bits for newly-written
// config files. Operators may have stricter permissions on an existing
// file; WriteAtomic preserves the original mode when one is found.
const writeAtomicMode os.FileMode = 0o600

// WriteAtomic rewrites the config file at path with the in-memory cfg,
// using a temp-file-and-rename pattern that survives crashes mid-write.
// The on-disk format is detected from the file extension: “.json“
// uses JSON, anything else (“.yml“, “.yaml“, or no extension) uses
// YAML. Comments and key ordering from the original file are NOT
// preserved; this trade-off is documented in the user-facing help.
//
// The freshly-marshaled file is validated by round-tripping through
// Load before the rename, so a malformed write never replaces a good
// config. Permission bits from the original file are preserved when
// they exist; new files default to 0600.
func WriteAtomic(path string, cfg *Config) error {
	if cfg == nil {
		return ErrNilConfig
	}

	format := detectFormat(path)

	data, err := marshalConfig(cfg, format)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if validateErr := validateRoundTrip(data); validateErr != nil {
		return fmt.Errorf("round-trip validation: %w", validateErr)
	}

	mode := preserveModeOrDefault(path, writeAtomicMode)

	return writeTempThenRename(path, data, mode)
}

// detectFormat picks the serializer by file extension. JSON requires
// the explicit “.json“ extension; everything else defaults to YAML
// (matches Load's parse-then-fallback behavior).
func detectFormat(path string) configFormat {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".json" {
		return formatJSON
	}

	return formatYAML
}

// marshalConfig serializes cfg in the requested format. JSON output is
// indented two spaces (matches the YAML default flow) for readability
// when operators inspect the rewritten file by hand.
func marshalConfig(cfg *Config, format configFormat) ([]byte, error) {
	if format == formatJSON {
		out, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encode JSON: %w", err)
		}

		return append(out, '\n'), nil
	}

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("encode YAML: %w", err)
	}

	return out, nil
}

// validateRoundTrip parses the freshly-marshaled data through the same
// pipeline Load uses. Catches schema regressions and any drift in the
// marshaling path that would otherwise produce a file the server can't
// read back. The applied side-effects (setDefaults, applyEnvironment-
// Overrides) are intentional: a config that fails validation here
// would also fail at server startup, so failing fast is better.
func validateRoundTrip(data []byte) error {
	var cfg Config

	if err := parseConfigData(data, &cfg); err != nil {
		return fmt.Errorf("re-parse: %w", err)
	}

	setDefaults(&cfg)

	if err := validateConfig(&cfg); err != nil {
		return fmt.Errorf("re-validate: %w", err)
	}

	return nil
}

// preserveModeOrDefault returns the existing file's mode bits when it
// exists, falling back to fallback for new files. Lets WriteAtomic
// keep a 0400 hardened config at 0400 across rewrites.
func preserveModeOrDefault(path string, fallback os.FileMode) os.FileMode {
	info, err := os.Stat(path) // #nosec G304 -- path is the operator-supplied config target
	if err != nil {
		return fallback
	}

	return info.Mode().Perm()
}

// writeTempThenRename performs the atomic-replace: temp file in the
// same directory (so rename is on the same filesystem), fsync, rename.
// On any failure the temp file is removed so the directory does not
// leak partial writes.
func writeTempThenRename(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	tmp, err := os.CreateTemp(dir, "."+base+".tmp.*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}

	tmpPath := tmp.Name()

	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()

		cleanup()

		return fmt.Errorf("write temp file %s: %w", tmpPath, err)
	}

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()

		cleanup()

		return fmt.Errorf("sync temp file %s: %w", tmpPath, err)
	}

	if err := tmp.Close(); err != nil {
		cleanup()

		return fmt.Errorf("close temp file %s: %w", tmpPath, err)
	}

	if err := os.Chmod(tmpPath, mode); err != nil {
		cleanup()

		return fmt.Errorf("chmod temp file %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()

		return fmt.Errorf("rename %s -> %s: %w", tmpPath, path, err)
	}

	return nil
}
