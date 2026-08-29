package main

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Scope comes from here rather than a path of this gate's own, so a module
// added later is walked; scripts/verify_proto_lint.py reads the same two keys.
const workspaceFile = "buf.yaml"

const protoExt = ".proto"

// bufWorkspace is the part of buf.yaml this gate reads.
type bufWorkspace struct {
	Modules []struct {
		Path string `yaml:"path"`
	} `yaml:"modules"`
	Lint struct {
		Ignore []string `yaml:"ignore"`
	} `yaml:"lint"`
}

// scanProto answers the declared proto files in a stable order. Disowned
// vendor trees are left out: a finding on one names nothing anybody here fixes.
func scanProto(fsys fs.FS) ([]string, error) {
	raw, err := fs.ReadFile(fsys, workspaceFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", workspaceFile, err)
	}

	var workspace bufWorkspace
	if unmarshalErr := yaml.Unmarshal(raw, &workspace); unmarshalErr != nil {
		return nil, fmt.Errorf("parse %s: %w", workspaceFile, unmarshalErr)
	}

	if len(workspace.Modules) == 0 {
		return nil, errNoModules
	}

	var found []string

	for _, module := range workspace.Modules {
		files, walkErr := walkModule(fsys, module.Path, workspace.Lint.Ignore)
		if walkErr != nil {
			return nil, walkErr
		}

		found = append(found, files...)
	}

	sort.Strings(found)

	return found, nil
}

// scanCarried answers the files the merge does not render. They travel
// byte-for-byte, so the merged output stands in whole for the tree it replaces.
func scanCarried(fsys fs.FS, rendered []string) ([]string, error) {
	skip := make(map[string]struct{}, len(rendered))
	for _, name := range rendered {
		skip[name] = struct{}{}
	}

	raw, err := fs.ReadFile(fsys, workspaceFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", workspaceFile, err)
	}

	var workspace bufWorkspace
	if unmarshalErr := yaml.Unmarshal(raw, &workspace); unmarshalErr != nil {
		return nil, fmt.Errorf("parse %s: %w", workspaceFile, unmarshalErr)
	}

	var found []string

	for _, module := range workspace.Modules {
		files, walkErr := walkCarried(fsys, module.Path, skip)
		if walkErr != nil {
			return nil, walkErr
		}

		found = append(found, files...)
	}

	sort.Strings(found)

	return found, nil
}

func walkCarried(fsys fs.FS, root string, skip map[string]struct{}) ([]string, error) {
	var found []string

	err := fs.WalkDir(fsys, root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if _, rendered := skip[name]; entry.IsDir() || rendered {
			return nil
		}

		found = append(found, name)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}

	return found, nil
}

func walkModule(fsys fs.FS, root string, ignored []string) ([]string, error) {
	var found []string

	err := fs.WalkDir(fsys, root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			return skipIgnored(name, ignored)
		}

		if path.Ext(name) == protoExt {
			found = append(found, name)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}

	return found, nil
}

// skipIgnored prunes the branch rather than filtering per file, so nothing
// under a disowned directory is read at all.
func skipIgnored(name string, ignored []string) error {
	for _, ignore := range ignored {
		if name == strings.TrimSuffix(ignore, "/") {
			return fs.SkipDir
		}
	}

	return nil
}
