package main

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
)

// pyTool is one contract read the way the Python emitter reads it: the derived
// facts come off the shared contract model, and the field-level ones come off
// the descriptors, which is where Python asks for presence, kind, and the
// options a member declares.
type pyTool struct {
	c *contract
	// input, response, and element are the messages the renderers walk. A tool
	// with no collection has no element, and a struct-bodied read has no
	// response worth walking.
	input protoreflect.MessageDescriptor
	// declared is the message the tool's own tool_response names, which is the
	// one every echo is read off: the shared model swaps in the inner resource
	// where a read answers through an envelope.
	declared protoreflect.MessageDescriptor
	element  protoreflect.MessageDescriptor
	// echoes names the response members a mutation carries back from its call,
	// in response declaration order.
	echoes []string
}

// pyPathArg is how one path argument is read out of a tool call: the value
// meaning the caller omitted it, and the builtin the read goes through before
// it reaches the route.
type pyPathArg struct {
	absent string
	coerce string
	reader linodev1.ArgumentReader
	// numeric is whether the slot carries an integer id rather than a label,
	// which is the one question every reader of this decides on.
	numeric bool
}

// newPyTool resolves the descriptors one contract is rendered against.
func newPyTool(built *contract) (*pyTool, error) {
	input, err := pyMessage(protoreflect.FullName(built.InputMessage))
	if err != nil {
		return nil, err
	}

	tool := &pyTool{c: built, input: input}

	if declared := stringOption(input.Options(), linodev1.E_ToolResponse); declared != "" {
		if tool.declared, err = pyMessage(protoreflect.FullName(declared)); err != nil {
			return nil, err
		}
	}

	if built.ElementGo.FullName != "" {
		if tool.element, err = pyMessage(built.ElementGo.FullName); err != nil {
			return nil, err
		}
	}

	tool.echoes = tool.writeEchoes()

	return tool, nil
}

// pyMessage resolves one message descriptor by full name.
func pyMessage(name protoreflect.FullName) (protoreflect.MessageDescriptor, error) {
	messageType, err := protoregistry.GlobalTypes.FindMessageByName(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", errNoGoType, name, err)
	}

	return messageType.Descriptor(), nil
}

// argument is the input field one tool argument names, nil when the message
// declares none.
func (t *pyTool) argument(name string) protoreflect.FieldDescriptor {
	return t.input.Fields().ByName(protoreflect.Name(name))
}

// bodyFields is the tool's BODY arguments as descriptors, in declared order.
func (t *pyTool) bodyFields() []protoreflect.FieldDescriptor {
	found := make([]protoreflect.FieldDescriptor, 0, len(t.c.Body))
	for i := range t.c.Body {
		found = append(found, t.argument(t.c.Body[i].ProtoName))
	}

	return found
}

// queryFields is the tool's QUERY arguments as descriptors, in declared order.
func (t *pyTool) queryFields() []protoreflect.FieldDescriptor {
	found := make([]protoreflect.FieldDescriptor, 0, len(t.c.Query))
	for i := range t.c.Query {
		found = append(found, t.argument(t.c.Query[i].ProtoName))
	}

	return found
}

// pathArg is how one path slot is read, from the type it declares. A URL
// segment carries an integer id or a label, so an enum reads as text: the
// caller sends "standard", not the number the generated enum holds it under.
func (t *pyTool) pathArg(slot string) pyPathArg {
	entry := t.argument(slot)
	reader := t.c.Readers[slot]

	if entry != nil && (entry.Kind() == protoreflect.Int32Kind || entry.Kind() == protoreflect.Int64Kind) {
		return pyPathArg{absent: "0", coerce: "int", reader: reader, numeric: true}
	}

	return pyPathArg{absent: `""`, coerce: "str", reader: reader}
}

