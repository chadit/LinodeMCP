package config_test

// Repeated literals extracted to satisfy goconst.
const (
	tcTestServer     = "TestServer"
	tcTest           = "Test"
	tcReloadedServer = "ReloadedServer"
	// envKeyProd is the non-default environment key the SelectEnvironment
	// rows share.
	envKeyProd = "prod"
)

// reloadedServerYAML is the post-reload config body shared by the reload tests.
const reloadedServerYAML = `
server:
  name: "ReloadedServer"
  logLevel: "info"
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
`
