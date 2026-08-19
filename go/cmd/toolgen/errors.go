package main

import "errors"

var (
	// errToolNotDeclared means the cohort and the descriptors were read apart.
	errToolNotDeclared = errors.New("tool is not declared by the proto contract")

	// errHandwrittenNotDeclared is the shape a typo in the hand-written list takes.
	errHandwrittenNotDeclared = errors.New("hand-written list names a tool the proto contract does not declare")

	// errNoLanguages means the registry names no language, so a run would emit
	// nothing and still report success.
	errNoLanguages = errors.New("languages registry names no language")

	// errNoRendererArm names a registered language this emitter cannot render.
	// Emitting the rest and staying silent is how a surface goes one language
	// short of what the registry promises.
	errNoRendererArm = errors.New("registered language has no renderer arm")

	// errNoOutputDir names a registered language whose tree has nowhere to land.
	errNoOutputDir = errors.New("registered language has no output directory")

	// errArmMisnamed names an arm table entry that pairs a registry name with a
	// renderer answering to another, which would emit one language's tree into
	// the other's directory and report the registered name while doing it.
	errArmMisnamed = errors.New("renderer arm answers to a different language than the one it is bound to")

	// errNoContractOptions means no option was found to hold the arms to, so the
	// coverage check below would pass over nothing.
	errNoContractOptions = errors.New("the contract declares no options")

	// errUnclaimedOption names an option a renderer arm says nothing about. An
	// option acted by one arm and left out of another is a declaration that
	// means one thing in one language and nothing in the other, which is what
	// errReaderWithValidate refuses at the field level.
	errUnclaimedOption = errors.New("renderer arm does not answer for a declared contract option")

	// errUnknownClaim names an arm answering for an option the contract no
	// longer declares, which reads as coverage of something that is gone.
	errUnknownClaim = errors.New("renderer arm answers for an option the contract does not declare")

	// errRepeatedClaim names an option one arm answers for twice, where the two
	// answers can disagree and only the first would be read.
	errRepeatedClaim = errors.New("renderer arm answers for one option more than once")

	// errSilentClaim names an arm that neither emits an option nor names where
	// its behavior lives, which is the coverage gap wearing an entry.
	errSilentClaim = errors.New("renderer arm neither emits an option nor names the file that acts it")

	// errEmptyCohort would leave every emitter and gate check passing over nothing.
	errEmptyCohort = errors.New("the hand-written list claims every declared tool, so there is nothing to generate")

	// errMetaNotEmitted names a tool with no route, which no emitted tier serves.
	errMetaNotEmitted = errors.New("meta tool reaches no route, and no emitted tier serves one")

	// errNoDescription would otherwise ship a tool the client sees unlabeled.
	errNoDescription = errors.New("tool declares no tool_description")

	// errNoResponse fires on free-form tools no generated serializer can fill.
	errNoResponse = errors.New("tool declares no tool_response")

	// errNoErrorMessage applies to tiers that report failure in prose.
	errNoErrorMessage = errors.New("tool declares no error_message")

	// errNoSuccessMessage would leave a completed mutation reporting nothing.
	errNoSuccessMessage = errors.New("tool declares no success_message")

	// errNoConfirmMessage would let a mutation run without a gate.
	errNoConfirmMessage = errors.New("tool declares no confirm_message")

	// errUnusedSuccessMessage names prose the emitted answer has nowhere to put.
	errUnusedSuccessMessage = errors.New("tool declares a success_message its response cannot carry")

	// errNoWarningMessage would ship the empty warning its response promises.
	errNoWarningMessage = errors.New("response declares a warning field and the tool declares no warning_message")

	// errUnusedWarningMessage names a notice the emitted answer would drop.
	errUnusedWarningMessage = errors.New("tool declares a warning_message its response cannot carry")

	// errNotAWriteEnvelope means the serializer would drop the resource.
	errNotAWriteEnvelope = errors.New("write response is not shaped like a mutation envelope")

	// errNotAWritePage names a mutation answering with a page that carries
	// members the page decode cannot fill, so the caller would read a zero over
	// a change the API has already made.
	errNotAWritePage = errors.New("write response is a page carrying members the route's envelope cannot fill")

	// errNotABodyRead names a read on a body route whose answer is not the
	// resource the call decodes into, so the emitted answer would leave a member
	// of it empty.
	errNotABodyRead = errors.New("read tool sends a body and declares a response that is not the API resource")

	// errNoMarkerCursor names a marker-paged collection whose response cannot
	// report the cursor, so a caller holding a truncated page would read it as
	// the whole collection.
	errNoMarkerCursor = errors.New("marker-paged list response declares no cursor members")

	// errGatedBodyRead names a gate over a call that changes nothing, which
	// would make a read start demanding confirm.
	errGatedBodyRead = errors.New("read tool sends a body and advertises confirm or dry_run, which its tier does not emit")

	// errUnsupportedBodyKind would drop an argument the tool advertises.
	errUnsupportedBodyKind = errors.New("body field has no request representation")

	// errUnsupportedItemKind would leave one member of a typed list item
	// unchecked, which is the one thing the typed reader exists to do.
	errUnsupportedItemKind = errors.New("list item member has no request representation")

	// errRecursiveItemMessage names a declaration whose members reach it
	// again, which no finite reader spec can describe.
	errRecursiveItemMessage = errors.New("body field message contains itself")

	// errNoFieldLocation leaves it unclear whether the field reaches the wire.
	errNoFieldLocation = errors.New("input field declares no field_location")

	// errFoldNotPlaced names an assembled body member the emitter cannot build:
	// a fold into a member no reader merges through, an argument with no single
	// value to place, or a fixed member whose value the contract cannot spell.
	// Every one of them would post a flat argument where the API does not read
	// it, which reads as a working call and changes nothing.
	errFoldNotPlaced = errors.New("body fold names something no assembled member can be built from")

	// errRoutedToolArgument names a domain argument on a tool that builds its
	// own request. FIELD_LOCATION_TOOL is placed by the meta tier and by a tool
	// whose execute hook owns the call; anywhere else the caller's value would
	// be read out of the schema and dropped before the call.
	errRoutedToolArgument = errors.New("routed tool declares a FIELD_LOCATION_TOOL argument without an execute hook to read it")

	// errRedactNotBody names a redaction with nothing to stand in for: only a
	// body member is reported back by a preview.
	errRedactNotBody = errors.New("preview_redact is declared on a field that is not BODY")

	// errBodyNameNotBody names a wire key on a field that reaches no body.
	errBodyNameNotBody = errors.New("body_name is declared on a field that is not BODY")

	// errBodyNameNoop names a rename to the name the field already has.
	errBodyNameNoop = errors.New("body_name repeats the field's own name, so it renames nothing")

	// errNullableNotBody names a declared null on a field that reaches no body.
	errNullableNotBody = errors.New("body_nullable is declared on a field that is not BODY")

	// errNullableNotObject names a declared null on a body field with no null to
	// send: only a free-form object and an optional string carry a value the
	// API reads as one.
	errNullableNotObject = errors.New("body_nullable is declared on a BODY field that is neither a free-form object nor an optional string")

	// errBodyRootNotObject names a hoisted body on a field with no members to
	// hoist: only a free-form object map carries the keys a root body is made
	// of.
	errBodyRootNotObject = errors.New("body_root is declared on a field that is not a BODY object map")

	// errBodyRootNotAlone names a hoisted body beside another body member,
	// which would have nowhere to travel once the map's own keys are the body.
	errBodyRootNotAlone = errors.New("body_root is declared on a tool that also sends another body member")

	// errReaderOnDestroy names a reader on a removal, whose ids are read through
	// tools.DestroyID and its own three sentences, so the declaration would be
	// dropped rather than applied.
	errReaderOnDestroy = errors.New("argument_reader is declared on a destroy tool, which reads its ids through DestroyID")

	// errUnsupportedReader names a declared reader the Go arm has no call for,
	// which is a member added to the contract and not to both renderer arms.
	errUnsupportedReader = errors.New("argument_reader names a reader the Go arm cannot write")

	// errPresentNotBody names a presence reader somewhere it answers nothing: a
	// path slot is always sent, so the check would never refuse.
	errPresentNotBody = errors.New("argument_reader ARGUMENT_READER_PRESENT is declared on a field that is not BODY")

	// errReaderNotPath names a reader on a field that reaches no path segment.
	// Every member answers sentences about a value the route is addressed by, so
	// anywhere else it refuses on grounds the caller was never held to.
	errReaderNotPath = errors.New("argument_reader is declared on a field that is not PATH")

	// errReaderNotPathText names a text reader on a field carrying a number,
	// whose value has no separators to refuse.
	errReaderNotPathText = errors.New("argument_reader ARGUMENT_READER_PATH_SAFE_TEXT is declared on a field that is not a PATH string")

	// errEnumMemberPlacement names a membership reader on a field whose raw
	// argument no handler reads: only PATH and BODY arguments are checked.
	errEnumMemberPlacement = errors.New("argument_reader ARGUMENT_READER_ENUM_MEMBER is declared on a field that is neither PATH nor BODY")

	// errEnumMemberRepeated names a membership reader on a list, whose raw
	// argument is no single string to hold to the vocabulary.
	errEnumMemberRepeated = errors.New("argument_reader ARGUMENT_READER_ENUM_MEMBER is declared on a repeated field")

	// errEnumMemberKind names a membership reader on a field with no vocabulary
	// to read: only an enum carries members and only a string takes declared
	// reader_values.
	errEnumMemberKind = errors.New("argument_reader ARGUMENT_READER_ENUM_MEMBER is declared on a field that is neither an enum nor a string")

	// errEnumMemberNoValues names a membership reader on a string field that
	// declares no reader_values, which leaves the check with nothing to accept.
	errEnumMemberNoValues = errors.New("argument_reader ARGUMENT_READER_ENUM_MEMBER is declared on a string field without reader_values")

	// errPresentTextShape names a text-presence reader on a field it cannot
	// read: its one sentence is about a required piece of text, so anywhere but
	// a singular BODY string it refuses on grounds the caller was never held to.
	errPresentTextShape = errors.New("argument_reader ARGUMENT_READER_PRESENT_TEXT is declared on a field that is not a singular BODY string")

	// errPresentBoolShape names a boolean-presence reader on a field whose
	// argument is not a singular bool, which has no boolean to demand. A query
	// flag is asked the same question as a body member: whether the caller sent
	// one, which only the argument map answers.
	errPresentBoolShape = errors.New("argument_reader ARGUMENT_READER_PRESENT_BOOL is declared on a field that is not a singular BODY or QUERY bool")

	// errPresentStringShape names a string-presence reader on a field whose
	// argument is not a singular BODY string, which has no string to demand.
	errPresentStringShape = errors.New("argument_reader ARGUMENT_READER_PRESENT_STRING is declared on a field that is not a singular BODY string")

	// errIDListShape names an id-list reader on a field that is not a repeated
	// BODY integer, which has no list of ids to hold to the shape.
	errIDListShape = errors.New("argument_reader ARGUMENT_READER_ID_LIST is declared on a field that is not a repeated BODY integer")

	// errReaderWithValidate names an argument_reader beside a validate hook.
	// The hook owns the whole argument check, so Go drops the reader and Python
	// acts it, and a declaration that means one thing in one language and
	// nothing in the other cannot be allowed to build.
	errReaderWithValidate = errors.New("argument_reader is declared beside a validate hook")

	// errRefuseArguments names a refuse_arguments declaring half of itself: a
	// name with no sentence refuses in words nobody wrote, and a sentence with
	// no name refuses nothing.
	errRefuseArguments = errors.New("refuse_arguments declares fields without a message, or a message without fields")

	// errRefuseWithValidate names a refuse_arguments beside a validate hook,
	// which already owns the whole argument check.
	errRefuseWithValidate = errors.New("refuse_arguments is declared beside a validate hook")

	// errRefuseUnknownWithValidate names a refuse_unknown_arguments beside a
	// validate hook, for the same reason.
	errRefuseUnknownWithValidate = errors.New("refuse_unknown_arguments is declared beside a validate hook")

	// errRefuseDeclaredField names a refuse_arguments entry the message
	// declares, which its own field already answers for.
	errRefuseDeclaredField = errors.New("refuse_arguments names a field the input message declares")

	// errAnyOfTooFew names a require_any_of naming fewer than two fields, which
	// is ARGUMENT_READER_PRESENT wearing a list.
	errAnyOfTooFew = errors.New("require_any_of names fewer than two fields")

	// errAnyOfUnknownField names a require_any_of entry the message declares no
	// BODY field for, which the check would wait on forever.
	errAnyOfUnknownField = errors.New("require_any_of names a field the message does not declare as BODY")

	// errAnyOfDuplicateField names a field require_any_of lists twice, which
	// counts one answer as two.
	errAnyOfDuplicateField = errors.New("require_any_of names the same field twice")

	// errAnyOfWithValidate names a require_any_of beside a validate hook, which
	// already owns the tool's whole argument check.
	errAnyOfWithValidate = errors.New("require_any_of is declared beside a validate hook")

	// The normalize_fields refusals. A rewrite that does not land is invisible
	// afterwards: the call goes through carrying the value the caller sent, and
	// nothing downstream says the declared transform never ran.

	// errNormalizeWithHook names a normalize_fields declaration beside a
	// normalize hook, which already answers the tool's whole argument map.
	errNormalizeWithHook = errors.New("normalize_fields is declared beside a normalize hook")

	// errNormalizeNoField names a declared transform with no argument to rewrite.
	errNormalizeNoField = errors.New("normalize_fields declares a transform over no field")

	// errNormalizeUnknownField names an argument the input message does not
	// declare, whose rewrite would never reach a value.
	errNormalizeUnknownField = errors.New("normalize_fields names a field the message does not declare")

	// errNormalizeRepeatedField names an argument two declarations rewrite,
	// whose result would be decided by declaration order.
	errNormalizeRepeatedField = errors.New("normalize_fields names an argument another declaration already rewrites")

	// errNormalizeSecretField names an argument whose name marks it as the
	// credential itself, which a rewrite would change out from under the caller.
	errNormalizeSecretField = errors.New("normalize_fields rewrites an argument named as a secret")

	// errUnsupportedTransform names a declared transform no arm renders, which
	// would reach one tree as a call and the other as silence.
	errUnsupportedTransform = errors.New("normalize_fields declares a transform the emitter does not render")

	// The object_walk refusals. Each names a declaration the walker would read
	// and drop, which is the one failure a declared check can have that nothing
	// downstream reports: the call goes through and the caller is told nothing.

	// errWalkWithValidate names an object_walk beside a validate hook, which
	// already owns the tool's whole argument check.
	errWalkWithValidate = errors.New("object_walk is declared beside a validate hook")

	// errWalkTier names an object_walk on a tier that emits no walk, where the
	// declaration would be read and dropped.
	errWalkTier = errors.New("object_walk is declared on a tier with no argument walk")

	// errWalkNoField names an object_walk naming no argument to walk.
	errWalkNoField = errors.New("object_walk names no field")

	// errWalkUnknownField names an object_walk entry the message declares no
	// open-object BODY field for.
	errWalkUnknownField = errors.New("object_walk names a field the message does not declare as an open-object BODY member")

	// errWalkRepeatedField names an argument two walks read, whose second
	// declaration would answer for a value the first already accepted.
	errWalkRepeatedField = errors.New("object_walk names an argument another walk already reads")

	// errWalkSilent names an object_walk that refuses nothing, which is a
	// declaration the walker reads and drops.
	errWalkSilent = errors.New("object_walk declares no sentence")

	// errWalkElementOnMap names an element sentence on a map argument, which has
	// no entries to be objects.
	errWalkElementOnMap = errors.New("object_walk words element on a map field, which has no list entries")

	// errWalkEmptyOnList names an empty sentence on a list argument, where proto3
	// reads an absent list and an empty one as the same value, so the sentence
	// would answer for a call the caller never made.
	errWalkEmptyOnList = errors.New("object_walk words empty on a list field, which cannot tell an absent list from an empty one")

	// errWalkValueWithoutKey names a shared value spec with no key to apply to.
	errWalkValueWithoutKey = errors.New("object_walk declares a value with no key to apply it to")

	// errWalkDuplicateKey names a key declared both as a shared-value key and as
	// a member of its own, which is two specs for one key.
	errWalkDuplicateKey = errors.New("object_walk declares the same key twice")

	// errWalkUnknownWithoutVocabulary names an unknown sentence over an empty
	// vocabulary, which would refuse every key the caller can send.
	errWalkUnknownWithoutVocabulary = errors.New("object_walk words unknown over an empty vocabulary")

	// errWalkRequireUnknownName names a require arm about a key the vocabulary
	// does not carry, which no object can name.
	errWalkRequireUnknownName = errors.New("object_walk requires a key outside its vocabulary")

	// errWalkBoundKind names a bound on a kind that does not carry one.
	errWalkBoundKind = errors.New("object_walk declares a bound the value's kind does not read")

	// errWalkMembersKind names nested members on a value that is not an object.
	errWalkMembersKind = errors.New("object_walk declares members on a value that is not an object")

	// errWalkUnknownEnum names a vocabulary whose enum the contract does not
	// declare, which would leave the value held to nothing.
	errWalkUnknownEnum = errors.New("object_walk names an enum the contract does not declare")

	// errWalkPlaceholder names a sentence naming something the walk cannot fill,
	// which reaches the caller with the braces still in it.
	errWalkPlaceholder = errors.New("object_walk sentence names a placeholder the walk cannot fill")

	// errAnyOfTier names a require_any_of on a tier that runs no body checks,
	// where the declaration would be read and dropped.
	errAnyOfTier = errors.New("require_any_of is declared on a tool that is not on the write tier")

	// errReaderValuesOnEnum names declared reader_values beside an enum's own
	// members, which is two vocabularies for one field.
	errReaderValuesOnEnum = errors.New("reader_values is declared on an enum field, whose descriptor already carries the vocabulary")

	// errReaderValuesAlone names reader_values without the one reader that acts
	// on them, so the declaration would be read and dropped.
	errReaderValuesAlone = errors.New("reader_values is declared without ARGUMENT_READER_ENUM_MEMBER")

	// errReaderMessageAlone names wording with no reader to word, which is a
	// sentence written into the contract that no caller can ever be answered.
	errReaderMessageAlone = errors.New("reader_message is declared without an argument_reader")

	// errReaderMessageArm names wording for a refusal the declared member never
	// answers, which is the same sentence going nowhere.
	errReaderMessageArm = errors.New("reader_message words an arm the declared argument_reader does not answer")

	// errReaderNotPathInteger names a positive-id reader on a field it cannot
	// read: the sentences it answers are about an id addressing a resource, so
	// anywhere else it would refuse a value on grounds the caller never met.
	errReaderNotPathInteger = errors.New("argument_reader ARGUMENT_READER_POSITIVE_ID is declared on a field that is not a PATH integer")

	// errCommaListNotBody names a comma composition on a field that reaches no
	// body, which has no member to compose.
	errCommaListNotBody = errors.New("body_comma_list is declared on a field that is not BODY")

	// errCommaListNotString names a comma composition on a field with no text
	// to split: a repeated field already carries the array, and one with no
	// presence would post an empty one over a caller who named nothing.
	errCommaListNotString = errors.New("body_comma_list is declared on a BODY field that is not an optional string")

	// errEchoArgumentShape names an echo alias on a response member no scalar
	// echo can fill.
	errEchoArgumentShape = errors.New("echo_argument is declared on a response member that is repeated or carries a message")

	// errEchoArgumentNoop names an echo alias to the name the member already
	// has, which resolves exactly where the member's own name does.
	errEchoArgumentNoop = errors.New("echo_argument repeats the response member's own name, so it aliases nothing")

	// errNoGoType means the descriptors and the generated tree disagree.
	errNoGoType = errors.New("no Go type generated for proto message")

	errAmbiguousEnvelope = errors.New("list response is not shaped like a page")

	// errUnmatchedFilter would advertise a filter that applies to nothing.
	errUnmatchedFilter = errors.New("filter argument matches no element field")

	// errUnsupportedTier keeps a cohort entry from silently producing nothing.
	errUnsupportedTier = errors.New("tool tier is not yet emitted")

	// errPyRender means the contract cannot answer something the Python
	// renderer needs, which stops the run rather than emitting a factory that
	// says less than the contract does.
	errPyRender = errors.New("contract has no Python rendering")

	// errPyFormat means the emitted tree could not be run through the formatter
	// the repo lints with.
	errPyFormat = errors.New("emitted Python could not be formatted")

	// errPyLineBudget names a sentence too long to render: ruff cannot rewrap
	// prose, so a line still over the budget after formatting fails the run.
	errPyLineBudget = errors.New("emitted Python line is over the budget")

	// errPathArity means the template would be filled from the wrong values.
	errPathArity = errors.New("route slots do not match the PATH fields")

	errUnsupportedPathKind = errors.New("path field has no route representation")

	// errUnsupportedQuery names page controls the reader cannot read as a pair.
	errUnsupportedQuery = errors.New("query arguments have no reader on this tier")

	// errNoReadPreview would answer a dry run by making the call it previews.
	errNoReadPreview = errors.New("read tool advertises dry_run with no confirm argument to gate the fetch behind")

	// errNoStructFailPrefix leaves a free-form read's two failures worded apart.
	errNoStructFailPrefix = errors.New("free-form read's error_message does not end with the error it reports")

	// errNoNormalizePoint names a tier whose arguments the driver reads, so a
	// rewrite emitted in the factory would run after the reads it must precede.
	errNoNormalizePoint = errors.New("tier reads its arguments in the driver, ahead of any normalize hook")

	// errNotAWrapper means a response named an envelope is shaped otherwise.
	errNotAWrapper = errors.New("response is named a get envelope and is not shaped like one")

	// errUnknownPlaceholder names no field of the input message.
	errUnknownPlaceholder = errors.New("message template names an unknown field")

	// errRepeatedPlaceholder would render a list as one value, which the two
	// languages spell differently.
	errRepeatedPlaceholder = errors.New("message template names a repeated field without the count form")

	// errNotRepeated counts a field that carries nothing to count.
	errNotRepeated = errors.New("message template counts a field that is not repeated")

	// errUngatedExecute names an execute hook on a tier with no call to hand
	// over. The acknowledge tier and the read tier both assemble their answer
	// from the call rather than decoding it, so a hook there owes back nothing
	// or one member. Any other tier needs the hook to answer with the decoded
	// message, which is a second signature for one kind.
	errUngatedExecute = errors.New("tool declares an execute hook on a tier that decodes its answer, which the hook cannot supply")

	// errNotAnAssembledRead names a read whose call a hook makes but whose
	// response the hook cannot fill: one LOCAL string member carries what the
	// hook brought back, and every other member echoes an argument.
	errNotAnAssembledRead = errors.New("read declares an execute hook and answers with a shape the hook cannot fill")

	// errUnknownHookKind catches a kind that would be read and then ignored.
	errUnknownHookKind = errors.New("tool declares an unknown hook kind")

	// errRepeatedHookKind leaves it unclear which of the two calls is emitted.
	errRepeatedHookKind = errors.New("tool declares one hook kind more than once")

	// errNotADeleteEnvelope means the answer could not be built from the call.
	errNotADeleteEnvelope = errors.New("delete response is not shaped like an id echo")

	// errNoFetchState would preview and hash a resource nothing read.
	errNoFetchState = errors.New("tool declares no fetch_state hook to plan through")
	// errStateRouteWithHook would read the resource twice, in an order nothing
	// states: the hook already answers the whole fetch.
	errStateRouteWithHook = errors.New("tool declares state_route beside a fetch_state hook")
	// errStateRouteWithPreviewHook would read the resource and drop it: the hook
	// answers the whole dry run, including whatever fetch its prose is diffed
	// against.
	errStateRouteWithPreviewHook = errors.New("tool declares state_route beside a preview hook")
	// errStateRouteWithNoReader names a declaration no step ever calls. A
	// removal reads state for its plan and its preview; every other tier reads
	// it for the dry run alone, so one without a dry run would fetch nothing.
	errStateRouteWithNoReader = errors.New("tool declares state_route but neither removes a resource nor previews one")
	// errStateRouteUnknownTool would resolve to no route at call time and answer
	// an empty preview.
	errStateRouteUnknownTool = errors.New("tool declares state_route naming no declared tool")
	// errStateRouteNotRead names a state route that changes what it reports.
	errStateRouteNotRead = errors.New("tool declares state_route naming a route that is not a GET")
	// errStateRouteNoResponse would leave the fetch with no message to decode
	// into.
	errStateRouteNoResponse = errors.New("tool declares state_route naming a read that declares no response")
	// errStateRoutePayloadMember names a payload member the read's answer does
	// not carry, which would project every key out of the state and leave the
	// preview reporting an empty resource.
	errStateRoutePayloadMember = errors.New("tool declares state_route naming a payload member its read does not carry")
	// errStateRoutePayloadWithBodyKey names both unwraps at once. One reads
	// past a response wrapper and the other past a raw-body one, and no read
	// wraps its resource both ways, so the pair would ship untested.
	errStateRoutePayloadWithBodyKey = errors.New("tool declares state_route with both payload_member and body_key")
	// errStateRouteUnknownSlot would address the collection instead of the
	// resource, since the missing slot would be filled with a zero.
	errStateRouteUnknownSlot = errors.New("tool declares state_route whose route names a path argument the tool does not carry")
	// errStateRouteSlotUnknownRead maps a slot the read route never had, so the
	// slot it was meant to cover would still go unfilled.
	errStateRouteSlotUnknownRead = errors.New("tool declares state_route mapping a read slot its read does not carry")
	// errStateRouteSlotUnknownArgument maps to an argument the removal does not
	// carry, which is the zero fill the mapping exists to prevent.
	errStateRouteSlotUnknownArgument = errors.New("tool declares state_route mapping a read slot to a path argument the tool does not carry")
	// errStateRouteSlotSameName maps a name onto itself, which matching by name
	// already does, so the entry is wiring with no effect.
	errStateRouteSlotSameName = errors.New("tool declares state_route mapping a read slot to its own name")

	// errPreviewSentenceWithHook would report the prose twice or not at all: the
	// hook already answers the whole dry run.
	errPreviewSentenceWithHook = errors.New("tool declares preview_sentence beside a preview hook")
	// errPreviewSentenceNoTemplate names a declared line with no wording, which
	// would report an empty sentence.
	errPreviewSentenceNoTemplate = errors.New("tool declares a preview_sentence with no template")
	// errPreviewSentenceNoLine names a line belonging to neither half of the
	// preview. Read as a side effect it would tell a caller the call does what
	// it was being warned about.
	errPreviewSentenceNoLine = errors.New("tool declares a preview_sentence naming no line")
	// errPreviewSentenceUnclosed names a wording whose braces do not pair, which
	// would report the placeholder rather than the value.
	errPreviewSentenceUnclosed = errors.New("tool declares a preview_sentence with an unclosed placeholder")
	// errPreviewSentenceUnknownArgument would report a value the call never
	// carries, leaving a blank where the sentence names something.
	errPreviewSentenceUnknownArgument = errors.New("tool declares a preview_sentence reading an argument the message does not declare")
	// errPreviewSentenceUnreportable names an argument with no single spelling
	// both languages write it with, which would put the two trees a character
	// apart on the same call.
	errPreviewSentenceUnreportable = errors.New("tool declares a preview_sentence reading an argument that is not text or a whole number")
	// errPreviewSentenceSecretArgument would put a credential in the prose the
	// same preview redacts out of the body.
	errPreviewSentenceSecretArgument = errors.New("tool declares a preview_sentence reading an argument named as a secret")
	// errPreviewSentenceUnreachable names a wording no call could reach, since
	// an earlier one reads a subset of the same arguments and wins first.
	errPreviewSentenceUnreachable = errors.New("tool declares a preview_sentence template an earlier template always wins over")
	// errStateRouteUnfilledQuery names a read addressed by a query parameter the
	// declaring tool fills nothing into, which would fetch the collection rather
	// than the resource and report it as the resource's own state.
	errStateRouteUnfilledQuery = errors.New("tool declares a state_route through a read whose required query parameter nothing fills")
	// errStateRouteQueryAmbiguous names a parameter spelled both in the read's
	// path and in its query, where nothing says which one a mapping means.
	errStateRouteQueryAmbiguous = errors.New("tool declares a state_route through a read naming one slot in both its path and its query")
	// errStateRouteQuerySecret would put a credential on the URL of the fetch.
	errStateRouteQuerySecret = errors.New("tool declares a state_route filling a query parameter from an argument named as a secret")

	// errPreviewChoiceWithTemplate names both ways of saying what one line
	// reports, which leaves the answer to declaration order rather than to the
	// contract.
	errPreviewChoiceWithTemplate = errors.New("tool declares a preview_sentence carrying both template and chosen_by")
	// errPreviewChoiceNoArgument names a selection with nothing to select on.
	errPreviewChoiceNoArgument = errors.New("tool declares a preview_sentence chosen_by naming no argument")
	// errPreviewChoiceNoWording names a choice whose every arm is empty, which
	// is a line no call could ever report.
	errPreviewChoiceNoWording = errors.New("tool declares a preview_sentence chosen_by with no wording in any arm")
	// errPreviewChoiceUnknownArgument would select on a value the call never
	// carries, so the line would answer its absent arm forever.
	errPreviewChoiceUnknownArgument = errors.New("tool declares a preview_sentence chosen_by an argument the message does not declare")
	// errPreviewChoiceNotFlag names a selector that is not a bool: an enum or a
	// number choosing between wordings is a different rule than this one.
	errPreviewChoiceNotFlag = errors.New("tool declares a preview_sentence chosen_by an argument that is not a bool")
	// errPreviewChoiceNotObject names a dotted selector whose head carries no
	// members, so the member named could never be read off it.
	errPreviewChoiceNotObject = errors.New("tool declares a preview_sentence chosen_by a member of an argument that is not an open object")
	// errPreviewChoiceDeepMember names a path past the one level the rule
	// reaches, where every further level guesses at a shape the contract does
	// not model.
	errPreviewChoiceDeepMember = errors.New("tool declares a preview_sentence chosen_by a member more than one level deep")

	// errPreviewOmitsBodyWithHook names the no-echo flag beside a hook that
	// decides the whole report itself.
	errPreviewOmitsBodyWithHook = errors.New("tool declares preview_omits_body beside a preview hook")
	// errPreviewOmitsBodyWithoutBody names the flag on a tool whose preview
	// reports no body to begin with, which would read as a rule that never runs.
	errPreviewOmitsBodyWithoutBody = errors.New("tool declares preview_omits_body and sends no body")

	// errPartialTwoStage names half the plan/apply flow: a caller handed one of
	// the two arguments can reach neither stage.
	errPartialTwoStage = errors.New("tool advertises one of mode and plan_id without the other")

	// errNoTwoStageDriver names the plan/apply arguments on a tier with no flow
	// behind them, which is the shape the hole took before this tier had one:
	// the schema advertises the stages and the handler drops the arguments.
	errNoTwoStageDriver = errors.New("tool advertises mode and plan_id on a tier that emits no plan/apply flow")

	// errUnstagedHook names a plan's state read or dependency walk on a tool
	// that advertises no plan, so the hook would be resolved and never called.
	errUnstagedHook = errors.New("tool declares a plan hook without advertising mode and plan_id")

	// errNoNullPayload names a tier with no decoded body to restore nulls into.
	errNoNullPayload = errors.New("tool declares explicit_null_fields and decodes no API body")

	// errUnknownNullField would restore a key the answer cannot carry.
	errUnknownNullField = errors.New("explicit_null_fields names a field the response does not declare")

	// errUnsupportedDestroyShape names a destroy no emitted driver addresses.
	errUnsupportedDestroyShape = errors.New("destroy tool has no emitted driver for its path shape")

	// errUnplacedLocalMember names a response member declared as assembled from
	// the call where nothing assembles it, so the declaration would be read and
	// then ignored.
	errUnplacedLocalMember = errors.New("response member is declared LOCAL on a tier that decodes its whole answer")

	// errUnknownResponseBody names a member the API's answer cannot fill, or
	// one something else already fills.
	errUnknownResponseBody = errors.New("response_body_fields names a member the answer cannot fill")

	// errUnusedResponseBody names a response that decodes its whole answer
	// already, so the declaration would be read and then ignored.
	errUnusedResponseBody = errors.New("tool declares response_body_fields on a response it decodes whole")

	// errEmptyDefault names a default that says nothing: absence already
	// renders as the field's zero, so {name|} declares the state it replaces.
	errEmptyDefault = errors.New("message template declares an empty default")

	// errModifiedDefault names a placeholder asking for a count or a width and
	// a default at once, which is two forms of one value.
	errModifiedDefault = errors.New("message template declares a default beside a count or a width")

	// errDefaultNotToolArg names a default on an argument that addresses the
	// request, where an absent value is refused rather than substituted.
	errDefaultNotToolArg = errors.New("message template declares a default on a field that is not a TOOL argument")

	// errUnsupportedToolArg names a tool argument no accessor reads.
	errUnsupportedToolArg = errors.New("message template names a TOOL argument with no request representation")

	// errUnreadableDefault names a default the field's kind cannot hold.
	errUnreadableDefault = errors.New("message template declares a default the field's kind cannot hold")

	// errMetaDryRun names a preview on a tool that makes no call, so there is
	// nothing for the dry run to report and nothing for it to hold back.
	errMetaDryRun = errors.New("meta tool advertises dry_run, and its answer reaches no route to preview")

	// errNoMetaAnswer names a meta tool that answers with neither a hook nor a
	// sentence, which leaves the emitted handler with nothing to return.
	errNoMetaAnswer = errors.New("meta tool declares no answer hook and no success_message")

	// errMetaTwoAnswers names a meta tool declaring both, where the emitted
	// handler would return the hook's result and drop the sentence.
	errMetaTwoAnswers = errors.New("meta tool declares an answer hook beside a success_message")

	// errNoMetaMessageField names a meta tool whose declared sentence has
	// nowhere to be reported.
	errNoMetaMessageField = errors.New("meta tool declares a success_message its response has no message field for")

	// errUngatedAnswer names an answer hook on a tier that builds a request.
	// The hook takes no client and returns the whole result, so a tool that
	// calls Linode has no point to hand over at.
	errUngatedAnswer = errors.New("tool declares an answer hook on a tier that reaches a route")

	// errUnservedMetaHook names a hook kind the meta tier has no step for: a
	// preview of a call it never makes, or a destroy's state read.
	errUnservedMetaHook = errors.New("meta tool declares a hook kind its tier does not run")

	// errNullsOverDecodedResponse would look the named keys up a level above the
	// resource they belong to: the raw body of a decoded envelope is the
	// envelope, not the resource inside it.
	errNullsOverDecodedResponse = errors.New("tool declares explicit_null_fields beside response_body_fields")
)
