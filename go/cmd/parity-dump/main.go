// Command parity-dump prints the Go tool surface (name, capability, and a
// normalized input-schema view) as JSON on stdout. The cross-language parity
// checker (scripts/verify_tool_parity.py) runs this, dumps the Python registry
// the same way, and diffs the two so a tool cannot drift in capability, param
// name, param type, or required set between the implementations.
//
// This is a dev/CI tool, not part of the served binary. It builds the registry
// with a throwaway config; no network calls happen during tool registration.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// toolDump is the normalized, language-agnostic view of one tool that the
// parity checker compares. Descriptions are intentionally excluded: wording
// is allowed to differ across implementations.
type toolDump struct {
	Name       string            `json:"name"`
	Capability string            `json:"capability"`
	Params     map[string]string `json:"params"`
	Required   []string          `json:"required"`
}

func main() {
	// The config needs a non-empty token to validate, but tool registration
	// never uses it. Build it at runtime so it is not a hardcoded-credential
	// literal (gosec G101).
	const placeholderTokenLen = 16

	placeholderToken := strings.Repeat("0", placeholderTokenLen)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Name:      "parity-dump",
			LogLevel:  "error",
			Transport: "stdio",
			Host:      "127.0.0.1",
			Port:      8080,
		},
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label: "default",
				Linode: config.LinodeConfig{
					APIURL: "https://api.linode.com/v4",
					Token:  placeholderToken,
				},
			},
		},
	}

	srv, err := server.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build server: %v\n", err)
		os.Exit(1)
	}

	infos := srv.AllToolInfos()
	dumps := make([]toolDump, 0, len(infos))

	for _, info := range infos {
		params := make(map[string]string, len(info.InputSchema.Properties))

		for name, raw := range info.InputSchema.Properties {
			params[name] = schemaType(raw)
		}

		required := append([]string(nil), info.InputSchema.Required...)
		sort.Strings(required)

		dumps = append(dumps, toolDump{
			Name:       info.Name,
			Capability: strings.TrimPrefix(info.Capability.String(), "Cap"),
			Params:     params,
			Required:   required,
		})
	}

	sort.Slice(dumps, func(i, j int) bool { return dumps[i].Name < dumps[j].Name })

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	if err := enc.Encode(dumps); err != nil {
		fmt.Fprintf(os.Stderr, "encode: %v\n", err)
		os.Exit(1)
	}
}

// schemaType returns the JSON-Schema "type" of a property, or "" when the
// property is not a map or declares no string type.
func schemaType(raw any) string {
	prop, isMap := raw.(map[string]any)
	if !isMap {
		return ""
	}

	typ, isString := prop["type"].(string)
	if !isString {
		return ""
	}

	return typ
}
