package main

import "strings"

// An option value nests brackets inside the field's own list, so depth is what
// tells an entry separator from a comma inside a value.
const (
	openBrackets  = "[{("
	closeBrackets = "]})"
)

// The width of the " [" a bracket list opens with.
const bracketOpen = 2

// parseTail splits everything after a declaration's number. rest followed the
// number on the head line; more is the rest of the statement.
func parseTail(rest string, more []string) (fieldTail, string, error) {
	if after, found := strings.CutPrefix(rest, ";"); found && len(more) == 0 {
		return fieldTail{Trailing: after}, "", nil
	}

	if !strings.HasPrefix(rest, " [") {
		return fieldTail{}, "", errUnparsedTail
	}

	if tail, location, ok := parseBlockTail(rest, more); ok {
		return tail, location, nil
	}

	return parseSpanTail(rest, more)
}

// parseBlockTail reads the one-entry-per-line form, answering false for any
// other layout, which parseSpanTail then takes.
func parseBlockTail(rest string, more []string) (fieldTail, string, bool) {
	if rest != " [" || len(more) == 0 {
		return fieldTail{}, "", false
	}

	last := more[len(more)-1]
	closeIndent := leadingSpace(last)

	trailing, found := strings.CutPrefix(strings.TrimPrefix(last, closeIndent), "];")
	if !found {
		return fieldTail{}, "", false
	}

	entries, location, err := blockEntries(more[:len(more)-1])
	if err != nil {
		return fieldTail{}, "", false
	}

	tail := fieldTail{Entries: entries, Trailing: trailing, CloseIndent: closeIndent, Multiline: true}

	return tail, location, true
}

// parseSpanTail reads a bracket list written as one span, cutting entries at
// the commas bracket depth leaves at the top level.
func parseSpanTail(rest string, more []string) (fieldTail, string, error) {
	text := strings.Join(append([]string{rest}, more...), "\n")

	endIdx, commas, ok := bracketSpan(text, 1)
	if !ok {
		return fieldTail{}, "", errUnparsedTail
	}

	after, found := strings.CutPrefix(text[endIdx+1:], ";")
	if !found {
		return fieldTail{}, "", errUnparsedTail
	}

	entries, location := spanEntries(text[bracketOpen:endIdx], shiftOffsets(commas, bracketOpen))

	return fieldTail{Entries: entries, Trailing: after}, location, nil
}

// spanEntries cuts a bracket body at the commas that separate its entries.
func spanEntries(body string, commas []int) ([]tailEntry, string) {
	entries := make([]tailEntry, 0, len(commas)+1)

	var (
		location string
		start    int
	)

	for _, cut := range append(commas, len(body)) {
		entry, found := readEntry(strings.TrimSpace(body[start:cut]))
		if found != "" {
			location = found
		}

		entries = append(entries, entry)
		start = min(cut+1, len(body))
	}

	return entries, location
}

// blockEntries groups the lines between the brackets. A comment above an entry
// belongs to it and travels verbatim.
func blockEntries(lines []string) ([]tailEntry, string, error) {
	var entries []tailEntry

	var (
		lead, buf []string
		location  string
		depth     int
	)

	for _, line := range lines {
		if len(buf) == 0 && depth == 0 && strings.HasPrefix(strings.TrimSpace(line), "//") {
			lead = append(lead, line)

			continue
		}

		code := codeOf(line)
		buf = append(buf, line)
		depth += bracketDepth(code)

		if depth != 0 || !strings.HasSuffix(strings.TrimRight(code, " \t"), ",") {
			continue
		}

		entry, found := readBlockEntry(lead, buf)
		if found != "" {
			location = found
		}

		entries = append(entries, entry)
		lead, buf = nil, nil
	}

	if len(buf) == 0 {
		return nil, "", errUnparsedTail
	}

	entry, found := readBlockEntry(lead, buf)
	if found != "" {
		location = found
	}

	return append(entries, entry), location, nil
}

// readBlockEntry turns one group of lines into an entry, dropping the comma
// that separated it from the next one.
func readBlockEntry(lead, buf []string) (tailEntry, string) {
	indent := leadingSpace(buf[0])

	text := strings.TrimPrefix(strings.Join(buf, "\n"), indent)
	if stripped, found := strings.CutSuffix(strings.TrimRight(text, " \t"), ","); found {
		text = stripped
	}

	entry, location := readEntry(text)
	entry.Lead = lead
	entry.Indent = indent

	return entry, location
}

// readEntry answers the entry and, when it is the request position, the member
// the surface carries instead of the text.
func readEntry(text string) (tailEntry, string) {
	if match := locationPattern.FindStringSubmatch(text); match != nil {
		return tailEntry{Location: true}, match[1]
	}

	return tailEntry{Text: text}, ""
}

// bracketSpan answers where the group closes and where its entry commas sit.
// String literals are tracked, so nothing inside one counts toward either.
func bracketSpan(text string, open int) (int, []int, bool) {
	var (
		commas   []int
		depth    int
		inString bool
		escaped  bool
	)

	for idx := open; idx < len(text); idx++ {
		char := text[idx]

		if escaped {
			escaped = false

			continue
		}

		if inString {
			escaped = char == '\\'
			inString = char != '"'

			continue
		}

		if char == '"' {
			inString = true

			continue
		}

		if char == ',' && depth == 1 {
			commas = append(commas, idx)

			continue
		}

		if strings.IndexByte(openBrackets, char) >= 0 {
			depth++

			continue
		}

		if strings.IndexByte(closeBrackets, char) < 0 {
			continue
		}

		depth--

		if depth == 0 {
			return idx, commas, true
		}
	}

	return 0, nil, false
}

// bracketDepth answers how far one line of code opens or closes.
func bracketDepth(code string) int {
	opened := strings.Count(code, "[") + strings.Count(code, "{") + strings.Count(code, "(")
	closed := strings.Count(code, "]") + strings.Count(code, "}") + strings.Count(code, ")")

	return opened - closed
}

// shiftOffsets rebases comma offsets onto the body slice they will index into.
func shiftOffsets(offsets []int, base int) []int {
	shifted := make([]int, 0, len(offsets))
	for _, offset := range offsets {
		shifted = append(shifted, offset-base)
	}

	return shifted
}
