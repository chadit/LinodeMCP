package main

import (
	"strings"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The Go arm of the verdict vocabulary: the type an operation reports one
// verdict as, and the words it answers for each.
//
// The words land in the answers tree beside the shapes, because the subsystem
// that decides a verdict is what reads them, and the answers tree is the one
// place both an engine and a handler can name. Nothing about that lets the
// subsystem tell which tool called it: a verdict's words are the operation's
// own, so every tool built on it answers the same ones.

// goVerdictSurface writes the verdict type, the words it carries, and the
// wording each operation declares, empty where no operation declares any.
func goVerdictSurface(operations []localOperation) string {
	wording := verdictOperations(operations)
	if len(wording) == 0 {
		return ""
	}

	var out strings.Builder

	out.Grow(len(wording) * goAnswerShapeBudget)
	out.WriteString(goVerdictType())

	for _, operation := range wording {
		out.WriteString(goVerdictWords(operation))
		out.WriteString(goVerdictBuckets(operation))
	}

	return out.String()
}

// goVerdictType writes the verdict vocabulary and the words one verdict
// answers with.
//
// The members open at one rather than at zero, so a verdict nothing assigned is
// not silently the one that permits an entry.
func goVerdictType() string {
	var out strings.Builder

	out.Grow(goAnswerShapeBudget)
	out.WriteString(`
// ` + localVerdictType + ` is one condition an operation reports about a single entry
// of what it was handed, rather than about the call as a whole.
type ` + localVerdictType + ` int

const (
`)

	for index, verdict := range localVerdictVocabulary() {
		if index > 0 {
			out.WriteString("\n")
		}

		out.WriteString("\t// ")
		out.WriteString(localVerdictName(verdict))
		out.WriteString(" is the verdict ")
		out.WriteString(verdict.String())
		out.WriteString(" names.\n")
		out.WriteString("\t")
		out.WriteString(localVerdictName(verdict))

		if index == 0 {
			out.WriteString(" " + localVerdictType + " = iota + 1")
		}

		out.WriteString("\n")
	}

	out.WriteString(`)

// ` + localVerdictWordsType + ` is what one verdict answers about the entry it was
// reached on: the summary bucket it counts in, why the entry was stopped, and
// what would let it through. All three are empty on a verdict that stops
// nothing, and on one no operation words.
type ` + localVerdictWordsType + ` struct {
	Bucket string
	Reason string
	Remedy string
}
`)

	return out.String()
}

// goVerdictWords writes one operation's wording: a case per declared verdict,
// with the entry's own values written into the sentences the contract declares.
func goVerdictWords(operation *localOperation) string {
	name := localVerdictWordsFunction(languageGo, operation.member())
	declared := goVerdictRows(operation)

	var out strings.Builder

	out.Grow(goAnswerShapeBudget)
	out.WriteString("\n// ")
	out.WriteString(name)
	out.WriteString(" is what ")
	out.WriteString(operation.member())
	out.WriteString(" answers for one verdict,\n")
	out.WriteString("// with the entry's own values written into the sentences the contract\n")
	out.WriteString("// declares. A verdict it words none for answers no words at all.\n")
	out.WriteString("func ")
	out.WriteString(name)
	out.WriteString("(")
	out.WriteString(goVerdictParameters(operation))
	out.WriteString(") ")
	out.WriteString(localVerdictWordsType)
	out.WriteString(" {\n")
	out.WriteString("\tswitch verdict {\n")

	for _, verdict := range localVerdictVocabulary() {
		out.WriteString("\tcase ")
		out.WriteString(localVerdictName(verdict))
		out.WriteString(":\n")
		out.WriteString("\t\treturn ")
		out.WriteString(goVerdictLiteral(declared[verdict]))
	}

	out.WriteString("\t}\n\n\treturn " + localVerdictWordsType + "{}\n}\n")

	return out.String()
}

// goVerdictParameters is the wording function's own parameter list: the verdict
// it answers for, then one local per value the operation's sentences name.
//
// Derived from what the sentences read rather than fixed at the whole
// vocabulary, so nothing here is a parameter the emitted body never touches.
func goVerdictParameters(operation *localOperation) string {
	values := localVerdictValues(operation)
	parts := make([]goParameter, 0, len(values)+1)
	parts = append(parts, goParameter{name: goVerdictLocal, carried: localVerdictType})

	for _, value := range values {
		parts = append(parts, goParameter{
			name: localVerdictValueStem(value), carried: textType,
		})
	}

	return goJoinParameters(parts)
}

// goVerdictLocal is the name the wording function takes its verdict under, and
// the name its own switch reads.
const goVerdictLocal = "verdict"

// goVerdictRows is one operation's declared rows keyed by the verdict each
// words. checkOperationVerdicts has already refused a row naming no verdict, a
// verdict worded twice, and a member left unworded, so every member of the
// vocabulary answers a row here.
func goVerdictRows(
	operation *localOperation,
) map[linodev1.LocalVerdict]*linodev1.LocalVerdictWording {
	declared := make(map[linodev1.LocalVerdict]*linodev1.LocalVerdictWording,
		len(operation.verdicts()))

	for _, row := range operation.verdicts() {
		declared[row.GetVerdict()] = row
	}

	return declared
}

// goVerdictLiteral is one row's three words as the value the case answers, each
// sentence rendered through the same filler a refusal sentence goes through.
func goVerdictLiteral(row *linodev1.LocalVerdictWording) string {
	var out strings.Builder

	out.Grow(goAnswerShapeBudget)
	out.WriteString(localVerdictWordsType + "{\n")

	for index, word := range localVerdictWords(row) {
		out.WriteString("\t\t\t")
		out.WriteString(localMemberPascal(localVerdictWordMembers()[index]))
		out.WriteString(": ")
		out.WriteString(goVerdictSentence(word))
		out.WriteString(",\n")
	}

	out.WriteString("\t\t}\n")

	return out.String()
}

// goVerdictSentence renders one declared word as the Go expression producing
// it. A placeholder names a value of the entry, which the wording function
// takes under the value's own stem.
func goVerdictSentence(sentence string) string {
	format, args := goLocalFilled(sentence, goVerdictRead)
	if len(args) == 0 {
		return goStringLiteral(sentence)
	}

	return "fmt.Sprintf(" +
		strings.Join(append([]string{goStringLiteral(format)}, args...), ", ") + ")"
}

// goVerdictRead is how a verdict sentence fills one placeholder: the local the
// named value arrives under, which is that value's own stem. A name outside the
// vocabulary is refused before any of this renders.
func goVerdictRead(named string) string {
	value, _ := localVerdictValueNamed(named)

	return localVerdictValueStem(value)
}

// goVerdictBuckets writes every bucket one operation declares, at zero, which
// is what keeps a bucket no entry reached answering 0 rather than going
// missing from the summary.
func goVerdictBuckets(operation *localOperation) string {
	name := localVerdictBucketsFunction(languageGo, operation.member())
	declared := goVerdictRows(operation)

	var out strings.Builder

	out.Grow(goAnswerShapeBudget)
	out.WriteString("\n// ")
	out.WriteString(name)
	out.WriteString(" is every bucket ")
	out.WriteString(operation.member())
	out.WriteString(" declares, at\n")
	out.WriteString("// zero, so one no entry reached still answers 0.\n")
	out.WriteString("func ")
	out.WriteString(name)
	out.WriteString("() map[string]int {\n")
	out.WriteString("\treturn map[string]int{\n")

	for _, verdict := range localVerdictVocabulary() {
		if bucket := declared[verdict].GetBucket(); bucket != "" {
			out.WriteString("\t\t")
			out.WriteString(goStringLiteral(bucket))
			out.WriteString(": 0,\n")
		}
	}

	out.WriteString("\t}\n}\n")

	return out.String()
}
