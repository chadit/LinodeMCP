package main_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
)

// The accounting: every refusal errors.go declares is either named by a case
// that raises it, or listed below with the reason no synthesized declaration
// reaches it. An error added with neither fails this test by name, which is
// what keeps the negative paths from quietly falling behind the checks.

// unreachedRefusals is every refusal no probe raises, each with the reason.
//
// Four groups. The first is already covered through the binary by main_test.go.
// The second answers a state the arm table or the linked-in descriptors cannot
// be put in. The third needs a response message shaped a particular wrong way,
// and a probe can only name messages the contract already declares, since a
// synthesized response has no Go type and the emitter refuses that first. The
// fourth is a tier or flow this probe set does not build yet, named so the next
// hand knows what is left rather than reading the gap as coverage.
func unreachedRefusals() map[string]string {
	return map[string]string{
		"errNoLanguages":            "covered by TestRefusesALanguagesRegistryThatNamesNothing, which drives the binary",
		"errNoRendererArm":          "covered by TestRefusesARegisteredLanguageWithNoRendererArm, which drives the binary",
		"errEmptyCohort":            "covered by the hand-written list cases in main_test.go",
		"errHandwrittenNotDeclared": "covered by the hand-written list cases in main_test.go",

		"errNoOutputDir":       "answers an arm bound to no directory, which only the arm table itself can produce",
		"errArmMisnamed":       "answers an arm table entry pairing a renderer with another language's name, which the same table alone can produce",
		"errNoContractOptions": "answers a registry holding no contract option, which a linked-in genpb cannot be",
		"errToolNotDeclared":   "answers two messages claiming one tool name, which the descriptor walk finds and one probe message cannot be",
		"errNoGoType":          "answers a message with no Go type, which the response check refuses one step earlier",
		"errUnsupportedTier":   "answers a tier no capability selects: a probe clearing tool_capability still lands on a served one",
		"errPyRender":          "answers a contract the Python arm cannot render, which the Go arm refuses first: both arms run from one contract and Go renders ahead of Python",

		"errAmbiguousEnvelope":   "needs a collection whose declared response is not a page",
		"errNoMarkerCursor":      "needs a marker-paged collection whose response declares no cursor members",
		"errNotAWriteEnvelope":   "needs a mutation response that is not shaped like an envelope",
		"errNotAWritePage":       "needs a mutation answering with a page carrying members the envelope cannot fill",
		"errNotAWrapper":         "needs a response named a get envelope and shaped otherwise",
		"errNotABodyRead":        "needs a body read whose response is not the resource the call decodes into",
		"errNotAnAssembledRead":  "needs a read whose declared transport cannot fill the declared response",
		"errUnplacedLocalMember": "needs a response declaring a LOCAL member on a tier that decodes its whole answer",
		"errUnusedResponseBody":  "needs a response that already decodes whole beside response_body_fields",
		"errNoNullPayload":       "needs a tier with no decoded body beside explicit_null_fields",
		"errEchoArgumentShape":   "reads echo_argument off a response member, and a response must be a generated message: a synthesized stand-in has no Go struct for the type lookup to read",
		"errEchoArgumentNoop":    "same as errEchoArgumentShape: the declaration lives on a response member, which cannot be synthesized",

		"errUnsupportedDestroyShape": "needs a removal whose path shape no emitted driver addresses",
		"errReaderOnDestroy":         "needs an argument_reader on a removal",
		"errMetaDryRun":              "needs a meta tool advertising dry_run",
		"errUnmatchedFilter":         "needs a list_filter matching no element field",
		"errUnsupportedItemKind":     "needs a typed list item member with no request representation",
		"errRecursiveItemMessage":    "needs a body message whose members reach it again",
		"errReaderValuesOnEnum":      "needs reader_values on an enum-typed argument",
		"errUnsupportedReader":       "needs an argument_reader the Go arm has no call for",
	}
}

// accountingFile is this source, whose own literals are the excuse list.
const accountingFile = "accounting_test.go"

// TestEveryRefusalIsAccountedFor holds the probe set to errors.go.
func TestEveryRefusalIsAccountedFor(t *testing.T) {
	t.Parallel()

	declared := refusalTexts(t)
	raised := namedRefusals(t)
	excused := unreachedRefusals()

	missing := make([]string, 0)

	for name := range declared {
		_, covered := raised[name]
		_, excused := excused[name]

		if !covered && !excused {
			missing = append(missing, name)
		}
	}

	sort.Strings(missing)

	if len(missing) > 0 {
		t.Errorf("%d refusal(s) with no case and no reason:\n%s",
			len(missing), strings.Join(missing, "\n"))
	}
}

// TestNoRefusalIsExcusedTwice keeps the list above from outliving its reason: a
// refusal a case reaches, or one errors.go no longer declares, has no business
// being excused.
func TestNoRefusalIsExcusedTwice(t *testing.T) {
	t.Parallel()

	declared := refusalTexts(t)
	raised := namedRefusals(t)

	for name := range unreachedRefusals() {
		if _, exists := declared[name]; !exists {
			t.Errorf("%s is excused and errors.go no longer declares it", name)
		}

		if _, covered := raised[name]; covered {
			t.Errorf("%s is excused and a case raises it", name)
		}
	}
}

// TestTheRefusalTableMatchesTheSource holds the name-to-error table the cases
// assert through to the errors the emitter declares. A refusal added to
// errors.go and left out of the table would leave its case unable to name it,
// and one left in the table after the error goes would name nothing.
func TestTheRefusalTableMatchesTheSource(t *testing.T) {
	t.Parallel()

	declared := refusalTexts(t)
	tabled := toolgen.ProbeRefusals()

	for name := range declared {
		if _, listed := tabled[name]; !listed {
			t.Errorf("errors.go declares %s and the refusal table does not list it", name)
		}
	}

	for name, refusal := range tabled {
		text, exists := declared[name]
		if !exists {
			t.Errorf("the refusal table lists %s and errors.go declares no such error", name)

			continue
		}

		if refusal.Error() != text {
			t.Errorf("the refusal table pairs %s with %q, and errors.go declares %q",
				name, refusal.Error(), text)
		}
	}
}

// namedRefusals is every refusal a case in this package names. Reading the
// sources rather than a list keeps the accounting from drifting from the cases
// it measures.
func namedRefusals(t *testing.T) map[string]struct{} {
	t.Helper()

	sources, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatalf("list test sources: %v", err)
	}

	named := make(map[string]struct{}, len(sources))

	for _, source := range sources {
		// This file names refusals to excuse them, which is the opposite of
		// raising one.
		if source == accountingFile {
			continue
		}

		for _, literal := range stringLiterals(t, source) {
			if strings.HasPrefix(literal, "err") {
				named[literal] = struct{}{}
			}
		}
	}

	return named
}

// stringLiterals is every string literal one source spells, which is where a
// case names the refusal it is about.
func stringLiterals(t *testing.T, path string) []string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	parsed, err := parser.ParseFile(token.NewFileSet(), path, raw, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	found := make([]string, 0)

	ast.Inspect(parsed, func(node ast.Node) bool {
		literal, isLiteral := node.(*ast.BasicLit)
		if !isLiteral || literal.Kind != token.STRING {
			return true
		}

		if text, unquoteErr := strconv.Unquote(literal.Value); unquoteErr == nil {
			found = append(found, text)
		}

		return true
	})

	return found
}
