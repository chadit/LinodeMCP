package cli

import (
	"fmt"
	"io"

	"github.com/chadit/LinodeMCP/go/internal/appinfo"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// RunVersionCommand prints the build and version metadata as JSON and
// returns 0. It serializes the same VersionResponse proto the `version` MCP
// tool returns, so the CLI subcommand and the tool emit identical bytes.
// Output stream is a parameter for tests.
func RunVersionCommand(stdout, stderr io.Writer) int {
	body, err := tools.MarshalProtoJSON(tools.VersionResponseProto())
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
