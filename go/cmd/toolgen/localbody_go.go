package main

import (
	"slices"
	"sort"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// The Go arm of the answer shapes: a value type per shape and the projection
// that turns one into the plain body its operation answers.
//
// The engine builds the value and the generator writes the mapping, which is
// the whole of what used to be hand-written twice. Nothing here knows a tool.

// answersPackage is the package both the engine and the handlers read the
// generated answers from. It sits apart from the tool tree because the engine
// builds these values and the handler tree already reads the engine.
const answersPackage = "genlocal"

// goAnswerShapeBudget is roughly the bytes one shape's type, constructor and
// projection run to, which is all a builder needs to size itself once.
const goAnswerShapeBudget = 1024

// answersFile is the one file the Go answers arm writes.
const answersFile = "answers" + generatedSuffix

// The width classes a member falls in, and what each is worth in bytes. The
// struct is ordered by these so the emitted tree passes the same alignment
// check hand-written code does.
const (
	widthList  = "list"
	widthText  = "text"
	widthBoxed = "boxed"
	widthWord  = "word"
	widthFlag  = "flag"
)

// goAnswerSizes is what one member of each width class costs.
func goAnswerSizes() map[string]int {
	return map[string]int{widthList: 24, widthText: 16, widthBoxed: 16, widthWord: 8, widthFlag: 1}
}

// goAnswerType is what one member is carried as, and the width class its
// ordering uses.
func goAnswerType(member *answerMember) (string, string) {
	element := goAnswerElement(member)

	// A nested answer is carried as a pointer wherever it sits, which is what
	// lets one constructor fill every position and what already carries the
	// absence a present member would otherwise need a second pointer for.
	if member.kind == answerNested {
		element = "*" + element
	}

	switch {
	case member.mapped:
		return "map[string]" + element, widthWord
	case member.list:
		return "[]" + element, widthList
	case member.present && member.kind != answerNested:
		return "*" + element, widthWord
	}

	return element, goAnswerWidth(element)
}

// goAnswerElement is the type one member's own values are carried as, before
// the list, map or presence wrapper the member puts around it.
func goAnswerElement(member *answerMember) string {
	switch member.kind {
	case answerString:
		return textType
	case answerBool:
		return typeWordBool
	case answerInt:
		return typeWordInt
	case answerInt64:
		return "int64"
	case answerUint64:
		return "uint64"
	case answerFree:
		return typeWordAny
	case answerNested:
		return member.shape
	}

	return typeWordAny
}

// goAnswerWidth is the ordering class one plain type falls in.
func goAnswerWidth(element string) string {
	widths := map[string]string{
		textType: widthText, typeWordBool: widthFlag, typeWordAny: widthBoxed,
	}

	width, known := widths[element]
	if !known {
		return widthWord
	}

	return width
}

// goAnswerMembers is one shape's members in the order the struct declares them:
// widest first, declaration order within a width, which is the ordering the
// alignment check asks for.
func goAnswerMembers(shape *answerShape) []answerMember {
	ordered := slices.Clone(shape.members)
	sizes := goAnswerSizes()

	sort.SliceStable(ordered, func(left, right int) bool {
		_, leftWidth := goAnswerType(&ordered[left])
		_, rightWidth := goAnswerType(&ordered[right])

		return sizes[leftWidth] > sizes[rightWidth]
	})

	return ordered
}

// renderGoAnswers writes the whole answers package: the projection helpers the
// shapes reach for, then a value type and a projection per shape.
func renderGoAnswers(
	shapes []answerShape, operations []localOperation,
) (emittedFile, error) {
	var out strings.Builder

	surface, raises, err := goOperationSurface(operations)
	if err != nil {
		return emittedFile{}, err
	}

	out.Grow(len(shapes) * goAnswerShapeBudget)
	out.WriteString(generatedHeader)
	out.WriteString("\n\n")
	out.WriteString(goAnswersDoc())
	out.WriteString("package " + answersPackage + "\n")

	helpers := goAnswerHelpers(shapes) + goRecordHelpers(shapes)
	out.WriteString(goAnswerImports(helpers, surface, raises, goOperationImports(operations)))
	out.WriteString(helpers)

	for index := range shapes {
		text, shapeErr := goAnswerShape(&shapes[index])
		if shapeErr != nil {
			return emittedFile{}, shapeErr
		}

		out.WriteString(text)
	}

	records, err := goRecordDirections(shapes)
	if err != nil {
		return emittedFile{}, err
	}

	out.WriteString(records)
	out.WriteString(surface)

	formatted, err := formatSource(out.String(), answersFile)
	if err != nil {
		return emittedFile{}, err
	}

	return emittedFile{Name: answersFile, Text: formatted}, nil
}

// goAnswerImportBudget is roughly the bytes one import line runs to, which is
// all the block needs to size itself once.
const goAnswerImportBudget = 64

// goAnswerImports is the answers file's own import block: the copy helper where
// a keyed member reaches for it, the error package where a condition is declared
// as a value, the formatter where a verdict sentence names an entry's own value,
// and whatever the subsystem types name in their signatures.
//
// The two helper packages are read off the text that was already written rather
// than tracked beside it, so a helper that stops using one cannot leave its
// import behind.
func goAnswerImports(helpers, surface string, raises bool, subsystem []string) string {
	paths := append(make([]string, 0, len(subsystem)+3), subsystem...)

	if raises {
		paths = append(paths, importErrors)
	}

	if strings.Contains(surface, "fmt.Sprintf(") {
		paths = append(paths, "fmt")
	}

	if strings.Contains(helpers, "maps.Copy") {
		paths = append(paths, "maps")
	}

	for _, reached := range []string{"bytes.", "json.", "strconv."} {
		if strings.Contains(helpers, reached) {
			paths = append(paths, goRecordImport(reached))
		}
	}

	if len(paths) == 0 {
		return ""
	}

	if len(paths) == 1 {
		return "\nimport " + goStringLiteral(paths[0]) + "\n"
	}

	sort.Strings(paths)

	var out strings.Builder

	out.Grow(len(paths) * goAnswerImportBudget)
	out.WriteString("\nimport (\n")
	writeImportBlock(&out, paths)
	out.WriteString(")\n")

	return out.String()
}

// goAnswersDoc is the package comment, which says where the types come from and
// why nothing here is written by hand.
func goAnswersDoc() string {
	return "// Package genlocal holds the answer a local operation fills and the\n" +
		"// projection that turns one into the plain body its tool answers. Both are\n" +
		"// emitted from the response the contract declares, top level and nested, so a\n" +
		"// member reaches every language or none. Nothing here is hand-written and\n" +
		"// nothing here survives a `make proto`: change the contract, not this tree.\n"
}

// goAnswerShape writes one shape's value type and its projection.
func goAnswerShape(shape *answerShape) (string, error) {
	goType, err := lookupGoMessage(protoreflect.FullName(shape.full))
	if err != nil {
		return "", err
	}

	var out strings.Builder

	out.Grow(goAnswerShapeBudget)
	out.WriteString("\n// ")
	out.WriteString(shape.name)
	out.WriteString(" is the answer ")
	out.WriteString(shape.full)
	out.WriteString(" declares.\n")
	out.WriteString("type ")
	out.WriteString(shape.name)
	out.WriteString(" struct {\n")

	for _, member := range goAnswerMembers(shape) {
		field, _ := goAnswerType(&member)

		out.WriteString("\t")
		out.WriteString(goType.goNameOf(member.name))
		out.WriteString(" ")
		out.WriteString(field)
		out.WriteString("\n")
	}

	out.WriteString("}\n")
	out.WriteString(goAnswerConstructor(shape, &goType))
	out.WriteString(goAnswerProjection(shape, &goType))

	return out.String(), nil
}

// goAnswerConstructor writes the one way a shape is built: every member filled
// positionally, in the message's own order.
//
// This is what makes a member added to the message a build failure rather than
// a zero nobody computed. A keyed literal would compile with the new member
// left at its zero, which is exactly the nested divergence the projection is
// here to close.
func goAnswerConstructor(shape *answerShape, goType *goMessage) string {
	var out strings.Builder

	out.Grow(goAnswerShapeBudget)
	out.WriteString("\n// New")
	out.WriteString(shape.name)
	out.WriteString(" is one ")
	out.WriteString(shape.name)
	out.WriteString(" with every member it declares filled.\n")
	out.WriteString("func New")
	out.WriteString(shape.name)
	out.WriteString("(")
	out.WriteString(goAnswerParameters(shape))
	out.WriteString(") *")
	out.WriteString(shape.name)
	out.WriteString(" {\n\treturn &")
	out.WriteString(shape.name)
	out.WriteString("{\n")

	for _, member := range shape.members {
		out.WriteString("\t\t")
		out.WriteString(goType.goNameOf(member.name))
		out.WriteString(": ")
		out.WriteString(goAnswerParameter(member.name))
		out.WriteString(",\n")
	}

	out.WriteString("\t}\n}\n")

	return out.String()
}

// goAnswerParameters is the constructor's parameter list, adjacent members of
// one type sharing that type the way the formatter's own rule writes them.
func goAnswerParameters(shape *answerShape) string {
	parts := make([]string, 0, len(shape.members))
	names := make([]string, 0, len(shape.members))

	var carried string

	for _, member := range shape.members {
		field, _ := goAnswerType(&member)
		if field != carried && len(names) > 0 {
			parts = append(parts, strings.Join(names, ", ")+" "+carried)
			names = names[:0]
		}

		carried = field

		names = append(names, goAnswerParameter(member.name))
	}

	if len(names) > 0 {
		parts = append(parts, strings.Join(names, ", ")+" "+carried)
	}

	return strings.Join(parts, ", ")
}

// goAnswerParameter is the local one member arrives under, kept clear of the
// words Go already gives a meaning: ProfileFieldDiff declares a member named
// new, which is a builtin the linter refuses as a parameter.
func goAnswerParameter(member string) string {
	local := goLocalName(member)
	if slices.Contains(goPredeclared(), local) {
		return local + "Value"
	}

	return local
}

// goPredeclared is the identifiers Go declares in the universe block, which a
// parameter may not take without shadowing one.
func goPredeclared() []string {
	return []string{
		typeWordAny, "append", typeWordBool, "byte", "cap", "clear", "close",
		"complex", "copy", "delete", "error", "false", "imag", typeWordInt,
		"len", "make", "max", "min", "new", "nil", "panic", "print", "println",
		"real", "recover", "rune", textType, "true",
	}
}

// goAnswerProjection writes the function one shape's plain body comes out of,
// member for member in the message's own declaration order.
func goAnswerProjection(shape *answerShape, goType *goMessage) string {
	project := goAnswerProject(shape.name)

	var out strings.Builder

	out.Grow(goAnswerShapeBudget)
	out.WriteString("\n// ")
	out.WriteString(project)
	out.WriteString(" is the plain body one ")
	out.WriteString(shape.name)
	out.WriteString(" fills.\n")
	out.WriteString("func ")
	out.WriteString(project)
	out.WriteString("(answer *")
	out.WriteString(shape.name)
	out.WriteString(") map[string]any {\n")
	out.WriteString("\treturn map[string]any{\n")

	for _, member := range shape.members {
		read := goAnswerRead(&member, "answer."+goType.goNameOf(member.name))

		out.WriteString("\t\t")
		out.WriteString(goStringLiteral(member.name))
		out.WriteString(": ")
		out.WriteString(read)
		out.WriteString(",\n")
	}

	out.WriteString("\t}\n}\n")

	return out.String()
}

// goAnswerProject is the projection's own name for one shape.
func goAnswerProject(shape string) string {
	return "Project" + shape
}

// goAnswerRead is the expression one member's body value comes from.
func goAnswerRead(member *answerMember, read string) string {
	if member.kind == answerNested {
		return goAnswerNestedRead(member, read)
	}

	switch {
	case member.mapped:
		return "answerEntries(" + read + ")"
	case member.list:
		return "answerValues(" + read + ")"
	case member.present:
		return "answerOptional(" + read + ")"
	}

	return read
}

// goAnswerNestedRead is the expression one shape-carrying member comes from.
func goAnswerNestedRead(member *answerMember, read string) string {
	project := goAnswerProject(member.shape)

	switch {
	case member.mapped:
		return "answerKeyed(" + read + ", " + project + ")"
	case member.list:
		return "answerMany(" + read + ", " + project + ")"
	}

	return "answerOne(" + read + ", " + project + ")"
}

// goAnswerHelper is one projection helper: the name a member reads through and
// the source emitted when some member does.
type goAnswerHelper struct {
	name string
	text string
}

// goAnswerHelpers is the source for every helper the shapes reach, in a settled
// order. Only the ones some member reads are written, since a helper nothing
// calls is dead code the linter would fail the tree on.
func goAnswerHelpers(shapes []answerShape) string {
	var out strings.Builder

	out.Grow(len(goAnswerHelperSet()) * goAnswerShapeBudget)

	wanted := goAnswerHelpersUsed(shapes)

	for _, helper := range goAnswerHelperSet() {
		if slices.Contains(wanted, helper.name) {
			out.WriteString(helper.text)
		}
	}

	return out.String()
}

// goAnswerHelpersUsed is the helper every member of every shape reads through.
func goAnswerHelpersUsed(shapes []answerShape) []string {
	used := make([]string, 0, len(shapes))

	for index := range shapes {
		for _, member := range shapes[index].members {
			read, _, _ := strings.Cut(goAnswerRead(&member, ""), "(")
			for _, name := range goAnswerHelperNeeds(read) {
				if !slices.Contains(used, name) {
					used = append(used, name)
				}
			}
		}
	}

	return used
}

// goAnswerHelperNeeds is the helpers one read pulls in: itself, and the nil
// rule the two nested collections project each element through.
func goAnswerHelperNeeds(read string) []string {
	switch read {
	case "":
		return nil
	case "answerMany", "answerKeyed":
		return []string{read, "answerOne"}
	}

	return []string{read}
}

// goAnswerHelperSet is every helper the projections can reach, in emitted
// order.
func goAnswerHelperSet() []goAnswerHelper {
	return []goAnswerHelper{
		{name: "answerValues", text: `
// answerValues copies a list so an empty one projects as an empty array rather
// than as null, which is the array-over-null contract every answer keeps.
func answerValues[T any](values []T) []T {
	return append([]T{}, values...)
}
`},
		{name: "answerEntries", text: `
// answerEntries copies a map of plain values, empty rather than null for the
// reason answerValues is.
func answerEntries[V any](values map[string]V) map[string]V {
	copied := make(map[string]V, len(values))
	maps.Copy(copied, values)

	return copied
}
`},
		{name: "answerOptional", text: `
// answerOptional is one member that carries its own absence, nil where the
// answer carried none, which projects as a member the message leaves unset.
func answerOptional[T any](value *T) any {
	if value == nil {
		return nil
	}

	return *value
}
`},
		{name: "answerOne", text: `
// answerOne projects one nested answer, nil where the answer carried none.
func answerOne[T any](answer *T, project func(*T) map[string]any) any {
	if answer == nil {
		return nil
	}

	return project(answer)
}
`},
		{name: "answerMany", text: `
// answerMany projects a list of nested answers, empty rather than null for the
// reason answerValues is.
func answerMany[T any](answers []*T, project func(*T) map[string]any) []any {
	projected := make([]any, len(answers))
	for index, answer := range answers {
		projected[index] = answerOne(answer, project)
	}

	return projected
}
`},
		{name: "answerKeyed", text: `
// answerKeyed projects a map of nested answers, empty rather than null for the
// reason answerValues is.
func answerKeyed[T any](answers map[string]*T, project func(*T) map[string]any) map[string]any {
	projected := make(map[string]any, len(answers))
	for key, answer := range answers {
		projected[key] = answerOne(answer, project)
	}

	return projected
}
`},
	}
}
