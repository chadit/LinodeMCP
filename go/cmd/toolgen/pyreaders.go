package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The path-segment readers, which live in linodemcp.tools.segment_readers
// rather than in linodemcp.tools.helpers, space-bounded so one lookup covers
// the group.
const pySegmentModuleReaders = " required_pathsafe_segment" +
	" declared_pathsafe_segment declared_fragment_safe_segment "

// pyHelpersModuleReaders is which readers linodemcp.tools.helpers exports,
// which is every one this renderer writes except the segment group above.
const pyHelpersModuleReaders = " required_int_id required_bounded_int_id" +
	" member_choice present_text present_bool declared_int_id" +
	" declared_member_choice declared_present_text declared_present_bool" +
	" present_string declared_present_string id_list declared_id_list" +
	" refused_arguments unknown_arguments service_type_slug region_slug" +
	" beta_slug declared_service_type_slug declared_region_slug" +
	" declared_beta_slug free_text declared_free_text "

// pyMemberChoicePlain and pyMemberChoiceWorded are the membership helper's two
// spellings, which both a path slot and a body member read through.
const (
	pyMemberChoicePlain  = "member_choice"
	pyMemberChoiceWorded = "declared_member_choice"
)

// pyIsPresenceReader reports whether a reader is one of the members read off
// the argument map beside a body, each asking whether the caller sent one
// argument and what shape it arrived in.
func pyIsPresenceReader(reader linodev1.ArgumentReader) bool {
	_, _, named := pyPresenceSpellings(reader)

	return named
}

// pyReaderHelper is the helper one slot's reader is spelled as, worded or
// plain. Each names the function that already answers that member's sentences,
// which is what holds the two languages to one wording without either
// restating it. A reader with no spelling here fails the run rather than
// reaching the tree unchecked.
func pyReaderHelper(reader linodev1.ArgumentReader, worded bool) (string, error) {
	plain, declared, named := pyReaderSpellings(reader)
	if !named {
		return "", fmt.Errorf("%w: argument_reader %s names a reader the Python renderer cannot write",
			errPyRender, reader)
	}

	if worded {
		return declared, nil
	}

	return plain, nil
}

// pyReaderSpellings is one reader's plain and worded helper names. The
// fragment-safe member has no unworded form: every tool that declares it words
// a refusal of its own, so the plain spelling would name nothing.
func pyReaderSpellings(reader linodev1.ArgumentReader) (string, string, bool) {
	switch reader {
	case linodev1.ArgumentReader_ARGUMENT_READER_POSITIVE_ID:
		return "required_int_id", "declared_int_id", true
	case linodev1.ArgumentReader_ARGUMENT_READER_BOUNDED_ID:
		return "required_bounded_int_id", "declared_int_id", true
	case linodev1.ArgumentReader_ARGUMENT_READER_PATH_SAFE_TEXT:
		return "required_pathsafe_segment", "declared_pathsafe_segment", true
	case linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER:
		return pyMemberChoicePlain, pyMemberChoiceWorded, true
	case linodev1.ArgumentReader_ARGUMENT_READER_FRAGMENT_SAFE_TEXT:
		return "declared_fragment_safe_segment", "declared_fragment_safe_segment", true
	case linodev1.ArgumentReader_ARGUMENT_READER_SERVICE_TYPE_SLUG:
		return "service_type_slug", "declared_service_type_slug", true
	case linodev1.ArgumentReader_ARGUMENT_READER_REGION_SLUG:
		return "region_slug", "declared_region_slug", true
	case linodev1.ArgumentReader_ARGUMENT_READER_BETA_SLUG:
		return "beta_slug", "declared_beta_slug", true
	case linodev1.ArgumentReader_ARGUMENT_READER_FREE_TEXT:
		return "free_text", "declared_free_text", true
	// The presence members are read off the argument map beside a body rather
	// than out of a path, so they are spelled by pyPresenceSpellings instead.
	case linodev1.ArgumentReader_ARGUMENT_READER_UNSPECIFIED,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_BOOL,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_STRING,
		linodev1.ArgumentReader_ARGUMENT_READER_ID_LIST:
	}

	return "", "", false
}

