package tools_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// goTestTreeRoot is the tree this guard reads, relative to this package's
// directory. Every hand-written Go test in the repo is under it.
const goTestTreeRoot = "../.."

// concatProseWords is the fewest space-separated words a literal needs before
// it reads as a sentence rather than as a token being assembled. "Bearer " and
// "GPU " are one word and build a header value or a label; "Failed to retrieve "
// is three and builds a sentence a caller sees.
const concatProseWords = 2

// reportingCall reports whether a method name is a test helper whose arguments
// are diagnostics rather than pins. A sweep never has to find the wording a
// failing test prints, so building one by concatenation is fine there.
func reportingCall(name string) bool {
	switch name {
	case "Error", "Errorf", "Fatal", "Fatalf", "Log", "Logf", "Skip", "Skipf",
		// Run takes a subtest NAME, which is a label rather than a sentence a
		// caller ever reads.
		"Run":
		return true
	}

	return false
}

// TestNoTestPinsADeclaredSentenceByConcatenation keeps every pinned prose
// sentence written as a single literal.
//
// A prose sweep rewrites declared sentences by reading them out of the sources,
// so a pin built as "Failed to retrieve " + toolName is invisible to it: three
// monitor pins survived a 65-site sweep that way, and the tests still passed
// because the concatenation still evaluated to the old sentence. Spelling the
// pin whole is what puts it back in front of the next sweep.
func TestNoTestPinsADeclaredSentenceByConcatenation(t *testing.T) {
	t.Parallel()

	var (
		scanned int
		found   []string
	)

	err := filepath.WalkDir(goTestTreeRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}

		scanned++

		found = append(found, concatPinsIn(t, path)...)

		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", goTestTreeRoot, err)
	}

	// Non-vacuity: a scan that read nothing would pass while proving nothing,
	// which is the failure mode this whole guard exists to close.
	if scanned == 0 {
		t.Fatalf("scanned no test files under %s, so this asserted nothing", goTestTreeRoot)
	}

	for _, site := range found {
		t.Errorf("%s builds a pinned sentence by concatenation; spell it as one literal", site)
	}
}

// concatPinsIn reports every prose concatenation one test file pins with,
// skipping the ones that only feed a failure report.
func concatPinsIn(t *testing.T, path string) []string {
	t.Helper()

	fileSet := token.NewFileSet()

	parsed, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	var (
		found     []string
		reporting []ast.Node
	)

	ast.Inspect(parsed, func(node ast.Node) bool {
		if node == nil {
			return false
		}

		if isReportingCall(node) {
			reporting = append(reporting, node)
		}

		binary, isBinary := node.(*ast.BinaryExpr)
		if !isBinary || binary.Op != token.ADD || withinAny(binary, reporting) {
			return true
		}

		if !prosePrefix(binary.X) || !namesAValue(binary.Y) {
			return true
		}

		found = append(found, fileSet.Position(binary.Pos()).String())

		return true
	})

	return found
}

// isReportingCall reports whether a node is a t.Errorf-style call, whose
// arguments are prose the test prints rather than prose it pins.
func isReportingCall(node ast.Node) bool {
	call, isCall := node.(*ast.CallExpr)
	if !isCall {
		return false
	}

	selector, isSelector := call.Fun.(*ast.SelectorExpr)

	return isSelector && reportingCall(selector.Sel.Name)
}

// withinAny reports whether a node sits inside any of the given ones.
func withinAny(node ast.Node, outer []ast.Node) bool {
	for _, candidate := range outer {
		if node.Pos() >= candidate.Pos() && node.End() <= candidate.End() {
			return true
		}
	}

	return false
}

// prosePrefix reports whether an expression is a string literal that opens a
// sentence: several words, ending where a value is about to be joined on, and
// starting with the sentence's own first character rather than punctuation that
// only glues a report together.
func prosePrefix(expr ast.Expr) bool {
	literal, isLiteral := expr.(*ast.BasicLit)
	if !isLiteral || literal.Kind != token.STRING {
		return false
	}

	// A raw literal holds source the emitter writes, not prose a caller reads:
	// the toolgen tests pin generated Go lines that way, and a sweep has no
	// sentence to find in them.
	if strings.HasPrefix(literal.Value, "`") {
		return false
	}

	text, err := strconv.Unquote(literal.Value)
	if err != nil {
		return false
	}

	if !strings.HasSuffix(text, " ") {
		return false
	}

	opening := strings.TrimLeft(text, " ")
	if opening == "" || !isSentenceOpener(rune(opening[0])) {
		return false
	}

	return len(strings.Fields(text)) >= concatProseWords
}

// isSentenceOpener reports whether a rune can begin a sentence a caller reads.
func isSentenceOpener(first rune) bool {
	return (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') ||
		(first >= '0' && first <= '9')
}

// namesAValue reports whether an expression is a plain name, which is the shape
// a sweep cannot resolve back to the sentence it belongs to.
func namesAValue(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return namesAValue(typed.X)
	default:
		return false
	}
}
