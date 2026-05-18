// Package cli implements the linodemcp command-line subcommands.
// Phase 7a ships read-only enumeration:
//
//	linodemcp profile list           lists every built-in and user profile
//	linodemcp profile show <name>    prints one profile's full details
//
// Mutation (use, clone, delete, enable, disable) lands in 7b/7c with
// atomic config writes. This package intentionally does not import the
// watcher or the linode client; subcommands here load the config once,
// build the catalog, and print. cmd/linodemcp dispatches into the
// exported entry points.
package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/server"
)

// ExitUsageError is the conventional exit code for argument-shape
// problems (matches sysexits' EX_USAGE). Exported so main can use the
// same constant.
const ExitUsageError = 2

// Column widths for the `profile list` table. Extracted so the row
// formatting and header stay in sync (and so mnd doesn't flag them).
const (
	listColMarker = 3
	listColName   = 20
	listColYolo   = 8
	listColState  = 8
)

const profileUsage = `Usage: linodemcp profile <subcommand> [args]

Read-only:
  list             List all built-in and user-defined profiles.
  show <name>      Show details for a single profile.

Mutators (atomic config write, comments and ordering not preserved):
  use <name>            Switch the active profile.
  enable <name>         Clear the disabled flag on a built-in profile.
  disable <name>        Set the disabled flag on a built-in profile.
  clone <src> <dst>     Copy any profile into a new user-defined entry.
  delete <name>         Remove a user-defined profile.`

// RunProfileCommand dispatches `linodemcp profile <subcommand> ...` and
// returns the exit code. Unknown subcommand or empty args print usage to
// stderr and exit with ExitUsageError. Output streams are parameters so
// tests can capture them without swapping os.Stdout/Stderr.
func RunProfileCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeln(stderr, profileUsage)

		return ExitUsageError
	}

	switch args[0] {
	case "list":
		return RunProfileList(args[1:], stdout, stderr)
	case "show":
		return RunProfileShow(args[1:], stdout, stderr)
	case "use":
		return RunProfileUse(args[1:], "", stdout, stderr)
	case "enable":
		return RunProfileEnable(args[1:], "", stdout, stderr)
	case "disable":
		return RunProfileDisable(args[1:], "", stdout, stderr)
	case "clone":
		return RunProfileClone(args[1:], "", stdout, stderr)
	case "delete":
		return RunProfileDelete(args[1:], "", stdout, stderr)
	default:
		writef(stderr, "unknown profile subcommand: %s\n\n%s\n", args[0], profileUsage)

		return ExitUsageError
	}
}

// RunProfileList enumerates every built-in plus user-defined profile in
// the active config and prints a one-line summary per profile. The
// active profile is marked with a leading '*'.
func RunProfileList(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		writef(stderr, "profile list takes no arguments, got: %v\n", args)

		return ExitUsageError
	}

	cfg, err := loadConfigOrError(stderr)
	if err != nil {
		return 1
	}

	all := AllProfiles(cfg)
	active := ResolveActiveName(cfg)
	names := sortedNames(all)

	header := fmt.Sprintf(
		"%-*s %-*s %-*s %-*s %s\n",
		listColMarker, "*", listColName, "name",
		listColYolo, "yolo", listColState, "state", "tools",
	)
	writef(stdout, "%s", header)

	for _, name := range names {
		prof := all[name]
		marker := " "

		if name == active {
			marker = "*"
		}

		state := "enabled"
		if prof.Disabled {
			state = "DISABLED"
		}

		yolo := "no"
		if prof.AllowYolo {
			yolo = "YES"
		}

		writef(
			stdout,
			"%-*s %-*s %-*s %-*s %d\n",
			listColMarker, marker, listColName, name,
			listColYolo, yolo, listColState, state, len(prof.AllowedTools),
		)
	}

	return 0
}

