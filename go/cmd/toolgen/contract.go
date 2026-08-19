package main

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
)

// protoPackage scopes the descriptor walk to this repo's contract. The global
// registry also holds descriptor.proto and whatever a dependency links in, none
// of which can carry a tool option.
const protoPackage = "linode.mcp.v1"

// tier is the shape of code one tool needs, read off the contract rather than
// declared: a response carrying a repeated field under a count is a collection,
// and a route with path slots under one is a sub-resource collection.
type tier int

const (
	tierUnknown tier = iota
	// tierGet fetches one resource and serializes it.
	tierGet
	// tierList fetches a page of a top-level collection.
	tierList
	// tierSubresourceList fetches a page of a collection nested under a path id.
	tierSubresourceList
	// tierMarkerList fetches a marker-paged collection, whose body answers a
	// cursor beside its elements.
	tierMarkerList
	// tierBodyRead fetches one resource through a route that takes a request
	// body rather than a query, which the presigned-URL create is: the call
	// signs what the body describes and stores nothing.
	tierBodyRead
	// tierWrite sends a body to create or change one resource.
	tierWrite
	// tierAcknowledge performs a mutation the API answers with nothing worth
	// reporting, so the tool's answer is built from the call the way a
	// destroy's is.
	tierAcknowledge
	// tierDestroy removes one resource and answers with the ids it was
	// addressed by, gated by confirm and reachable as a two-stage plan.
	tierDestroy
	// tierMeta answers from local state and reaches no Linode route: the audit
	// stores, the profile builder's registry, the version the binary reports.
	tierMeta
)

// field is one input field with everything the emitter needs to read it out of
// a tool request and put it where the contract says it goes.
type field struct {
	// Filter is the client-side match a QUERY field declares, nil when it
	// declares none and the element decides instead.
	Filter *linodev1.ListFilterSpec
	// ItemMessage is the named message a repeated BODY field carries an array
	// of, nil for every other field. It is what lets the emitter name each
	// item member, where a free-form ObjectList fixes none.
	ItemMessage protoreflect.MessageDescriptor
	// Message is the named message a singular BODY field carries, nil for
	// every other field. Separate from ItemMessage because a response echo
	// maps an argument's items onto its own, which only a list has.
	Message protoreflect.MessageDescriptor
	// ProtoName is the tool argument name, which is also the proto field name.
	ProtoName string
	// GoLocal is the local variable a handler reads the argument into.
	GoLocal string
	// Description comes from the generated JSON schema so the emitter and the
	// shipped schema cannot disagree about it.
	Description string
	// BodyName is the wire key a BODY field is written under when the API
	// names the member differently than the tool names its argument, "" when
	// the two agree.
	BodyName string
	Location linodev1.FieldLocation
	Kind     protoreflect.Kind
	// Presence is true when the caller's having supplied the field is what
	// decides whether it travels: a proto3 `optional` scalar or a repeated
	// field. A field without it is written on every call, which is what a bare
	// proto3 scalar declares.
	Presence bool
	// Repeated is true for a list field.
	Repeated bool
	// ObjectMap is true for the one free-form shape a body can carry: a map
	// from string to google.protobuf.Value, which is how a payload whose
	// members vary by kind (a payment method's details) is declared.
	ObjectMap bool
	// StringMap is true for a map from string to string, the shape an API takes
	// when every member is text (a node pool's Kubernetes labels).
	StringMap bool
	// Nullable is true when a BODY object field also accepts an explicit JSON
	// null, which the API acts on rather than reading as absent.
	Nullable bool
	// ObjectList is true for a repeated google.protobuf.Struct, the free-form
	// shape a body carries as an array: one account user grant section is a
	// list of objects whose members the contract does not fix.
	ObjectList bool
	// Redact is true when a preview must stand in for this member rather than
	// echo it back.
	Redact bool
	// CommaList is true when a BODY string argument reaches the wire as the
	// array of its comma-separated segments rather than as text.
	CommaList bool
	// BodyRoot is true when a BODY object field IS the request body rather than
	// a member of it, which is the shape the profile preferences PUT reads.
	BodyRoot bool
}

// wireName is the body key one field is written under, which is its own name
// unless the tool declares another.
func (f *field) wireName() string {
	if f.BodyName != "" {
		return f.BodyName
	}

	return f.ProtoName
}

// stateRead is the read a synthesized state fetch performs: the tool whose
// route it resolves, and the message its answer decodes into.
type stateRead struct {
	Message goMessage
	Tool    string
	// Select is the element member the trailing path id picks one element of a
	// collection sibling by, empty when the sibling reads the resource itself.
	Select string
	// BodyKey is the raw-body member the resource sits under, "" when the body
	// is the resource.
	BodyKey string
	// Payload is the response member the resource itself is, "" when the
	// response is the resource. It is carried past resolution because the other
	// arm resolves the message it names at call time.
	Payload string
	// Slots names the removal's path arguments that fill the read route, in the
	// read's own slot order. Empty for a derived sibling, which shares the
	// removal's template and so takes its arguments as they come.
	Slots []string
	// Query names the read's query parameters and the argument each is filled
	// from, for the reads a query addresses rather than a path alone.
	Query []stateReadQuery
}

// stateReadQuery is one query parameter of a declared read and the argument
// that fills it.
type stateReadQuery struct {
	Parameter string
	Argument  string
}

// collection reports whether the sibling answers with the page the resource
// belongs to rather than with the resource, which is the fetch that has to
// select before it can report.
func (s *stateRead) collection() bool {
	return s.Select != ""
}

// declared reports whether a removal resolved a sibling to read its state
// through, which is what separates a synthesized fetch from a hand-written one.
func (s *stateRead) declared() bool {
	return s.Tool != ""
}

// contract is everything the descriptors say about one tool.
type contract struct {
	ResponseGo     goMessage
	ElementGo      goMessage
	PayloadGo      goMessage
	Name           string
	SourceFile     string
	Description    string
	ErrorMessage   string
	ConfirmMessage string
	SuccessMessage string
	WarningMessage string
	InputMessage   string
	EnvelopeField  string
	CountField     string
	FilterField    string
	// TruncatedField and NextMarkerField are the members a marker-paged answer
	// reports its cursor in, "" on every other shape.
	TruncatedField  string
	NextMarkerField string
	PayloadField    string
	// PayloadMember is the wire name of PayloadField, which is what an answer
	// carries the decoded resource under, "" for a bare resource.
	PayloadMember string
	MessageField  string
	// WarningField is the response member the declared warning_message is
	// reported in, "" when the response declares none.
	WarningField string
	// Assembled is the response members an execute-backed tool fills from what
	// its hook brought back, empty for every tool whose answer is decoded. The
	// hook answers the response message carrying them; the emitter copies only
	// these, so a member the hook fills that is not one of them is dropped.
	Assembled []assembledMember
	// WrapperField is the member of a tool's own response envelope that the
	// API body decodes into, "" when the response is the API body itself.
	WrapperField string
	// WrapperType is the Go type of that envelope, set with WrapperField.
	WrapperType string
	Method      string
	// Surface is the base path segment the route answers on, already resolved
	// from the declared enum so an unrenderable one fails generation.
	Surface      string
	PathTemplate string
	ResourceType string
	Hooks        toolHooks
	// StateRead is the sibling this removal synthesizes its state fetch from,
	// zero for every tool that names a fetch_state hook of its own.
	StateRead stateRead
	// StateRouteDecl is the read this removal declares its state fetch through,
	// nil when it derives one or names a hook. It is kept raw until the other
	// tools are in scope to resolve the named one against.
	StateRouteDecl *linodev1.StateRoute
	Slots          []string
	// ExplicitNulls names the payload fields the API sends as an explicit null
	// that the serializer drops, restored from the raw body after the marshal.
	ExplicitNulls []string
	// ResponseBody names the response members the API's answer fills rather
	// than the call, which is what makes the declared envelope the message the
	// body decodes into.
	ResponseBody []string
	// Refused names the arguments the tool answers for rather than sends, and
	// RefuseUnknown holds it to the arguments its message declares. Both read
	// the whole argument map, which is the only place a name the message does
	// not declare is still visible.
	Refused       *linodev1.RefuseArguments
	RefuseUnknown *linodev1.RefuseUnknownArguments
	// AllArguments is every argument the input message names, in field order,
	// system params included. It is the allowlist RefuseUnknown is measured
	// against, so a caller may still send dry_run beside the settings they are
	// changing.
	AllArguments []string
	Path         []field
	Query        []field
	Body         []field
	// Constants are the body members the tool always sends at the top of its
	// body with a fixed value and takes no argument for.
	Constants []bodyConstant
	// FoldDecls is the assembly each BODY member declares, keyed by the member,
	// and Folds is that declaration resolved. Folded names the arguments that
	// travel inside one rather than at the top of the body, so the body builder
	// skips them. All three are keyed rather than kept on the field, which is
	// copied per iteration everywhere the emitter walks a tool's arguments.
	FoldDecls map[string]*linodev1.BodyFold
	Folds     map[string][]foldMember
	Folded    map[string]bool
	// Readers is the shared argument check each field declares, keyed by the
	// argument, absent for the fields declaring none. Keyed for the same reason
	// the folds above are: `field` sits at the size gocritic starts reporting a
	// per-iteration copy at, and one more member there costs a rewrite of every
	// loop the emitter walks arguments with.
	Readers map[string]linodev1.ArgumentReader
	// Members is the vocabulary each ENUM_MEMBER field is held to, keyed like
	// Readers: an enum field's descriptor names without the zero sentinel in
	// number order, or the declared reader_values for a string field.
	Members map[string][]string
	// Sentences is the wording each field gives its reader's refusals, keyed
	// like Readers and absent for the fields wording none.
	Sentences map[string]*linodev1.ReaderMessage
	// AnyOf is the message-level require_any_of declaration resolved: the BODY
	// arguments at least one of which the caller must send, and the sentence
	// the refusal answers. Nil for the tools that declare none.
	AnyOf *anyOfCheck
	// Normalizes is the declared rewrites the tool's arguments get before
	// anything reads them, in declaration order because that is the order they
	// run in. Empty for a tool whose arguments are read as they arrived, and for
	// the few that still answer their whole map through a normalize hook.
	Normalizes []normalizeRewrite
	// PreviewSentences is the prose the tool's dry run reports, in declaration
	// order because that is the order a preview lists it in. Empty for a tool
	// whose preview reports the call alone, and for the ones still answering
	// through a preview hook.
	PreviewSentences []previewSentence
	// Local is the tool's own domain arguments, which reach no request at all.
	// Only a meta tool declares any: its answer is built from local state, and
	// these are what it is built from.
	Local []field
	// Forwarded is the query arguments the route itself filters on, which the
	// request carries rather than the handler applying.
	Forwarded []field
	// Echoed names the forwarded arguments a marker-paged answer reports it was
	// narrowed by, which is every one of them except the cursor controls.
	Echoed        []field
	Filters       []listFilter
	Echoes        []destroyEcho
	Tier          tier
	Capability    linodev1.ToolCapability
	RetryDisabled bool
	// PreviewOmitsBody marks a tool whose dry run reports no request body, which
	// is the shape a handful of families answered with before the echo existed.
	PreviewOmitsBody bool
	// Walks is whether the tool declares any object_walk. The declaration is
	// read from the descriptor at call time rather than emitted, so all the
	// generated handler needs to know is whether to name the seam.
	Walks bool
	// WarningDeclared is whether warning_message was written at all, which is
	// what separates a tool answering with a deliberately empty notice from one
	// declaring no notice: the text alone reads back as "" for both.
	WarningDeclared bool
	// BareResource is true when a mutation's declared response is the API
	// resource itself rather than an envelope wrapping it, so the answer is the
	// decoded body with no prose over it.
	BareResource bool
	// DryRun is true when the tool advertises a dry_run argument, which is the
	// promise that a caller can see the call without it being made.
	DryRun bool
	// Confirm is true when the tool advertises a confirm argument, which is
	// what sends a read to the gated shape rather than the plain one.
	Confirm bool
	// Mode and PlanID are whether the tool advertises the plan/apply flow's two
	// arguments. They are read separately so a tool declaring one alone is
	// refused by name rather than served the half of the flow it can reach.
	Mode   bool
	PlanID bool
	// StructResponse is true for a read whose API body has no proto model, so
	// the answer is the decoded object serialized through a bare Struct.
	StructResponse bool
	// DecodedResponse is true when the declared envelope is the message the API
	// body decodes into, which is what ResponseBody asks for.
	DecodedResponse bool
	// PayloadStruct is true when the member a mutation's body decodes into has
	// no proto model, so it round-trips through a bare Struct.
	PayloadStruct bool
	// EnvelopeShape is the declared JSON shape the route's collection arrives
	// in, SHAPE_UNSPECIFIED for the standard page every other route answers.
	EnvelopeShape linodev1.ListEnvelope_Shape
	// PagedWrite is true when a mutation's route answers with a page of a
	// collection rather than the resource it changed, so the answer is decoded
	// through the list envelope and packed into the count and the repeated
	// member the way a collection's is.
	PagedWrite bool
	// Meta is true for a tool that reaches no route and answers from local
	// state, which is what puts it on the meta tier rather than leaving it to
	// fall through the mutation shapes a routeless contract otherwise derives.
	Meta bool
}

// anyOfCheck is one resolved require_any_of declaration: the arguments the
// check reads and the sentence it refuses with, derived from the field names
// when the contract declares none.
type anyOfCheck struct {
	Message string
	Fields  []string
}