// pyThreeArmReader reports whether a reader answers an absent, an unusable, and
// a refused arm, which is what decides how many sentences its call carries.
func pyThreeArmReader(reader linodev1.ArgumentReader) bool {
	switch reader {
	case linodev1.ArgumentReader_ARGUMENT_READER_PATH_SAFE_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_FRAGMENT_SAFE_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_SERVICE_TYPE_SLUG,
		linodev1.ArgumentReader_ARGUMENT_READER_REGION_SLUG,
		linodev1.ArgumentReader_ARGUMENT_READER_BETA_SLUG:
		return true
	// An id answers no unusable arm and a presence member answers no refused
	// one, so neither carries the third sentence.
	case linodev1.ArgumentReader_ARGUMENT_READER_UNSPECIFIED,
		linodev1.ArgumentReader_ARGUMENT_READER_POSITIVE_ID,
		linodev1.ArgumentReader_ARGUMENT_READER_BOUNDED_ID,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT,
		linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_BOOL,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_STRING,
		linodev1.ArgumentReader_ARGUMENT_READER_ID_LIST,
		linodev1.ArgumentReader_ARGUMENT_READER_FREE_TEXT:
	}

	return false
}

// pyPresenceSpellings is one presence member's plain and worded helper names,
// and whether the reader is one of the members read off the argument map beside
// a body.
func pyPresenceSpellings(reader linodev1.ArgumentReader) (string, string, bool) {
	switch reader {
	case linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_TEXT:
		return "present_text", "declared_present_text", true
	case linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_BOOL:
		return "present_bool", "declared_present_bool", true
	case linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_STRING:
		return "present_string", "declared_present_string", true
	case linodev1.ArgumentReader_ARGUMENT_READER_ID_LIST:
		return "id_list", "declared_id_list", true
	// Every other reader answers for a value's shape rather than for whether
	// the caller sent one, so pyReaderSpellings names it instead.
	case linodev1.ArgumentReader_ARGUMENT_READER_UNSPECIFIED,
		linodev1.ArgumentReader_ARGUMENT_READER_POSITIVE_ID,
		linodev1.ArgumentReader_ARGUMENT_READER_BOUNDED_ID,
		linodev1.ArgumentReader_ARGUMENT_READER_PATH_SAFE_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT,
		linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER,
		linodev1.ArgumentReader_ARGUMENT_READER_FRAGMENT_SAFE_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_SERVICE_TYPE_SLUG,
		linodev1.ArgumentReader_ARGUMENT_READER_REGION_SLUG,
		linodev1.ArgumentReader_ARGUMENT_READER_BETA_SLUG,
		linodev1.ArgumentReader_ARGUMENT_READER_FREE_TEXT:
	}

	return "", "", false
}

// pyReaderCall is the Python expression one declared path reader is spelled as.
func pyReaderCall(tool *pyTool, slot string) (string, error) {
	reader := tool.c.Readers[slot]
	worded := tool.worded(slot)

	helper, err := pyReaderHelper(reader, worded)
	if err != nil {
		return "", err
	}

	absent, unusable, refused := tool.sentences(slot)

	if reader == linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER {
		members := "(" + pyNameTuple(tool.c.Members[slot]) + ")"
		if !worded {
			return helper + "(arguments, " + pyQuote(slot) + ", " + pyTrue + ", " + members + ")", nil
		}

		return pyReaderExpression(helper, []string{
			pyArgumentsMap, pyQuote(slot), pyTrue,
			pySentenceArgument(absent), pySentenceArgument(refused), members,
		}), nil
	}

	if !worded {
		return helper + "(arguments, " + pyQuote(slot) + ")", nil
	}

	if reader == linodev1.ArgumentReader_ARGUMENT_READER_FREE_TEXT {
		return pyReaderExpression(helper, []string{
			pyArgumentsMap, pyQuote(slot),
			pySentenceArgument(absent), pySentenceArgument(unusable),
		}), nil
	}

	if pyThreeArmReader(reader) {
		return pyReaderExpression(helper, []string{
			pyArgumentsMap, pyQuote(slot),
			pySentenceArgument(absent), pySentenceArgument(unusable), pySentenceArgument(refused),
		}), nil
	}

	maximum := "0"
	if reader == linodev1.ArgumentReader_ARGUMENT_READER_BOUNDED_ID {
		maximum = "MAX_JSON_SAFE_ID"
	}

	return pyReaderExpression(helper, []string{
		pyArgumentsMap, pyQuote(slot), maximum,
		pySentenceArgument(absent), pySentenceArgument(refused),
	}), nil
}

