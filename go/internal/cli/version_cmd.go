package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/chadit/LinodeMCP/internal/appinfo"
)

// RunVersionCommand prints the build and version metadata as JSON and
// returns 0. Mirrors the `version` MCP tool's output so the CLI and the
// tool agree. Output stream is a parameter for tests.
func RunVersionCommand(stdout, stderr io.Writer) int {
	body, err := json.MarshalIndent(appinfo.Get(), "", "  ")
	if err != nil {
		writef(stderr, "marshal version info: %v\n", err)

		return 1
	}

	writeln(stdout, string(body))

	return 0
}

// VersionLine returns a one-line human version string for help banners or
// a `--version` style flag. Kept distinct from RunVersionCommand so a
// caller that wants a terse line doesn't parse JSON.
func VersionLine() string {
	info := appinfo.Get()

	return fmt.Sprintf("linodemcp %s (%s)", info.Version, info.Platform)
}