// anyOfMinimumFields is the fewest arguments a require_any_of may name: one
// field asking the presence question is ARGUMENT_READER_PRESENT. It is also
// the count whose derived sentence joins with a plain "or" instead of a list.
const anyOfMinimumFields = 2

// assembledMember is one response member an execute hook fills. The hook
// answers the tool's own response message, so the pair is what the emitter
// copies across: nothing else in that message is read, since every other member
// is either the declared sentence or an echo the call already holds.
type assembledMember struct {
	// GoName is the response struct field, "ThumbnailPngBase64".
	GoName string
	// ProtoName is its wire name, which is what a refusal reports.
	ProtoName string
}

// destroyEcho is one argument an answer carries back. The API returns nothing
// to decode, so the response is built from the call, and each scalar beside the
// message names the argument that fills it.
type destroyEcho struct {
	// Items is how a repeated named-message echo is rebuilt, nil for every
	// other shape: the response declares an item message of its own, so the
	// items the caller sent are mapped into it member by member.
	Items *echoItems
	// Nested is how a single named-message echo is rebuilt, nil for every other
	// shape. It carries no Arg because it names no one argument: each of its
	// members is filled from the argument spelled the same way.
	Nested *echoItems
	// GoName is the response struct field, "DomainId".
	GoName string
	// Arg is the input field whose value fills it.
	Arg field
}

// echoItems is the mapping one repeated named-message echo is rebuilt through.
type echoItems struct {
	// TypeName is the Go type of the response item, "IPAssignment".
	TypeName string
	// Members are the response item's fields in declaration order, which is the
	// order the assembled answer serializes them in.
	Members []echoMember
}

// echoMember is one field of a response item and the item member filling it.
type echoMember struct {
	// GoName is the response item's struct field, "LinodeId".
	GoName string
	// Argument is the input item member of the same name, "linode_id".
	Argument string
	Kind     protoreflect.Kind
}

// listFilter is one client-side filter a list tool applies, derived by matching
// a query argument against a field of the collection's element message.
type listFilter struct {
	Param string
	// Field is the element's proto field name, "domain", which is what a
	// renderer with no Go accessors names the match by.
	Field string
	// Getter is the element accessor the filter reads, "GetDomain".
	Getter      string
	Description string
	// Kind is the element field's declared kind, which picks how the value the
	// caller sent is compared: text one way, a flag another.
	Kind protoreflect.Kind
	// Match is how the value is compared, declared or derived.
	Match linodev1.ListFilterSpec_Match
}

// containsSuffix marks a substring filter: domain_contains matches Domain.domain.
const containsSuffix = "_contains"

// dryRunArgument is the local field a tool advertises a preview under.
const dryRunArgument = "dry_run"

// confirmArgument is the local field a tool advertises its gate under.
const confirmArgument = "confirm"

// The local fields a tool advertises the plan/apply flow under. Both or
// neither: one alone advertises a stage the caller cannot reach.
const (
	modeArgument   = "mode"
	planIDArgument = "plan_id"
)

// warningFieldName is the response member a declared warning_message is
// reported in. It is selected by name because the notice is the response's, not
// any one tool's: every mutation that carries one calls it `warning`.
const warningFieldName = "warning"

// messageFieldName is the member a tool reports its own prose in, which is what
// separates an envelope the tool assembles from the API resource itself.
const messageFieldName = "message"

// structFullName is the free-form object the API round-trips whole: profile
// preferences are whatever the caller stored, so there is no schema to model.
const structFullName protoreflect.FullName = "google.protobuf.Struct"

// wrapperSuffix names a response that is the tool's own envelope rather than
// the API's body, the same way a ListResponse names a page. VolumeGetResponse
// is the one on the surface: /volumes/{id} answers a bare volume and the tool
// has always answered {"volume": ...}, so the API body decodes into the member
// instead of into the envelope.
const wrapperSuffix = "GetResponse"

// envelopeSuffix names a message the tools own rather than one the API sends.
// Shape cannot say it: PlacementGroup holds a nested object too and is the real
// API resource, so a mutation answering with one has to be told apart by name.
const envelopeSuffix = "Response"

// isPaginationParam reports whether a query argument selects a page. The shared
// pagination reader owns those, so they never become client-side filters.
func isPaginationParam(name string) bool {
	return name == "page" || name == "page_size"
}

// markerArgument is the cursor a marker-paged route resumes from, which travels
// beside the page bound rather than instead of it.
const markerArgument = "marker"

// The response members a marker-paged answer reports its cursor in, named the
// same way the route's own body names them.
const (
	markerTruncatedMember = "is_truncated"
	markerNextMember      = "next_marker"
)

// isCursorParam reports whether a marker-paged route's query argument says where
// a page starts rather than which objects it holds. The answer echoes what it
// was narrowed by, and a position is not that.
func isCursorParam(name string) bool {
	return isPaginationParam(name) || name == markerArgument
}

// checkPageControlKinds holds a route's page controls to being the integers the
// shared reader reads. linode_object_storage_bucket_object_list publishes
// page_size as text and forwards it to S3 verbatim, so reading it as an integer
// would send a bound the caller never asked for beside the one they did.
//
// The marker tier is exempt because it never reads them: its bound is forwarded
// as declared, which is the whole reason that shape publishes text.
func (c *contract) checkPageControlKinds() error {
	if c.Tier == tierMarkerList {
		return nil
	}

	for _, entry := range c.Query {
		if !isPaginationParam(entry.ProtoName) {
			continue
		}

		if entry.Kind != protoreflect.Int32Kind && entry.Kind != protoreflect.Int64Kind {
			return fmt.Errorf("%w: %s declares %s as %s, and the page controls are read as integers",
				errUnsupportedQuery, c.Name, entry.ProtoName, entry.Kind)
		}
	}

	return nil
}

// readContracts returns the contract for every tool in cohort, in cohort order.
// The cohort is derived from declared, so a name missing from it means the two
// were read from different descriptor sets rather than that a list has a typo.
func readContracts(
	cohort []string,
	declared map[string]protoreflect.MessageDescriptor,
	docs schemaDocs,
) ([]contract, error) {
	found := make([]contract, 0, len(cohort))

	for _, name := range cohort {
		message, listed := declared[name]
		if !listed {
			return nil, fmt.Errorf("%w: %s", errToolNotDeclared, name)
		}

		built, buildErr := buildContract(name, message, docs)
		if buildErr != nil {
			return nil, buildErr
		}

		if readErr := built.resolveStateRead(declared); readErr != nil {
			return nil, readErr
		}

		found = append(found, built)
	}

	return found, nil
}

// resolveStateRead names the read a removal that declares no fetch_state hook
// synthesizes its state fetch from: the tool declared beside it that GETs the
// same route.
//
// A per-tool hook for this would be a hand-written line per delete, which is
// what the generator exists to remove, while a removal with no state fetch at
// all would preview and hash an empty resource. The contract already says both
// halves: the delete names its route, and a read on that same route names the
// message its answer decodes into.
func (c *contract) resolveStateRead(declared map[string]protoreflect.MessageDescriptor) error {
	if c.StateRouteDecl != nil {
		return c.resolveStateRoute(declared)
	}

	if !c.removesResource() || c.Hooks.FetchState != "" {
		return nil
	}

	sibling, response, found := siblingRead(c, declared)
	if !found {
		return c.resolveCollectionStateRead(declared)
	}

	message, err := lookupGoMessage(response)
	if err != nil {
		return err
	}

	c.StateRead = stateRead{Tool: sibling, Message: message}

	return nil
}

// resolveCollectionStateRead names the collection a removal reads its state out
// of when the API publishes no GET on the route it deletes.
//
// Three certificate routes exist on an identity-provider configuration and none
// of them reads one certificate, so an exact-path sibling finds nothing and the
// removal would have no state to preview or hash. The contract still says where
// the resource can be read: the DELETE's path is one segment past a collection
// declared beside it, and the trailing segment is the id an element of that
// collection carries. The parent's ids fill the list route, and the trailing one
// picks the element out of the page.
//
// The exact-path sibling stays preferred where both exist: reading one resource
// is a smaller answer than reading the page it sits in, and it needs no
// selection to be right.
func (c *contract) resolveCollectionStateRead(declared map[string]protoreflect.MessageDescriptor) error {
	ordered, err := c.orderedPathFields()
	if err != nil {
		return err
	}

	if len(ordered) < collectionRemovalIDs {
		return nil
	}

	sibling, response, found := collectionSiblingRead(c, declared)
	if !found {
		return nil
	}

	page, err := lookupGoMessage(response)
	if err != nil {
		return err
	}

	element, ok, err := pageElementMessage(&page)
	if err != nil || !ok {
		return err
	}

	trailing := ordered[len(ordered)-1]
	if element.goNameOf(elementIDField) == "" || element.kinds[elementIDField] != trailing.Kind {
		return fmt.Errorf("%w: %s selects by %s (%s), which %s does not carry",
			errNoFetchState, c.Name, elementIDField, trailing.Kind, element.FullName)
	}

	c.StateRead = stateRead{Tool: sibling, Message: element, Select: element.goNameOf(elementIDField)}

	return nil
}

// collectionRemovalIDs is the fewest path ids a removal served by a collection
// sibling carries: the collection's own, plus the one that names the element.
const collectionRemovalIDs = 2

// elementIDField is the element member a collection sibling selects on. Every
// Linode collection names its element's identifier this way, so the selection
// is derived rather than declared per removal.
const elementIDField = "id"

// collectionSiblingRead finds the collection declared beside a removal whose
// route is the removal's parent: same proto file, GET, and a page to select an
// element out of.
//
// Sorted for the reason siblingRead sorts, and refusing a route that is not
// exactly the parent for the reason it refuses one that is not exactly the
// same: a fetch addressed at something else reports the wrong resource.
func collectionSiblingRead(
	tool *contract, declared map[string]protoreflect.MessageDescriptor,
) (string, protoreflect.FullName, bool) {
	parent := parentPath(tool.PathTemplate)
	if parent == "" {
		return "", "", false
	}

	names := make([]string, 0, len(declared))
	for name := range declared {
		names = append(names, name)
	}

	slices.Sort(names)

	for _, name := range names {
		message := declared[name]
		if name == tool.Name || message.ParentFile().Path() != tool.SourceFile {
			continue
		}

		options := message.Options()

		route, _ := proto.GetExtension(options, linodev1.E_ToolRoute).(*linodev1.ToolRoute)
		if route.GetMethod() != http.MethodGet || route.GetPath() != parent {
			continue
		}

		if capabilityOption(options) != linodev1.ToolCapability_TOOL_CAPABILITY_READ {
			continue
		}

		response := stringOption(options, linodev1.E_ToolResponse)
		if response == "" {
			continue
		}

		return name, protoreflect.FullName(response), true
	}

	return "", "", false
}

// parentPath is the route template one segment up, empty when the template has
// no segment to drop.
func parentPath(template string) string {
	cut := strings.LastIndex(template, "/")
	if cut <= 0 {
		return ""
	}

	return template[:cut]
}

// pageElementMessage is the element a list response carries, reported absent
// when the message is not a page.
func pageElementMessage(page *goMessage) (goMessage, bool, error) {
	repeated := page.repeatedFields()
	if page.goNameOf("count") == "" || len(repeated) != 1 {
		return goMessage{}, false, nil
	}

	element, err := lookupGoMessage(repeated[0].message)
	if err != nil {
		return goMessage{}, false, err
	}

	return element, true, nil
}

// siblingRead finds the read declared beside a removal on the same route: same
// proto file, same path, GET, and an answer of its own to decode into.
//
// Sorted rather than map order because two reads on one route would otherwise
// pick differently per run, and a generator whose output moves is worse than
// one that picks the wrong sibling in a way a reader can see.
func siblingRead(
	tool *contract, declared map[string]protoreflect.MessageDescriptor,
) (string, protoreflect.FullName, bool) {
	names := make([]string, 0, len(declared))
	for name := range declared {
		names = append(names, name)
	}

	slices.Sort(names)

	for _, name := range names {
		message := declared[name]
		if name == tool.Name || message.ParentFile().Path() != tool.SourceFile {
			continue
		}

		options := message.Options()

		route, _ := proto.GetExtension(options, linodev1.E_ToolRoute).(*linodev1.ToolRoute)
		if route.GetMethod() != http.MethodGet || route.GetPath() != tool.PathTemplate {
			continue
		}

		if capabilityOption(options) != linodev1.ToolCapability_TOOL_CAPABILITY_READ {
			continue
		}

		response := stringOption(options, linodev1.E_ToolResponse)
		if response == "" {
			continue
		}

		return name, protoreflect.FullName(response), true
	}

	return "", "", false
}

// declaredTools maps every tool name the contract declares to its input message
// descriptor, meta tools included.
//
// Meta tools are here because the hand-written list is what scopes this run, and
// that list covers the whole surface: leaving them out would make every meta
// entry on it read as a typo. A meta tool that reached the cohort is refused by
// buildContract instead, which is the honest report that no tier emits one yet.
func declaredTools() (map[string]protoreflect.MessageDescriptor, error) {
	found := make(map[string]protoreflect.MessageDescriptor)

	var conflict error

	walkMessages(func(message protoreflect.MessageDescriptor) bool {
		name := declaredToolName(message)
		if name == "" {
			return true
		}

		if first, taken := found[name]; taken {
			conflict = fmt.Errorf("%w: %s claimed by %s and %s",
				errToolNotDeclared, name, first.FullName(), message.FullName())

			return false
		}

		found[name] = message

		return true
	})

	return found, conflict
}