// pyMemberChoiceLines is the membership call rendered the way ruff format would
// write it: one line when it fits, otherwise the arguments on their own
// indented line, which is the wrap ruff settles on for a call whose arguments
// fit together.
func pyMemberChoiceLines(indent, target, arguments, helper string) []string {
	single := indent + target + " = " + helper + "(" + arguments + ")"
	if len(single) <= pyLineBudget {
		return []string{single}
	}

	return []string{
		indent + target + " = " + helper + "(",
		indent + "    " + arguments,
		indent + ")",
	}
}

// pyMemberChoiceHelper is the membership helper one BODY field reads through,
// worded or plain.
func pyMemberChoiceHelper(entry protoreflect.FieldDescriptor) string {
	absent, unusable, refused := readerMessageOf(entry)
	if absent != "" || unusable != "" || refused != "" {
		return pyMemberChoiceWorded
	}

	return pyMemberChoicePlain
}

// pyMemberChoiceArguments is the argument list a membership call is handed, in
// helper order.
func pyMemberChoiceArguments(entry protoreflect.FieldDescriptor, members []string) string {
	quoted := "(" + pyNameTuple(members) + ")"

	required := pyTrue
	if entry.HasPresence() {
		required = pyFalse
	}

	absent, _, refused := readerMessageOf(entry)
	if absent != "" || refused != "" {
		return "arguments, " + pyQuote(string(entry.Name())) + ", " + required + ", " +
			pySentenceArgument(absent) + ", " + pySentenceArgument(refused) + ", " + quoted
	}

	return "arguments, " + pyQuote(string(entry.Name())) + ", " + required + ", " + quoted
}

// pyPresenceHelper is the presence helper one BODY field reads through, worded
// or plain.
func pyPresenceHelper(entry protoreflect.FieldDescriptor) string {
	plain, worded, _ := pyPresenceSpellings(readerOf(entry))

	absent, unusable, _ := readerMessageOf(entry)
	if absent != "" || unusable != "" {
		return worded
	}

	return plain
}

// pyPresenceCall is the Python expression one BODY presence check is spelled
// as. All three members read the same two arms, so one spelling covers them and
// the declared member picks the reader.
func pyPresenceCall(entry protoreflect.FieldDescriptor) string {
	absent, unusable, refused := readerMessageOf(entry)
	helper := pyPresenceHelper(entry)
	name := pyQuote(string(entry.Name()))

	if readerOf(entry) == linodev1.ArgumentReader_ARGUMENT_READER_ID_LIST {
		if absent == "" && unusable == "" && refused == "" {
			return helper + "(arguments, " + name + ")"
		}

		return pyReaderExpression(helper, []string{
			pyArgumentsMap, name, pySentenceArgument(absent),
			pySentenceArgument(unusable), pySentenceArgument(refused),
		})
	}

	required := pyTrue
	if entry.HasPresence() {
		required = pyFalse
	}

	if absent != "" || unusable != "" {
		return pyReaderExpression(helper, []string{
			pyArgumentsMap, name, required,
			pySentenceArgument(absent), pySentenceArgument(unusable),
		})
	}

	return helper + "(arguments, " + name + ", " + required + ")"
}

// pyBodyReaderFields is the BODY arguments declaring one reader, in declared
// order.
func pyBodyReaderFields(tool *pyTool, reader linodev1.ArgumentReader) []protoreflect.FieldDescriptor {
	found := make([]protoreflect.FieldDescriptor, 0, len(tool.c.Body))

	for _, entry := range tool.bodyFields() {
		if readerOf(entry) == reader {
			found = append(found, entry)
		}
	}

	return found
}

// pyBodyPresenceReaders is the BODY fields read through one of the presence
// members.
func pyBodyPresenceReaders(tool *pyTool) []protoreflect.FieldDescriptor {
	found := make([]protoreflect.FieldDescriptor, 0, len(tool.c.Body))

	for _, entry := range tool.bodyFields() {
		if pyIsPresenceReader(readerOf(entry)) {
			found = append(found, entry)
		}
	}

	return found
}

