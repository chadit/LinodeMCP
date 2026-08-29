package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// The tree is buf-formatted, so these match the one spelling it produces. A
	// declaration written any other way fails to parse rather than copying through.
	blockOpenPattern  = regexp.MustCompile(`^(\s*)(message|enum|oneof|extend) (\S+) \{$`)
	fieldPattern      = regexp.MustCompile(`^(\s*)(?:(optional|repeated|required) )?(map<[^>]*>|[A-Za-z_][\w.]*) ([A-Za-z_]\w*) = (\d+)(.*)$`)
	enumValuePattern  = regexp.MustCompile(`^(\s*)([A-Za-z_]\w*) = (\d+)(.*)$`)
	locationPattern   = regexp.MustCompile(`^\(linode\.mcp\.v1\.field_location\) = (FIELD_LOCATION_\w+)$`)
	stringPattern     = regexp.MustCompile(`"(\\.|[^"\\])*"`)
	routeOpenPattern  = regexp.MustCompile(`^(\s*)option \(linode\.mcp\.v1\.tool_route\) = \{$`)
	routeEntryPattern = regexp.MustCompile(`^(\s*)(tool|method|path): ("(?:\\.|[^"\\])*")$`)
)

// block is one open scope; the stack gives a declaration its path and enum-ness.
type block struct {
	Kind string
	Name string
}

// parser walks one file's lines and splits them into the two halves.
type parser struct {
	file  *overlayFile
	surf  *surface
	lines []string
	raw   []string
	stack []block
}

// extract decomposes one proto file into its surface facts and the overlay that
// puts it back. surf is shared across the tree, as a descriptor set would be.
func extract(path, content string, surf *surface) (*overlayFile, error) {
	lines := strings.Split(content, "\n")

	final := strings.HasSuffix(content, "\n")
	if final {
		lines = lines[:len(lines)-1]
	}

	par := &parser{
		file:  &overlayFile{Path: path, FinalNewline: final},
		surf:  surf,
		lines: lines,
	}

	if err := par.run(); err != nil {
		return nil, err
	}

	return par.file, nil
}

func (p *parser) run() error {
	var idx int

	for idx < len(p.lines) {
		next, err := p.step(idx)
		if err != nil {
			return err
		}

		idx = next
	}

	p.flushRaw()

	if len(p.stack) > 0 {
		return fmt.Errorf("%w: %s: %s", errUnclosedBlock, p.file.Path, p.stack[len(p.stack)-1].Name)
	}

	return nil
}

// step consumes one line or one whole statement and answers the next index.
func (p *parser) step(idx int) (int, error) {
	line := p.lines[idx]
	trimmed := strings.TrimSpace(line)

	if trimmed == "" || strings.HasPrefix(trimmed, "//") {
		p.raw = append(p.raw, line)

		return idx + 1, nil
	}

	if open := blockOpenPattern.FindStringSubmatch(line); open != nil {
		p.flushRaw()
		p.file.Elements = append(p.file.Elements, blockOpen{Indent: open[1], Kind: open[2], Name: open[3]})
		p.stack = append(p.stack, block{Kind: open[2], Name: open[3]})

		return idx + 1, nil
	}

	if trimmed == "}" {
		return p.closeBlock(idx, line)
	}

	end := statementEnd(p.lines, idx)

	if routeOpenPattern.MatchString(line) {
		return end, p.readRoute(idx, end)
	}

	if isOverlayStatement(trimmed) {
		p.raw = append(p.raw, p.lines[idx:end]...)

		return end, nil
	}

	return end, p.readDecl(idx, end)
}

// A brace, three entries, and a closing brace. It bounds the read, so a
// statement that ends early cannot have entry lines read past its own end.
const routeStatementLines = 5

// readRoute sends method and path to the surface, tool and layout to the overlay.
func (p *parser) readRoute(start, end int) error {
	p.flushRaw()

	anchor, route, err := parseRoute(p.lines, start, end, p.blockPath())
	if err != nil {
		return fmt.Errorf("%w: %s:%d", err, p.file.Path, start+1)
	}

	if claimErr := p.claimRoute(anchor.Message, route); claimErr != nil {
		return fmt.Errorf("%w: %s:%d", claimErr, p.file.Path, start+1)
	}

	p.file.Elements = append(p.file.Elements, anchor)

	return nil
}

func (p *parser) claimRoute(message string, route surfaceRoute) error {
	if _, dup := p.surf.Routes[message]; dup {
		return fmt.Errorf("%w: %s", errDuplicateRoute, message)
	}

	p.surf.Routes[message] = route

	return nil
}

// parseRoute refuses every shape the model cannot re-render: another block
// length, an entry out of order, a misaligned indent, or no enclosing message.
func parseRoute(lines []string, start, end int, path string) (*routeAnchor, surfaceRoute, error) {
	if path == "" {
		return nil, surfaceRoute{}, errRouteOutsideMessage
	}

	if end-start < routeStatementLines {
		return nil, surfaceRoute{}, errUnparsedRoute
	}

	indent := leadingSpace(lines[start])

	inner, entries, err := routeEntries(lines, start)
	if err != nil {
		return nil, surfaceRoute{}, err
	}

	if lines[start+4] != indent+"};" {
		return nil, surfaceRoute{}, errUnparsedRoute
	}

	method, err := strconv.Unquote(entries[1])
	if err != nil {
		return nil, surfaceRoute{}, errUnparsedRoute
	}

	route, err := strconv.Unquote(entries[2])
	if err != nil {
		return nil, surfaceRoute{}, errUnparsedRoute
	}

	anchor := &routeAnchor{Indent: indent, Inner: inner, Message: path, Tool: entries[0], Line: start + 1}

	return anchor, surfaceRoute{Method: method, Path: route}, nil
}