// declaredToolName reads the tool one message belongs to out of whichever
// marker names it, "" when neither does.
func declaredToolName(message protoreflect.MessageDescriptor) string {
	if route, ok := proto.GetExtension(
		message.Options(), linodev1.E_ToolRoute,
	).(*linodev1.ToolRoute); ok && route.GetTool() != "" {
		return route.GetTool()
	}

	if meta, ok := proto.GetExtension(
		message.Options(), linodev1.E_ToolMeta,
	).(*linodev1.ToolMeta); ok {
		return meta.GetTool()
	}

	return ""
}

// walkMessages hands every top-level message of this repo's proto package to
// visit, stopping early when visit returns false.
func walkMessages(visit func(message protoreflect.MessageDescriptor) bool) {
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if file.Package() != protoPackage {
			return true
		}

		messages := file.Messages()
		for i := range messages.Len() {
			if !visit(messages.Get(i)) {
				return false
			}
		}

		return true
	})
}

// buildContract reads one tool's whole declaration.
func buildContract(name string, message protoreflect.MessageDescriptor, docs schemaDocs) (contract, error) {
	options := message.Options()

	route, _ := proto.GetExtension(options, linodev1.E_ToolRoute).(*linodev1.ToolRoute)
	capability := capabilityOption(options)

	// A routeless tool is served by the meta tier and nothing else. Every other
	// tier builds a request, and a contract with no route to build one against
	// falls through this reader into the shape whose answer needs the least
	// from the API, which would register a mutation that calls nothing.
	if route.GetTool() == "" && capability != linodev1.ToolCapability_TOOL_CAPABILITY_META {
		return contract{}, fmt.Errorf("%w: %s", errMetaNotEmitted, name)
	}

	// Resolved through the same function the client resolves it with, so a
	// surface the client could not address cannot be generated against either.
	surface, err := linoderoute.SurfaceSegment(surfaceOption(options))
	if err != nil {
		return contract{}, fmt.Errorf("%w: %s", err, name)
	}

	built := contract{
		Name:           name,
		Meta:           route.GetTool() == "",
		SourceFile:     message.ParentFile().Path(),
		Method:         route.GetMethod(),
		Surface:        surface,
		PathTemplate:   route.GetPath(),
		Slots:          routeSlots(route.GetPath()),
		InputMessage:   string(message.FullName()),
		Description:    surfaceMarked(surface, stringOption(options, linodev1.E_ToolDescription)),
		ErrorMessage:   stringOption(options, linodev1.E_ErrorMessage),
		ConfirmMessage: stringOption(options, linodev1.E_ConfirmMessage),
		SuccessMessage: stringOption(options, linodev1.E_SuccessMessage),
		WarningMessage: stringOption(options, linodev1.E_WarningMessage),
		ResourceType:   stringOption(options, linodev1.E_ResourceType),
		ResponseBody:   stringsOption(options, linodev1.E_ResponseBodyFields),
		Capability:     capability,
		RetryDisabled:  boolOption(options, linodev1.E_RetryDisabled),
		// Presence rather than text, so a response answering with a deliberately
		// empty notice can say so instead of reading as undeclared.
		WarningDeclared: proto.HasExtension(options, linodev1.E_WarningMessage),
		EnvelopeShape:   listEnvelopeShape(options),
		Constants:       bodyConstants(options),
	}

	if built.Description == "" {
		return contract{}, fmt.Errorf("%w: %s", errNoDescription, name)
	}

	hooks, err := readHooks(name, options)
	if err != nil {
		return contract{}, err
	}

	built.Hooks = hooks

	if err := built.readFields(message, docs); err != nil {
		return contract{}, err
	}

	if err := built.deriveFolds(); err != nil {
		return contract{}, err
	}

	if err := built.readResponse(options); err != nil {
		return contract{}, err
	}

	if built.WarningDeclared && built.WarningField == "" {
		return contract{}, fmt.Errorf("%w: %s answers with %s, which has no warning field to report it in",
			errUnusedWarningMessage, name, built.ResponseGo.FullName)
	}

	built.deriveTier()

	if err := built.checkReaderHookExclusion(); err != nil {
		return contract{}, err
	}

	if err := built.readArgumentDeclarations(options); err != nil {
		return contract{}, err
	}

	if err := built.checkTwoStage(); err != nil {
		return contract{}, err
	}

	if err := built.readExplicitNulls(options); err != nil {
		return contract{}, err
	}

	if err := built.readStateRoute(options); err != nil {
		return contract{}, err
	}

	if err := built.readPreviewSentences(options); err != nil {
		return contract{}, err
	}

	if err := built.derivePaging(); err != nil {
		return contract{}, err
	}

	return built, nil
}

// derivePaging settles what the tool's query arguments mean, which is the last
// thing a build does: the page controls and the filters are read off arguments
// every earlier step has already placed and checked.
func (c *contract) derivePaging() error {
	if err := c.checkPageControlKinds(); err != nil {
		return err
	}

	if err := c.deriveFilters(); err != nil {
		return err
	}

	if err := c.checkPageNullFilters(); err != nil {
		return err
	}

	return c.checkQueryPlaced()
}

// checkQueryPlaced refuses a QUERY argument on a tier that builds no query, so a
// parameter the schema advertises cannot be read out of the request and dropped
// before the call. The write tier forwards its own; the reads split theirs into
// page controls, forwarded parameters and client-side filters. The rest send
// the route and a body, and nothing there has ever carried a query.
func (c *contract) checkQueryPlaced() error {
	switch c.Tier {
	case tierGet, tierList, tierSubresourceList, tierMarkerList, tierWrite:
		return nil
	case tierBodyRead, tierAcknowledge, tierDestroy, tierMeta, tierUnknown:
	}

	if len(c.Query) == 0 {
		return nil
	}

	return fmt.Errorf("%w: %s publishes %s", errUnsupportedQuery, c.Name, queryNames(c.Query))
}

// readFields sorts the input's fields into the places the contract says they
// travel, keeping each group in declaration order. Body order is the wire order,
// so it is the one group whose order a reader can observe. Local fields are not
// collected: they never leave the MCP layer.
func (c *contract) readFields(message protoreflect.MessageDescriptor, docs schemaDocs) error {
	fields := message.Fields()

	for i := range fields.Len() {
		descriptor := fields.Get(i)
		name := string(descriptor.Name())

		entry := field{
			ProtoName:   name,
			GoLocal:     goLocalName(name),
			Description: docs.describe(c.InputMessage, name),
			Location:    fieldLocation(descriptor),
			Filter:      fieldFilter(descriptor),
			Kind:        descriptor.Kind(),
			Presence:    descriptor.HasPresence() || descriptor.IsList(),
			Repeated:    descriptor.IsList(),
			BodyName:    stringOption(descriptor.Options(), linodev1.E_BodyName),
			ObjectMap:   isObjectMap(descriptor),
			StringMap:   isStringMap(descriptor),
			Nullable:    fieldNullable(descriptor),
			ObjectList:  isObjectList(descriptor),
			ItemMessage: itemMessage(descriptor),
			Message:     bodyMessage(descriptor),
			Redact:      fieldRedacted(descriptor),
			CommaList:   fieldCommaList(descriptor),
			BodyRoot:    fieldBodyRoot(descriptor),
		}

		c.AllArguments = append(c.AllArguments, name)

		c.recordFold(name, descriptor)

		if err := c.recordReader(name, descriptor); err != nil {
			return err
		}

		if err := c.checkFieldOptions(&entry); err != nil {
			return err
		}

		switch entry.Location {
		case linodev1.FieldLocation_FIELD_LOCATION_PATH:
			c.Path = append(c.Path, entry)
		case linodev1.FieldLocation_FIELD_LOCATION_QUERY:
			c.Query = append(c.Query, entry)
		case linodev1.FieldLocation_FIELD_LOCATION_BODY:
			c.Body = append(c.Body, entry)
		case linodev1.FieldLocation_FIELD_LOCATION_LOCAL:
			c.DryRun = c.DryRun || name == dryRunArgument
			c.Confirm = c.Confirm || name == confirmArgument
			c.Mode = c.Mode || name == modeArgument
			c.PlanID = c.PlanID || name == planIDArgument
		case linodev1.FieldLocation_FIELD_LOCATION_TOOL:
			// The meta tier places one, and so does a tool whose hook owns the
			// call: an upload's source path names bytes the hook reads and the
			// request never carries. Every other routed tier builds its request
			// out of PATH, QUERY and BODY, so a domain argument declared there
			// would be advertised in the schema and dropped before the call.
			if !c.Meta && c.Hooks.Execute == "" {
				return fmt.Errorf("%w: %s field %s", errRoutedToolArgument, c.Name, name)
			}

			c.Local = append(c.Local, entry)
		case linodev1.FieldLocation_FIELD_LOCATION_UNSPECIFIED:
			return fmt.Errorf("%w: %s field %s", errNoFieldLocation, c.Name, name)
		}
	}

	return nil
}

// recordFold files the assembly one member declares, keyed by the member rather
// than kept on the field, which is copied per iteration wherever the emitter
// walks a tool's arguments.
func (c *contract) recordFold(name string, descriptor protoreflect.FieldDescriptor) {
	fold := foldOption(descriptor)
	if fold == nil {
		return
	}

	if c.FoldDecls == nil {
		c.FoldDecls = make(map[string]*linodev1.BodyFold, 1)
	}

	c.FoldDecls[name] = fold
}

// recordReader files the shared argument check one field declares, if it
// declares one, and resolves the vocabulary a membership check reads: the enum
// descriptor's own members, or the reader_values a string field declares.
func (c *contract) recordReader(name string, descriptor protoreflect.FieldDescriptor) error {
	values, _ := proto.GetExtension(
		descriptor.Options(), linodev1.E_ReaderValues,
	).([]string)

	sentences, _ := proto.GetExtension(
		descriptor.Options(), linodev1.E_ReaderMessage,
	).(*linodev1.ReaderMessage)

	reader, ok := proto.GetExtension(
		descriptor.Options(), linodev1.E_ArgumentReader,
	).(linodev1.ArgumentReader)
	if !ok || reader == linodev1.ArgumentReader_ARGUMENT_READER_UNSPECIFIED {
		if len(values) > 0 {
			return fmt.Errorf("%w: %s field %s", errReaderValuesAlone, c.Name, name)
		}

		if wordsAnArm(sentences) {
			return fmt.Errorf("%w: %s field %s", errReaderMessageAlone, c.Name, name)
		}

		return nil
	}

	if err := checkReaderArms(c.Name, name, reader, sentences); err != nil {
		return err
	}

	if wordsAnArm(sentences) {
		if c.Sentences == nil {
			c.Sentences = make(map[string]*linodev1.ReaderMessage, 1)
		}

		c.Sentences[name] = sentences
	}

	if reader != linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER && len(values) > 0 {
		return fmt.Errorf("%w: %s field %s", errReaderValuesAlone, c.Name, name)
	}

	if reader == linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER {
		members, err := c.memberVocabulary(name, descriptor, values)
		if err != nil {
			return err
		}

		if c.Members == nil {
			c.Members = make(map[string][]string, 1)
		}

		c.Members[name] = members
	}

	if c.Readers == nil {
		c.Readers = make(map[string]linodev1.ArgumentReader, 1)
	}

	c.Readers[name] = reader

	return nil
}

// wordsAnArm reports whether a declaration words any refusal at all, which is
// what separates a field carrying the option from one that only mentions it.
func wordsAnArm(sentences *linodev1.ReaderMessage) bool {
	if sentences == nil {
		return false
	}

	return sentences.GetAbsent() != "" || sentences.GetUnusable() != "" || sentences.GetRefused() != ""
}

// checkReaderArms holds a declaration's wording to the refusals its member
// actually answers. A member that never tells an unusable value from an absent
// one has nowhere to put a second sentence, so wording one there is a sentence
// no caller can reach, which is the same defect as a reader declared and never
// acted on.
func checkReaderArms(tool, name string, reader linodev1.ArgumentReader, sentences *linodev1.ReaderMessage) error {
	if !wordsAnArm(sentences) {
		return nil
	}

	var unwordable string

	switch reader {
	case linodev1.ArgumentReader_ARGUMENT_READER_POSITIVE_ID,
		linodev1.ArgumentReader_ARGUMENT_READER_BOUNDED_ID,
		linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER:
		if sentences.GetUnusable() != "" {
			unwordable = "unusable"
		}
	case linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_BOOL,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_STRING:
		if sentences.GetRefused() != "" {
			unwordable = "refused"
		}
	case linodev1.ArgumentReader_ARGUMENT_READER_PRESENT:
		if sentences.GetUnusable() != "" || sentences.GetRefused() != "" {
			unwordable = "unusable or refused"
		}
	case linodev1.ArgumentReader_ARGUMENT_READER_FREE_TEXT:
		if sentences.GetRefused() != "" {
			unwordable = "refused"
		}
	case linodev1.ArgumentReader_ARGUMENT_READER_PATH_SAFE_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_FRAGMENT_SAFE_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_SERVICE_TYPE_SLUG,
		linodev1.ArgumentReader_ARGUMENT_READER_REGION_SLUG,
		linodev1.ArgumentReader_ARGUMENT_READER_BETA_SLUG,
		linodev1.ArgumentReader_ARGUMENT_READER_ID_LIST,
		linodev1.ArgumentReader_ARGUMENT_READER_UNSPECIFIED:
	}

	if unwordable != "" {
		return fmt.Errorf("%w: %s field %s words %s for %s",
			errReaderMessageArm, tool, name, unwordable, reader)
	}

	return nil
}

