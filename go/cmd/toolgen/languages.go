package main

import (
	"fmt"
	"os"
	"strings"
)

// The language names the renderer arms answer to, spelled here because both the
// arm table and the registry file use them.
const (
	languageGo     = "go"
	languagePython = "python"
)

// languageArm is one registered language's half of a run: the renderer that
// turns the contract model into that language's files, and where they land.
type languageArm struct {
	lang renderer
	out  string
}

// registeredLanguages is the language column of docs/contracts/languages.txt in
// file order: one name per line, tab-separated from the working directory and
// dump command the parity gates run. Read the same way the gates read it, so
// registering a language is the single act that turns both on for it.
//
// A registry naming nothing stops the run, since emitting no tree while
// reporting success is how a build keeps shipping yesterday's one.
func registeredLanguages(path string) ([]string, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- reads a repo contract at the path the build passes
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	names := make([]string, 0)

	for line := range strings.SplitSeq(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		name, _, _ := strings.Cut(trimmed, "\t")

		names = append(names, strings.TrimSpace(name))
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("%w: %s", errNoLanguages, path)
	}

	return names, nil
}

// armFor answers the arm that emits one registered language's tree.
//
// A registered language with no arm here fails by name rather than being
// skipped: a run that emitted the languages it recognized and said nothing
// about the rest is how a surface goes one language short of its registry.
func armFor(name string, paths *runPaths) (languageArm, error) {
	switch name {
	case languageGo:
		return boundArm(name, goRenderer{}, paths.goOut)
	case languagePython:
		return boundArm(name, pyRenderer{outDir: paths.pyOut, ruff: paths.ruff}, paths.pyOut)
	}

	return languageArm{}, fmt.Errorf("%w: %s", errNoRendererArm, name)
}

// boundArm ties a renderer to its output directory, refusing a run that
// registered the language and left its tree nowhere to land.
//
// The renderer is held to the name it is bound under as well, since the table
// above is three literals per line and a copy-paste that pairs one language
// with another's renderer would write that language's tree into this one's
// directory while every refusal about it named the wrong arm.
func boundArm(name string, lang renderer, out string) (languageArm, error) {
	if lang.language() != name {
		return languageArm{}, fmt.Errorf("%w: %s bound as %s", errArmMisnamed, lang.language(), name)
	}

	if out == "" {
		return languageArm{}, fmt.Errorf("%w: %s", errNoOutputDir, name)
	}

	return languageArm{lang: lang, out: out}, nil
}