// sentences is the wording one field gives its reader's refusals, arm by arm.
func (t *pyTool) sentences(name string) (string, string, string) {
	declared := t.c.Sentences[name]

	return declared.GetAbsent(), declared.GetUnusable(), declared.GetRefused()
}

// worded reports whether a slot's reader answers sentences the contract wrote.
// The fragment-safe member has no unworded form, so it counts as worded even
// where the field words nothing.
func (t *pyTool) worded(slot string) bool {
	absent, unusable, refused := t.sentences(slot)

	return absent != "" || unusable != "" || refused != "" ||
		t.c.Readers[slot] == linodev1.ArgumentReader_ARGUMENT_READER_FRAGMENT_SAFE_TEXT
}

// derivedSlots is the path slots checked by the derived required check. A slot
// declaring a shared reader is left out: the reader both reads it and answers
// its refusal.
func (t *pyTool) derivedSlots() []string {
	found := make([]string, 0, len(t.c.Slots))

	for _, slot := range t.c.Slots {
		if t.c.Readers[slot] == 0 {
			found = append(found, slot)
		}
	}

	return found
}

// readerSlots is the path slots checked by a shared reader, in declared order.
func (t *pyTool) readerSlots() []string {
	found := make([]string, 0, len(t.c.Slots))

	for _, slot := range t.c.Slots {
		if t.c.Readers[slot] != 0 {
			found = append(found, slot)
		}
	}

	return found
}

// writeEchoes is the response fields a mutation carries back from the call it
// made. Every member beside the message and the warning names the argument that
// fills it. A decoded envelope and a page both have nothing to rebuild: one
// decode places every message member the answer carries, and a count is the
// length of what the route sent back rather than an argument.
func (t *pyTool) writeEchoes() []string {
	if t.declared == nil {
		return nil
	}

	// An execute-backed read assembles its whole answer: one member is what the
	// hook brought back and every other one echoes an argument.
	if t.c.Tier == tierGet && len(t.c.Assembled) > 0 {
		return t.assembledEchoes()
	}

	if t.c.Tier != tierWrite && t.c.Tier != tierBodyRead {
		return nil
	}

	if t.c.BareResource || t.c.PagedWrite {
		return nil
	}

	// The declared entries carry the prefix that says the API's value wins over
	// an argument spelled the same way, and the member is what the echo asks
	// about either way.
	answered := make(map[string]bool, len(t.c.ResponseBody))

	for _, declared := range t.c.ResponseBody {
		member, _ := responseBodyMember(declared)
		answered[member] = true
	}

	echoesLists := len(answered) == 0
	found := make([]string, 0, t.declared.Fields().Len())

	for i := range t.declared.Fields().Len() {
		entry := t.declared.Fields().Get(i)
		name := string(entry.Name())

		if pyMessageOf(entry) != nil && (!pyRepeated(entry) || !echoesLists) {
			continue
		}

		if name == messageFieldName || name == warningFieldName || answered[name] {
			continue
		}

		found = append(found, name)
	}

	return found
}

// assembledEchoes is every response member an execute-backed read did not fill
// from its hook, which is the rest of the answer the call already holds.
func (t *pyTool) assembledEchoes() []string {
	filled := make(map[string]bool, len(t.c.Assembled))
	for _, member := range t.c.Assembled {
		filled[member.ProtoName] = true
	}

	found := make([]string, 0, t.declared.Fields().Len())

	for i := range t.declared.Fields().Len() {
		name := string(t.declared.Fields().Get(i).Name())
		if !filled[name] {
			found = append(found, name)
		}
	}

	return found
}

// assembledNames is the response members an execute hook fills, in the order
// the contract resolved them.
func (t *pyTool) assembledNames() []string {
	found := make([]string, 0, len(t.c.Assembled))
	for _, member := range t.c.Assembled {
		found = append(found, member.ProtoName)
	}

	return found
}

