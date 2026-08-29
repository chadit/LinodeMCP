// Command toolgen emits the MCP tool factories and handlers for every tool the
// linode.mcp.v1 contract declares, except the ones
// docs/contracts/handwritten-tools.txt still claims. A tool's route, tier,
// response message, argument locations, filters, and prose all live on
// its *Input message, so the factory is read out of the compiled descriptors
// rather than hand-written per language where no gate could check it against
// the contract.
//
// Generated is the default, which is the point of taking the hand-written list
// rather than a list of what to generate: a new tool is a proto message and
// nothing else, and it appears here without being named anywhere.
//
// The output tree is gitignored and rewritten whole on every run, the same way
// internal/genpb is: a stale file left behind by a tool that went away would
// keep compiling long after its contract did.
//
// Run it through `make proto`, which sequences it after buf and the post-passes
// so the descriptors and schemas it reads are the current ones.
//
// One run emits every language docs/contracts/languages.txt registers. The
// descriptors are read once into the contract model and each language's
// renderer arm turns that one model into that language's tree, so a tool cannot
// reach one language and miss another through an emitter that was never run.
//
// Usage: toolgen -languages <file> -handwritten <file> -schemas <dir>
//
//	-out <dir> -python-out <dir>
//	-answers-out <dir> -python-answers-out <dir> [-ruff <path>]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// The emitted tree is build input, neither secret nor executable.
const (
	outputPerm = 0o600
	dirPerm    = 0o750
)

func main() {
	paths := runPaths{
		languages:   flagValue("-languages", "docs/contracts/languages.txt"),
		handwritten: flagValue("-handwritten", "docs/contracts/handwritten-tools.txt"),
		schemas:     flagValue("-schemas", "go/internal/toolschemas/data"),
		goOut:       flagValue("-out", "go/internal/gentools"),
		pyOut:       flagValue("-python-out", "python/src/linodemcp/gentools"),
		goAnswers:   flagValue("-answers-out", "go/internal/genlocal"),
		pyAnswers:   flagValue("-python-answers-out", "python/src/linodemcp/genlocal"),
		ruff:        flagValue("-ruff", "python/.venv/bin/ruff"),
	}

	if err := run(&paths); err != nil {
		fmt.Fprintf(os.Stderr, "toolgen: %v\n", err)
		os.Exit(1)
	}
}

// flagValue reads a named flag, answering fallback when it is absent. The flag
// package would do this, but its error path calls os.Exit from inside the
// library, which this repo's linters read as a second exit point.
func flagValue(name, fallback string) string {
	args := os.Args[1:]

	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1]
		}

		if value, found := strings.CutPrefix(arg, name+"="); found {
			return value
		}
	}

	return fallback
}

// runPaths is what one run reads and where each language's tree lands. ruff is
// the formatter the Python arm renders through, named here because the build
// runs from a directory the emitter cannot assume.
type runPaths struct {
	languages   string
	handwritten string
	schemas     string
	goOut       string
	pyOut       string
	goAnswers   string
	pyAnswers   string
	ruff        string
}

func run(paths *runPaths) error {
	arms, err := resolveArms(paths)
	if err != nil {
		return err
	}

	if coverErr := checkRendererCoverage(arms); coverErr != nil {
		return coverErr
	}

	handwritten, err := readNames(paths.handwritten)
	if err != nil {
		return err
	}

	declared, err := declaredTools()
	if err != nil {
		return err
	}

	cohort, err := deriveCohort(declared, handwritten)
	if err != nil {
		return err
	}

	operations, err := localArms()
	if err != nil {
		return err
	}

	// Held to the whole declared surface rather than the emitted cohort: an
	// operation named after a hand-written tool is the same mistake as one
	// named after a generated tool, and the cohort shrinks as tools move.
	if armErr := checkLocalArms(operations, languageNames(arms), sortedNames(declared)); armErr != nil {
		return armErr
	}

	generated, err := generatedOperations(operations)
	if err != nil {
		return err
	}

	contracts, err := readContracts(cohort, declared, newSchemaDocs(paths.schemas))
	if err != nil {
		return err
	}

	shapes, err := localAnswerShapes(operations)
	if err != nil {
		return err
	}

	return emit(arms, contracts, shapes, generated)
}

