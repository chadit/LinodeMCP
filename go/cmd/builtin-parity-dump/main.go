// Command builtin-parity-dump reads a JSON tool catalog from stdin and prints
// the resolved built-in profile catalog plus the category each tool falls in,
// as canonical JSON. scripts/verify_profile_resolution.py runs this against
// the Python twin over one catalog and diffs both halves: the profiles answer
// what each built-in serves today, the categories answer what any future
// profile would serve.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// dump is the shape both languages print. profiles carries each
// implementation's own canonical catalog export verbatim, so the export the
// gate reads is the one the language already ships.
type dump struct {
	Categories map[string][]string `json:"categories"`
	Profiles   json.RawMessage     `json:"profiles"`
}

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
	if parseErr := json.Unmarshal(data, &raw); parseErr != nil {
		return fmt.Errorf("parse fixture: %w", parseErr)
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

	resolved, err := profiles.BuiltinCatalogJSON(catalog)
	if err != nil {
		return fmt.Errorf("build catalog: %w", err)
	}

	categories := make(map[string][]string, len(catalog))
	for _, descriptor := range catalog {
		categories[descriptor.Name] = profiles.Categories(descriptor.Name)
	}

	out, err := json.MarshalIndent(dump{Profiles: resolved, Categories: categories}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dump: %w", err)
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
