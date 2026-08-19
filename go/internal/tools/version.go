package tools

import (
	"github.com/chadit/LinodeMCP/go/internal/appinfo"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// VersionResponseProto builds the canonical VersionResponse proto from the
// build-time metadata. The `version` MCP tool and the CLI `version` subcommand
// both serialize this message so the two surfaces (and the Python server) emit
// the same field set.
func VersionResponseProto() *linodev1.VersionResponse {
	info := appinfo.Get()

	return &linodev1.VersionResponse{
		Version:    info.Version,
		ApiVersion: info.APIVersion,
		BuildDate:  info.BuildDate,
		Commit:     info.Commit,
		Platform:   info.Platform,
	}
}
