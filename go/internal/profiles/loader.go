package profiles

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chadit/LinodeMCP/internal/config"
)

// wildcardChar is the only glob metacharacter recognized in profile tool
// patterns. Anything else in a pattern is treated literally. Keeping this as
// a constant documents the intentional limit (no regex, no character
// classes).
const wildcardChar = "*"

// ResolveActiveProfile resolves the configured active profile against the
// supplied tool registry and returns the resulting Profile.
//
// Lookup order:
//
//  1. If cfg.ActiveProfile is empty, treat it as the built-in "default".
//  2. If the name matches a built-in, honor any
//     cfg.ProfilesBuiltinOverrides[name].Disabled and return the resolved
//     built-in Profile.
//  3. If the name matches a key in cfg.Profiles, build a Profile by
//     expanding wildcards in AllowedTools and DeniedTools against the
//     registry (denied wins over allowed).
//  4. Otherwise return ErrActiveProfileUnknown.
//
// The registry slice carries (tool name, capability) pairs. Callers (the
// server in Phase 4, tests today) build it from their own tool inventory and
// pass it in. Keeping the registry an argument rather than a package-level
// import keeps internal/profiles free of cycles with internal/server and
// internal/tools.
func ResolveActiveProfile(cfg *config.Config, registry []ToolDescriptor) (Profile, error) {
	if cfg == nil {
		return Profile{}, fmt.Errorf("active profile %q: %w", BuiltinDefault, ErrActiveProfileUnknown)
	}

	name := cfg.ActiveProfile
	if name == "" {
		name = BuiltinDefault
	}

	builtins := BuiltinProfiles(registry)

	if builtin, ok := builtins[name]; ok {
		builtinCopy := builtin

		return resolveBuiltin(name, &builtinCopy, cfg)
	}

	if userCfg, ok := cfg.Profiles[name]; ok {
		warnOverrideIgnored(name, cfg)

		userCopy := userCfg

		return resolveUserDefined(name, &userCopy, registry), nil
	}

	return Profile{}, fmt.Errorf("active profile %q: %w", name, ErrActiveProfileUnknown)
}

// LookupProfile resolves a profile by name across both built-ins and
// user-defined entries, returning the materialized Profile and true on
// hit, or the zero Profile and false on miss. Unlike ResolveActiveProfile
// it ignores the Disabled flag: the Phase 8 builder uses this to clone
// from any profile in the catalog, including disabled built-ins like
// full-access and emergency. User-defined entries shadow built-ins by
// name, matching the precedence ResolveActiveProfile uses.
//
// nil cfg or empty name both return (Profile{}, false); callers that
// need empty-equals-default semantics should fall back themselves.
func LookupProfile(name string, cfg *config.Config, registry []ToolDescriptor) (Profile, bool) {
	if cfg == nil || name == "" {
		return Profile{}, false
	}

	if userCfg, ok := cfg.Profiles[name]; ok {
		userCopy := userCfg

		return resolveUserDefined(name, &userCopy, registry), true
	}

	builtins := BuiltinProfiles(registry)
	if builtin, ok := builtins[name]; ok {
		// Strip Disabled so a cloned-from-disabled draft doesn't carry
		// the flag into the new profile. The user can re-disable via
		// config after the save.
		resolved := builtin
		resolved.Disabled = false

		return resolved, true
	}

	return Profile{}, false
}

// resolveBuiltin applies override toggles (currently just Disabled) to a
// resolved built-in profile. The returned Profile keeps the catalog's
// AllowedTools verbatim; only the disable state is mutable from config.
func resolveBuiltin(name string, builtin *Profile, cfg *config.Config) (Profile, error) {
	override, hasOverride := cfg.ProfilesBuiltinOverrides[name]

	disabled := builtin.Disabled
	if hasOverride {
		disabled = override.Disabled
	}

	if disabled {
		return Profile{}, fmt.Errorf("active profile %q: %w", name, ErrActiveProfileDisabled)
	}

	resolved := *builtin
	resolved.Disabled = false

	return resolved, nil
}