// pyQueryPresentBoolFields is the query flags a tool asks the presence question
// of, in declared order. A collection has no body to read them beside: the flag
// travels in the query string, and whether the caller sent one is still a
// question only the argument map answers.
func pyQueryPresentBoolFields(tool *pyTool) []protoreflect.FieldDescriptor {
	found := make([]protoreflect.FieldDescriptor, 0, len(tool.c.Query))

	for _, entry := range tool.queryFields() {
		if readerOf(entry) == linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_BOOL {
			found = append(found, entry)
		}
	}

	return found
}

// pySlotReaders is every helper the path slots in one module read through.
func pySlotReaders(tools []*pyTool) (map[string]bool, error) {
	named := make(map[string]bool, len(tools))

	for _, tool := range tools {
		for _, slot := range tool.readerSlots() {
			helper, err := pyReaderHelper(tool.c.Readers[slot], tool.worded(slot))
			if err != nil {
				return nil, err
			}

			named[helper] = true
		}
	}

	return named, nil
}

// pyReadsBoundedWords reports whether a module words a bounded id, whose call
// names the ceiling.
func pyReadsBoundedWords(tools []*pyTool) bool {
	for _, tool := range tools {
		for _, slot := range tool.readerSlots() {
			if tool.c.Readers[slot] == linodev1.ArgumentReader_ARGUMENT_READER_BOUNDED_ID &&
				tool.worded(slot) {
				return true
			}
		}
	}

	return false
}

// pyHelperImports is the linodemcp.tools.helpers names a module takes, "" when
// it takes none. Only what the tiers present actually use, so an unused import
// cannot reach the emitted tree.
func pyHelperImports(tools []*pyTool) (string, error) {
	wanted, err := pyHelperNames(tools)
	if err != nil {
		return "", err
	}

	if len(wanted) == 0 {
		return "", nil
	}

	return strings.Join(wanted, ", "), nil
}

// pyHelperState is what one module's tools ask of the helper layer, gathered
// once so the import list and the decision to write one read the same facts.
type pyHelperState struct {
	answers     bool
	paged       bool
	numericPath bool
	textPath    bool
	removesByID bool
	sentences   bool
	readsReader bool
	refuses     bool
	rewrites    bool
}

// pyHelperFacts gathers what a module's tiers need from the helper layer.
//
// A destroy reports its own missing-id sentence through the driver, which is
// what puts that report after the confirm gate on a live call, so it is left
// out of the tiers that answer where they stand. It also names a different path
// reader than the rest: it refuses an id the others read past, since truncating
// one onto a live path removes a resource the caller never named.
func pyHelperFacts(tools []*pyTool) pyHelperState {
	state := pyHelperState{}

	for _, tool := range tools {
		derived := tool.c.Tier == tierWrite || tool.c.Tier == tierAcknowledge ||
			tool.c.Tier == tierDestroy

		// Every tier outside those answers a broken rule where it stands. A
		// gated read is the exception among reads: its verdict is the driver's
		// to time, the way a mutation's is.
		state.answers = state.answers || (!derived && !tool.gatedRead())

		// Both tiers that spell the page controls in their own handler: a read
		// of a single resource whose route publishes them, and a mutation
		// forwarding the query it publishes.
		state.paged = state.paged ||
			((tool.c.Tier == tierGet || tool.c.Tier == tierWrite) && tool.publishesPage())

		state.readPaths(tool)

		// A meta tool answering its declared sentence builds the message its
		// contract names, the one answer nothing at runtime assembles. A local
		// answer projects what its operation brought back instead, through the
		// engine's own helper.
		state.sentences = state.sentences ||
			(tool.c.Tier == tierMeta && !tool.c.answersLocally())

		state.readsReader = state.readsReader || pyReadsReader(tool)
		state.refuses = state.refuses ||
			len(tool.c.Refused.GetFields()) > 0 || tool.c.RefuseUnknown != nil
		state.rewrites = state.rewrites || len(tool.c.Normalizes) > 0
	}

	return state
}

// readPaths records which path readers one tool's slots reach.
func (s *pyHelperState) readPaths(tool *pyTool) {
	reads := len(tool.c.Path) > 0 && tool.c.Tier != tierDestroy
	removal := tool.c.Tier == tierDestroy && len(tool.c.Path) > 0

	for _, slot := range tool.pathReadSlots() {
		numeric := tool.pathArg(slot).numeric

		s.numericPath = s.numericPath || (reads && numeric)
		s.textPath = s.textPath || ((reads || removal) && !numeric)
		s.removesByID = s.removesByID || (removal && numeric)
	}
}