// readAnyOf resolves the message-level require_any_of declaration, deriving
// the refusal sentence from the field names when the contract declares none.
// It runs after the tier is derived because only the write tier emits the
// check, so anywhere else the declaration would be read and dropped.
func (c *contract) readAnyOf(options protoreflect.ProtoMessage) error {
	declared, ok := proto.GetExtension(options, linodev1.E_RequireAnyOf).(*linodev1.RequireAnyOf)
	if !ok || declared == nil || (len(declared.GetFields()) == 0 && declared.GetMessage() == "") {
		return nil
	}

	if c.Tier != tierWrite {
		return fmt.Errorf("%w: %s", errAnyOfTier, c.Name)
	}

	if c.Hooks.Validate != "" {
		return fmt.Errorf("%w: %s", errAnyOfWithValidate, c.Name)
	}

	names := declared.GetFields()
	if len(names) < anyOfMinimumFields {
		return fmt.Errorf("%w: %s names %v", errAnyOfTooFew, c.Name, names)
	}

	for index, name := range names {
		if !slices.ContainsFunc(c.Body, func(entry field) bool { return entry.ProtoName == name }) {
			return fmt.Errorf("%w: %s names %s", errAnyOfUnknownField, c.Name, name)
		}

		if slices.Contains(names[:index], name) {
			return fmt.Errorf("%w: %s names %s", errAnyOfDuplicateField, c.Name, name)
		}
	}

	sentence := declared.GetMessage()
	if sentence == "" {
		sentence = anyOfSentence(names)
	}

	c.AnyOf = &anyOfCheck{Message: sentence, Fields: names}

	return nil
}

// checkReaderHookExclusion refuses a declared reader beside a validate hook.
//
// The two cannot both be honored: a hook owns the whole argument check, so Go
// writes no path reads at all for a tool that declares one, and a reader
// declared beside it is silently dropped there while Python still acts it. A
// declaration that means one thing in one language and nothing in the other is
// worse than either answer, so the pair fails the build the way require_any_of
// beside a hook does. Retiring a hook and declaring its readers stays one
// change, which is how every migration has landed.
func (c *contract) checkReaderHookExclusion() error {
	if c.Hooks.Validate == "" || len(c.Readers) == 0 {
		return nil
	}

	named := make([]string, 0, len(c.Readers))
	for name := range c.Readers {
		named = append(named, name)
	}

	slices.Sort(named)

	return fmt.Errorf("%w: %s declares one on %v", errReaderWithValidate, c.Name, named)
}

// readRefusals resolves the two whole-map refusals a tool declares: the
// argument names it answers for rather than sends, and whether it takes any
// name its input message does not declare.
//
// Both are refused beside a validate hook for the reason require_any_of is: a
// tool declaring one owns its whole argument check, so a second answer here
// would be one nothing holds to the hook's wording.
func (c *contract) readRefusals(options protoreflect.ProtoMessage) error {
	refused, _ := proto.GetExtension(options, linodev1.E_RefuseArguments).(*linodev1.RefuseArguments)
	if refused != nil && (len(refused.GetFields()) > 0 || refused.GetMessage() != "") {
		if err := c.acceptRefusal(refused.GetFields(), refused.GetMessage(), errRefuseArguments); err != nil {
			return err
		}

		c.Refused = refused
	}

	unknown, _ := proto.GetExtension(
		options, linodev1.E_RefuseUnknownArguments,
	).(*linodev1.RefuseUnknownArguments)
	if unknown == nil || unknown.GetMessage() == "" {
		return nil
	}

	if c.Hooks.Validate != "" {
		return fmt.Errorf("%w: %s", errRefuseUnknownWithValidate, c.Name)
	}

	c.RefuseUnknown = unknown

	return nil
}

// acceptRefusal holds a refuse_arguments declaration to naming something the
// message does not, which is the whole reason it exists: a name the message
// declares is checked by its own field, and refusing it here would answer twice
// for one argument.
func (c *contract) acceptRefusal(names []string, sentence string, unnamed error) error {
	if len(names) == 0 || sentence == "" {
		return fmt.Errorf("%w: %s", unnamed, c.Name)
	}

	if c.Hooks.Validate != "" {
		return fmt.Errorf("%w: %s", errRefuseWithValidate, c.Name)
	}

	for _, name := range names {
		if slices.Contains(c.AllArguments, name) {
			return fmt.Errorf("%w: %s names %s", errRefuseDeclaredField, c.Name, name)
		}
	}

	return nil
}

// anyOfSentence is the refusal a require_any_of derives from its field names,
// spelled the way the hooks it replaces already spelled it: a plain "or" for
// two fields, a comma list for more.
func anyOfSentence(names []string) string {
	if len(names) == anyOfMinimumFields {
		return "at least one of " + names[0] + " or " + names[1] + " is required"
	}

	listed := strings.Join(names[:len(names)-1], ", ")

	return "at least one of " + listed + ", or " + names[len(names)-1] + " is required"
}

// memberVocabulary is the value set one ENUM_MEMBER field is held to. An enum
// field carries it in its descriptor, so declared values beside it are refused
// as a second vocabulary; a string field has to declare it.
func (c *contract) memberVocabulary(
	name string, descriptor protoreflect.FieldDescriptor, values []string,
) ([]string, error) {
	switch descriptor.Kind() {
	case protoreflect.EnumKind:
		if len(values) > 0 {
			return nil, fmt.Errorf("%w: %s field %s", errReaderValuesOnEnum, c.Name, name)
		}

		return enumMemberNames(descriptor.Enum()), nil
	case protoreflect.StringKind:
		if len(values) == 0 {
			return nil, fmt.Errorf("%w: %s field %s", errEnumMemberNoValues, c.Name, name)
		}

		return values, nil
	case protoreflect.BoolKind, protoreflect.Int32Kind, protoreflect.Sint32Kind,
		protoreflect.Uint32Kind, protoreflect.Int64Kind, protoreflect.Sint64Kind,
		protoreflect.Uint64Kind, protoreflect.Sfixed32Kind, protoreflect.Fixed32Kind,
		protoreflect.FloatKind, protoreflect.Sfixed64Kind, protoreflect.Fixed64Kind,
		protoreflect.DoubleKind, protoreflect.BytesKind, protoreflect.MessageKind,
		protoreflect.GroupKind:
	}

	return nil, fmt.Errorf("%w: %s field %s", errEnumMemberKind, c.Name, name)
}

// enumMemberNames lists an enum's declared values without the zero sentinel,
// in number order, which is the order the refusal sentence reads them in and
// the order tools.EnumChoiceError has always answered.
func enumMemberNames(enum protoreflect.EnumDescriptor) []string {
	values := enum.Values()
	names := make([]string, 0, values.Len())

	for i := range values.Len() {
		if values.Get(i).Number() != 0 {
			names = append(names, string(values.Get(i).Name()))
		}
	}

	slices.SortFunc(names, func(left, right string) int {
		return int(enum.Values().ByName(protoreflect.Name(left)).Number() -
			enum.Values().ByName(protoreflect.Name(right)).Number())
	})

	return names
}

// checkFieldOptions holds one field's declarations to the location that can
// serve them, before it is filed under where it travels.
func (c *contract) checkFieldOptions(entry *field) error {
	if entry.Redact && entry.Location != linodev1.FieldLocation_FIELD_LOCATION_BODY {
		return fmt.Errorf("%w: %s field %s", errRedactNotBody, c.Name, entry.ProtoName)
	}

	if err := checkBodyNullable(c.Name, entry); err != nil {
		return err
	}

	if err := checkBodyName(c.Name, entry); err != nil {
		return err
	}

	if err := c.checkBodyRoot(entry); err != nil {
		return err
	}

	if err := c.checkArgumentReader(entry); err != nil {
		return err
	}

	return checkBodyCommaList(c.Name, entry)
}

// checkArgumentReader holds a declared reader to a field it can actually read.
// The positive-id reader answers "<name> is required" and "<name> must be a
// positive integer", which are sentences about an id addressing a resource, so
// on a body member or a string it would refuse values on grounds the caller was
// never held to.
func (c *contract) checkArgumentReader(entry *field) error {
	reader, declared := c.Readers[entry.ProtoName]
	if !declared {
		return nil
	}

	if handled, err := c.checkMapReader(reader, entry); handled {
		return err
	}

	if entry.Location != linodev1.FieldLocation_FIELD_LOCATION_PATH {
		return fmt.Errorf("%w: %s field %s", errReaderNotPath, c.Name, entry.ProtoName)
	}

	// Each member reads one shape. A number reader on a slug refuses every value
	// the route accepts; a text reader on an id lets through one the path cannot
	// carry.
	switch reader {
	case linodev1.ArgumentReader_ARGUMENT_READER_POSITIVE_ID,
		linodev1.ArgumentReader_ARGUMENT_READER_BOUNDED_ID:
		if !integerKind(entry.Kind) {
			return fmt.Errorf("%w: %s field %s", errReaderNotPathInteger, c.Name, entry.ProtoName)
		}
	case linodev1.ArgumentReader_ARGUMENT_READER_PATH_SAFE_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_FRAGMENT_SAFE_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_SERVICE_TYPE_SLUG,
		linodev1.ArgumentReader_ARGUMENT_READER_REGION_SLUG,
		linodev1.ArgumentReader_ARGUMENT_READER_BETA_SLUG,
		linodev1.ArgumentReader_ARGUMENT_READER_FREE_TEXT:
		if entry.Kind != protoreflect.StringKind {
			return fmt.Errorf("%w: %s field %s", errReaderNotPathText, c.Name, entry.ProtoName)
		}
	case linodev1.ArgumentReader_ARGUMENT_READER_PRESENT,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_BOOL,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_STRING,
		linodev1.ArgumentReader_ARGUMENT_READER_ID_LIST,
		linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER,
		linodev1.ArgumentReader_ARGUMENT_READER_UNSPECIFIED:
	}

	return nil
}

// checkMapReader holds the members that read the argument map to the shapes
// they can answer for, reporting handled for exactly those members so the path
// checks never see them. PRESENT asks only where a value travels; its TEXT and
// BOOL kin each read one body shape, and ENUM_MEMBER reads one raw argument
// from a path slot or a body member.
func (c *contract) checkMapReader(reader linodev1.ArgumentReader, entry *field) (bool, error) {
	body := entry.Location == linodev1.FieldLocation_FIELD_LOCATION_BODY

	switch reader {
	case linodev1.ArgumentReader_ARGUMENT_READER_PRESENT:
		if !body {
			return true, fmt.Errorf("%w: %s field %s", errPresentNotBody, c.Name, entry.ProtoName)
		}
	case linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_BOOL,
		linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_STRING,
		linodev1.ArgumentReader_ARGUMENT_READER_ID_LIST:
		return true, c.checkValueShape(reader, entry)
	case linodev1.ArgumentReader_ARGUMENT_READER_ENUM_MEMBER:
		if !body && entry.Location != linodev1.FieldLocation_FIELD_LOCATION_PATH {
			return true, fmt.Errorf("%w: %s field %s", errEnumMemberPlacement, c.Name, entry.ProtoName)
		}

		if entry.Repeated {
			return true, fmt.Errorf("%w: %s field %s", errEnumMemberRepeated, c.Name, entry.ProtoName)
		}
	case linodev1.ArgumentReader_ARGUMENT_READER_POSITIVE_ID,
		linodev1.ArgumentReader_ARGUMENT_READER_BOUNDED_ID,
		linodev1.ArgumentReader_ARGUMENT_READER_PATH_SAFE_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_FRAGMENT_SAFE_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_SERVICE_TYPE_SLUG,
		linodev1.ArgumentReader_ARGUMENT_READER_REGION_SLUG,
		linodev1.ArgumentReader_ARGUMENT_READER_BETA_SLUG,
		linodev1.ArgumentReader_ARGUMENT_READER_FREE_TEXT,
		linodev1.ArgumentReader_ARGUMENT_READER_UNSPECIFIED:
		return false, nil
	}

	return true, nil
}

