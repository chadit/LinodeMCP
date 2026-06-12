// Package appinfo provides build-time version information for LinodeMCP.
package appinfo

import (
	"fmt"
	"runtime"
)

// APIVersion is the current MCP API version.
const APIVersion = "0.1.0"

// Version is the LinodeMCP version. The literals below are the
// development defaults; release builds overwrite all three so a tagged
// binary reports the tag and commit it was built from.
//
//nolint:gochecknoglobals // set via -ldflags at build time
var (
	Version   = "0.1.0"
	commit    = "unknown"
	buildDate = "unknown"
)

// Info holds build and version metadata for the LinodeMCP server.
type Info struct {
	Version    string `json:"version"`
	APIVersion string `json:"api_version"`
	BuildDate  string `json:"build_date"`
	Commit     string `json:"commit"`
	Platform   string `json:"platform"`
}

// Get returns the current version information.
func Get() Info {
	return Info{
		Version:    Version,
		APIVersion: APIVersion,
		BuildDate:  buildDate,
		Commit:     commit,
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