// RunProfileShow prints one profile's full detail. Looks up by exact
// name across built-ins and user-defined profiles. An unknown name
// exits 1 with a list of valid names to help the user recover.
func RunProfileShow(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		writeln(stderr, "Usage: linodemcp profile show <name>")

		return ExitUsageError
	}

	name := args[0]

	cfg, err := loadConfigOrError(stderr)
	if err != nil {
		return 1
	}

	all := AllProfiles(cfg)

	prof, ok := all[name]
	if !ok {
		writef(stderr, "profile %q not found.\n", name)
		writeln(stderr, "Available profiles:")

		for _, n := range sortedNames(all) {
			writef(stderr, "  %s\n", n)
		}

		return 1
	}

	PrintProfileDetail(stdout, &prof, ResolveActiveName(cfg))

	return 0
}

// PrintProfileDetail writes one Profile in a stable human-readable
// shape. Exported so tests can exercise the formatting in isolation.
// Takes prof by pointer to dodge gocritic's hugeParam check.
func PrintProfileDetail(stdout io.Writer, prof *profiles.Profile, active string) {
	writef(stdout, "Profile: %s", prof.Name)

	if prof.Name == active {
		writef(stdout, " (active)")
	}

	writeln(stdout)
	writef(stdout, "Description: %s\n", prof.Description)
	writef(stdout, "Disabled: %t\n", prof.Disabled)
	writef(stdout, "Allow yolo: %t\n", prof.AllowYolo)

	if len(prof.AllowedEnvironments) == 0 {
		writeln(stdout, "Allowed environments: <all>")
	}

	if len(prof.AllowedEnvironments) > 0 {
		writef(stdout, "Allowed environments: %s\n",
			strings.Join(prof.AllowedEnvironments, ", "))
	}

	writef(stdout, "Required token scopes (%d):\n", len(prof.RequiredTokenScopes))

	for _, s := range prof.RequiredTokenScopes {
		writef(stdout, "  %s\n", s)
	}

	writef(stdout, "Allowed tools (%d):\n", len(prof.AllowedTools))

	for _, t := range prof.AllowedTools {
		writef(stdout, "  %s\n", t)
	}
}

// loadConfigOrError reads the user config from the standard path,
// emitting a friendly error to stderr on failure. Returns nil cfg on
// error so callers can early-return.
func loadConfigOrError(stderr io.Writer) (*config.Config, error) {
	path := config.GetConfigPath()

	cfg, err := config.Load(path)
	if err != nil {
		writef(stderr, "load config from %s: %v\n", path, err)

		return nil, fmt.Errorf("load config: %w", err)
	}

	return cfg, nil
}

// RunProfileUse switches the active profile. The named profile must
// exist (built-in or user-defined); unknown names exit 1 without
// writing. On success the rewrite is atomic: a malformed write never
// replaces the existing file.
//
// configPath identifies the config file to load and rewrite; passing
// an empty string falls back to “config.GetConfigPath()“ so the
// production dispatcher (cmd/linodemcp) doesn't have to repeat the
// lookup. Tests pass a tempdir-rooted path.
//
// Comments and key ordering in the source YAML/JSON are NOT preserved;
// this trade-off is documented in the usage block above.
func RunProfileUse(args []string, configPath string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		writeln(stderr, "Usage: linodemcp profile use <name>")

		return ExitUsageError
	}

	name := args[0]
	path := resolveConfigPath(configPath)

	cfg, err := loadConfigFromPath(path, stderr)
	if err != nil {
		return 1
	}

	if _, ok := AllProfiles(cfg)[name]; !ok {
		writef(stderr, "profile %q not found.\n", name)

		return 1
	}

	cfg.ActiveProfile = name

	return writeAndReport(path, cfg, stdout, stderr, "active profile switched to "+name)
}

// RunProfileEnable clears the disabled flag on a built-in profile via
// the ProfilesBuiltinOverrides map. Only built-ins are subject to this
// override (user-defined profiles cannot be disabled today). See
// RunProfileUse for the configPath parameter semantics.
func RunProfileEnable(args []string, configPath string, stdout, stderr io.Writer) int {
	return runProfileToggle(args, configPath, stdout, stderr, false, "enabled")
}

// RunProfileDisable sets the disabled flag on a built-in profile via
// the same override map RunProfileEnable clears. Disabling the active
// profile is rejected so the server cannot get stuck.
func RunProfileDisable(args []string, configPath string, stdout, stderr io.Writer) int {
	return runProfileToggle(args, configPath, stdout, stderr, true, "disabled")
}

