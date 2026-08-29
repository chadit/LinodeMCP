package main

import (
	"slices"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// The Go arm of a record: the bytes one is written as and the value one is read
// back into, both walking the member list the projection already walks.

// goRecordWrite is the emitted name a record is written through.
func goRecordWrite(shape string) string {
	return "Record" + shape
}

// goRecordRead is the emitted name a record is read back through.
func goRecordRead(shape string) string {
	return shape + "FromRecord"
}

// goRecordReader is the helper one member is read back through. A record
// carries no nested shape, which checkAnswerRecords refuses, so every member
// here is a value the reader set already spells.
func goRecordReader(member *answerMember) string {
	switch {
	case member.mapped:
		return "recordEntries"
	case member.list:
		return "recordTexts"
	case member.present:
		return "recordOptionalText"
	}

	switch member.kind {
	case answerBool:
		return "recordFlag"
	case answerInt:
		return "recordCount"
	case answerInt64:
		return "recordWhole"
	case answerUint64:
		return "recordUnsigned"
	case answerString, answerFree, answerNested:
	}

	return "recordText"
}

// goRecordDirections is the two directions every record shape carries, written
// after the projections so a reader meets the value type first.
func goRecordDirections(shapes []answerShape) (string, error) {
	records := recordShapes(shapes)

	var out strings.Builder

	out.Grow(len(records) * goAnswerShapeBudget)

	for _, shape := range records {
		goType, err := lookupGoMessage(protoreflect.FullName(shape.full))
		if err != nil {
			return "", err
		}

		out.WriteString(goRecordWriter(shape, &goType))
		out.WriteString(goRecordDecoder(shape))
	}

	return out.String(), nil
}

// goRecordWriter writes the direction a record leaves the engine through.
func goRecordWriter(shape *answerShape, goType *goMessage) string {
	write := goRecordWrite(shape.name)

	var out strings.Builder

	out.Grow(goAnswerShapeBudget)
	out.WriteString("\n// ")
	out.WriteString(write)
	out.WriteString(" is one ")
	out.WriteString(shape.name)
	out.WriteString(" as the canonical bytes a record is\n")
	out.WriteString("// written as: the members ")
	out.WriteString(shape.full)
	out.WriteString(" declares, in its own\n// order, which is the order the format carries.\n")
	out.WriteString("func ")
	out.WriteString(write)
	out.WriteString("(answer *")
	out.WriteString(shape.name)
	out.WriteString(", prefix, indent string) ([]byte, error) {\n")
	out.WriteString("\treturn recordJSON(prefix, indent, []recordMember{\n")

	for _, member := range shape.members {
		read := goAnswerRead(&member, "answer."+goType.goNameOf(member.name))

		out.WriteString("\t\t{name: ")
		out.WriteString(goStringLiteral(member.name))
		out.WriteString(", value: ")
		out.WriteString(read)
		out.WriteString("},\n")
	}

	out.WriteString("\t})\n}\n")

	return out.String()
}

// goRecordDecoder writes the direction a record comes back in through.
//
// It fills the shape's own constructor, so a member added to the message stops
// the build here as well as at every other place one is built.
func goRecordDecoder(shape *answerShape) string {
	read := goRecordRead(shape.name)

	var out strings.Builder

	out.Grow(goAnswerShapeBudget)
	out.WriteString("\n// ")
	out.WriteString(read)
	out.WriteString(" is one ")
	out.WriteString(shape.name)
	out.WriteString(" read back off the bytes ")
	out.WriteString(goRecordWrite(shape.name))
	out.WriteString("\n// wrote.\n//\n")
	out.WriteString("// A member the record leaves out reads as its own zero rather than refusing,\n")
	out.WriteString("// because a record an EARLIER version wrote is the case this exists for.\n")
	out.WriteString("func ")
	out.WriteString(read)
	out.WriteString("(line []byte) (*")
	out.WriteString(shape.name)
	out.WriteString(", error) {\n")
	out.WriteString("\trecord, err := recordFields(line)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\n")
	out.WriteString("\treturn New")
	out.WriteString(shape.name)
	out.WriteString("(\n")

	for index := range shape.members {
		member := &shape.members[index]

		out.WriteString("\t\t")
		out.WriteString(goRecordReader(member))
		out.WriteString("(record[")
		out.WriteString(goStringLiteral(member.name))
		out.WriteString("]),\n")
	}

	out.WriteString("\t), nil\n}\n")

	return out.String()
}

// goRecordImport is the package one helper reaches through its own name, since
// the encoder's package and the path it is imported from differ by one word.
func goRecordImport(reached string) string {
	if reached == "json." {
		return "encoding/json"
	}

	return strings.TrimSuffix(reached, ".")
}

// goRecordHelpers is the source for every record helper the shapes reach, in a
// settled order.
func goRecordHelpers(shapes []answerShape) string {
	wanted := recordHelpersUsed(shapes, goRecordReader)
	if len(wanted) == 0 {
		return ""
	}

	var out strings.Builder

	out.Grow(len(wanted) * goAnswerShapeBudget)
	out.WriteString(goRecordMemberType())

	for _, helper := range goRecordHelperSet() {
		if slices.Contains(wanted, helper.name) {
			out.WriteString(helper.text)
		}
	}

	return out.String()
}

// goRecordMemberType is the pair a written record walks, emitted whenever any
// record is.
func goRecordMemberType() string {
	return `
// recordMember is one member of a record as it is written: the key the on-disk
// line carries and the value under it.
type recordMember struct {
	value any
	name  string
}

// recordBudget is roughly the bytes one written record runs to, which is all a
// builder needs to size itself once.
const recordBudget = 1024
`
}

// goRecordHelperSet is every record helper the two directions can reach, in
// emitted order.
func goRecordHelperSet() []goAnswerHelper {
	return append(goRecordWriterSet(), goRecordReaderSet()...)
}

// goRecordWriterSet is the helpers any record at all reaches: the member walk,
// the per-member encoder, and the decode the reader opens with.
func goRecordWriterSet() []goAnswerHelper {
	return []goAnswerHelper{
		{name: recordHelperJSON, text: `
// recordJSON is a record as the canonical bytes it is written and read as: its
// members in the order the contract declares them, indented under prefix where
// indent asks for indentation at all.
//
// A member name is written between plain quotes because a proto member name
// carries no character JSON would escape, which is what lets every language
// spell the key the same way without agreeing on a string encoder.
func recordJSON(prefix, indent string, members []recordMember) ([]byte, error) {
	inner := prefix + indent
	gap, lead, tail := "", "", ""

	if indent != "" {
		gap, lead, tail = " ", "\n"+inner, "\n"+prefix
	}

	out := make([]byte, 0, recordBudget)
	out = append(out, '{')

	for index := range members {
		if index > 0 {
			out = append(out, ',')
		}

		out = append(out, lead...)
		out = append(out, '"')
		out = append(out, members[index].name...)
		out = append(out, '"', ':')
		out = append(out, gap...)

		value, err := RecordValue(members[index].value, inner, indent)
		if err != nil {
			return nil, err
		}

		out = append(out, value...)
	}

	out = append(out, tail...)

	return append(out, '}'), nil
}
`},
		{name: recordHelperValue, text: `
// RecordValue is one member's own JSON, which is also what a caller writing a
// record's value into a cell of its own has to spell it as.
//
// HTML stands for itself because no reader of a record is a browser, and the
// encoder's trailing newline belongs to the stream rather than to the value.
func RecordValue(value any, prefix, indent string) ([]byte, error) {
	var out bytes.Buffer

	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent(prefix, indent)

	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("record member: %w", err)
	}

	return bytes.TrimRight(out.Bytes(), "\n"), nil
}
`},
		{name: recordHelperFields, text: `
// recordFields is one written record's members.
//
// Numbers stay as their own text, because a 64-bit count does not survive the
// double a plain decode would widen it to and the audit log carries one.
func recordFields(line []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()

	var record map[string]any

	if err := decoder.Decode(&record); err != nil {
		return nil, fmt.Errorf("record: %w", err)
	}

	return record, nil
}
`},
	}
}

// goRecordReaderSet is one reader per member kind a record can carry.
func goRecordReaderSet() []goAnswerHelper {
	return []goAnswerHelper{
		{name: "recordText", text: `
// recordText is one text member, empty where the record wrote something else.
func recordText(value any) string {
	text, _ := value.(string)

	return text
}
`},
		{name: "recordTexts", text: `
// recordTexts is one list-of-text member, empty rather than nil so a record
// that carried none reads back the way one that carried an empty list does.
func recordTexts(value any) []string {
	held, _ := value.([]any)
	texts := make([]string, 0, len(held))

	for _, item := range held {
		texts = append(texts, recordText(item))
	}

	return texts
}
`},
		{name: "recordOptionalText", text: `
// recordOptionalText is one text member carrying its own absence, nil where the
// record wrote null.
func recordOptionalText(value any) *string {
	text, held := value.(string)
	if !held {
		return nil
	}

	return &text
}
`},
		{name: "recordEntries", text: `
// recordEntries is one keyed member, its own values under its own keys.
func recordEntries(value any) map[string]any {
	held, _ := value.(map[string]any)

	return held
}
`},
		{name: "recordFlag", text: `
// recordFlag is one flag member.
func recordFlag(value any) bool {
	flag, _ := value.(bool)

	return flag
}
`},
		{name: "recordCount", text: `
// recordCount is one 32-bit member, read through the text a wider one is.
func recordCount(value any) int {
	return int(recordWhole(value))
}
`},
		{name: "recordWhole", text: `
// recordWhole is one 64-bit member, read off the number's own text.
func recordWhole(value any) int64 {
	number, held := value.(json.Number)
	if !held {
		return 0
	}

	whole, _ := number.Int64()

	return whole
}
`},
		{name: "recordUnsigned", text: `
// recordUnsigned is one unsigned 64-bit member, which a signed read would wrap
// at half the range the counter can reach.
func recordUnsigned(value any) uint64 {
	number, held := value.(json.Number)
	if !held {
		return 0
	}

	counter, _ := strconv.ParseUint(number.String(), 10, 64)

	return counter
}
`},
	}
}