// wrapperMember is the envelope member an API body decodes into, "" when the
// response is the API body itself. The shared model swaps the inner resource in
// for the response, so the envelope is resolved back by the type name it kept.
func (t *pyTool) wrapperMember() (string, error) {
	if t.c.WrapperField == "" {
		return "", nil
	}

	envelope, err := pyMessage(protoreflect.FullName(protoPackage + "." + t.c.WrapperType))
	if err != nil {
		return "", err
	}

	for i := range envelope.Fields().Len() {
		entry := envelope.Fields().Get(i)
		if pyMessageOf(entry) != nil && !pyRepeated(entry) {
			return string(entry.Name()), nil
		}
	}

	return "", fmt.Errorf("%w: %s answers through %s, which holds no resource member",
		errPyRender, t.c.Name, t.c.WrapperType)
}

// routeText is the operation a tool performs, for the emitted doc comments, so
// a reader of the generated module sees it without opening the proto.
func (t *pyTool) routeText() string {
	return t.c.Method + " " + t.routePath()
}

// routePath is the path a doc comment names, prefixed when the surface is not
// the default. The default comes from the route reader rather than a literal
// here, so the emitter and the client cannot disagree about which surface needs
// no prefix.
func (t *pyTool) routePath() string {
	if t.c.Surface == linoderoute.DefaultSurfaceSegment {
		return t.c.PathTemplate
	}

	return "/" + t.c.Surface + t.c.PathTemplate
}

// capabilityMember is the Capability member a tool registers under. Spelled out
// rather than derived from the enum value, so a new tier fails here instead of
// registering as something plausible.
func (t *pyTool) capabilityMember() (string, error) {
	named := map[linodev1.ToolCapability]string{
		linodev1.ToolCapability_TOOL_CAPABILITY_READ:    "Read",
		linodev1.ToolCapability_TOOL_CAPABILITY_WRITE:   "Write",
		linodev1.ToolCapability_TOOL_CAPABILITY_DESTROY: "Destroy",
		linodev1.ToolCapability_TOOL_CAPABILITY_ADMIN:   "Admin",
		linodev1.ToolCapability_TOOL_CAPABILITY_META:    "Meta",
	}

	member, known := named[t.c.Capability]
	if !known {
		return "", fmt.Errorf("%w: %s registers at %s", errUnsupportedTier, t.c.Name, t.c.Capability)
	}

	return member, nil
}

// gatedRead reports whether a read is previewed and gated rather than answered
// outright, which is the write tier's shape under a read's verb.
func (t *pyTool) gatedRead() bool {
	return t.c.Tier == tierGet && t.c.DryRun
}

// assembledRead reports whether a tool reads through the assembled-read driver.
// The write tier also fills members from its hook, but it keeps its own driver:
// what separates the two is the message and the confirm gate a mutation
// carries, not where the values came from.
func (t *pyTool) assembledRead() bool {
	return t.c.Tier == tierGet && len(t.c.Assembled) > 0
}

// buildsBody reports whether a tool assembles a request body of its own. An
// acknowledged mutation does only when it declares body fields: a tool with
// none sends no body at all, since an empty JSON object is a body.
func (t *pyTool) buildsBody() bool {
	if t.c.Tier == tierWrite || t.c.Tier == tierBodyRead {
		return true
	}

	return (t.c.Tier == tierAcknowledge || t.c.Tier == tierDestroy) && len(t.c.Body) > 0
}

// forwardedQuery reports whether a read builds a forwarded query in its own
// handler. A collection hands its forwarded names to the list driver, which
// builds the query itself; only the read tier spells the call.
func (t *pyTool) forwardedQuery() bool {
	if t.c.Tier != tierGet {
		return false
	}

	for i := range t.c.Query {
		if !isPaginationParam(t.c.Query[i].ProtoName) {
			return true
		}
	}

	return false
}

// publishesPage reports whether a single-resource route publishes the standard page
// controls.
func (t *pyTool) publishesPage() bool {
	for i := range t.c.Query {
		if isPaginationParam(t.c.Query[i].ProtoName) {
			return true
		}
	}

	return false
}

