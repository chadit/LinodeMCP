package tools

import (
	"github.com/chadit/LinodeMCP/go/internal/appinfo"
	"github.com/chadit/LinodeMCP/go/internal/genlocal"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// BuildInfo is the build-time metadata the binary reports about itself, and the
// operation LOCAL_CALL_BUILD_INFO declares. It reads no state and takes no
// input, so the generated arm is handed this function rather than a value.
func BuildInfo() *genlocal.VersionResponse {
	info := appinfo.Get()

	return genlocal.NewVersionResponse(
		info.Version, info.APIVersion, info.BuildDate, info.Commit, info.Platform,
	)
}

// VersionResponseBody is that metadata as the plain body both version surfaces
// answer: the `version` tool through the generated arm, the CLI `version`
// subcommand through VersionResponseJSON. One construction is what keeps the
// two surfaces (and the Python server) on one field set.
func VersionResponseBody() map[string]any {
	return genlocal.ProjectVersionResponse(BuildInfo())
}

// VersionResponseJSON is that same body as the canonical bytes the CLI
// subcommand prints. It goes through the projection a tool answer goes
// through, so the subcommand cannot answer a field the tool does not.
func VersionResponseJSON() ([]byte, error) {
	return LocalBodyJSON(&linodev1.VersionResponse{}, VersionResponseBody())
}
