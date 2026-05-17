// Command builtin-parity-dump reads a JSON tool catalog from stdin and prints
// the resolved built-in profile catalog as canonical JSON. Used by the
// cross-language parity verification script to compare Go and Python
// catalogs built from the same input fixture.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

// inputTool matches the fixture JSON shape: {name, capability}.
type inputTool struct {
	Name       string `json:"name"`
	Capability string `json:"capability"`
}

func parseCapability(name string) (profiles.Capability, error) {
	switch name {
	case "Unknown":
		return profiles.CapUnknown, nil
	case "Read":
		return profiles.CapRead, nil
	case "Write":
		return profiles.CapWrite, nil
	case "Destroy":
		return profiles.CapDestroy, nil
	case "Admin":
		return profiles.CapAdmin, nil
	case "Meta":
		return profiles.CapMeta, nil
	default:
		return profiles.CapUnknown, fmt.Errorf("%w: %q", ErrUnknownCapability, name)
	}
}

func run() error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	var raw []inputTool
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse fixture: %w", err)
	}

	catalog := make([]profiles.ToolDescriptor, 0, len(raw))

	for _, entry := range raw {
		capability, capErr := parseCapability(entry.Capability)
		if capErr != nil {
			return fmt.Errorf("tool %q: %w", entry.Name, capErr)
		}

		catalog = append(catalog, profiles.ToolDescriptor{
			Name:       entry.Name,
			Capability: capability,
		})
	}

	out, err := profiles.BuiltinCatalogJSON(catalog)
	if err != nil {
		return fmt.Errorf("build catalog: %w", err)
	}

	if _, err := os.Stdout.Write(out); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
