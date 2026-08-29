package main

import (
	"fmt"
	"slices"
	"strings"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The per-entry verdict vocabulary: the second set of conditions a local
// operation can report, and the only one whose words are declared on the
// operation rather than on a tool.
//
// A LocalGuard ends the call, so the tool words it and the operation reports it
// without prose. A verdict is a member of a SUCCESSFUL answer, one per entry of
// what the operation was handed, and its bucket is a key a caller counts by. A
// caller reading two binaries has to read one bucket and one sentence, so both
// belong to the operation, and the emitter writes each language's copy from the
// one declaration.

// localVerdictPrefix opens every member of the verdict vocabulary.
const localVerdictPrefix = "LOCAL_VERDICT_"

// localVerdictValuePrefix opens every member of the value vocabulary, and
// closes over localVerdictPrefix, which is why a verdict's stem is taken with
// its own prefix rather than by trimming twice.
const localVerdictValuePrefix = "LOCAL_VERDICT_VALUE_"

// localVerdictPermitted is the one member that stops nothing, so it is the one
// that words nothing. Named here because the all-or-nothing check reads it.
const localVerdictPermitted = linodev1.LocalVerdict_LOCAL_VERDICT_PERMITTED

// localVerdictVocabulary is every verdict the contract declares, in the
// vocabulary's own order, with the unset member left out.
func localVerdictVocabulary() []linodev1.LocalVerdict {
	members := linodev1.LocalVerdict(0).Descriptor().Values()
	ordered := make([]linodev1.LocalVerdict, 0, members.Len())

	for index := range members.Len() {
		verdict := linodev1.LocalVerdict(members.Get(index).Number())
		if verdict != linodev1.LocalVerdict_LOCAL_VERDICT_UNSPECIFIED {
			ordered = append(ordered, verdict)
		}
	}

	return ordered
}

// localVerdictValueVocabulary is every value a verdict sentence may name, in
// the vocabulary's own order, with the unset member left out.
func localVerdictValueVocabulary() []linodev1.LocalVerdictValue {
	members := linodev1.LocalVerdictValue(0).Descriptor().Values()
	ordered := make([]linodev1.LocalVerdictValue, 0, members.Len())

	for index := range members.Len() {
		value := linodev1.LocalVerdictValue(members.Get(index).Number())
		if value != linodev1.LocalVerdictValue_LOCAL_VERDICT_VALUE_UNSPECIFIED {
			ordered = append(ordered, value)
		}
	}

	return ordered
}

// verdicts is the rows the operation declares, which is empty on every
// operation that answers about the call rather than about each entry.
func (o *localOperation) verdicts() []*linodev1.LocalVerdictWording {
	return o.arm.declared.GetVerdict()
}

// localVerdictName is the Go spelling of one verdict, derived from the declared
// member so a member added to the contract cannot reach an engine under a name
// nobody wrote.
func localVerdictName(verdict linodev1.LocalVerdict) string {
	return localVerdictType + localMemberPascal(localVerdictMember(verdict))
}

// localVerdictMember is the Python spelling of one verdict, which is the
// declared member with its prefix removed.
func localVerdictMember(verdict linodev1.LocalVerdict) string {
	return strings.TrimPrefix(verdict.String(), localVerdictPrefix)
}

// localVerdictValueStem is the name a sentence spells one value under, which is
// the declared member's own stem in lower case: {tool} is the value
// LOCAL_VERDICT_VALUE_TOOL. It is also the local every language takes it under,
// so the contract's spelling and the parameter's cannot drift apart.
func localVerdictValueStem(value linodev1.LocalVerdictValue) string {
	return strings.ToLower(strings.TrimPrefix(value.String(), localVerdictValuePrefix))
}

// localVerdictValueNamed is the value one sentence's placeholder names, and
// whether the vocabulary carries it at all.
func localVerdictValueNamed(named string) (linodev1.LocalVerdictValue, bool) {
	for _, value := range localVerdictValueVocabulary() {
		if localVerdictValueStem(value) == named {
			return value, true
		}
	}

	return linodev1.LocalVerdictValue_LOCAL_VERDICT_VALUE_UNSPECIFIED, false
}

// localVerdictValues is the values one operation's sentences name, in the
// vocabulary's own order rather than the order any sentence spells them.
//
// The parameter list is derived from what is named rather than fixed at the
// whole vocabulary, so a language never emits a parameter its own sentences do
// not read, which its linter would answer for.
func localVerdictValues(operation *localOperation) []linodev1.LocalVerdictValue {
	named := make(map[linodev1.LocalVerdictValue]bool, len(localVerdictValueVocabulary()))

	for _, row := range operation.verdicts() {
		for _, sentence := range localVerdictSentences(row) {
			for _, placeholder := range localPlaceholders(sentence) {
				if value, carried := localVerdictValueNamed(placeholder.name); carried {
					named[value] = true
				}
			}
		}
	}

	wanted := make([]linodev1.LocalVerdictValue, 0, len(named))

	for _, value := range localVerdictValueVocabulary() {
		if named[value] {
			wanted = append(wanted, value)
		}
	}

	return wanted
}

// localVerdictSentences is the prose one row carries, which is the two members
// a placeholder can appear in. The bucket is a key rather than a sentence, so
// it takes no placeholder and is not read here.
func localVerdictSentences(row *linodev1.LocalVerdictWording) []string {
	return []string{row.GetReason(), row.GetRemedy()}
}

// localVerdictWords is one row's three words, which are stated together or not
// at all.
func localVerdictWords(row *linodev1.LocalVerdictWording) []string {
	return append([]string{row.GetBucket()}, localVerdictSentences(row)...)
}

// checkOperationVerdicts holds one operation's declared verdicts to what an
// emitted vocabulary can carry.
//
// An operation that reports no verdict is exempt: it answers about the call as
// a whole, which every operation but one does.
func checkOperationVerdicts(operation *localOperation) error {
	rows := operation.verdicts()
	if len(rows) == 0 {
		return nil
	}

	declared, err := verdictRowsByMember(operation, rows)
	if err != nil {
		return err
	}

	// The whole vocabulary rather than the rows alone: a member left out is a
	// verdict the emitted type says exists and no language can word, so each
	// engine would carry a lookup over that type with one arm missing.
	for _, verdict := range localVerdictVocabulary() {
		if declared[verdict] == nil {
			return fmt.Errorf("%w: %s leaves %s unworded",
				errLocalVerdictIncomplete, operation.call, verdict)
		}
	}

	for _, verdict := range localVerdictVocabulary() {
		if err := checkVerdictRow(operation, verdict, declared[verdict]); err != nil {
			return err
		}
	}

	return nil
}

// verdictRowsByMember is the operation's rows keyed by the verdict each words,
// refusing a row that names none and one that words a verdict twice.
func verdictRowsByMember(
	operation *localOperation, rows []*linodev1.LocalVerdictWording,
) (map[linodev1.LocalVerdict]*linodev1.LocalVerdictWording, error) {
	declared := make(map[linodev1.LocalVerdict]*linodev1.LocalVerdictWording, len(rows))

	for _, row := range rows {
		verdict := row.GetVerdict()
		if verdict == linodev1.LocalVerdict_LOCAL_VERDICT_UNSPECIFIED {
			return nil, fmt.Errorf("%w: %s words a row naming no verdict",
				errLocalVerdictUnnamed, operation.call)
		}

		if declared[verdict] != nil {
			return nil, fmt.Errorf("%w: %s words %s twice",
				errLocalVerdictRepeated, operation.call, verdict)
		}

		declared[verdict] = row
	}

	return declared, nil
}

// checkVerdictRow holds one row to the all-or-nothing rule and to the closed
// value vocabulary its sentences may name.
func checkVerdictRow(
	operation *localOperation, verdict linodev1.LocalVerdict,
	row *linodev1.LocalVerdictWording,
) error {
	words := localVerdictWords(row)
	stated := slices.IndexFunc(words, func(word string) bool { return word != "" }) >= 0

	if verdict == localVerdictPermitted && stated {
		return fmt.Errorf("%w: %s words %s, which stops no entry",
			errLocalVerdictWorded, operation.call, verdict)
	}

	if verdict != localVerdictPermitted && slices.Contains(words, "") {
		return fmt.Errorf("%w: %s leaves a bucket, a reason or a remedy off %s",
			errLocalVerdictUnworded, operation.call, verdict)
	}

	return checkVerdictPlaceholders(operation, verdict, row)
}

// checkVerdictPlaceholders refuses a sentence naming a value no entry carries.
// The vocabulary holds two members and neither reaches the request, the
// argument map or a tool's name, which is what keeps a tool's identity out of
// prose the operation's own answer carries.
func checkVerdictPlaceholders(
	operation *localOperation, verdict linodev1.LocalVerdict,
	row *linodev1.LocalVerdictWording,
) error {
	for _, sentence := range localVerdictSentences(row) {
		for _, placeholder := range localPlaceholders(sentence) {
			if _, carried := localVerdictValueNamed(placeholder.name); !carried {
				return fmt.Errorf("%w: %s names %s on %s in %q",
					errLocalVerdictPlaceholder, operation.call,
					placeholder.name, verdict, sentence)
			}
		}
	}

	return nil
}

// verdictOperations is every generated operation that declares a verdict
// vocabulary, in the order the operations are written.
func verdictOperations(operations []localOperation) []*localOperation {
	wording := make([]*localOperation, 0, len(operations))

	for index := range operations {
		if len(operations[index].verdicts()) > 0 {
			wording = append(wording, &operations[index])
		}
	}

	return wording
}

// localVerdictWordsFunction is the name one language answers an operation's
// wording through, and localVerdictBucketsFunction the name it answers the
// zeroed bucket set through. Both derive from the operation's own stem under
// the naming rule the subsystem method already uses, so neither carries a
// per-language suffix the contract would have to hold.
func localVerdictWordsFunction(language, member string) string {
	return localVerdictDerived(language, member, "words")
}

func localVerdictBucketsFunction(language, member string) string {
	return localVerdictDerived(language, member, "buckets")
}

// localVerdictDerived is the operation's stem closed by one word, spelled the
// way that language spells a subsystem method.
func localVerdictDerived(language, member, word string) string {
	stem, derived := localArmStem(member)
	if !derived {
		return ""
	}

	naming, ruled := localSubsystemNaming()[language]
	if !ruled {
		return ""
	}

	return naming.spell(stem + "_" + word)
}

// The names both languages give the emitted vocabulary. They name the contract
// rather than anything a language owns, so one spelling answers for every
// language, the way the subsystem type's does.
const (
	localVerdictType      = "LocalVerdict"
	localVerdictWordsType = "LocalVerdictWords"
)

// localVerdictWordMembers is the members of the emitted words type, in the
// order a row states them, which is the order localVerdictWords answers in.
func localVerdictWordMembers() []string {
	return []string{"bucket", "reason", "remedy"}
}