// pathReadSlots is the slots a tool's handler reads through path_int or
// path_str. An ungated read takes the value its reader already handed back, so
// a slot declaring one is not read again there.
func (t *pyTool) pathReadSlots() []string {
	if t.c.Tier == tierWrite || t.c.Tier == tierAcknowledge || t.gatedRead() {
		return t.c.Slots
	}

	return t.derivedSlots()
}

// hook is the Python function one tool's hook of a kind lives under, "" when
// the tool declares no hook of that kind. The name is derived from the tool and
// the kind, the same pair the Go arm derives its own spelling from, so neither
// language carries a name the other has to be kept in step with.
func (t *pyTool) hook(kind string) string {
	slot, known := (&t.c.Hooks).slot(kind)
	if !known || *slot == "" {
		return ""
	}

	return t.c.Name + "_" + kind
}

// hookNames is the hook functions one tool declares, in kind order.
func (t *pyTool) hookNames() []string {
	declared := []string{
		t.hook(hookKindNormalize), t.hook(hookKindValidate), t.hook(hookKindPreview),
		t.hook(hookKindFetchState), t.hook(hookKindDependencyWalk),
		t.hook(hookKindExecute), t.hook(hookKindAnswer),
	}

	found := make([]string, 0, len(declared))

	for _, name := range declared {
		if name != "" {
			found = append(found, name)
		}
	}

	return found
}

// responseSubject is the call a malformed response is reported against, in
// prose: the tool's own name once the prefix every tool carries is dropped.
func (t *pyTool) responseSubject() string {
	return strings.ReplaceAll(strings.TrimPrefix(t.c.Name, toolPrefix), "_", " ")
}

// readerOf is the shared reader one argument declares, 0 when it declares none.
func readerOf(entry protoreflect.FieldDescriptor) linodev1.ArgumentReader {
	value, ok := proto.GetExtension(entry.Options(), linodev1.E_ArgumentReader).(linodev1.ArgumentReader)
	if !ok {
		return linodev1.ArgumentReader_ARGUMENT_READER_UNSPECIFIED
	}

	return value
}

// readerMessageOf is the wording one field gives its reader's refusals, three
// empty strings for a field that words none.
func readerMessageOf(entry protoreflect.FieldDescriptor) (string, string, string) {
	declared, ok := proto.GetExtension(entry.Options(), linodev1.E_ReaderMessage).(*linodev1.ReaderMessage)
	if !ok {
		return "", "", ""
	}

	return declared.GetAbsent(), declared.GetUnusable(), declared.GetRefused()
}

// echoArgumentOf is the tool argument one response member is filled from,
// resolved by its own name unless it declares an alias.
func echoArgumentOf(entry protoreflect.FieldDescriptor) string {
	declared, ok := proto.GetExtension(entry.Options(), linodev1.E_EchoArgument).(string)
	if !ok || declared == "" {
		return string(entry.Name())
	}

	return declared
}

// redactsPreview reports whether a preview stands in for one field's value
// rather than echoing it back.
func redactsPreview(entry protoreflect.FieldDescriptor) bool {
	value, ok := proto.GetExtension(entry.Options(), linodev1.E_PreviewRedact).(bool)

	return ok && value
}

// bodyNameOf is the declared wire key one field is written under, "" when it
// declares none.
func bodyNameOf(entry protoreflect.FieldDescriptor) string {
	value, ok := proto.GetExtension(entry.Options(), linodev1.E_BodyName).(string)
	if !ok {
		return ""
	}

	return value
}

// wireNameOf is the body key one field is written under, its own name unless
// the tool renames it.
func wireNameOf(entry protoreflect.FieldDescriptor) string {
	if declared := bodyNameOf(entry); declared != "" {
		return declared
	}

	return string(entry.Name())
}
