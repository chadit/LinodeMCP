package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"sort"
)

// surfaceDoc is the on-disk shape a techdocs-derived contract reaches the
// merger through. JSON because the comparator emits JSON and a text descriptor
// is reviewable in the diff that carries it.
type surfaceDoc struct {
	Fields map[string]surfaceField `json:"fields"`
	// Routes is keyed by the declaring message, so a dropped route is an
	// absent key in the next descriptor.
	Routes map[string]surfaceRoute `json:"routes"`
	// Values is sorted so two runs over one tree write the same bytes.
	Values []string `json:"values"`
}

// encodeSurface renders the descriptor bytes. encoding/json sorts map keys and
// the value list is sorted here, so two runs diff cleanly.
func encodeSurface(surf *surface) ([]byte, error) {
	values := make([]string, 0, len(surf.Values))
	for name := range surf.Values {
		values = append(values, name)
	}

	sort.Strings(values)

	doc := surfaceDoc{Fields: surf.Fields, Values: values, Routes: surf.Routes}

	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode surface: %w", err)
	}

	return append(raw, '\n'), nil
}

// decodeSurface refuses an empty descriptor: every anchor would dangle, which
// reads as drift rather than as an upstream that never loaded.
func decodeSurface(raw []byte) (*surface, error) {
	var doc surfaceDoc

	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%w: %w", errUnreadableSurface, err)
	}

	if len(doc.Fields) == 0 && len(doc.Values) == 0 && len(doc.Routes) == 0 {
		return nil, errEmptySurface
	}

	surf := newSurface()
	maps.Copy(surf.Fields, doc.Fields)
	maps.Copy(surf.Routes, doc.Routes)

	for _, name := range doc.Values {
		surf.Values[name] = struct{}{}
	}

	return surf, nil
}

// readSurface reads through a directory FS, so the tool keeps one way of
// reading a file it was pointed at.
func readSurface(path string) (*surface, error) {
	dir, name := filepath.Split(path)
	if dir == "" {
		dir = "."
	}

	raw, err := fs.ReadFile(os.DirFS(dir), name)
	if err != nil {
		return nil, fmt.Errorf("read surface descriptor %s: %w", path, err)
	}

	return decodeSurface(raw)
}

// emitSurface writes the tree's own surface, which stands in for the techdocs
// comparator's output until it emits one. At zero drift they are one document.
func emitSurface(fsys fs.FS, path string) (int, error) {
	files, err := scanProto(fsys)
	if err != nil {
		return 0, err
	}

	if measureErr := measured("proto surface extraction", len(files)); measureErr != nil {
		return 0, measureErr
	}

	_, _, surf, err := extractAll(fsys, files)
	if err != nil {
		return 0, err
	}

	raw, err := encodeSurface(surf)
	if err != nil {
		return 0, err
	}

	if writeErr := os.WriteFile(path, raw, mergedFileMode); writeErr != nil {
		return 0, fmt.Errorf("write surface descriptor %s: %w", path, writeErr)
	}

	say(fmt.Sprintf(
		"wrote a surface descriptor of %d field(s), %d enum value(s), and %d route(s) from %d proto file(s)",
		len(surf.Fields), len(surf.Values), len(surf.Routes), len(files),
	))

	return 0, nil
}