// runProfileToggle is the shared body for enable/disable. disabled
// selects the target value; verb is the past-tense word used in the
// success message ("enabled"/"disabled").
func runProfileToggle(
	args []string,
	configPath string,
	stdout, stderr io.Writer,
	disabled bool,
	verb string,
) int {
	if len(args) != 1 {
		writef(stderr, "Usage: linodemcp profile %s <name>\n", verb)

		return ExitUsageError
	}

	name := args[0]
	path := resolveConfigPath(configPath)

	cfg, err := loadConfigFromPath(path, stderr)
	if err != nil {
		return 1
	}

	if !isBuiltinName(name) {
		writef(
			stderr,
			"profile %q is not a built-in; enable/disable only applies to built-in profiles.\n",
			name,
		)

		return 1
	}

	if disabled && ResolveActiveName(cfg) == name {
		writef(
			stderr,
			"profile %q is the active profile; switch first via `profile use <other>` before disabling.\n",
			name,
		)

		return 1
	}

	if cfg.ProfilesBuiltinOverrides == nil {
		cfg.ProfilesBuiltinOverrides = map[string]config.BuiltinOverride{}
	}

	cfg.ProfilesBuiltinOverrides[name] = config.BuiltinOverride{Disabled: disabled}

	return writeAndReport(path, cfg, stdout, stderr, "profile "+name+" "+verb)
}

// resolveConfigPath returns the explicit configPath when non-empty,
// otherwise falls back to “config.GetConfigPath()“. The fallback
// branch covers the production runtime; the explicit-path branch
// supports tempdir-rooted config files in unit tests.
func resolveConfigPath(configPath string) string {
	if configPath != "" {
		return configPath
	}

	return config.GetConfigPath()
}

// loadConfigFromPath reads the config from path, mirroring the error-
// reporting shape of the original loadConfigOrError but without
// reaching for “GetConfigPath“ itself.
func loadConfigFromPath(path string, stderr io.Writer) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		writef(stderr, "load config from %s: %v\n", path, err)

		return nil, fmt.Errorf("load config: %w", err)
	}

	return cfg, nil
}

// writeAndReport calls config.WriteAtomic and prints either the
// success message or the failure. Exists so the three mutator
// subcommands share the same I/O footprint.
func writeAndReport(
	path string,
	cfg *config.Config,
	stdout, stderr io.Writer,
	success string,
) int {
	if err := config.WriteAtomic(path, cfg); err != nil {
		writef(stderr, "write config to %s: %v\n", path, err)

		return 1
	}

	writef(stdout, "%s\n", success)

	return 0
}

// RunProfileClone copies a source profile into a new user-defined
// entry under dst. The source can be a built-in or a user-defined
// profile. The dst name must be free: it cannot collide with a
// built-in (those are immutable in the catalog), with another user-
// defined entry (no silent overwrite), or be empty. The clone
// captures the source's description, allowed_tools, scopes, etc; the
// user can then edit the YAML to customize.
func RunProfileClone(args []string, configPath string, stdout, stderr io.Writer) int {
	const expectedArgs = 2

	if len(args) != expectedArgs {
		writeln(stderr, "Usage: linodemcp profile clone <src> <dst>")

		return ExitUsageError
	}

	src, dst := args[0], args[1]

	if dst == "" {
		writeln(stderr, "destination name cannot be empty")

		return 1
	}

	if isBuiltinName(dst) {
		writef(stderr, "destination %q collides with a built-in profile name; pick another.\n", dst)

		return 1
	}

	path := resolveConfigPath(configPath)

	cfg, err := loadConfigFromPath(path, stderr)
	if err != nil {
		return 1
	}

	if _, exists := cfg.Profiles[dst]; exists {
		writef(stderr, "user-defined profile %q already exists; pick another or delete it first.\n", dst)

		return 1
	}

	source, ok := AllProfiles(cfg)[src]
	if !ok {
		writef(stderr, "source profile %q not found.\n", src)

		return 1
	}

	if cfg.Profiles == nil {
		cfg.Profiles = map[string]config.UserProfileConfig{}
	}

	cfg.Profiles[dst] = config.UserProfileConfig{
		Description:         source.Description,
		AllowedTools:        append([]string(nil), source.AllowedTools...),
		AllowedEnvironments: append([]string(nil), source.AllowedEnvironments...),
		RequiredTokenScopes: append([]string(nil), source.RequiredTokenScopes...),
		AllowYolo:           source.AllowYolo,
	}

	return writeAndReport(
		path, cfg, stdout, stderr,
		"profile "+dst+" cloned from "+src,
	)
}

