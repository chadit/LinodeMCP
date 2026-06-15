package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/server"
)

// Fixture config constants. The default environment carries a
// placeholder token so the server constructs; meta tools (hello, version,
// the audit queries) never reach the Linode API, so no real credential is
// needed.
const (
	testServerName = "Test"
	testLogLevel   = "info"
	testTransport  = "stdio"
	testHost       = "127.0.0.1"
	testPort       = 8080
	testEnvKey     = "default"
	testEnvLabel   = "Default"
	testAPIURL     = "https://api.linode.com/v4"
	testToken      = "tok"
)

// testConfig returns a minimal in-memory config sufficient to build a
// server whose meta tools run without touching the network.
func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Name:      testServerName,
			LogLevel:  testLogLevel,
			Transport: testTransport,
			Host:      testHost,
			Port:      testPort,
		},
		Environments: map[string]config.EnvironmentConfig{
			testEnvKey: {
				Label:  testEnvLabel,
				Linode: config.LinodeConfig{APIURL: testAPIURL, Token: testToken},
			},
		},
	}
}

// newTestServer builds an in-process server from the test config. Used by
// the parity and dispatch tests, which inspect the catalog and drive
// HandleMessage directly rather than going through the config-file path.
func newTestServer(t *testing.T) *server.Server {
	t.Helper()

	srv, err := server.New(testConfig())
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	return srv
}

// writeTestConfigFile stages the test config as a YAML file in a tempdir
// and returns its path. The end-to-end command tests point
// LINODEMCP_CONFIG_PATH at this file so RunCallCommand loads it through
// the real config path instead of the user's actual config.
func writeTestConfigFile(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	contents := `
server:
  name: "Test"
  logLevel: "info"
  transport: "stdio"
  host: "127.0.0.1"
  port: 8080
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
`

	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	return path
}
