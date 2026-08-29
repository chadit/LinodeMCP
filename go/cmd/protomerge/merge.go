package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The merged tree is a review artifact, so nothing beyond its writer reaches it.
const (
	mergedDirMode  = 0o750
	mergedFileMode = 0o600
)

const mergeFix = "The upstream surface and the overlay disagree. Add the missing overlay\n" +
	"semantics, or retire the dropped declaration with -retire so its number and\n" +
	"name land as reserved; never edit the descriptor to match the tree."

// mergeOptions is one merge run.
type mergeOptions struct {
	SurfacePath string
	OutDir      string
	Retire      []string
}

// mergeTree renders every declared file from the overlay and an upstream
// surface. Nothing is written when a refusal fires, so no half-merged tree.
func mergeTree(fsys fs.FS, opts mergeOptions) (int, error) {
	files, err := scanProto(fsys)
	if err != nil {
		return 0, err
	}

	if measureErr := measured("proto merge", len(files)); measureErr != nil {
		return 0, measureErr
	}

	_, overlays, _, err := extractAll(fsys, files)
	if err != nil {
		return 0, err
	}

	upstream, err := readSurface(opts.SurfacePath)
	if err != nil {
		return 0, err
	}

	if retireErr := retireAll(overlays, opts.Retire); retireErr != nil {
		return 0, retireErr
	}

	rendered, findings := renderAll(overlays, upstream)
	findings = append(findings, uncovered(overlays, upstream)...)

	say(fmt.Sprintf(
		"merged %d proto file(s) against %d upstream field(s), %d upstream enum value(s), and %d upstream route(s), retiring %d declaration(s)",
		len(files), len(upstream.Fields), len(upstream.Values), len(upstream.Routes), len(opts.Retire),
	))

	if code := report("declarations the upstream surface and the overlay disagree on", findings, mergeFix); code != 0 {
		return code, nil
	}

	carried, err := scanCarried(fsys, files)
	if err != nil {
		return 0, err
	}

	if writeErr := writeTree(opts.OutDir, overlays, rendered); writeErr != nil {
		return 0, writeErr
	}

	if copyErr := copyCarried(fsys, opts.OutDir, carried); copyErr != nil {
		return 0, copyErr
	}

	say(fmt.Sprintf("wrote the merged tree to %s, carrying %d unrendered file(s) through", opts.OutDir, len(carried)))

	return 0, nil
}

// renderAll collects refusals rather than stopping at the first, so one run
// tells the reader everything the drift touched.
func renderAll(overlays []*overlayFile, upstream *surface) ([]string, []string) {
	rendered := make([]string, len(overlays))

	var findings []string

	for idx, overlay := range overlays {
		text, err := overlay.render(upstream)
		if err != nil {
			findings = append(findings, err.Error())

			continue
		}

		rendered[idx] = text
	}

	return rendered, findings
}

// uncovered names upstream surface no overlay entry claims, which the merge
// refuses rather than invent a wire number and MCP semantics for.
func uncovered(overlays []*overlayFile, upstream *surface) []string {
	claimed := anchored(overlays)

	findings := missing(sortedKeys(upstream.Fields), claimed.Fields, errUncoveredField)
	findings = append(findings, missing(sortedSet(upstream.Values), claimed.Values, errUncoveredValue)...)

	return append(findings, missing(sortedRoutes(upstream.Routes), claimed.Routes, errUncoveredRoute)...)
}

func missing(want []string, have map[string]struct{}, reason error) []string {
	var findings []string

	for _, key := range want {
		if _, ok := have[key]; ok {
			continue
		}

		findings = append(findings, fmt.Sprintf("%v: %s", reason, key))
	}

	return findings
}

// claimSet is one key set per half of the surface.
type claimSet struct {
	Fields map[string]struct{}
	Values map[string]struct{}
	Routes map[string]struct{}
}

// anchored answers the surface keys the overlays still claim after retirement.
func anchored(overlays []*overlayFile) claimSet {
	claimed := claimSet{
		Fields: map[string]struct{}{},
		Values: map[string]struct{}{},
		Routes: map[string]struct{}{},
	}

	for _, overlay := range overlays {
		collectAnchors(overlay, claimed)
	}

	return claimed
}

func collectAnchors(overlay *overlayFile, claimed claimSet) {
	for _, elem := range overlay.Elements {
		if route, ok := elem.(*routeAnchor); ok {
			claimed.Routes[route.Message] = struct{}{}

			continue
		}

		anchor, ok := elem.(*declAnchor)
		if !ok {
			continue
		}

		if anchor.InEnum {
			claimed.Values[declKey(anchor.Path, anchor.Name)] = struct{}{}

			continue
		}

		claimed.Fields[declKey(anchor.Path, anchor.Name)] = struct{}{}
	}
}

// retireAll refuses a name the tree does not carry: a retirement that matched
// nothing would report a spent number the file never states.
func retireAll(overlays []*overlayFile, keys []string) error {
	for _, key := range keys {
		if !retireKey(overlays, key) {
			return fmt.Errorf("%w: %s", errNoRetiredAnchor, key)
		}
	}

	return nil
}

func retireKey(overlays []*overlayFile, key string) bool {
	for _, overlay := range overlays {
		if retireIn(overlay, key) {
			return true
		}
	}

	return false
}

func retireIn(overlay *overlayFile, key string) bool {
	for idx, elem := range overlay.Elements {
		anchor, ok := elem.(*declAnchor)
		if !ok || declKey(anchor.Path, anchor.Name) != key {
			continue
		}

		overlay.Elements[idx] = retiredField{
			Indent: anchor.Indent,
			Name:   anchor.Name,
			Number: anchor.Number,
		}
		dropLeadComment(overlay, idx)

		return true
	}

	return false
}

// dropLeadComment keeps prose about a dropped field from surviving into the
// merged tree as documentation for the reserved line.
func dropLeadComment(overlay *overlayFile, idx int) {
	if idx == 0 {
		return
	}

	span, ok := overlay.Elements[idx-1].(rawSpan)
	if !ok {
		return
	}

	kept := len(span.Lines)
	for kept > 0 && strings.HasPrefix(strings.TrimSpace(span.Lines[kept-1]), "//") {
		kept--
	}

	overlay.Elements[idx-1] = rawSpan{Lines: span.Lines[:kept]}
}

// writeTree keeps each file's repo-relative path, so the result diffs straight
// against proto/.
func writeTree(outDir string, overlays []*overlayFile, rendered []string) error {
	for idx, overlay := range overlays {
		if err := writeUnder(outDir, overlay.Path, rendered[idx]); err != nil {
			return err
		}
	}

	return nil
}

func writeUnder(outDir, name, content string) error {
	path := filepath.Join(outDir, filepath.FromSlash(name))

	if err := os.MkdirAll(filepath.Dir(path), mergedDirMode); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, []byte(content), mergedFileMode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// copyCarried carries the disowned vendor trees through, so the output tree is
// whole rather than a subset.
func copyCarried(fsys fs.FS, outDir string, carried []string) error {
	for _, name := range carried {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		if writeErr := writeUnder(outDir, name, string(raw)); writeErr != nil {
			return writeErr
		}
	}

	return nil
}

func sortedKeys(fields map[string]surfaceField) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

func sortedRoutes(routes map[string]surfaceRoute) []string {
	keys := make([]string, 0, len(routes))
	for key := range routes {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

func sortedSet(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}