// RunProfileDelete removes a user-defined profile by name. Built-ins
// cannot be deleted (they live in code, not config) and the currently-
// active profile cannot be removed since that would prevent the
// server from starting.
func RunProfileDelete(args []string, configPath string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		writeln(stderr, "Usage: linodemcp profile delete <name>")

		return ExitUsageError
	}

	name := args[0]

	if isBuiltinName(name) {
		writef(stderr, "profile %q is a built-in; built-ins cannot be deleted (try `profile disable`).\n", name)

		return 1
	}

	path := resolveConfigPath(configPath)

	cfg, err := loadConfigFromPath(path, stderr)
	if err != nil {
		return 1
	}

	if _, exists := cfg.Profiles[name]; !exists {
		writef(stderr, "user-defined profile %q not found.\n", name)

		return 1
	}

	if ResolveActiveName(cfg) == name {
		writef(
			stderr,
			"profile %q is the active profile; switch first via `profile use <other>` before deleting.\n",
			name,
		)

		return 1
	}

	delete(cfg.Profiles, name)

	return writeAndReport(path, cfg, stdout, stderr, "profile "+name+" deleted")
}

// isBuiltinName reports whether name matches any of the eight
// built-in profile names. Used by enable/disable to refuse to operate
// on user-defined profiles where the override map has no effect.
func isBuiltinName(name string) bool {
	switch name {
	case profiles.BuiltinDefault,
		profiles.BuiltinReadonlyFull,
		profiles.BuiltinComputeAdmin,
		profiles.BuiltinNetworkAdmin,
		profiles.BuiltinKubernetesAdmin,
		profiles.BuiltinStorageAdmin,
		profiles.BuiltinFullAccess,
		profiles.BuiltinEmergency:
		return true
	default:
		return false
	}
}

// AllProfiles returns every profile the user could activate, keyed by
// name. Built-ins come first; user-defined entries from config.Profiles
// shadow built-ins of the same name (matching the resolver's order).
func AllProfiles(cfg *config.Config) map[string]profiles.Profile {
	catalog := server.ToolDescriptors(cfg)
	builtins := profiles.BuiltinProfiles(catalog)

	overrides := cfg.ProfilesBuiltinOverrides
	if overrides == nil {
		overrides = map[string]config.BuiltinOverride{}
	}

	out := make(map[string]profiles.Profile, len(builtins)+len(cfg.Profiles))

	for name, prof := range builtins {
		if override, ok := overrides[name]; ok {
			prof.Disabled = override.Disabled
		}

		out[name] = prof
	}

	for name, userCfg := range cfg.Profiles {
		out[name] = profiles.Profile{
			Name:                name,
			Description:         userCfg.Description,
			AllowedTools:        append([]string(nil), userCfg.AllowedTools...),
			AllowedEnvironments: append([]string(nil), userCfg.AllowedEnvironments...),
			RequiredTokenScopes: append([]string(nil), userCfg.RequiredTokenScopes...),
			AllowYolo:           userCfg.AllowYolo,
		}
	}

	return out
}

// ResolveActiveName returns the active profile name from config,
// defaulting to "default" when unset.
func ResolveActiveName(cfg *config.Config) string {
	if cfg.ActiveProfile == "" {
		return profiles.BuiltinDefault
	}

	return cfg.ActiveProfile
}

// sortedNames returns the keys of a profile map in ascending order so
// list/show output stays stable. The parameter is named `catalog` to
// avoid shadowing the imported `profiles` package.
func sortedNames(catalog map[string]profiles.Profile) []string {
	names := make([]string, 0, len(catalog))
	for name := range catalog {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// writef wraps fmt.Fprintf and discards its (n, err) result. CLI output
// failures (broken pipe, full disk) cannot be meaningfully recovered
// here; using a helper keeps every call site free of `_, _ =` noise.
func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// writeln is the println-flavored sibling of writef. Same rationale.
func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}
