package main

import (
	"maps"
	"slices"
	"strings"
	"unicode"
)

// The Python arm of the answer shapes: a frozen dataclass per shape and the
// projection that turns one into the plain body its operation answers.
//
// Written in dependency order so a reader meets a nested shape before the one
// naming it, the same order the Go arm emits.

// pyAnswersModule is the module the engine and the handlers read the generated
// answers from. One module rather than a file per group: the shapes are a
// vocabulary rather than a per-tool surface.
const pyAnswersModule = "__init__" + pySuffix

// pyAnswerType is the annotation one member is carried under.
func pyAnswerType(member *answerMember) string {
	element := pyAnswerElement(member)

	switch {
	case member.mapped:
		return "dict[str, " + element + "]"
	case member.list:
		return "list[" + element + "]"
	case member.kind == answerNested:
		return element + " | None"
	case member.present:
		return element + " | None"
	}

	return element
}

// pyAnyType is what a member the contract declares free-form is annotated as,
// and what a handed-over record arrives as.
const pyAnyType = "Any"

// pyAnswerElement is the annotation one member's own values take, before the
// list, map or absence wrapper the member puts around it.
func pyAnswerElement(member *answerMember) string {
	switch member.kind {
	case answerString:
		return typeWordStr
	case answerBool:
		return typeWordBool
	case answerInt, answerInt64, answerUint64:
		return typeWordInt
	case answerFree:
		return pyAnyType
	case answerNested:
		return member.shape
	}

	return pyAnyType
}

// shapeLineBudget is roughly what one shape's dataclass and projection run to,
// which is all the emitted slice needs to size itself once.
const shapeLineBudget = 24

// renderPyAnswers writes the whole answers module: the helpers the shapes reach
// for, then a dataclass and a projection per shape.
func renderPyAnswers(shapes []answerShape, operations []localOperation) (string, error) {
	surface, err := pyOperationSurface(operations)
	if err != nil {
		return "", err
	}

	lines := make([]string, 0, len(shapes)*shapeLineBudget)
	lines = append(lines,
		pyHeader,
		`"""The answer a local operation fills and the body it projects to.`,
		"",
		"Both are emitted from the response the contract declares, top level and",
		"nested, so a member reaches every language or none. Nothing here is",
		"hand-written and nothing here survives a `make proto`: change the contract,",
		"not this tree.",
		`"""`,
		"",
		"from __future__ import annotations",
		"",
	)

	helpers := append(pyAnswerHelpers(shapes), pyRecordHelpers(shapes)...)
	lines = append(lines, pyAnswerImports(helpers, surface, pyOperationTypeImports(operations))...)
	lines = append(lines, helpers...)

	for index := range shapes {
		lines = append(lines, pyAnswerShape(&shapes[index])...)
	}

	lines = append(lines, pyRecordDirections(shapes)...)
	lines = append(lines, surface...)

	return strings.Join(lines, "\n") + "\n", nil
}

// pyAnswerImports is the module's import block. The container types and
// whatever the subsystem types name in a signature are annotations alone, so
// they sit in the type-checking block the linter holds them to, and the shapes
// are the type parameters each helper declares inline.
func pyAnswerImports(helpers, surface []string, subsystem map[string][]string) []string {
	text := strings.Join(append(slices.Clone(helpers), surface...), "\n")

	abstract := make([]string, 0, 3)

	for _, wanted := range []string{"Callable", "Mapping", "Sequence"} {
		if strings.Contains(text, wanted+"[") {
			abstract = append(abstract, wanted)
		}
	}

	typed := "from typing import " + strings.Join(pyAnswerTyping(text), ", ")

	// The vocabulary type is the one emitted class this module builds out of
	// the standard library rather than out of a dataclass.
	opening := []string{"from dataclasses import dataclass"}
	if strings.Contains(text, "json.") {
		opening = append([]string{"import json"}, opening...)
	}

	if strings.Contains(text, "(Enum)") {
		opening = append(opening, "from enum import Enum, auto")
	}

	if len(abstract) == 0 && len(subsystem) == 0 {
		return append(opening, typed, "")
	}

	typed = strings.Replace(typed, "import ", "import TYPE_CHECKING, ", 1)

	hinted := maps.Clone(subsystem)
	if len(abstract) > 0 {
		hinted["collections.abc"] = abstract
	}

	// Nothing here is imported at run time for a hint, which is what the linter
	// holds the type-checking block to.
	lines := make([]string, 0, len(hinted)+len(opening)+3)
	lines = append(lines, opening...)
	lines = append(lines, typed, "", "if TYPE_CHECKING:")

	return append(lines, pyTypedImportLines(hinted, "    ")...)
}

// pyAnswerTyping is the names the module takes from typing, in the order the
// import sorter puts them: the free-form value every shape can carry, the
// subsystem surface's own base where one is emitted, and the narrowing a record
// reader needs to name the container it just checked.
func pyAnswerTyping(text string) []string {
	names := []string{pyAnyType}

	if strings.Contains(text, "(Protocol)") {
		names = append(names, "Protocol")
	}

	if strings.Contains(text, "cast(") {
		names = append(names, "cast")
	}

	return names
}

