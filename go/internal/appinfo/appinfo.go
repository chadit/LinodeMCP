// Package appinfo provides build-time version information for LinodeMCP.
package appinfo

import (
	"fmt"
	"runtime"
)

// Version is the current LinodeMCP version.
const Version = "0.1.0"

// APIVersion is the current MCP API version.
const APIVersion = "0.1.0"

//nolint:gochecknoglobals // set via -ldflags at build time
var buildDate = "unknown"

// Info holds build and version metadata for the LinodeMCP server.
type Info struct {
	Version    string `json:"version"`
	APIVersion string `json:"api_version"`
	BuildDate  string `json:"build_date"`
	Platform   string `json:"platform"`
}

// Get returns the current version information.
func Get() Info {
	return Info{
		Version:    Version,
		APIVersion: APIVersion,
		BuildDate:  buildDate,
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