// checkValueShape holds the four members that read one body value to the shape
// each of them can answer for. They are split from checkMapReader because that
// function's question is which members read the argument map at all, and this
// one's is what each of those members needs to find there.
func (c *contract) checkValueShape(reader linodev1.ArgumentReader, entry *field) error {
	body := entry.Location == linodev1.FieldLocation_FIELD_LOCATION_BODY
	singularText := body && entry.Kind == protoreflect.StringKind && !entry.Repeated

	switch reader {
	case linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_TEXT:
		if !singularText {
			return fmt.Errorf("%w: %s field %s", errPresentTextShape, c.Name, entry.ProtoName)
		}
	case linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_STRING:
		if !singularText {
			return fmt.Errorf("%w: %s field %s", errPresentStringShape, c.Name, entry.ProtoName)
		}
	case linodev1.ArgumentReader_ARGUMENT_READER_PRESENT_BOOL:
		// A query flag is asked the same question as a body member: whether the
		// caller sent one, which only the argument map answers.
		query := entry.Location == linodev1.FieldLocation_FIELD_LOCATION_QUERY
		if (!body && !query) || entry.Kind != protoreflect.BoolKind || entry.Repeated {
			return fmt.Errorf("%w: %s field %s", errPresentBoolShape, c.Name, entry.ProtoName)
		}
	case linodev1.ArgumentReader_ARGUMENT_READER_ID_LIST:
		if !body || !entry.Repeated || !integerKind(entry.Kind) {
			return fmt.Errorf("%w: %s field %s", errIDListShape, c.Name, entry.ProtoName)
		}
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

	return nil
}

// integerKind reports whether a field reads its argument as a whole number,
// which is what the positive-id reader hands back.
func integerKind(kind protoreflect.Kind) bool {
	switch kind {
	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Fixed32Kind, protoreflect.Fixed64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		return true
	case protoreflect.BoolKind, protoreflect.EnumKind, protoreflect.FloatKind,
		protoreflect.DoubleKind, protoreflect.StringKind, protoreflect.BytesKind,
		protoreflect.MessageKind, protoreflect.GroupKind:
		return false
	}

	return false
}

// checkBodyRoot holds a hoisted object to the one shape that can be one: a
// free-form BODY map, on a tool declaring no other body member. A second member
// would have nowhere to travel once the map's own keys are the body.
//
// The BODY fields read so far are what it counts, which is every one declared
// ahead of this field, plus the emitted body build refuses a later one the same
// way. Reading the whole message here would mean reading the fields twice.
func (c *contract) checkBodyRoot(entry *field) error {
	if !entry.BodyRoot {
		if entry.Location == linodev1.FieldLocation_FIELD_LOCATION_BODY &&
			len(c.Body) > 0 && c.Body[0].BodyRoot {
			return fmt.Errorf("%w: %s also declares %s",
				errBodyRootNotAlone, c.Name, entry.ProtoName)
		}

		return nil
	}

	if entry.Location != linodev1.FieldLocation_FIELD_LOCATION_BODY || !entry.ObjectMap {
		return fmt.Errorf("%w: %s field %s", errBodyRootNotObject, c.Name, entry.ProtoName)
	}

	if len(c.Body) > 0 {
		return fmt.Errorf("%w: %s also declares %s",
			errBodyRootNotAlone, c.Name, c.Body[0].ProtoName)
	}

	return nil
}

// readResponse resolves the message a tool answers with, and for a collection
// the element inside it. A missing response is the free-form shape, which only
// a read has an answer for.
func (c *contract) readResponse(options protoreflect.ProtoMessage) error {
	responseName := stringOption(options, linodev1.E_ToolResponse)
	if responseName == "" {
		return c.readStructResponse()
	}

	response, err := lookupGoMessage(protoreflect.FullName(responseName))
	if err != nil {
		return err
	}

	c.ResponseGo = response

	if c.Meta {
		return c.readMetaResponse(&response)
	}

	if !c.readsResource() {
		if c.removesResource() {
			if err := c.checkNoLocalMembers(&response); err != nil {
				return err
			}

			return c.readDestroyEnvelope(&response)
		}

		return c.readWriteEnvelope(&response)
	}

	// Ahead of the guard because an execute hook IS the assembly step: the route
	// answers with something no decode can place, so one member is what the hook
	// brings back rather than a member the API filled.
	if c.Hooks.Execute != "" {
		return c.readAssembledRead(&response)
	}

	if err := c.checkNoLocalMembers(&response); err != nil {
		return err
	}

	return c.readEnvelope(&response)
}

// readAssembledRead records the answer of a read whose call a hook makes: the
// OAuth client thumbnail route sends raw PNG bytes, so nothing decodes into the
// response and the whole answer is built here.
//
// The LOCAL members carry what the hook brought back, declared LOCAL because no
// other declaration separates them from a member the API fills. The rest echo
// the arguments the call was addressed by, resolved by the same rule an
// acknowledged mutation's echoes are, so a response naming a value the call was
// never given fails the run rather than reaching a client as a blank.
func (c *contract) readAssembledRead(response *goMessage) error {
	if len(response.messageFields())+len(response.repeatedFields()) > 0 {
		return fmt.Errorf("%w: %s answers with %s, which carries a message no hook can fill",
			errNotAnAssembledRead, c.Name, response.FullName)
	}

	if err := c.claimAssembled(response); err != nil {
		return err
	}

	if len(c.Assembled) == 0 {
		return fmt.Errorf("%w: %s answers with %s, which declares no member for the hook to fill",
			errNotAnAssembledRead, c.Name, response.FullName)
	}

	return c.readWriteScalars(response)
}

// assembles reports whether one response field is filled by the tool's hook
// rather than by an argument or the declared sentence.
func (c *contract) assembles(goName string) bool {
	return slices.ContainsFunc(c.Assembled, func(member assembledMember) bool {
		return member.GoName == goName
	})
}

// claimAssembled records every LOCAL scalar the hook fills. A repeated one is
// refused because the hook answers the response message and the emitter copies
// member by member, which no slice share survives without aliasing the hook's.
func (c *contract) claimAssembled(response *goMessage) error {
	for _, entry := range response.scalarFields() {
		if !entry.local {
			continue
		}

		if entry.repeated {
			return fmt.Errorf("%w: %s answers with %s.%s",
				errNotAnAssembledRead, c.Name, response.FullName, entry.protoName)
		}

		c.Assembled = append(c.Assembled, assembledMember{
			GoName:    entry.goName,
			ProtoName: entry.protoName,
		})
	}

	return nil
}

// readMetaResponse records the one thing the emitter needs from a meta tool's
// answer: the member its declared sentence is reported in.
//
// Nothing else is read, because nothing else is assembled. A meta tool with an
// answer hook builds its whole result from local state and hands it back, and
// the readers below all describe a request's answer: which member the API body
// decodes into, which scalar echoes an argument the call carried. Running them
// over a response no call fills reads AuditHealthResponse as an envelope
// wrapping its optional SQLite section, and the tool would answer with that
// section instead of the report.
func (c *contract) readMetaResponse(response *goMessage) error {
	c.MessageField = response.goNameOf(messageFieldName)

	return nil
}

// checkNoLocalMembers refuses a response member declared as assembled from the
// call on a tier that has no assembly step. A read decodes its whole answer and
// a delete carries no resource at all, so the declaration there would be read
// and then ignored, which is the silent half of the same trap the assembled
// member exists to close.
//
// Scalars are held to it as readily as messages: a member the API never sends
// reaches a client as the empty string either way.
func (c *contract) checkNoLocalMembers(response *goMessage) error {
	for _, entry := range response.messageFields() {
		if entry.local {
			return fmt.Errorf("%w: %s answers with %s.%s",
				errUnplacedLocalMember, c.Name, response.FullName, entry.protoName)
		}
	}

	for _, entry := range response.scalarFields() {
		if entry.local {
			return fmt.Errorf("%w: %s answers with %s.%s",
				errUnplacedLocalMember, c.Name, response.FullName, entry.protoName)
		}
	}

	return nil
}

// readStructResponse records a tool whose API body has no proto model: the
// database engine configs, profile preferences, managed stats. There is no
// message to fill, so the decoded object is the answer and protojson sorts it
// through a bare Struct.
//
// Only a read is served this way. A mutation's answer is assembled from its
// declared envelope, and with no response there is no message field to report
// under and no resource to carry, so it is refused as it always was.
func (c *contract) readStructResponse() error {
	if !c.readsResource() {
		return fmt.Errorf("%w: %s", errNoResponse, c.Name)
	}

	c.StructResponse = true

	return nil
}

// removesResource reports whether a tool deletes what it is addressed at, which
// is the one tier whose answer is built from the call rather than decoded from
// what the API sent back.
func (c *contract) removesResource() bool {
	return c.Capability == linodev1.ToolCapability_TOOL_CAPABILITY_DESTROY
}

// readDestroyEnvelope records the id echo a delete answers with: the prose
// field, plus one scalar per path argument, each named after the argument that
// fills it.
//
// That naming is the whole binding, unless the member declares echo_argument
// and names its id outright, so a response naming something the call was never
// given is refused rather than answered with a zero. Refusing here is what keeps
// the failure ahead of the deletion: the echo is resolved before the request
// goes out, so a mis-declared response cannot surface over a resource that is
// already gone.
func (c *contract) readDestroyEnvelope(response *goMessage) error {
	messageField := response.goNameOf(messageFieldName)
	if messageField == "" {
		return fmt.Errorf("%w: %s answers with %s, which has no message field to report under",
			errNotADeleteEnvelope, c.Name, response.FullName)
	}

	c.MessageField = messageField

	if err := c.readDestroyPayload(response); err != nil {
		return err
	}

	for _, entry := range response.scalarFields() {
		if entry.protoName == messageFieldName {
			continue
		}

		argument, named := c.pathField(entry.argumentName())
		if !named || entry.repeated {
			return fmt.Errorf("%w: %s answers with %s.%s, which names no path id it was given",
				errNotADeleteEnvelope, c.Name, response.FullName, entry.protoName)
		}

		c.Echoes = append(c.Echoes, destroyEcho{GoName: entry.goName, Arg: argument})
	}

	return nil
}

// readDestroyPayload records the resource a removal's route answers with, for
// the destroys whose call is a rebuild rather than a delete. A response
// declaring none is the empty-bodied shape every plain delete has.
//
// More than one, or a repeated one, is refused rather than served: the answer is
// one decode, so a second member would be dropped by the serializer over a
// change that has already been made.
func (c *contract) readDestroyPayload(response *goMessage) error {
	if repeated := response.repeatedFields(); len(repeated) > 0 {
		return fmt.Errorf("%w: %s answers with %s.%s, which repeats a message no delete decodes",
			errNotADeleteEnvelope, c.Name, response.FullName, repeated[0].protoName)
	}

	payloads := response.messageFields()
	if len(payloads) == 0 {
		return nil
	}

	if len(payloads) > 1 {
		return fmt.Errorf("%w: %s answers with %d resources, and a removal decodes one",
			errNotADeleteEnvelope, c.Name, len(payloads))
	}

	return c.readPayloadMessage(response, payloads[0])
}

// readWriteEnvelope records the parts of a mutation's answer: the prose field,
// and the resource the API sent back when there is one. Any other shape is
// refused rather than served, because the serializer drops what it cannot place
// and the tool would answer with a message over an empty resource.
//
// A response with no resource is the acknowledged shape: the API reports the
// change by status alone, so the answer is assembled from the call the way a
// destroy's is, and every scalar beside the message names the argument that
// fills it.
//
// A response with no prose field at all is read by readNamelessEnvelope, which
// separates the tool's own envelope from the API resource itself.
func (c *contract) readWriteEnvelope(response *goMessage) error {
	paged, err := c.readWritePage(response)
	if err != nil || paged {
		return err
	}

	messageField := response.goNameOf(messageFieldName)
	payloads := response.messageFields()

	if messageField == "" {
		if len(c.ResponseBody) > 0 {
			return fmt.Errorf("%w: %s answers with %s, which decodes its whole answer already",
				errUnusedResponseBody, c.Name, response.FullName)
		}

		return c.readNamelessEnvelope(response, payloads)
	}

	if len(c.ResponseBody) > 0 {
		return c.readDecodedEnvelope(response, messageField, payloads)
	}

	if len(payloads) == 0 {
		return c.readAcknowledgeEnvelope(response, messageField)
	}

	if len(payloads) != 1 {
		return fmt.Errorf("%w: %s answers with %s, which has %d resource field(s) beside its message field",
			errNotAWriteEnvelope, c.Name, response.FullName, len(payloads))
	}

	c.MessageField = messageField

	if payloads[0].local {
		return c.readEchoedPayload(response, payloads[0])
	}

	return c.readPayload(response, payloads[0])
}

// readEchoedPayload records a mutation whose resource member is assembled from
// the call rather than decoded: the route answers with no body, so the member is
// rebuilt from the arguments the caller sent.
//
// Both bucket-access routes are the shape. Decoding their empty answer into the
// resource message generates cleanly and reports an ACL of "" over a change that
// has already been made, which is the one failure this whole reader exists to
// refuse, and the only one no gate downstream can see.
//
// Nothing is set on PayloadField, so the tool lands on the acknowledge tier: an
// answer built entirely from the call is what that tier already is, and this
// gives it a nested member beside the loose scalars it carried before.
func (c *contract) readEchoedPayload(response *goMessage, entry repeatedField) error {
	nested, err := c.readEchoMembers(response, entry)
	if err != nil {
		return err
	}

	c.Echoes = append(c.Echoes, destroyEcho{Nested: nested, GoName: entry.goName})

	return c.readWriteScalars(response)
}

// readEchoMembers maps the assembled member's message onto the tool's own
// arguments, member by member. The names are the whole mapping, the same rule a
// repeated named-message echo is held to: a member naming no argument, one whose
// kind disagrees, or one carrying a message or a list of its own fails the run by
// name rather than reaching a client as a zero.
func (c *contract) readEchoMembers(response *goMessage, entry repeatedField) (*echoItems, error) {
	member, err := lookupGoMessage(entry.message)
	if err != nil {
		return nil, err
	}

	if len(member.messageFields())+len(member.repeatedFields()) > 0 {
		return nil, fmt.Errorf("%w: %s answers with %s.%s, whose %s carries a message of its own",
			errNotAWriteEnvelope, c.Name, response.FullName, entry.protoName, member.FullName)
	}

	built := &echoItems{TypeName: member.TypeName, Members: make([]echoMember, 0, len(member.scalarFields()))}

	for _, field := range member.scalarFields() {
		kind, known := member.kindOf(field.protoName)
		argument, named := c.inputField(field.protoName)

		if !known || field.repeated || !named || argument.Repeated || !echoKindsAgree(argument.Kind, kind) {
			return nil, fmt.Errorf("%w: %s answers with %s.%s, whose member %s names no argument it was given",
				errNotAWriteEnvelope, c.Name, response.FullName, entry.protoName, field.protoName)
		}

		built.Members = append(built.Members, echoMember{
			GoName: field.goName, Argument: field.protoName, Kind: kind,
		})
	}

	return built, nil
}

// readNamelessEnvelope records a mutation whose answer reports no prose. The
// token create's {warning, token} still wraps the decoded body in a member, and
// everything else of that shape is the API resource itself.
//
// The name is what separates the two. Shape alone cannot: PlacementGroup holds
// one nested object too and is the real API resource, so reading every such
// message as an envelope would refuse the resource's own fields as echoes it was
// never given. A declared success_message is refused rather than dropped, since
// there is no member to report the sentence in either way.
func (c *contract) readNamelessEnvelope(response *goMessage, payloads []repeatedField) error {
	// An assembled member needs prose to sit beside: the acknowledge tier is
	// what builds an answer from the call, and it reports under a message field
	// this response does not have.
	if err := c.checkNoLocalMembers(response); err != nil {
		return err
	}

	if len(payloads) != 1 || !strings.HasSuffix(string(response.FullName), envelopeSuffix) {
		return c.readBareResource(response)
	}

	if c.SuccessMessage != "" {
		return fmt.Errorf("%w: %s answers with %s, which has no message field to report it in",
			errUnusedSuccessMessage, c.Name, response.FullName)
	}

	return c.readPayload(response, payloads[0])
}

// readDecodedEnvelope records a mutation whose declared envelope is the body the
// API sent. The members response_body_fields names are values the answer carries
// rather than arguments echoed back, and the only shape that reads them is
// decoding into the envelope itself: POST /images/upload answers
// {image, upload_to}, so decoding into Image alone reads neither member.
//
// The resource lands in its own member from that same decode, which is why
// nothing is assembled around it here. The prose, the notice and any echo are
// written onto the decoded message afterwards.
func (c *contract) readDecodedEnvelope(
	response *goMessage, messageField string, payloads []repeatedField,
) error {
	if len(payloads) > 1 {
		return fmt.Errorf("%w: %s answers with %s, which has %d resource field(s) beside its message field",
			errNotAWriteEnvelope, c.Name, response.FullName, len(payloads))
	}

	if err := c.checkNoLocalMembers(response); err != nil {
		return err
	}

	if err := c.checkResponseBody(response); err != nil {
		return err
	}

	c.MessageField = messageField
	c.DecodedResponse = true
	c.PayloadGo = *response

	if len(payloads) == 1 {
		if err := c.readPayloadMessage(response, payloads[0]); err != nil {
			return err
		}
	}

	return c.readWriteScalars(response)
}

// responseBodyOverArg is the prefix a response_body_fields entry carries when
// the member it names is spelled the same as an argument the tool takes and the
// API's value is the one to report. The interface upgrade is the shape: it
// takes an optional config_id and answers with the profile it actually
// upgraded, which is the one the caller has to be told about.
const responseBodyOverArg = "api:"

// responseBodyMember is the member one response_body_fields entry names, and
// whether the entry declares the API's value wins over an argument spelled the
// same way.
func responseBodyMember(declared string) (string, bool) {
	return strings.CutPrefix(declared, responseBodyOverArg)
}

// answersBody reports whether the API's answer fills one response member rather
// than the call.
func (c *contract) answersBody(name string) bool {
	return slices.ContainsFunc(c.ResponseBody, func(declared string) bool {
		member, _ := responseBodyMember(declared)

		return member == name
	})
}

// checkResponseBody holds every declared response-body member to being one the
// answer can fill and nothing else already fills.
//
// The prose, the notice and an argument echo each have a filler of their own, so
// a member claimed by two would answer with whichever the handler wrote last.
// A name the response does not declare as a singular scalar has nothing to fill
// at all.
func (c *contract) checkResponseBody(response *goMessage) error {
	for _, declared := range c.ResponseBody {
		name, over := responseBodyMember(declared)

		if name == messageFieldName || name == warningFieldName {
			return fmt.Errorf("%w: %s names %s, which the tool fills itself",
				errUnknownResponseBody, c.Name, name)
		}

		// Asked before the response is consulted: an argument is wrong here
		// whether or not the answer declares a member spelled the same way,
		// unless the declaration says the API's value is the one to report.
		_, named := c.inputField(name)

		if named && !over {
			return fmt.Errorf("%w: %s names %s, which is an argument it was given",
				errUnknownResponseBody, c.Name, name)
		}

		if over && !named {
			return fmt.Errorf("%w: %s names %s over an argument it was never given",
				errUnknownResponseBody, c.Name, name)
		}

		kind, known := response.kindOf(name)
		if !known || kind == protoreflect.MessageKind || kind == protoreflect.GroupKind || response.isList(name) {
			return fmt.Errorf("%w: %s names %s, which %s declares no singular scalar member for",
				errUnknownResponseBody, c.Name, name, response.FullName)
		}
	}

	return nil
}

// readPayload records the member a mutation's decoded body lands in, plus the
// declared warning and the argument echoes beside it.
func (c *contract) readPayload(response *goMessage, entry repeatedField) error {
	if err := c.readPayloadMessage(response, entry); err != nil {
		return err
	}

	return c.readWriteScalars(response)
}

// readPayloadMessage records the member and the message a mutation's decoded
// body lands in.
//
// The message is a generated one or the free-form Struct the API round-trips:
// profile preferences are whatever the caller stored, so there is no schema to
// decode them into. Any other foreign message is refused rather than served,
// since the handler names the type in the generated package and one declared
// elsewhere has no spelling there.
func (c *contract) readPayloadMessage(response *goMessage, entry repeatedField) error {
	if entry.message != structFullName && !strings.HasPrefix(string(entry.message), protoPackage+".") {
		return fmt.Errorf("%w: %s answers with %s.%s, whose %s is declared outside the contract",
			errNotAWriteEnvelope, c.Name, response.FullName, entry.protoName, entry.message)
	}

	c.PayloadField = entry.goName
	c.PayloadMember = entry.protoName
	c.PayloadStruct = entry.message == structFullName

	payload, err := lookupGoMessage(entry.message)
	if err != nil {
		return err
	}

	c.PayloadGo = payload

	return nil
}

// readWriteScalars places every scalar a mutation's answer carries beside its
// message and its resource: the declared warning, and one echo per argument the
// answer reports back.
//
// A member is resolved by its own name unless it declares echo_argument, which
// is the whole of the alias: the three assembled-answer readers share this
// resolution, so a member the API spells its own way reaches its argument from
// any of them.
//
// Both are resolved before the request goes out, so a response promising a
// sentence or a value the call cannot fill is refused rather than answered with
// a blank over a change that has already been made. Anything left unplaced would
// be dropped by the serializer, which is the silent shape this refuses.
func (c *contract) readWriteScalars(response *goMessage) error {
	for _, entry := range response.scalarFields() {
		if entry.protoName == messageFieldName || c.answersBody(entry.protoName) {
			continue
		}

		// The members no argument fills are the ones a hook brought back, already
		// claimed. Any other LOCAL scalar reaches here on a tier with nothing to
		// fill it.
		if c.assembles(entry.goName) {
			continue
		}

		if entry.local {
			return fmt.Errorf("%w: %s answers with %s.%s",
				errUnplacedLocalMember, c.Name, response.FullName, entry.protoName)
		}

		if entry.protoName == warningFieldName {
			if err := c.readWarning(response, entry); err != nil {
				return err
			}

			continue
		}

		argument, named := c.inputField(entry.argumentName())
		if !named || argument.Repeated != entry.repeated {
			return fmt.Errorf("%w: %s answers with %s.%s, which names no argument it was given",
				errNotAWriteEnvelope, c.Name, response.FullName, entry.protoName)
		}

		c.Echoes = append(c.Echoes, destroyEcho{GoName: entry.goName, Arg: argument})
	}

	return c.readListEchoes(response)
}

// readListEchoes places the repeated named-message members a mutation's answer
// carries back. The API returned nothing to decode, so each item the caller sent
// is rebuilt into the item message the response declares.
//
// A response repeating something the call was never given is refused rather than
// answered with an empty list, the same rule the scalars beside it are held to.
// Silence is the shape this replaces: a repeated message field sits in neither
// the scalar nor the single-message group, so it used to reach the serializer
// unset over a change that had already been made.
//
// A decoded envelope has nothing to rebuild: one decode places every message
// member the answer carries, repeated as readily as singular, so the interface
// upgrade's `interfaces` arrives filled by the API rather than echoed from a
// call that never named one.
func (c *contract) readListEchoes(response *goMessage) error {
	if c.DecodedResponse {
		return nil
	}

	for _, entry := range response.repeatedFields() {
		argument, named := c.inputField(entry.protoName)
		if !named || argument.ItemMessage == nil {
			return fmt.Errorf("%w: %s answers with %s.%s, which names no named-message argument it was given",
				errNotAWriteEnvelope, c.Name, response.FullName, entry.protoName)
		}

		items, err := c.readEchoItems(response, entry, &argument)
		if err != nil {
			return err
		}

		c.Echoes = append(c.Echoes, destroyEcho{Items: items, GoName: entry.goName, Arg: argument})
	}

	return nil
}

// readEchoItems maps the response's item message onto the input's, member by
// member. The two are separate declarations that happen to describe the same
// binding, so the names are the whole mapping: every response member takes the
// input member spelled the same way, and one with no counterpart, one whose kind
// disagrees, or one carrying a message of its own fails the run by name rather
// than reaching a client as a zero.
func (c *contract) readEchoItems(
	response *goMessage, entry repeatedField, argument *field,
) (*echoItems, error) {
	item, err := lookupGoMessage(entry.message)
	if err != nil {
		return nil, err
	}

	if len(item.messageFields())+len(item.repeatedFields()) > 0 {
		return nil, fmt.Errorf("%w: %s answers with %s.%s, whose item %s carries a message of its own",
			errNotAWriteEnvelope, c.Name, response.FullName, entry.protoName, item.FullName)
	}

	members := argument.ItemMessage.Fields()
	built := &echoItems{TypeName: item.TypeName, Members: make([]echoMember, 0, len(item.scalarFields()))}

	for _, member := range item.scalarFields() {
		kind, known := item.kindOf(member.protoName)
		source := members.ByName(protoreflect.Name(member.protoName))

		if !known || member.repeated || source == nil || !echoKindsAgree(source.Kind(), kind) {
			return nil, fmt.Errorf("%w: %s answers with %s.%s, whose item member %s names no matching member of %s",
				errNotAWriteEnvelope, c.Name, response.FullName, entry.protoName,
				member.protoName, argument.ItemMessage.FullName())
		}

		built.Members = append(built.Members, echoMember{
			GoName: member.goName, Argument: member.protoName, Kind: kind,
		})
	}

	return built, nil
}

// echoKindsAgree reports whether an input member can fill a response member.
// Text fills text, an id fills an id, and a flag fills a flag; every other kind
// has no reader on either side.
func echoKindsAgree(source, target protoreflect.Kind) bool {
	if source == target && (source == protoreflect.StringKind || source == protoreflect.BoolKind) {
		return true
	}

	numeric := map[protoreflect.Kind]bool{protoreflect.Int32Kind: true, protoreflect.Int64Kind: true}

	return numeric[source] && numeric[target]
}

// readWarning records the member a declared warning_message is reported in. A
// response promising the notice while the tool declares none is refused, since
// a client reading an empty warning learns nothing about the secret it sits
// beside.
func (c *contract) readWarning(response *goMessage, entry scalarField) error {
	if !c.WarningDeclared {
		return fmt.Errorf("%w: %s answers with %s, which promises a warning",
			errNoWarningMessage, c.Name, response.FullName)
	}

	if entry.repeated {
		return fmt.Errorf("%w: %s answers with %s.%s, which repeats",
			errNotAWriteEnvelope, c.Name, response.FullName, entry.protoName)
	}

	c.WarningField = entry.goName

	return nil
}

// readBareResource records a mutation whose declared response is the API
// resource itself: the routed call decodes into it and the tool answers with it,
// so there is no member to place the body in and no prose to report over it.
//
// A declared success_message is refused rather than dropped, since this answer
// carries nowhere to report the sentence. A warning is different: the TFA enable
// decodes {secret, expiry} and writes the notice onto the same message, so the
// field is filled after the decode rather than assembled around it.
func (c *contract) readBareResource(response *goMessage) error {
	if c.SuccessMessage != "" {
		return fmt.Errorf("%w: %s answers with %s, which has no message field to report it in",
			errUnusedSuccessMessage, c.Name, response.FullName)
	}

	c.BareResource = true
	c.PayloadGo = *response

	for _, entry := range response.scalarFields() {
		if entry.protoName != warningFieldName {
			continue
		}

		if err := c.readWarning(response, entry); err != nil {
			return err
		}
	}

	return nil
}

// readEnvelope records the collection shape when the response carries one: the
// repeated field holding the elements, plus the count and filter echo beside it.
// An empty envelope puts the tool on the single-resource tier.
//
// A page needs a count plus exactly one repeated message field, not repetition
// alone. Repetition alone reads a resource carrying sub-object lists (user
// grants, LKE pool disks and nodes, instance interfaces) as a page, and the list
// machinery would answer the caller with a count and one of those sections.
func (c *contract) readEnvelope(response *goMessage) error {
	countField := response.goNameOf("count")
	repeated := response.repeatedFields()

	if countField == "" || len(repeated) != 1 {
		if err := c.readWrapper(response); err != nil {
			return err
		}

		return c.checkNotAPage(response, countField, len(repeated))
	}

	entry := repeated[0]
	c.EnvelopeField = entry.goName
	c.CountField = countField
	c.FilterField = response.goNameOf("filter")

	element, err := lookupGoMessage(entry.message)
	if err != nil {
		return err
	}

	c.ElementGo = element

	return c.readMarkerCursor(response)
}

// markerPaged reports whether the route pages by a cursor rather than by number.
func (c *contract) markerPaged() bool {
	return c.EnvelopeShape == linodev1.ListEnvelope_SHAPE_MARKER
}

// readWritePage records a mutation whose route answers with a page of a
// collection rather than the resource it changed. Both firewall replacements
// are the shape: the PUT reports the assignments that now exist, and the API
// sends them in the same {data, page, pages, results} envelope its list route
// does.
//
// Without this the answer decodes into the declared response directly, and
// since the envelope carries none of that response's members the caller is told
// the replacement left zero firewalls assigned. Nothing downstream can see
// that: the call succeeded and the serializer filled every member it was given.
//
// The members are held to the three a page carries, so a response that also
// wants prose or an echo fails by name rather than answering with it empty.
func (c *contract) readWritePage(response *goMessage) (bool, error) {
	countField := response.goNameOf("count")
	repeated := response.repeatedFields()

	if countField == "" || len(repeated) != 1 {
		return false, c.checkNotAPage(response, countField, len(repeated))
	}

	if err := c.checkPageMembers(response); err != nil {
		return false, err
	}

	element, err := lookupGoMessage(repeated[0].message)
	if err != nil {
		return false, err
	}

	c.PagedWrite = true
	c.CountField = countField
	c.EnvelopeField = repeated[0].goName
	c.ElementGo = element

	return true, nil
}

// checkPageMembers holds a paged mutation's answer to what a page can be filled
// from. The elements and their count come off the decode; a filter echo is the
// list tier's and stays empty here, since a mutation narrows nothing.
//
// Everything else refuses. A prose member has nothing to report from (the API's
// answer is the page), a declared sentence has nowhere to land, and a declared
// envelope shape names a body this reader does not decode.
func (c *contract) checkPageMembers(response *goMessage) error {
	for _, entry := range response.scalarFields() {
		if entry.protoName == "count" || entry.protoName == "filter" {
			continue
		}

		return fmt.Errorf("%w: %s answers with %s.%s, which a page carries nothing to fill",
			errNotAWritePage, c.Name, response.FullName, entry.protoName)
	}

	if len(response.messageFields()) > 0 {
		return fmt.Errorf("%w: %s answers with %s, which holds a resource beside its page",
			errNotAWritePage, c.Name, response.FullName)
	}

	if c.SuccessMessage != "" || len(c.ResponseBody) > 0 {
		return fmt.Errorf("%w: %s answers with %s, which reports a page and no members of its own",
			errNotAWritePage, c.Name, response.FullName)
	}

	if c.EnvelopeShape != linodev1.ListEnvelope_SHAPE_UNSPECIFIED {
		return fmt.Errorf("%w: %s answers with a page declared as %s, and a mutation decodes the standard one",
			errNotAWritePage, c.Name, c.EnvelopeShape)
	}

	return nil
}

// readMarkerCursor records the members a marker-paged answer reports its cursor
// in. A response missing either one has nowhere to put it, and the caller of a
// truncated page would read the end of the collection where there is more.
func (c *contract) readMarkerCursor(response *goMessage) error {
	if !c.markerPaged() {
		return nil
	}

	c.TruncatedField = response.goNameOf(markerTruncatedMember)
	c.NextMarkerField = response.goNameOf(markerNextMember)

	if c.TruncatedField == "" || c.NextMarkerField == "" {
		return fmt.Errorf("%w: %s pages by marker and %s declares no %s and %s to report it in",
			errNoMarkerCursor, c.Name, response.FullName, markerTruncatedMember, markerNextMember)
	}

	return nil
}

// readWrapper records the member an API body decodes into when the declared
// response is the tool's own envelope rather than the body itself.
//
// The name is what says so, the same way ListResponse says a message is a page.
// Shape alone cannot: FirewallSettings holds one nested object too and is the
// real API resource, so reading every such message as a wrapper would answer a
// caller with the inner object instead of the resource.
func (c *contract) readWrapper(response *goMessage) error {
	if !strings.HasSuffix(string(response.FullName), wrapperSuffix) {
		return nil
	}

	wrapped := response.messageFields()

	if len(wrapped) != 1 || len(response.scalarFields()) > 0 {
		return fmt.Errorf("%w: %s answers with %s, which is named an envelope and holds %d resource field(s)",
			errNotAWrapper, c.Name, response.FullName, len(wrapped))
	}

	resource, err := lookupGoMessage(wrapped[0].message)
	if err != nil {
		return err
	}

	c.WrapperField = wrapped[0].goName
	c.WrapperType = response.TypeName
	c.ResponseGo = resource

	return nil
}

// checkNotAPage confirms that a response the envelope test rejected really is
// one resource. That test is a shape test, and every page envelope here is also
// named ListResponse, so a message spelled that way but shaped otherwise means
// name and shape disagree. Serving it as a resource would answer the caller with
// an empty collection rather than fail, so the disagreement is reported.
func (c *contract) checkNotAPage(response *goMessage, countField string, repeated int) error {
	if !strings.HasSuffix(string(response.FullName), "ListResponse") {
		return nil
	}

	return fmt.Errorf("%w: %s answers with %s, which has %d repeated message field(s) and %s",
		errAmbiguousEnvelope, c.Name, response.FullName, repeated,
		countPresence(countField))
}

// countPresence words the count half of the envelope mismatch above.
func countPresence(countField string) string {
	if countField == "" {
		return "no count field"
	}

	return "a count field"
}

// readAcknowledgeEnvelope records the answer a mutation with nothing to decode
// reports. Its members are placed the way a mutation carrying a resource places
// the scalars beside it, which is what keeps the two tiers echoing an argument
// under one rule.
func (c *contract) readAcknowledgeEnvelope(response *goMessage, messageField string) error {
	c.MessageField = messageField

	// Only a hook can fill a LOCAL member here, so the claim is gated on one
	// being declared: without it the member reaches readWriteScalars, which
	// refuses it as a value nothing on this tier fills.
	if c.Hooks.Execute != "" {
		if err := c.claimAssembled(response); err != nil {
			return err
		}
	}

	return c.readWriteScalars(response)
}

// inputField returns the PATH or BODY field one name refers to. A body field
// counts because an acknowledged mutation echoes what it was asked to apply,
// which is as often something it sent as something it was addressed by.
func (c *contract) inputField(name string) (field, bool) {
	for _, entry := range append(append([]field{}, c.Path...), c.Body...) {
		if entry.ProtoName == name {
			return entry, true
		}
	}

	return field{}, false
}

// deriveTier reads the shape of code a tool needs off the rest of the contract,
// so no per-tool declaration decides it. A tier the emitter has no shape for
// stays unknown, which emitTool reports by name rather than serving the tool as
// the nearest shape it does have.
//
// The route's method decides first, because the tier is about the shape of the
// handler and a GET fetches whatever its capability says about who may call it.
// Reading the capability instead sent the three GETs registered above the read
// tier (both database credential reads and the Managed credential read) to the
// mutation shape, where the envelope check refuses them: each answers with the
// bare resource a read answers with, not the {message, resource} a mutation
// does. Capability still gates and registers them; it just no longer picks the
// code shape.
func (c *contract) deriveTier() {
	// Ahead of everything, because every test below reads a route: the method
	// that is not GET, the body that is not empty, the response the API fills.
	// A contract with no route answers each of them the way the acknowledge
	// tier does, so this is the branch that keeps a meta tool off it.
	if c.Meta {
		c.Tier = tierMeta

		return
	}

	if !c.readsResource() {
		if c.removesResource() {
			c.Tier = tierDestroy

			return
		}

		if c.signsBody() {
			c.Tier = tierBodyRead

			return
		}

		// A response the API's body fills is on the write tier however few
		// members it carries: the acknowledge tier decodes nothing, so a tool
		// landing there would answer with the members left empty. A page is the
		// same fact with the body arriving as a collection.
		if c.PayloadField == "" && !c.BareResource && !c.DecodedResponse && !c.PagedWrite {
			c.Tier = tierAcknowledge

			return
		}

		c.Tier = tierWrite

		return
	}

	if c.EnvelopeField == "" {
		c.Tier = tierGet

		return
	}

	// Ahead of the path split because the marker tier covers both: its route
	// happens to be nested today, and the cursor is what picks the shape.
	if c.markerPaged() {
		c.Tier = tierMarkerList

		return
	}

	if len(c.Path) > 0 {
		c.Tier = tierSubresourceList

		return
	}

	c.Tier = tierList
}

// readsResource reports whether a tool's route only reads, which is the method
// the API assigns rather than the tier the tool is registered under.
func (c *contract) readsResource() bool {
	return c.Method == http.MethodGet
}

// signsBody reports a read the route method alone reads as a mutation: a tool
// registered at the read tier whose route is not a GET. The presigned-URL create
// signs what its body describes and the metric query asks for a window of
// samples; neither stores anything, so the write tier's confirm gate would sit
// over an answer the caller can ask for again.
//
// Body arity takes no part. A route declaring no body member still sends one,
// the empty object, so arity would pick which of the two reads lands here rather
// than whether either is a mutation.
func (c *contract) signsBody() bool {
	return c.Capability == linodev1.ToolCapability_TOOL_CAPABILITY_READ
}

// deriveFilters splits a collection's non-pagination query arguments into the
// two unrelated things they are: the ones this tool narrows its own page with,
// and the ones the route filters on. field_location cannot separate them,
// because both are arguments a caller filters with and both are QUERY. The
// element is what separates them: an argument it can answer is applied here,
// and one it cannot is forwarded to the route.
//
// A declared list_filter overrides that, for the two shapes the element cannot
// settle: an argument matching its field under another name, and one matching a
// substring where the name says nothing about it.
func (c *contract) deriveFilters() error {
	if c.Tier == tierMarkerList {
		c.deriveMarkerQuery()

		return nil
	}

	if c.Tier != tierList && c.Tier != tierSubresourceList {
		return nil
	}

	for _, query := range c.Query {
		if isPaginationParam(query.ProtoName) {
			continue
		}

		filter, applies, err := c.filterFor(&query)
		if err != nil {
			return err
		}

		if !applies {
			c.Forwarded = append(c.Forwarded, query)

			continue
		}

		c.Filters = append(c.Filters, filter)
	}

	return nil
}

// deriveMarkerQuery sends every one of a marker-paged route's query arguments
// to the route and echoes the ones that narrow the answer.
//
// None becomes a client-side filter, however well its name matches the element:
// the route already applied it, so filtering the page again would drop objects
// the API deliberately returned. That is why the cursor controls travel here
// too, where the page envelope's reader owns them instead.
func (c *contract) deriveMarkerQuery() {
	for _, query := range c.Query {
		c.Forwarded = append(c.Forwarded, query)

		if !isCursorParam(query.ProtoName) {
			c.Echoed = append(c.Echoed, query)
		}
	}
}

// filterFor resolves one query argument to the client-side filter it declares
// or derives, and whether it is one at all.
func (c *contract) filterFor(query *field) (listFilter, bool, error) {
	target, match := declaredFilter(query)

	if target == "" {
		derived, contains := strings.CutSuffix(query.ProtoName, containsSuffix)
		if c.ElementGo.goNameOf(derived) == "" {
			return listFilter{}, false, nil
		}

		target = derived

		if contains {
			match = linodev1.ListFilterSpec_MATCH_CONTAINS
		}
	}

	getter := c.ElementGo.goNameOf(target)
	if getter == "" {
		return listFilter{}, false, fmt.Errorf("%w: %s filters on %s, which %s has no field for",
			errUnmatchedFilter, c.Name, query.ProtoName, c.ElementGo.FullName)
	}

	kind, _ := c.ElementGo.kindOf(target)
	if !filterableKind(kind, match) {
		return listFilter{}, false, fmt.Errorf(
			"%w: %s filters on %s.%s, which is %s and has no %s comparison both languages share",
			errUnmatchedFilter, c.Name, c.ElementGo.FullName, target, kind, match,
		)
	}

	return listFilter{
		Param:       query.ProtoName,
		Field:       target,
		Getter:      "Get" + getter,
		Description: query.Description,
		Kind:        kind,
		Match:       match,
	}, true, nil
}

// declaredFilter reads a query argument's list_filter, answering the element
// field it names and how it matches. An empty field name means the argument
// declares nothing and the element decides.
func declaredFilter(query *field) (string, linodev1.ListFilterSpec_Match) {
	if query.Filter == nil {
		return "", linodev1.ListFilterSpec_MATCH_UNSPECIFIED
	}

	target := query.Filter.GetField()
	if target == "" {
		target = query.ProtoName
	}

	return target, query.Filter.GetMatch()
}

// filterableKind reports whether a client-side filter can compare an element
// field of this kind under this match. Text and flags are the two the Python
// side compares the same way; a member match reads a repeated string, which is
// how a list of capabilities answers a single one. Anything else would be
// applied differently by each language rather than refused by both.
func filterableKind(kind protoreflect.Kind, match linodev1.ListFilterSpec_Match) bool {
	if match == linodev1.ListFilterSpec_MATCH_MEMBER {
		return kind == protoreflect.StringKind
	}

	return kind == protoreflect.StringKind || kind == protoreflect.BoolKind
}

// structValueName is the well-known message a free-form JSON value decodes
// into, which is what makes a map of it the only object shape a body can carry.
const structValueName protoreflect.FullName = "google.protobuf.Value"

// isObjectMap reports whether a field is a map from string to
// google.protobuf.Value. Any other map is refused by the body setter rather
// than guessed at, since no other map kind has a JSON form both languages
// already agree on.
func isObjectMap(descriptor protoreflect.FieldDescriptor) bool {
	if !descriptor.IsMap() {
		return false
	}

	value := descriptor.MapValue()

	return descriptor.MapKey().Kind() == protoreflect.StringKind &&
		value.Kind() == protoreflect.MessageKind &&
		value.Message().FullName() == structValueName
}

// isStringMap reports whether a field is a map from string to string, the one
// typed map a body carries. It is exclusive with isObjectMap: a value that is
// always text needs no free-form decode, and the reader that holds it to text
// is what answers the caller who sent a number.
func isStringMap(descriptor protoreflect.FieldDescriptor) bool {
	if !descriptor.IsMap() {
		return false
	}

	return descriptor.MapKey().Kind() == protoreflect.StringKind &&
		descriptor.MapValue().Kind() == protoreflect.StringKind
}

// structName is the well-known message a free-form JSON object decodes into,
// which is what makes a list of it the only array of objects a body can carry.
const structName protoreflect.FullName = "google.protobuf.Struct"

// isObjectList reports whether a field is a repeated google.protobuf.Struct.
// Any other repeated message is refused by the body setter rather than guessed
// at, since no other message kind has a JSON form both languages already agree
// on.
func isObjectList(descriptor protoreflect.FieldDescriptor) bool {
	if !descriptor.IsList() || descriptor.Kind() != protoreflect.MessageKind {
		return false
	}

	return descriptor.Message().FullName() == structName
}

// itemMessage is the named message a repeated field carries an array of, nil
// for everything else. It is deliberately exclusive with isObjectList: a
// repeated google.protobuf.Struct declares no members, so only a named item
// message gives the body reader a shape to hold each entry to.
func itemMessage(descriptor protoreflect.FieldDescriptor) protoreflect.MessageDescriptor {
	if !descriptor.IsList() || descriptor.Kind() != protoreflect.MessageKind {
		return nil
	}

	message := descriptor.Message()
	if message.FullName() == structName {
		return nil
	}

	return message
}

// bodyMessage is the named message a singular field carries, nil for
// everything else. It is exclusive with itemMessage on cardinality and with the
// free-form shapes on type: a map entry and a google.protobuf.Struct declare no
// members the body reader could be told about.
func bodyMessage(descriptor protoreflect.FieldDescriptor) protoreflect.MessageDescriptor {
	if descriptor.IsList() || descriptor.IsMap() || descriptor.Kind() != protoreflect.MessageKind {
		return nil
	}

	message := descriptor.Message()
	if message.FullName() == structName {
		return nil
	}

	return message
}

// itemMemberMessage is the named message one member of a typed message carries,
// nil for every member that carries a value. It is what lets a nested member be
// read under the members its own message declares rather than refused.
func itemMemberMessage(member protoreflect.FieldDescriptor) protoreflect.MessageDescriptor {
	if member.IsMap() || member.Kind() != protoreflect.MessageKind {
		return nil
	}

	message := member.Message()
	if message.FullName() == structName {
		return nil
	}

	return message
}

// checkBodyName holds a declared wire key to being a rename of a body member.
// A key on a field that never reaches the body would name nothing, and a key
// equal to the field's own name declares a rename that does not rename.
func checkBodyName(tool string, entry *field) error {
	if entry.BodyName == "" {
		return nil
	}

	if entry.Location != linodev1.FieldLocation_FIELD_LOCATION_BODY {
		return fmt.Errorf("%w: %s field %s", errBodyNameNotBody, tool, entry.ProtoName)
	}

	if entry.BodyName == entry.ProtoName {
		return fmt.Errorf("%w: %s field %s", errBodyNameNoop, tool, entry.ProtoName)
	}

	return nil
}

// checkBodyNullable holds a declared null to a field that has somewhere to send
// one. Three body shapes carry a value the API reads as null: a free-form
// object, a named message, and an optional string whose null the API reads as
// "clear this". Every other field answers absence by leaving itself out, so the
// declaration would name a state the wire cannot spell.
func checkBodyNullable(tool string, entry *field) error {
	if !entry.Nullable {
		return nil
	}

	if entry.Location != linodev1.FieldLocation_FIELD_LOCATION_BODY {
		return fmt.Errorf("%w: %s field %s", errNullableNotBody, tool, entry.ProtoName)
	}

	if entry.ObjectMap || entry.Message != nil || nullableString(entry) {
		return nil
	}

	return fmt.Errorf("%w: %s field %s", errNullableNotObject, tool, entry.ProtoName)
}

// checkBodyCommaList holds a declared comma list to the one shape that has text
// to split. A repeated field already carries the array the composition builds,
// and a field with no presence travels on every call, which would post an empty
// array over a caller who named nothing.
func checkBodyCommaList(tool string, entry *field) error {
	if !entry.CommaList {
		return nil
	}

	if entry.Location != linodev1.FieldLocation_FIELD_LOCATION_BODY {
		return fmt.Errorf("%w: %s field %s", errCommaListNotBody, tool, entry.ProtoName)
	}

	if !nullableString(entry) {
		return fmt.Errorf("%w: %s field %s", errCommaListNotString, tool, entry.ProtoName)
	}

	return nil
}

// nullableString reports whether a field is the one scalar shape a null means
// something to. Presence is part of it: a null is the state between "left this
// alone" and "set it to text", which a field that travels on every call has no
// room for.
func nullableString(entry *field) bool {
	return entry.Kind == protoreflect.StringKind &&
		!entry.Repeated && entry.Presence
}

// fieldNullable reads whether a body object field also accepts an explicit null.
func fieldNullable(descriptor protoreflect.FieldDescriptor) bool {
	nullable, ok := proto.GetExtension(
		descriptor.Options(), linodev1.E_BodyNullable,
	).(bool)

	return ok && nullable
}

// fieldBodyRoot reads whether a body object field is the request body itself
// rather than a member of it.
func fieldBodyRoot(descriptor protoreflect.FieldDescriptor) bool {
	root, ok := proto.GetExtension(
		descriptor.Options(), linodev1.E_BodyRoot,
	).(bool)

	return ok && root
}

// fieldCommaList reads whether a body string argument is composed into an array
// of its comma-separated segments.
func fieldCommaList(descriptor protoreflect.FieldDescriptor) bool {
	split, ok := proto.GetExtension(
		descriptor.Options(), linodev1.E_BodyCommaList,
	).(bool)

	return ok && split
}

// fieldRedacted reads whether a preview stands in for one field's value.
func fieldRedacted(descriptor protoreflect.FieldDescriptor) bool {
	redact, ok := proto.GetExtension(
		descriptor.Options(), linodev1.E_PreviewRedact,
	).(bool)

	return ok && redact
}

// fieldLocation reads where one field travels.
func fieldLocation(descriptor protoreflect.FieldDescriptor) linodev1.FieldLocation {
	location, ok := proto.GetExtension(
		descriptor.Options(), linodev1.E_FieldLocation,
	).(linodev1.FieldLocation)
	if !ok {
		return linodev1.FieldLocation_FIELD_LOCATION_UNSPECIFIED
	}

	return location
}

// fieldFilter reads the client-side match a query field declares, nil when it
// declares none.
func fieldFilter(descriptor protoreflect.FieldDescriptor) *linodev1.ListFilterSpec {
	spec, ok := proto.GetExtension(
		descriptor.Options(), linodev1.E_ListFilter,
	).(*linodev1.ListFilterSpec)
	if !ok {
		return nil
	}

	return spec
}

// readExplicitNulls records the payload fields whose explicit null the
// serializer drops, refusing a declaration nothing can act on.
//
// The names resolve against the message the raw body decodes into rather than
// the tool's whole response, because that is the object the restoration writes
// into: an envelope's own members are assembled from the call and never come
// back as null.
func (c *contract) readExplicitNulls(options protoreflect.ProtoMessage) error {
	names, ok := proto.GetExtension(options, linodev1.E_ExplicitNullFields).([]string)
	if !ok || len(names) == 0 {
		return nil
	}

	if c.readsResource() {
		return c.readReadNulls(names)
	}

	if c.PayloadGo.TypeName == "" {
		return fmt.Errorf("%w: %s decodes no body to restore them into", errNoNullPayload, c.Name)
	}

	if c.DecodedResponse {
		return fmt.Errorf("%w: %s reads its raw body as the envelope, not as %s",
			errNullsOverDecodedResponse, c.Name, c.PayloadGo.FullName)
	}

	for _, name := range names {
		if c.PayloadGo.goNameOf(name) == "" {
			return fmt.Errorf("%w: %s names %s, which %s does not declare",
				errUnknownNullField, c.Name, name, c.PayloadGo.FullName)
		}
	}

	c.ExplicitNulls = names

	return nil
}

// readReadNulls holds a read's declared nulls to the message its answer decodes
// into, which for a read is the response itself: there is no envelope assembled
// around a fetched resource, so nothing else can carry the restored key.
//
// A collection resolves its names against the element instead. Its nulls belong
// to each address in the page, and restoring them onto the envelope would write
// six keys beside the page rather than into the resources in it.
func (c *contract) readReadNulls(names []string) error {
	if c.EnvelopeField != "" {
		return c.readPageNulls(names)
	}

	if c.WrapperField != "" {
		return fmt.Errorf("%w: %s decodes its raw body into %s",
			errNullsOverDecodedResponse, c.Name, c.WrapperField)
	}

	if c.StructResponse {
		return fmt.Errorf("%w: %s answers with the body it decoded, nulls and all",
			errNoNullPayload, c.Name)
	}

	for _, name := range names {
		if c.ResponseGo.goNameOf(name) == "" {
			return fmt.Errorf("%w: %s names %s, which %s does not declare",
				errUnknownNullField, c.Name, name, c.ResponseGo.FullName)
		}
	}

	c.ExplicitNulls = names

	return nil
}

// readPageNulls holds a collection's declared nulls to its element message, the
// object each restored key belongs to.
//
// A marker page is refused: its body carries a cursor beside the elements, and
// the primitive that keeps their raw bodies hands back no cursor to report, so
// the page a caller resumes from would go missing.
func (c *contract) readPageNulls(names []string) error {
	if c.markerPaged() {
		return fmt.Errorf("%w: %s pages by a cursor its raw fetch cannot carry back",
			errNullsOverDecodedResponse, c.Name)
	}

	for _, name := range names {
		if c.ElementGo.goNameOf(name) == "" {
			return fmt.Errorf("%w: %s names %s, which %s does not declare",
				errUnknownNullField, c.Name, name, c.ElementGo.FullName)
		}
	}

	c.ExplicitNulls = names

	return nil
}

// checkPageNullFilters refuses a collection that both restores nulls and
// narrows its page, because the restoration pairs by position: a filter drops
// elements from the serialized page and none from the bodies it decoded, so
// element i would take a null belonging to a resource further up the page.
//
// It runs after deriveFilters rather than beside the other null checks because
// that is when the filters a query declares are known.
func (c *contract) checkPageNullFilters() error {
	if len(c.ExplicitNulls) == 0 || c.EnvelopeField == "" || len(c.Filters) == 0 {
		return nil
	}

	named := make([]string, 0, len(c.Filters))
	for _, filter := range c.Filters {
		named = append(named, filter.Param)
	}

	return fmt.Errorf("%w: %s narrows its page by %s beside explicit_null_fields",
		errNullsOverDecodedResponse, c.Name, strings.Join(named, ", "))
}

// stringsOption reads a repeated string message option, answering nil when it
// is absent.
func stringsOption(options protoreflect.ProtoMessage, extension protoreflect.ExtensionType) []string {
	value, ok := proto.GetExtension(options, extension).([]string)
	if !ok {
		return nil
	}

	return value
}

// stringOption reads a string message option, answering "" when it is absent.
func stringOption(options protoreflect.ProtoMessage, extension protoreflect.ExtensionType) string {
	value, ok := proto.GetExtension(options, extension).(string)
	if !ok {
		return ""
	}

	return value
}

// listEnvelopeShape reads the declared list envelope, answering the unspecified
// shape when the message carries none. Absence is the standard page, which is
// what nearly every collection answers with.
func listEnvelopeShape(options protoreflect.ProtoMessage) linodev1.ListEnvelope_Shape {
	declared, ok := proto.GetExtension(options, linodev1.E_ListEnvelope).(*linodev1.ListEnvelope)
	if !ok {
		return linodev1.ListEnvelope_SHAPE_UNSPECIFIED
	}

	return declared.GetShape()
}

// boolOption reads a bool message option, answering false when it is absent.
func boolOption(options protoreflect.ProtoMessage, extension protoreflect.ExtensionType) bool {
	value, ok := proto.GetExtension(options, extension).(bool)
	if !ok {
		return false
	}

	return value
}

// capabilityOption reads the tier a tool is registered at.
// surfaceMarked is the description a tool advertises, led by its surface when
// that surface is not the default.
//
// The description is the only thing a model choosing a tool reads about where
// the call goes, so the marker leads rather than trails: a listing truncated
// after the first words still says which surface this is. It is derived from
// the annotation at generation time rather than written into tool_description,
// so it cannot disagree with the surface the client actually picks.
func surfaceMarked(surface, description string) string {
	if surface == "" || surface == linoderoute.DefaultSurfaceSegment {
		return description
	}

	return "[" + surface + "] " + description
}

// surfaceOption is the API surface a message declares, the zero value when it
// declares none, which SurfaceSegment reads as the default.
func surfaceOption(options protoreflect.ProtoMessage) linodev1.ApiSurface {
	value, ok := proto.GetExtension(options, linodev1.E_ToolApiSurface).(linodev1.ApiSurface)
	if !ok {
		return linodev1.ApiSurface_API_SURFACE_UNSPECIFIED
	}

	return value
}

func capabilityOption(options protoreflect.ProtoMessage) linodev1.ToolCapability {
	value, ok := proto.GetExtension(options, linodev1.E_ToolCapability).(linodev1.ToolCapability)
	if !ok {
		return linodev1.ToolCapability_TOOL_CAPABILITY_UNSPECIFIED
	}

	return value
}
