package main

import (
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// formatted is a message template split into a Sprintf format string and args.
type formatted struct {
	format string
	args   []string
}

// call renders the Sprintf argument list, substituting errorArg wherever the
// template named {error}.
func (f formatted) call(errorArg string) string {
	parts := make([]string, 0, len(f.args)+1)
	parts = append(parts, goStringLiteral(f.format))

	for _, arg := range f.args {
		if arg == errorPlaceholder {
			parts = append(parts, errorArg)

			continue
		}

		parts = append(parts, arg)
	}

	return strings.Join(parts, ", ")
}

// literal renders a template as the Go expression that produces it: the string
// itself when it names no field, and the Sprintf that fills it otherwise.
func (f formatted) literal() string {
	if len(f.args) == 0 {
		return goStringLiteral(f.format)
	}

	return "fmt.Sprintf(" + f.call("") + ")"
}

// errorSuffix is how every declared failure sentence ends, and the part a
// struct read's serializer appends for itself.
const errorSuffix = ": {" + errorPlaceholder + "}"

// structFailPrefix is the sentence a free-form read hands its serializer, which
// words a conversion failure by appending ": " and the error to it. That is the
// same sentence error_message declares, minus the error it already carries, so
// the prefix is derived from the declaration rather than declared twice.
func (c *contract) structFailPrefix() (formatted, error) {
	trimmed, ends := strings.CutSuffix(c.ErrorMessage, errorSuffix)
	if !ends {
		return formatted{}, fmt.Errorf("%w: %s declares %q", errNoStructFailPrefix, c.Name, c.ErrorMessage)
	}

	return c.formatMessage(trimmed)
}

// formatMessage turns a declared message template into a format string and its
// arguments. {error} becomes %v; every other placeholder names a field of this
// input message and takes the verb its proto kind prints under. A placeholder
// naming nothing is refused here so no client sees a literal "{domain_id}".
func (c *contract) formatMessage(template string) (formatted, error) {
	return render(template, c.placeholder)
}

// formatSuccess turns a success_message template into a format string and its
// arguments, resolving each placeholder against the resource the API answered
// with before falling back to the call's own arguments.
//
// The response wins a name both carry because that is the value the API settled
// on: a create echoes the label it actually stored, which can differ from the
// one asked for. Names the response does not carry are how an update reports the
// path id it was addressed by.
func (c *contract) formatSuccess(template, payloadVar string) (formatted, error) {
	return render(template, func(ref reference, whole string) (string, string, error) {
		goName, kind, found := c.payloadField(ref)
		if !found {
			return c.placeholder(ref, whole)
		}

		if c.PayloadGo.isList(ref.name) {
			return "", "", repeatedPlaceholder(c.Name, ref.name)
		}

		return kindVerb(kind), payloadVar + ".Get" + goName + "()", nil
	})
}

// payloadField resolves one name against the resource a mutation answers with.
// A count is never resolved here: the entries the caller sent are what both
// languages count the same way, where a decoded response list is not.
func (c *contract) payloadField(ref reference) (string, protoreflect.Kind, bool) {
	if ref.length {
		return "", 0, false
	}

	goName := c.PayloadGo.goNameOf(ref.name)
	if goName == "" {
		return "", 0, false
	}

	kind, known := c.PayloadGo.kindOf(ref.name)

	return goName, kind, known
}

// render walks a template, handing each placeholder to resolve.
func render(
	template string,
	resolve func(ref reference, whole string) (string, string, error),
) (formatted, error) {
	var (
		built formatted
		text  strings.Builder
		rest  = template
	)

	text.Grow(len(template))

	for {
		open := strings.Index(rest, "{")
		if open < 0 {
			text.WriteString(rest)

			break
		}

		closed := strings.Index(rest[open:], "}")
		if closed < 0 {
			text.WriteString(rest)

			break
		}

		ref, width := splitPlaceholder(rest[open+1 : open+closed])
		text.WriteString(rest[:open])

		verb, arg, err := resolve(ref, template)
		if err != nil {
			return formatted{}, err
		}

		text.WriteString(padVerb(verb, width))

		built.args = append(built.args, arg)
		rest = rest[open+closed+1:]
	}

	built.format = text.String()

	return built, nil
}

// zeroPadSeparator introduces a placeholder's zero-padded width: {month:02}
// renders March as 03, which is what a date reads as. Both languages read the
// same spelling, so a template written once renders identically in each.
const zeroPadSeparator = ":0"

// numberVerb is the Sprintf verb every integer a template renders prints
// under, including a count.
const numberVerb = "%d"

// countSuffix introduces a placeholder's count form: {linodes:len} renders how
// many entries the caller sent for a repeated field. It reads as one more
// modifier after the colon, the shape {month:02} already established, and both
// languages parse the same spelling.
const countSuffix = ":len"

// defaultSeparator introduces a placeholder's declared default: {name|World}
// renders World for a caller who left name out. It is what lets a meta tool
// answer a whole sentence from the contract instead of writing a body by hand
// for one absent argument, and it needs no proto surface, since the default is
// prose belonging to the sentence that reads it. Both languages parse the same
// spelling, the way they already do for {month:02} and {linodes:len}.
const defaultSeparator = "|"

// reference is one parsed placeholder: the field it names, the value it renders
// when the caller sent none, and whether it asks for that field's entry count
// rather than its value.
type reference struct {
	name     string
	fallback string
	defaults bool
	length   bool
}

// splitPlaceholder separates a placeholder's field name from the form it asks
// for: a default, a count, a zero-padded width, or none of them.
func splitPlaceholder(body string) (reference, string) {
	if name, fallback, defaulted := strings.Cut(body, defaultSeparator); defaulted {
		return reference{name: name, fallback: fallback, defaults: true}, ""
	}

	if name, counted := strings.CutSuffix(body, countSuffix); counted {
		return reference{name: name, length: true}, ""
	}

	name, width, padded := strings.Cut(body, zeroPadSeparator)
	if !padded {
		return reference{name: body}, ""
	}

	return reference{name: name}, width
}

// padVerb widens a verb to a zero-padded width. A width on anything but a
// number is dropped rather than rendered, since Sprintf's zero flag pads text
// with spaces and a template asking for 03 would answer " 3".
func padVerb(verb, width string) string {
	if width == "" || verb != numberVerb {
		return verb
	}

	return "%0" + width + "d"
}

// placeholder maps one template name to its verb and the expression filling it.
func (c *contract) placeholder(ref reference, template string) (string, string, error) {
	if ref.defaults {
		return c.toolArg(ref, template)
	}

	if ref.length {
		return c.count(ref.name, template)
	}

	if ref.name == errorPlaceholder {
		return "%v", errorPlaceholder, nil
	}

	for _, entry := range c.Local {
		if entry.ProtoName == ref.name {
			return toolArgRead(c.Name, &entry, "")
		}
	}

	for _, entry := range c.allFields() {
		if entry.ProtoName != ref.name {
			continue
		}

		if entry.Repeated {
			return "", "", repeatedPlaceholder(c.Name, entry.ProtoName)
		}

		return kindVerb(entry.Kind), entry.GoLocal, nil
	}

	for _, entry := range c.Body {
		if entry.ProtoName != ref.name {
			continue
		}

		if entry.Repeated {
			return "", "", repeatedPlaceholder(c.Name, entry.ProtoName)
		}

		// The echo reader rather than the path one: a body argument reports what
		// the caller asked for, which a flag is as much of as text is, and %t
		// against Python's own true/false renders the same either way.
		reader, zero, ok := echoArgReader(entry.Kind)
		if !ok {
			return "", "", fmt.Errorf("%w: %s is %s", errUnsupportedPathKind, entry.ProtoName, entry.Kind)
		}

		return kindVerb(entry.Kind), fmt.Sprintf("request.%s(%s, %s)", reader, goStringLiteral(ref.name), zero), nil
	}

	return "", "", fmt.Errorf("%w: %s in %q", errUnknownPlaceholder, ref.name, template)
}

// toolArg resolves a placeholder carrying a declared default. Only a TOOL
// argument takes one: every other location either addresses the request, where
// an absent value is refused rather than substituted, or is MCP plumbing the
// sentence has no reason to name.
func (c *contract) toolArg(ref reference, template string) (string, string, error) {
	if ref.fallback == "" {
		return "", "", fmt.Errorf("%w: %s in %q", errEmptyDefault, ref.name, template)
	}

	// The modifiers and the default are alternatives rather than a grammar, so
	// {limit:len|3} would be read as a field named "limit:len".
	if strings.Contains(ref.name, zeroPadSeparator) || strings.HasSuffix(ref.name, countSuffix) {
		return "", "", fmt.Errorf("%w: %s in %q", errModifiedDefault, ref.name, template)
	}

	for _, entry := range c.Local {
		if entry.ProtoName == ref.name {
			return toolArgRead(c.Name, &entry, ref.fallback)
		}
	}

	return "", "", fmt.Errorf("%w: %s in %q", errDefaultNotToolArg, ref.name, template)
}

// toolArgRead renders the verb and the read one TOOL argument is filled
// through, with the declared default as the value an absent argument answers.
//
// The read is the accessor a path id goes through, because a tool argument
// reports what the caller asked for and Python's `arguments.get(name, default)`
// is the same shape, so a template written once renders identically in each
// language. Text and a number are the only kinds that holds for, because a
// declared default is part of the shape and neither toolArgDefault nor the
// schema has a spelling for a flag's.
func toolArgRead(tool string, entry *field, fallback string) (string, string, error) {
	if entry.Repeated {
		return "", "", repeatedPlaceholder(tool, entry.ProtoName)
	}

	reader, zero, ok := pathArgReader(entry.Kind)
	if !ok || entry.Kind == protoreflect.EnumKind {
		return "", "", fmt.Errorf("%w: %s names %s, which is %s",
			errUnsupportedToolArg, tool, entry.ProtoName, entry.Kind)
	}

	absent, err := toolArgDefault(tool, entry, fallback, zero)
	if err != nil {
		return "", "", err
	}

	return kindVerb(entry.Kind), fmt.Sprintf("request.%s(%s, %s)",
		reader, goStringLiteral(entry.ProtoName), absent), nil
}

// toolArgDefault renders a declared default as the literal the accessor takes,
// answering the kind's own zero when none is declared. A default the kind
// cannot hold is refused rather than rendered, since Go would not compile it
// and Python would answer the text where a number was asked for.
func toolArgDefault(tool string, entry *field, fallback, zero string) (string, error) {
	if fallback == "" {
		return zero, nil
	}

	switch entry.Kind {
	case protoreflect.StringKind:
		return goStringLiteral(fallback), nil
	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		if _, err := strconv.Atoi(fallback); err == nil {
			return fallback, nil
		}
	case protoreflect.BoolKind, protoreflect.EnumKind, protoreflect.FloatKind,
		protoreflect.DoubleKind, protoreflect.BytesKind, protoreflect.MessageKind,
		protoreflect.GroupKind, protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind, protoreflect.Fixed32Kind,
		protoreflect.Fixed64Kind, protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
	}

	return "", fmt.Errorf("%w: %s defaults %s to %q, which is not %s",
		errUnreadableDefault, tool, entry.ProtoName, fallback, entry.Kind)
}

// count resolves a count placeholder against the repeated body field it names.
// It counts the argument the caller sent rather than anything decoded, which is
// the one list Python counts the same way.
func (c *contract) count(name, template string) (string, string, error) {
	for _, entry := range c.Body {
		if entry.ProtoName != name {
			continue
		}

		if !entry.Repeated {
			return "", "", fmt.Errorf("%w: %s counts %s, which holds one value",
				errNotRepeated, c.Name, name)
		}

		return numberVerb, fmt.Sprintf("tools.ArgumentLen(request, %s)", goStringLiteral(name)), nil
	}

	return "", "", fmt.Errorf("%w: %s in %q", errUnknownPlaceholder, name, template)
}

// repeatedPlaceholder refuses a list where one value was asked for. Go prints a
// list as [a b] and Python as ['a'], and the body reader the emitter would fall
// through to answers 0 for one, so the count form is the only rendering the two
// languages share.
func repeatedPlaceholder(tool, name string) error {
	return fmt.Errorf("%w: %s names %s, so write {%s%s} to render how many entries it carries",
		errRepeatedPlaceholder, tool, name, name, countSuffix)
}

// allFields lists every input field a message template may name through the
// local a handler already reads it into. Local fields are excluded because they
// never leave the MCP layer into a handler variable.
func (c *contract) allFields() []field {
	all := make([]field, 0, len(c.Path)+len(c.Query))
	all = append(all, c.Path...)

	return append(all, c.Query...)
}

// pathExpression renders the endpoint a tool's route resolves to, as a Go
// expression over the handler's path locals. A preview reports the call it would
// have made, so the path is filled the same way the routed call fills it rather
// than being written out a second time.
// A text slot is escaped the way linoderoute escapes it before the live call,
// so an id carrying a slash ("private/123") is reported as the one segment it
// addresses rather than as two the route never had.
func (c *contract) pathExpression(out *source) (string, error) {
	if len(c.Slots) == 0 {
		return goStringLiteral(routePath(c)), nil
	}

	filled, err := render(routePath(c), func(ref reference, whole string) (string, string, error) {
		verb, arg, err := c.placeholder(ref, whole)
		if err != nil || verb != "%s" {
			return verb, arg, err
		}

		out.need(importLinoderoute)

		return verb, "linoderoute.EscapeSegment(" + arg + ")", nil
	})
	if err != nil {
		return "", err
	}

	return "fmt.Sprintf(" + filled.call("") + ")", nil
}

// kindVerb is the Sprintf verb a proto kind prints under. Picking by kind rather
// than defaulting to %v keeps an id rendering as 5, not as a typed spelling.
func kindVerb(kind protoreflect.Kind) string {
	switch kind {
	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Fixed32Kind, protoreflect.Fixed64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		return numberVerb
	case protoreflect.StringKind:
		return "%s"
	case protoreflect.BoolKind:
		return "%t"
	case protoreflect.FloatKind, protoreflect.DoubleKind,
		protoreflect.BytesKind, protoreflect.EnumKind,
		protoreflect.MessageKind, protoreflect.GroupKind:
	}

	return "%v"
}

// orderedPathFields lists the tool's PATH fields in route-template order, not
// message order: the routed call fills slots positionally, so a two-id route
// whose fields are declared the other way round would swap the ids it targets.
func (c *contract) orderedPathFields() ([]field, error) {
	if len(c.Slots) != len(c.Path) {
		return nil, fmt.Errorf("%w: %s has %d slots and %d PATH fields",
			errPathArity, c.Name, len(c.Slots), len(c.Path))
	}

	ordered := make([]field, 0, len(c.Slots))

	for _, slot := range c.Slots {
		entry, found := c.pathField(slot)
		if !found {
			return nil, fmt.Errorf("%w: %s names slot %s, which is no PATH field",
				errPathArity, c.Name, slot)
		}

		ordered = append(ordered, entry)
	}

	return ordered, nil
}

// pathField returns the PATH field one route slot names.
func (c *contract) pathField(slot string) (field, bool) {
	for _, entry := range c.Path {
		if entry.ProtoName == slot {
			return entry, true
		}
	}

	return field{}, false
}

// routeSlots lists a path template's slot names in template order.
func routeSlots(template string) []string {
	var (
		slots []string
		rest  = template
	)

	for {
		open := strings.Index(rest, "{")
		if open < 0 {
			return slots
		}

		closed := strings.Index(rest[open:], "}")
		if closed < 0 {
			return slots
		}

		slots = append(slots, rest[open+1:open+closed])
		rest = rest[open+closed+1:]
	}
}