// pyReadsReader reports whether one tool calls a shared argument reader at all,
// which is what puts error_response and the reader names in the import list.
func pyReadsReader(tool *pyTool) bool {
	readers := []linodev1.ArgumentReader{
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT,
		linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_BOOL,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_STRING,
		linodev1.ArgumentReader_ARGUMENT_READER_ID_LIST,
	}

	for _, reader := range readers {
		if len(pyBodyReaderFields(tool, reader)) > 0 {
			return true
		}
	}

	return len(tool.readerSlots()) > 0 ||
		len(pyQueryPresentBoolFields(tool)) > 0 || tool.c.AnyOf != nil
}

// pyHelperNames is the sorted helper names a module imports.
func pyHelperNames(tools []*pyTool) ([]string, error) {
	state := pyHelperFacts(tools)

	if !state.answers && !state.paged && !state.numericPath && !state.textPath &&
		!state.sentences && !state.removesByID && !state.readsReader &&
		!state.refuses && !state.rewrites {
		return nil, nil
	}

	helpers := make([]string, 0, 16)
	if state.answers || state.paged || state.readsReader {
		helpers = append(helpers, "error_response")
	}

	// Only the readers this module actually calls, so an unused import cannot
	// reach the generated tree.
	named, err := pySlotReaders(tools)
	if err != nil {
		return nil, err
	}

	for _, tool := range tools {
		for _, entry := range pyBodyReaderFields(tool, linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER) {
			named[pyMemberChoiceHelper(entry)] = true
		}

		for _, entry := range pyBodyPresenceReaders(tool) {
			named[pyPresenceHelper(entry)] = true
		}

		for _, entry := range pyQueryPresentBoolFields(tool) {
			named[pyPresenceHelper(entry)] = true
		}
	}

	helpers = append(helpers, pyNamedIn(named, pyHelpersModuleReaders)...)
	helpers = append(helpers, pyConditionalHelpers(tools, state)...)

	sort.Strings(helpers)

	return helpers, nil
}

// pyConditionalHelpers is the helpers a module takes for the shapes its tiers
// spell rather than for the readers they call.
func pyConditionalHelpers(tools []*pyTool, state pyHelperState) []string {
	helpers := make([]string, 0, 10)

	if slices.ContainsFunc(tools, func(tool *pyTool) bool {
		return len(pyBodyReaderFields(tool, linodev1.ArgumentReader_ARGUMENT_READER_PRESENT)) > 0
	}) {
		helpers = append(helpers, "required_present")
	}

	if pyReadsBoundedWords(tools) {
		helpers = append(helpers, "MAX_JSON_SAFE_ID")
	}

	if slices.ContainsFunc(tools, func(tool *pyTool) bool { return tool.c.AnyOf != nil }) {
		helpers = append(helpers, "require_any_argument")
	}

	if slices.ContainsFunc(tools, func(tool *pyTool) bool { return len(tool.c.Refused.GetFields()) > 0 }) {
		helpers = append(helpers, "refused_arguments")
	}

	if slices.ContainsFunc(tools, func(tool *pyTool) bool { return tool.c.RefuseUnknown != nil }) {
		helpers = append(helpers, "unknown_arguments")
	}

	if state.numericPath {
		helpers = append(helpers, "path_int")
	}

	if state.textPath {
		helpers = append(helpers, "path_str")
	}

	if state.removesByID {
		helpers = append(helpers, "destroy_id")
	}

	if state.paged {
		helpers = append(helpers, "pagination_int_argument", "pagination_query")
	}

	if state.sentences {
		helpers = append(helpers, "meta_response")
	}

	return append(helpers, pyNormalizeHelpers(tools)...)
}

// pyNormalizeHelpers is the rewrite functions a module's declared transforms
// call, one name per transform the module actually declares.
func pyNormalizeHelpers(tools []*pyTool) []string {
	rendering := normalizeRendering()
	named := make(map[string]bool, len(rendering))

	for _, tool := range tools {
		for _, rewrite := range tool.c.Normalizes {
			named[rendering[rewrite.Transform].Python] = true
		}

		if tool.c.Fold != nil {
			named["fold_int_list"] = true
		}
	}

	return pySortedKeys(named)
}
