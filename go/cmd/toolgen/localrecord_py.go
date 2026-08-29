package main

import "slices"

// The Python arm of a record: the text one is written as and the value one is
// read back into, both walking the member list the projection already walks.

// pyRecordWrite is the emitted name a record is written through.
func pyRecordWrite(shape string) string {
	return "record_" + pySnake(shape)
}

// pyRecordRead is the emitted name a record is read back through.
func pyRecordRead(shape string) string {
	return pySnake(shape) + "_from_record"
}

// pyRecordReader is the helper one member is read back through. Python carries
// one whole-number type, so the three widths Go reads apart arrive here as one.
func pyRecordReader(member *answerMember) string {
	switch {
	case member.mapped:
		return "_record_entries"
	case member.list:
		return "_record_texts"
	case member.present:
		return "_record_optional_text"
	}

	switch member.kind {
	case answerBool:
		return "_record_flag"
	case answerInt, answerInt64, answerUint64:
		return "_record_whole"
	case answerString, answerFree, answerNested:
	}

	return "_record_text"
}

// pyRecordDirections is the two directions every record shape carries, written
// after the projections so a reader meets the dataclass first.
func pyRecordDirections(shapes []answerShape) []string {
	lines := make([]string, 0, len(shapes)*shapeLineBudget)

	for _, shape := range recordShapes(shapes) {
		lines = append(lines, pyRecordWriter(shape)...)
		lines = append(lines, pyRecordDecoder(shape)...)
	}

	return lines
}

// pyRecordWriter writes the direction a record leaves the engine through.
func pyRecordWriter(shape *answerShape) []string {
	lines := []string{
		"",
		"",
		"def " + pyRecordWrite(shape.name) + "(",
		"    answer: " + shape.name + ", prefix: str, indent: str",
		") -> str:",
		`    """One ` + shape.name + ` as the canonical text a record is written as.`,
		"",
		"    The members " + shape.full + " declares, in its own order,",
		"    which is the order the format carries.",
		pyDocClose,
		"    return _record_json(",
		"        prefix,",
		"        indent,",
		"        [",
	}

	for index := range shape.members {
		member := &shape.members[index]
		read := pyAnswerRead(member, "answer."+member.name)
		lines = append(lines, "            ("+pyQuote(member.name)+", "+read+"),")
	}

	return append(lines, "        ],", "    )")
}

// pyRecordDecoder writes the direction a record comes back in through.
func pyRecordDecoder(shape *answerShape) []string {
	lines := []string{
		"",
		"",
		"def " + pyRecordRead(shape.name) + "(line: str) -> " + shape.name + ":",
		`    """One ` + shape.name + ` read back off the text ` +
			pyRecordWrite(shape.name) + ` wrote.`,
		"",
		"    A member the record leaves out reads as its own zero rather than",
		"    refusing, because a record an EARLIER version wrote is the case this",
		"    exists for.",
		pyDocClose,
		"    record = _record_fields(line)",
		"",
		"    return " + shape.name + "(",
	}

	for index := range shape.members {
		member := &shape.members[index]
		lines = append(lines, "        "+member.name+"="+pyRecordReader(member)+
			"(record.get("+pyQuote(member.name)+")),")
	}

	return append(lines, "    )")
}

// pyRecordHelpers is the source for every record helper the shapes reach, in a
// settled order.
func pyRecordHelpers(shapes []answerShape) []string {
	wanted := recordHelpersUsed(shapes, pyRecordReader)
	if len(wanted) == 0 {
		return nil
	}

	lines := make([]string, 0, len(wanted)*shapeLineBudget)

	for _, helper := range pyRecordHelperSet() {
		if slices.Contains(wanted, helper.name) {
			lines = append(lines, helper.lines...)
		}
	}

	return lines
}

// pyRecordHelperSet is every record helper the two directions can reach, in
// emitted order.
func pyRecordHelperSet() []pyAnswerHelper {
	return append(pyRecordWriterSet(), pyRecordReaderSet()...)
}

