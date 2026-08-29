package main

import (
	"strings"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The Python arm of the verdict vocabulary. It writes what the Go arm writes,
// through this language's own idioms: an Enum for the vocabulary, a frozen
// dataclass for the words, and a lookup where Go writes a switch.
//
// The lookup rather than a chain of returns: this language holds a function to
// a return count, and a chain would sit at that limit with the vocabulary as it
// stands, so a verdict added to the contract would fail the run on the shape of
// the emitted code rather than on anything the contract got wrong.

// pyVerdictSurface writes the verdict type, the words it carries, and the
// wording each operation declares, empty where no operation declares any.
func pyVerdictSurface(operations []localOperation) []string {
	wording := verdictOperations(operations)
	if len(wording) == 0 {
		return nil
	}

	lines := pyVerdictType()

	for _, operation := range wording {
		lines = append(lines, pyVerdictWords(operation)...)
		lines = append(lines, pyVerdictBuckets(operation)...)
	}

	return lines
}

// pyVerdictType writes the verdict vocabulary and the words one verdict answers
// with.
func pyVerdictType() []string {
	lines := make([]string, 0, len(localVerdictVocabulary())*shapeLineBudget)
	lines = append(lines,
		"",
		"",
		"class "+localVerdictType+"(Enum):",
		`    """One condition an operation reports about a single entry.`,
		"")
	verdictProse := "It answers about one entry of what the operation was handed" +
		" rather than about the call as a whole."
	lines = append(lines, pyProseWrapped(verdictProse)...)
	lines = append(lines, pyDocClose, "")

	for _, verdict := range localVerdictVocabulary() {
		lines = append(lines, "    "+localVerdictMember(verdict)+" = auto()")
	}

	lines = append(lines,
		"",
		"",
		"@dataclass(frozen=True)",
		"class "+localVerdictWordsType+":",
		`    """What one verdict answers about the entry it was reached on.`,
		"")
	wordsProse := "The summary bucket it counts in, why the entry was stopped," +
		" and what would let it through. All three are empty on a verdict that" +
		" stops nothing, and on one no operation words."
	lines = append(lines, pyProseWrapped(wordsProse)...)
	lines = append(lines, pyDocClose, "")

	for _, member := range localVerdictWordMembers() {
		lines = append(lines, "    "+member+": "+typeWordStr)
	}

	return lines
}

// pyVerdictWords writes one operation's wording as a lookup over the whole
// vocabulary, with the entry's own values written into the sentences.
func pyVerdictWords(operation *localOperation) []string {
	name := localVerdictWordsFunction(languagePython, operation.member())
	declared := goVerdictRows(operation)

	lines := []string{"", ""}
	lines = append(lines, pyVerdictSignature(name, operation)...)
	lines = append(lines,
		`    """What `+operation.member()+` answers for one verdict.`,
		"")
	prose := "The entry's own values are written into the sentences the contract" +
		" declares. A verdict it words none for answers no words at all."
	lines = append(lines, pyProseWrapped(prose)...)
	lines = append(lines, pyDocClose, "    declared = {")

	for _, verdict := range localVerdictVocabulary() {
		lines = append(lines, pyVerdictRow(verdict, declared[verdict])...)
	}

	return append(lines, "    }", "",
		"    return declared.get(verdict, "+pyVerdictEmpty()+")")
}

// pyVerdictSignature is the wording function's own definition line, wrapped the
// way ruff wraps one that does not fit on a single line.
func pyVerdictSignature(name string, operation *localOperation) []string {
	values := localVerdictValues(operation)
	taken := make([]string, 0, len(values)+1)
	taken = append(taken, goVerdictLocal+": "+localVerdictType)

	for _, value := range values {
		taken = append(taken, localVerdictValueStem(value)+": "+typeWordStr)
	}

	opening := "def " + name + "(" + strings.Join(taken, ", ") + ") -> " +
		localVerdictWordsType + ":"
	if len(opening) <= pyLineBudget {
		return []string{opening}
	}

	return []string{
		"def " + name + "(",
		"    " + strings.Join(taken, ", "),
		") -> " + localVerdictWordsType + ":",
	}
}

// pyVerdictRow is one member of the lookup: the verdict it answers for and the
// words the contract declares for it.
func pyVerdictRow(
	verdict linodev1.LocalVerdict, row *linodev1.LocalVerdictWording,
) []string {
	words := localVerdictWords(row)

	lines := []string{
		"        " + localVerdictType + "." + localVerdictMember(verdict) + ": " +
			localVerdictWordsType + "(",
	}

	for index, word := range words {
		lines = append(lines,
			pyVerdictWordLines(localVerdictWordMembers()[index], word)...)
	}

	return append(lines, "        ),")
}

// pyVerdictWordIndent is where one word's own value starts inside the lookup,
// which is three levels in: the function body, the lookup, and the call.
const pyVerdictWordIndent = 12

// pyVerdictWordLines is one keyword member of the words call, split across
// lines where the sentence does not fit beside its own keyword.
func pyVerdictWordLines(member, sentence string) []string {
	opening := strings.Repeat(" ", pyVerdictWordIndent) + member + "="
	single := opening + pyVerdictSentence(sentence) + ","

	if len(single) <= pyLineBudget {
		return []string{single}
	}

	// A split literal inside a call is an implicit concatenation, which the
	// linter holds to being parenthesized.
	lines := []string{opening + "("}
	lines = append(lines,
		pyVerdictSentenceWrapped(sentence, pyVerdictWordIndent+pyGroupIndent)...)

	return append(lines, strings.Repeat(" ", pyVerdictWordIndent)+"),")
}

// pyVerdictSentence renders one declared word as the Python expression
// producing it: the text itself where it names nothing, and an f-string
// otherwise.
func pyVerdictSentence(sentence string) string {
	body, filled := pyLocalFilled(sentence, pyVerdictRead)
	if !filled {
		return pyQuote(sentence)
	}

	return `f"` + body + `"`
}

// pyVerdictSentenceWrapped is one sentence split across lines at the given
// indent.
//
// The split runs over the sentence's own words rather than over the rendered
// text, because a rendered placeholder is one token no line break may fall
// inside. Each segment renders on its own, so a segment naming no value stays a
// plain literal and only the ones that do become f-strings.
func pyVerdictSentenceWrapped(sentence string, indent int) []string {
	pad := strings.Repeat(" ", indent)
	lines := make([]string, 0, len(sentence)/pyLineBudget+1)

	var current string

	for word := range strings.SplitSeq(sentence, " ") {
		candidate := current + word + " "
		if current != "" && len(pad+pyVerdictSentence(candidate)) > pyLineBudget {
			lines = append(lines, pad+pyVerdictSentence(current))
			current = word + " "

			continue
		}

		current = candidate
	}

	return append(lines, pad+pyVerdictSentence(strings.TrimSuffix(current, " ")))
}

// pyVerdictRead is how a verdict sentence fills one placeholder, which is the
// Go arm's own answer: the local the named value arrives under.
func pyVerdictRead(named string) string {
	return goVerdictRead(named)
}

// pyVerdictEmpty is the words a verdict no operation worded answers.
func pyVerdictEmpty() string {
	filled := make([]string, 0, len(localVerdictWordMembers()))
	for _, member := range localVerdictWordMembers() {
		filled = append(filled, member+`=""`)
	}

	return localVerdictWordsType + "(" + strings.Join(filled, ", ") + ")"
}

// pyVerdictBuckets writes every bucket one operation declares, at zero, which
// is what keeps a bucket no entry reached answering 0 rather than going missing
// from the summary.
func pyVerdictBuckets(operation *localOperation) []string {
	name := localVerdictBucketsFunction(languagePython, operation.member())
	declared := goVerdictRows(operation)

	lines := []string{
		"",
		"",
		"def " + name + "() -> dict[" + typeWordStr + ", " + typeWordInt + "]:",
		`    """Every bucket ` + operation.member() + ` declares, at zero.`,
		"",
	}
	prose := "A bucket no entry reached still answers 0 rather than going missing" +
		" from the summary."
	lines = append(lines, pyProseWrapped(prose)...)
	lines = append(lines, pyDocClose, "    return {")

	for _, verdict := range localVerdictVocabulary() {
		if bucket := declared[verdict].GetBucket(); bucket != "" {
			lines = append(lines, "        "+pyQuote(bucket)+": 0,")
		}
	}

	return append(lines, "    }")
}