// pyAnswerShape writes one shape's dataclass and its projection.
func pyAnswerShape(shape *answerShape) []string {
	lines := []string{
		"",
		"@dataclass(frozen=True)",
		"class " + shape.name + ":",
		`    """The answer ` + shape.full + ` declares."""`,
		"",
	}

	for _, member := range shape.members {
		lines = append(lines, "    "+member.name+": "+pyAnswerType(&member))
	}

	return append(lines, pyAnswerProjection(shape)...)
}

// pyAnswerProjection writes the function one shape's plain body comes out of,
// member for member in the message's own declaration order.
func pyAnswerProjection(shape *answerShape) []string {
	project := pyAnswerProject(shape.name)

	lines := []string{
		"",
		"",
		"def " + project + "(answer: " + shape.name + ") -> dict[str, Any]:",
		`    """The plain body one ` + shape.name + ` fills."""`,
		"    return {",
	}

	for _, member := range shape.members {
		read := pyAnswerRead(&member, "answer."+member.name)
		lines = append(lines, "        "+pyQuote(member.name)+": "+read+",")
	}

	return append(lines, "    }")
}

// pyAnswerProject is the projection's own name for one shape.
func pyAnswerProject(shape string) string {
	return "project_" + pySnake(shape)
}

// pySnake is one type name in the underscore spelling Python names functions
// with: AuditHealthSQLite becomes audit_health_sqlite.
//
// A word opens where an upper-case letter follows a lower-case one, so a run of
// upper-case letters stays one word rather than breaking at every letter.
func pySnake(name string) string {
	var built strings.Builder

	built.Grow(len(name) + len(name)/2)

	for index, letter := range name {
		if pyWordOpensAt(name, index) {
			built.WriteString("_")
		}

		built.WriteRune(unicode.ToLower(letter))
	}

	return built.String()
}

// pyWordOpensAt is whether the letter at index starts a new word.
func pyWordOpensAt(name string, index int) bool {
	if index == 0 {
		return false
	}

	return unicode.IsUpper(rune(name[index])) && unicode.IsLower(rune(name[index-1]))
}

// pyAnswerRead is the expression one member's body value comes from.
func pyAnswerRead(member *answerMember, read string) string {
	if member.kind == answerNested {
		return pyAnswerNestedRead(member, read)
	}

	switch {
	case member.mapped:
		return "dict(" + read + ")"
	case member.list:
		return "list(" + read + ")"
	}

	return read
}

// pyAnswerNestedRead is the expression one shape-carrying member comes from.
func pyAnswerNestedRead(member *answerMember, read string) string {
	project := pyAnswerProject(member.shape)

	switch {
	case member.mapped:
		return "_answer_keyed(" + read + ", " + project + ")"
	case member.list:
		return "_answer_many(" + read + ", " + project + ")"
	}

	return "_answer_one(" + read + ", " + project + ")"
}

// pyAnswerHelpers is the source for every helper the shapes reach, in a settled
// order. Only the ones some member reads are written, since a helper nothing
// calls is dead code the linter would fail the tree on.
func pyAnswerHelpers(shapes []answerShape) []string {
	wanted := pyAnswerHelpersUsed(shapes)
	lines := make([]string, 0, len(wanted)*10)

	for _, helper := range pyAnswerHelperSet() {
		if slices.Contains(wanted, helper.name) {
			lines = append(lines, helper.lines...)
		}
	}

	return lines
}

// pyAnswerHelpersUsed is the helper every member of every shape reads through.
func pyAnswerHelpersUsed(shapes []answerShape) []string {
	used := make([]string, 0, len(shapes))

	for index := range shapes {
		for _, member := range shapes[index].members {
			read, _, _ := strings.Cut(pyAnswerRead(&member, ""), "(")
			if strings.HasPrefix(read, "_answer") && !slices.Contains(used, read) {
				used = append(used, read)
			}
		}
	}

	return used
}

// pyAnswerHelper is one projection helper: the name a member reads through and
// the source emitted when some member does.
type pyAnswerHelper struct {
	name  string
	lines []string
}

// pyAnswerHelperSet is every helper the projections can reach, in emitted
// order. Each takes the projection as a typed callable so a shape handed the
// wrong projection is a type error rather than a call-time surprise.
func pyAnswerHelperSet() []pyAnswerHelper {
	return []pyAnswerHelper{
		{name: "_answer_one", lines: []string{
			"",
			"def _answer_one[Answer](",
			"    answer: Answer | None, project: Callable[[Answer], dict[str, Any]]",
			") -> dict[str, Any] | None:",
			`    """Project one nested answer, None where the answer carried none."""`,
			"    if answer is None:",
			"        return None",
			"",
			"    return project(answer)",
			"",
		}},
		{name: "_answer_many", lines: []string{
			"",
			"def _answer_many[Answer](",
			"    answers: Sequence[Answer], project: Callable[[Answer], dict[str, Any]]",
			") -> list[dict[str, Any]]:",
			`    """Project a list of nested answers."""`,
			"    return [project(answer) for answer in answers]",
			"",
		}},
		{name: "_answer_keyed", lines: []string{
			"",
			"def _answer_keyed[Answer](",
			"    answers: Mapping[str, Answer], project: Callable[[Answer], dict[str, Any]]",
			") -> dict[str, Any]:",
			`    """Project a map of nested answers."""`,
			"    return {key: project(answer) for key, answer in answers.items()}",
			"",
		}},
	}
}
