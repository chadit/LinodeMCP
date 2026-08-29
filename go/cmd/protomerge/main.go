// Command protomerge splits each declared proto file into the surface upstream
// techdocs can state and the overlay this repo owns, renders the pair back, and
// fails by name on any file whose bytes moved. Merge mode swaps in an upstream
// descriptor instead, refusing a declaration it dropped or added; -retire turns
// a drop into the reserved statements protoc will not let anything reuse. It
// reads the local tree and that descriptor only, so it runs offline. Documented
// in docs/gates.md under overlay-roundtrip and overlay-merge.
//
// Usage: protomerge [-repo <dir>]
//
//	protomerge [-repo <dir>] -emit-surface <file>
//	protomerge [-repo <dir>] -surface <file> -out <dir> [-retire <key>[,<key>...]]
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

const fixLine = "Extraction lost or reshaped part of the contract. Fix the extractor\n" +
	"so the overlay carries what it dropped; never edit proto/ to match it.\n" +
	"The proto tree is the control this gate measures against."

func main() {
	repo := flag.String("repo", ".", "repository root holding buf.yaml and the proto tree")
	emit := flag.String("emit-surface", "", "write the tree's upstream surface descriptor to this file")
	descriptor := flag.String("surface", "", "upstream surface descriptor to merge the overlay against")
	out := flag.String("out", "", "directory the merged proto tree is written to, required with -surface")
	retire := flag.String("retire", "", "comma-separated Message.field keys this merge retires to reserved")

	flag.Parse()

	code, err := dispatch(os.DirFS(*repo), *emit, mergeOptions{
		SurfacePath: *descriptor,
		OutDir:      *out,
		Retire:      splitKeys(*retire),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "protomerge: %v\n", err)
		os.Exit(1)
	}

	os.Exit(code)
}

// dispatch picks the run the flags asked for; the three modes are exclusive.
func dispatch(fsys fs.FS, emit string, opts mergeOptions) (int, error) {
	if emit != "" && opts.SurfacePath != "" {
		return 0, errModeConflict
	}

	if emit != "" {
		return emitSurface(fsys, emit)
	}

	if opts.SurfacePath == "" {
		return run(fsys)
	}

	if opts.OutDir == "" {
		return 0, errNoOutDir
	}

	return mergeTree(fsys, opts)
}

// splitKeys drops the empty element an unset flag would otherwise contribute.
func splitKeys(list string) []string {
	if list == "" {
		return nil
	}

	return strings.Split(list, ",")
}

// run renders every declared file back from the two halves and reports the
// files whose bytes moved.
func run(fsys fs.FS) (int, error) {
	files, err := scanProto(fsys)
	if err != nil {
		return 0, err
	}

	if measureErr := measured("proto overlay extraction", len(files)); measureErr != nil {
		return 0, measureErr
	}

	originals, overlays, surf, err := extractAll(fsys, files)
	if err != nil {
		return 0, err
	}

	findings, facts, spans := compareAll(originals, overlays, surf)

	say(fmt.Sprintf(
		"round-tripped %d proto file(s): %d re-rendered fact element(s), %d verbatim overlay span(s), %d surface field(s), %d surface enum value(s), %d surface route(s)",
		len(files), facts, spans, len(surf.Fields), len(surf.Values), len(surf.Routes),
	))

	return report("proto files the overlay does not reproduce", findings, fixLine), nil
}

func extractAll(fsys fs.FS, files []string) ([]string, []*overlayFile, *surface, error) {
	originals := make([]string, 0, len(files))
	overlays := make([]*overlayFile, 0, len(files))
	surf := newSurface()

	for _, name := range files {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("read %s: %w", name, err)
		}

		content := string(raw)

		overlay, extractErr := extract(name, content, surf)
		if extractErr != nil {
			return nil, nil, nil, extractErr
		}

		originals = append(originals, content)
		overlays = append(overlays, overlay)
	}

	return originals, overlays, surf, nil
}

func compareAll(originals []string, overlays []*overlayFile, surf *surface) ([]string, int, int) {
	var findings []string

	facts, spans := 0, 0

	for idx, overlay := range overlays {
		fileFacts, fileSpans := overlay.counts()
		facts += fileFacts
		spans += fileSpans

		rendered, err := overlay.render(surf)
		if err != nil {
			findings = append(findings, err.Error())

			continue
		}

		if rendered == originals[idx] {
			continue
		}

		findings = append(findings, firstDifference(overlay.Path, originals[idx], rendered))
	}

	return findings, facts, spans
}

// firstDifference quotes both spellings, so a finding says what changed rather
// than that something did.
func firstDifference(name, original, rendered string) string {
	want := strings.Split(original, "\n")
	got := strings.Split(rendered, "\n")

	for idx := range max(len(want), len(got)) {
		wantLine, gotLine := lineAt(want, idx), lineAt(got, idx)
		if wantLine == gotLine {
			continue
		}

		return fmt.Sprintf("%s:%d: tree has %s, overlay renders %s", name, idx+1, quote(wantLine), quote(gotLine))
	}

	return name + ": bytes differ with no differing line, which means the final newline moved"
}

func lineAt(lines []string, idx int) string {
	if idx >= len(lines) {
		return "<end of file>"
	}

	return lines[idx]
}