// routeEntries reads the three entries in the one order the tree writes them.
func routeEntries(lines []string, start int) (string, [3]string, error) {
	var (
		inner   string
		entries [3]string
	)

	for offset, want := range [3]string{"tool", "method", "path"} {
		match := routeEntryPattern.FindStringSubmatch(lines[start+1+offset])
		if len(match) == 0 || match[2] != want {
			return "", entries, errUnparsedRoute
		}

		if offset > 0 && match[1] != inner {
			return "", entries, errUnparsedRoute
		}

		inner = match[1]
		entries[offset] = match[3]
	}

	return inner, entries, nil
}

func (p *parser) closeBlock(idx int, line string) (int, error) {
	if len(p.stack) == 0 {
		return 0, fmt.Errorf("%w: %s:%d", errStrayClose, p.file.Path, idx+1)
	}

	p.flushRaw()
	p.file.Elements = append(p.file.Elements, blockClose{Indent: leadingSpace(line)})
	p.stack = p.stack[:len(p.stack)-1]

	return idx + 1, nil
}

// readDecl sends label, type, name, and position to the surface; the number,
// the file line, and the rest of the tail stay in the overlay.
func (p *parser) readDecl(start, end int) error {
	p.flushRaw()

	inEnum := len(p.stack) > 0 && p.stack[len(p.stack)-1].Kind == "enum"

	head, ok := parseDeclHead(p.lines[start], inEnum)
	if !ok {
		return fmt.Errorf("%w: %s:%d", errUnparsedDeclaration, p.file.Path, start+1)
	}

	tail, location, err := parseTail(head.Rest, p.lines[start+1:end])
	if err != nil {
		return fmt.Errorf("%w: %s:%d", err, p.file.Path, start+1)
	}

	path := p.blockPath()
	if claimErr := p.claim(declKey(path, head.Name), head, location, inEnum); claimErr != nil {
		return fmt.Errorf("%w: %s:%d", claimErr, p.file.Path, start+1)
	}

	p.file.Elements = append(p.file.Elements, &declAnchor{
		Tail:   tail,
		Indent: head.Indent,
		Path:   path,
		Name:   head.Name,
		Number: head.Number,
		Line:   start + 1,
		InEnum: inEnum,
	})

	return nil
}

// claim refuses a second declaration under one key: the later would answer for
// the earlier everywhere the surface is read.
func (p *parser) claim(key string, head *declHead, location string, inEnum bool) error {
	if inEnum {
		if _, dup := p.surf.Values[key]; dup {
			return fmt.Errorf("%w: %s", errDuplicateDeclaration, key)
		}

		p.surf.Values[key] = struct{}{}

		return nil
	}

	if _, dup := p.surf.Fields[key]; dup {
		return fmt.Errorf("%w: %s", errDuplicateDeclaration, key)
	}

	p.surf.Fields[key] = surfaceField{Label: head.Label, Type: head.Type, Location: location}

	return nil
}

func (p *parser) flushRaw() {
	if len(p.raw) == 0 {
		return
	}

	p.file.Elements = append(p.file.Elements, rawSpan{Lines: p.raw})
	p.raw = nil
}

func (p *parser) blockPath() string {
	names := make([]string, 0, len(p.stack))
	for _, scope := range p.stack {
		names = append(names, scope.Name)
	}

	return strings.Join(names, ".")
}

// declHead is one declaration's first line taken apart.
type declHead struct {
	Indent string
	Label  string
	Type   string
	Name   string
	Rest   string
	Number int
}

func parseDeclHead(line string, inEnum bool) (*declHead, bool) {
	if inEnum {
		match := enumValuePattern.FindStringSubmatch(line)
		if match == nil {
			return nil, false
		}

		number, err := strconv.Atoi(match[3])
		if err != nil {
			return nil, false
		}

		return &declHead{Indent: match[1], Name: match[2], Number: number, Rest: match[4]}, true
	}

	match := fieldPattern.FindStringSubmatch(line)
	if match == nil {
		return nil, false
	}

	number, err := strconv.Atoi(match[5])
	if err != nil {
		return nil, false
	}

	return &declHead{
		Indent: match[1],
		Label:  match[2],
		Type:   match[3],
		Name:   match[4],
		Number: number,
		Rest:   match[6],
	}, true
}

// statementEnd answers the line after the statement opening at start. Strings
// are blanked first, so a semicolon inside one cannot end a statement early.
func statementEnd(lines []string, start int) int {
	var depth int

	for idx := start; idx < len(lines); idx++ {
		code := codeOf(lines[idx])
		depth += bracketDepth(code)

		if depth <= 0 && strings.HasSuffix(strings.TrimRight(code, " \t"), ";") {
			return idx + 1
		}
	}

	return len(lines)
}

// codeOf blanks string contents and drops comments, leaving what the grammar sees.
func codeOf(line string) string {
	code := stringPattern.ReplaceAllString(line, `""`)
	if cut := strings.Index(code, "//"); cut >= 0 {
		code = code[:cut]
	}

	return code
}

func leadingSpace(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

// isOverlayStatement reports whether a statement is wholly this repo's, so it
// travels verbatim. reserved is here because a spent number is wire identity.
func isOverlayStatement(trimmed string) bool {
	return strings.HasPrefix(trimmed, "syntax ") ||
		strings.HasPrefix(trimmed, "package ") ||
		strings.HasPrefix(trimmed, "import ") ||
		strings.HasPrefix(trimmed, "option ") ||
		strings.HasPrefix(trimmed, reservedKeyword+" ")
}
