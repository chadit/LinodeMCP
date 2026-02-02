// Package version provides build-time version information for LinodeMCP.
package version

import (
	"fmt"
	"runtime"
)

// Version is the current LinodeMCP version.
const Version = "0.1.0"

// APIVersion is the current MCP API version.
const APIVersion = "0.1.0"

const (
	defaultBuildDate = "unknown"

	featureKeyTools    = "tools"
	featureKeyLogging  = "logging"
	featureKeyProtocol = "protocol"

	featureToolsList = "hello,version"
	featureLogging   = "basic"
	featureProtocol  = "mcp"
)

// Build-time variables set via ldflags.
var (
	BuildDate = ""     //nolint:gochecknoglobals // Build-time variable set via ldflags
	GitCommit = "dev"  //nolint:gochecknoglobals // Build-time variable set via ldflags
	GitBranch = "main" //nolint:gochecknoglobals // Build-time variable set via ldflags
)

// Info holds build and version metadata for the LinodeMCP server.
type Info struct {
	Version    string            `json:"version"`
	APIVersion string            `json:"api_version"` //nolint:tagliatelle // Linode API snake_case convention
	BuildDate  string            `json:"build_date"`  //nolint:tagliatelle // Linode API snake_case convention
	GitCommit  string            `json:"git_commit"`  //nolint:tagliatelle // Linode API snake_case convention
	GitBranch  string            `json:"git_branch"`  //nolint:tagliatelle // Linode API snake_case convention
	GoVersion  string            `json:"go_version"`  //nolint:tagliatelle // Linode API snake_case convention
	Platform   string            `json:"platform"`
	Features   map[string]string `json:"features"`
}

// Get returns the current version information.
func Get() Info {
	buildDate := BuildDate
	if buildDate == "" {
		buildDate = defaultBuildDate
	}

	return Info{
		Version:    Version,
		APIVersion: APIVersion,
		BuildDate:  buildDate,
		GitCommit:  GitCommit,
		GitBranch:  GitBranch,
		GoVersion:  runtime.Version(),
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		Features: map[string]string{
			featureKeyTools:    featureToolsList,
			featureKeyLogging:  featureLogging,
			featureKeyProtocol: featureProtocol,
		},
	}
}

// String returns a human-readable version string.
func (i Info) String() string {
	return fmt.Sprintf("LinodeMCP v%s (MCP: v%s, %s, %s)",
		i.Version, i.APIVersion, i.Platform, i.GitCommit)
}