// pyRecordWriterSet is the helpers any record at all reaches: the member walk,
// the per-member encoder, and the decode the reader opens with.
func pyRecordWriterSet() []pyAnswerHelper {
	return []pyAnswerHelper{
		{name: recordHelperJSON, lines: []string{
			"",
			"def _record_json(",
			"    prefix: str, indent: str, members: list[tuple[str, Any]]",
			") -> str:",
			`    """A record as the canonical text it is written and read as.`,
			"",
			"    Its members come out in the order the contract declares them,",
			"    indented under prefix where indent asks for indentation at all. A",
			"    member name is written between plain quotes because a proto member",
			"    name carries no character JSON would escape, which is what lets every",
			"    language spell the key the same way without agreeing on a string",
			"    encoder.",
			pyDocClose,
			"    inner = prefix + indent",
			`    gap, lead, tail = "", "", ""`,
			"",
			"    if indent:",
			`        gap, lead, tail = " ", "\n" + inner, "\n" + prefix`,
			"",
			"    written = [",
			`        ("," if index else "")`,
			"        + lead",
			`        + '"'`,
			"        + name",
			`        + '":'`,
			"        + gap",
			"        + record_value(value, inner, indent)",
			"        for index, (name, value) in enumerate(members)",
			"    ]",
			"",
			`    return "{" + "".join(written) + tail + "}"`,
			"",
		}},
		{name: recordHelperValue, lines: []string{
			"",
			"def record_value(value: Any, prefix: str, indent: str) -> str:",
			`    """One member's own JSON, which is also what a caller writing a`,
			"    record's value into a cell of its own has to spell it as.",
			"",
			"    Non-ASCII stands for itself and a keyed value's keys come out sorted,",
			"    which is what a record written by two languages has to agree on.",
			pyDocClose,
			"    if not indent:",
			"        return json.dumps(",
			`            value, ensure_ascii=False, sort_keys=True, separators=(",", ":")`,
			"        )",
			"",
			"    return json.dumps(",
			"        value, ensure_ascii=False, sort_keys=True, indent=indent",
			`    ).replace("\n", "\n" + prefix)`,
			"",
		}},
		{name: recordHelperFields, lines: []string{
			"",
			"def _record_fields(line: str) -> dict[str, Any]:",
			`    """One written record's members.`,
			"",
			"    A line that is not a record at all raises, which is the reader's own",
			"    signal to skip it.",
			pyDocClose,
			"    record = json.loads(line)",
			"",
			"    if not isinstance(record, dict):",
			`        msg = "record is not an object"`,
			"        raise TypeError(msg)",
			"",
			`    return cast("dict[str, Any]", record)`,
			"",
		}},
	}
}

// pyRecordReaderSet is one reader per member kind a record can carry.
func pyRecordReaderSet() []pyAnswerHelper {
	return []pyAnswerHelper{
		{name: "_record_text", lines: []string{
			"",
			"def _record_text(value: Any) -> str:",
			`    """One text member, empty where the record wrote something else."""`,
			"    return value if isinstance(value, str) else \"\"",
			"",
		}},
		{name: "_record_texts", lines: []string{
			"",
			"def _record_texts(value: Any) -> list[str]:",
			`    """One list-of-text member, empty where the record carried none."""`,
			"    if not isinstance(value, list):",
			"        return []",
			"",
			`    return [_record_text(item) for item in cast("list[object]", value)]`,
			"",
		}},
		{name: "_record_optional_text", lines: []string{
			"",
			"def _record_optional_text(value: Any) -> str | None:",
			`    """One text member carrying its own absence, None where it wrote null."""`,
			"    return value if isinstance(value, str) else None",
			"",
		}},
		{name: "_record_entries", lines: []string{
			"",
			"def _record_entries(value: Any) -> dict[str, Any]:",
			`    """One keyed member, its own values under its own keys."""`,
			"    if not isinstance(value, dict):",
			"        return {}",
			"",
			`    return cast("dict[str, Any]", value)`,
			"",
		}},
		{name: "_record_flag", lines: []string{
			"",
			"def _record_flag(value: Any) -> bool:",
			`    """One flag member."""`,
			"    return value if isinstance(value, bool) else False",
			"",
		}},
		{name: "_record_whole", lines: []string{
			"",
			"def _record_whole(value: Any) -> int:",
			`    """One whole-number member.`,
			"",
			"    A flag is a whole number in this language and is not one in the",
			"    contract, so it is refused here rather than counted as zero or one.",
			pyDocClose,
			"    if isinstance(value, bool) or not isinstance(value, int):",
			"        return 0",
			"",
			"    return value",
			"",
		}},
	}
}
