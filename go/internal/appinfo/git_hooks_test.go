package appinfo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const parentDir = ".."

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()

	pathParts := append([]string{parentDir, parentDir, parentDir}, parts...)

	contents, err := os.ReadFile(filepath.Join(pathParts...))
	if err != nil {
		t.Fatalf("read repo file: %v", err)
	}

	return string(contents)
}

func TestGitHookBootstrapPolicy(t *testing.T) {
	t.Parallel()

	rootMakefile := readRepoFile(t, "Makefile")
	if !strings.Contains(rootMakefile, "install-hooks:") || !strings.Contains(rootMakefile, "./scripts/git-hooks.sh install") {
		t.Fatalf("root Makefile must expose install-hooks through scripts/git-hooks.sh")
	}

	if !strings.Contains(rootMakefile, "check-hooks:") || !strings.Contains(rootMakefile, "./scripts/git-hooks.sh check") {
		t.Fatalf("root Makefile must expose check-hooks through scripts/git-hooks.sh")
	}

	for _, makefile := range []string{"go/Makefile", "python/Makefile"} {
		contents := readRepoFile(t, strings.Split(makefile, "/")...)
		if !strings.Contains(contents, "$(MAKE) -C .. install-hooks") {
			t.Fatalf("%s _ensure-hooks must use the shared root hook installer", makefile)
		}
	}

	hookScript := readRepoFile(t, "scripts", "git-hooks.sh")
	for _, required := range []string{
		"pre-commit install && pre-commit install --hook-type pre-push",
		"require_generated_hook \"$root\" pre-commit",
		"require_generated_hook \"$root\" pre-push",
		"main \"$@\"",
	} {
		if !strings.Contains(hookScript, required) {
			t.Fatalf("scripts/git-hooks.sh missing required policy fragment %q", required)
		}
	}
}