// resolveUserDefined materializes a Profile from a UserProfileConfig by
// expanding wildcards in AllowedTools and DeniedTools against the registry.
// Denied entries are subtracted from the allowed set after expansion so an
// explicit deny always wins over a wildcard allow.
//
// Patterns that match no tool in the registry, and literal names that do not
// appear in the registry, produce a warning but never an error: the spec
// treats both as drift warnings the user can ignore or fix.
func resolveUserDefined(name string, userCfg *config.UserProfileConfig, registry []ToolDescriptor) Profile {
	allowed := expandPatterns(name, "allowed_tools", userCfg.AllowedTools, registry)
	denied := expandPatterns(name, "denied_tools", userCfg.DeniedTools, registry)

	for tool := range denied {
		delete(allowed, tool)
	}

	tools := make([]string, 0, len(allowed))
	for tool := range allowed {
		tools = append(tools, tool)
	}

	sort.Strings(tools)

	return Profile{
		Name:                name,
		Description:         userCfg.Description,
		AllowedTools:        tools,
		AllowedEnvironments: append([]string(nil), userCfg.AllowedEnvironments...),
		RequiredTokenScopes: append([]string(nil), userCfg.RequiredTokenScopes...),
		AllowYolo:           userCfg.AllowYolo,
		Disabled:            false,
	}
}

// expandPatterns walks the supplied patterns and returns the set of registry
// tool names they cover. Wildcards (entries containing "*") are matched with
// shell-glob semantics via filepath.Match; literal entries must match an
// existing tool name. Both forms emit a warning when they resolve to nothing.
func expandPatterns(profileName, fieldName string, patterns []string, registry []ToolDescriptor) map[string]struct{} {
	result := make(map[string]struct{}, len(registry))

	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}

		matches := matchPattern(pattern, registry)
		if len(matches) == 0 {
			logUnmatchedPattern(profileName, fieldName, pattern)

			continue
		}

		for _, name := range matches {
			result[name] = struct{}{}
		}
	}

	return result
}

// matchPattern returns the registry tool names that match the given pattern.
// If the pattern contains "*", filepath.Match is used; otherwise the pattern
// must equal a registered tool name exactly. Malformed glob patterns (e.g. an
// unmatched bracket) match nothing, which surfaces as an unmatched-pattern
// warning to the caller.
func matchPattern(pattern string, registry []ToolDescriptor) []string {
	matches := make([]string, 0)

	if !strings.Contains(pattern, wildcardChar) {
		for _, descriptor := range registry {
			if descriptor.Name == pattern {
				matches = append(matches, descriptor.Name)

				break
			}
		}

		return matches
	}

	for _, descriptor := range registry {
		ok, err := filepath.Match(pattern, descriptor.Name)
		if err != nil || !ok {
			continue
		}

		matches = append(matches, descriptor.Name)
	}

	return matches
}

// logUnmatchedPattern emits a warning for a pattern that resolved to zero
// tools. Literal misses and wildcard misses use the same log line shape so
// operators see consistent diagnostics regardless of which form they used.
func logUnmatchedPattern(profileName, fieldName, pattern string) {
	slog.Warn(
		"profile pattern matched no registered tools",
		"profile", profileName,
		"field", fieldName,
		"pattern", pattern,
	)
}

// warnOverrideIgnored emits a warning when a user-defined profile name
// appears in ProfilesBuiltinOverrides. Overrides only apply to built-ins; the
// entry is ignored for user-defined profiles. The check is best-effort and
// fires only for the active profile to avoid noisy startup logs.
func warnOverrideIgnored(name string, cfg *config.Config) {
	if _, ok := cfg.ProfilesBuiltinOverrides[name]; !ok {
		return
	}

	slog.Warn(
		"builtin override ignored for user-defined profile",
		"profile", name,
	)
}