// resolveArms answers an arm per registered language, ahead of any rendering:
// a registry naming a language this emitter cannot serve stops the run before
// one tree is rewritten and the other is left behind.
func resolveArms(paths *runPaths) ([]languageArm, error) {
	languages, err := registeredLanguages(paths.languages)
	if err != nil {
		return nil, err
	}

	arms := make([]languageArm, 0, len(languages))

	for _, name := range languages {
		arm, armErr := armFor(name, paths)
		if armErr != nil {
			return nil, armErr
		}

		arms = append(arms, arm)
	}

	return arms, nil
}

// emit writes one tool tree and one answers tree per arm, both from the single
// contract model.
func emit(
	arms []languageArm, contracts []contract, shapes []answerShape,
	operations []localOperation,
) error {
	for _, arm := range arms {
		files, err := renderTree(arm.lang, contracts, operations)
		if err != nil {
			return err
		}

		if err := writeTree(arm.out, arm.lang, files); err != nil {
			return err
		}

		if err := emitAnswers(arm, shapes, operations); err != nil {
			return err
		}
	}

	return nil
}

// emitAnswers writes one language's answer shapes into their own tree.
func emitAnswers(arm languageArm, shapes []answerShape, operations []localOperation) error {
	file, err := arm.lang.renderAnswers(shapes, operations)
	if err != nil {
		return err
	}

	return writeTree(arm.answers, arm.lang, map[string]string{file.Name: file.Text})
}

// deriveCohort answers which tools this run emits: every one the descriptors
// declare that the hand-written list does not claim, tool-name sorted.
//
// A listed name the contract does not declare stops the run. Such an entry
// exempts nothing from the generator while reading as though it exempts
// something, and it is the shape a typo or a renamed tool takes.
//
// An empty result stops the run too. The emitter and the ratchet both scope
// their checks to what is generated, so a list that swallowed the whole surface
// would leave every one of them passing over nothing.
// sortedNames is every tool one descriptor walk found, in a settled order so a
// refusal names the same one on every run.
func sortedNames(declared map[string]protoreflect.MessageDescriptor) []string {
	names := make([]string, 0, len(declared))
	for name := range declared {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func deriveCohort(
	declared map[string]protoreflect.MessageDescriptor, handwritten []string,
) ([]string, error) {
	claimed := make(map[string]struct{}, len(handwritten))

	for _, name := range handwritten {
		if _, exists := declared[name]; !exists {
			return nil, fmt.Errorf("%w: %s", errHandwrittenNotDeclared, name)
		}

		claimed[name] = struct{}{}
	}

	cohort := make([]string, 0, len(declared)-len(claimed))

	for name := range declared {
		if _, held := claimed[name]; !held {
			cohort = append(cohort, name)
		}
	}

	if len(cohort) == 0 {
		return nil, errEmptyCohort
	}

	sort.Strings(cohort)

	return cohort, nil
}

// readNames returns one name per line, comments and blanks dropped, in file
// order.
func readNames(path string) ([]string, error) {
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

		names = append(names, trimmed)
	}

	return names, nil
}

// groupName is the emitted file's base name for a proto source path:
// linode/mcp/v1/domain.proto becomes domain.
func groupName(protoPath string) string {
	return strings.TrimSuffix(filepath.Base(protoPath), ".proto")
}

// writeTree replaces the emitted tree with files. Removing what was there first
// keeps a tool that left the cohort from leaving a factory behind, which would
// register a tool the contract no longer describes.
func writeTree(dir string, lang renderer, files map[string]string) error {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	if err := removeGenerated(dir, lang); err != nil {
		return err
	}

	for name, text := range files {
		path := filepath.Join(dir, name)

		if err := os.WriteFile(path, []byte(text), outputPerm); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	return nil
}

func removeGenerated(dir string, lang renderer) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !lang.owns(entry.Name()) {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}

	return nil
}
