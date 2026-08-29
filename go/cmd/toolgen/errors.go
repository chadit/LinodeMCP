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
	// whose declared transport owns the call; anywhere else the caller's value
	// would be read out of the schema and dropped before the call.
	errRoutedToolArgument = errors.New("routed tool declares a FIELD_LOCATION_TOOL argument without an execute_transport to read it")

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

	// errRefuseArguments names a refuse_arguments declaring half of itself: a
	// name with no sentence refuses in words nobody wrote, and a sentence with
	// no name refuses nothing.
	errRefuseArguments = errors.New("refuse_arguments declares fields without a message, or a message without fields")

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

	// The normalize_fields refusals. A rewrite that does not land is invisible
	// afterwards: the call goes through carrying the value the caller sent, and
	// nothing downstream says the declared transform never ran.

	// errNormalizeNoField names a declared transform with no argument to rewrite.
	errNormalizeNoField = errors.New("normalize_fields declares a transform over no field")

	// errNormalizeUnknownField names an argument the input message does not
	// declare, whose rewrite would never reach a value.
	errNormalizeUnknownField = errors.New("a normalize declaration names a field the message does not declare")

	// errNormalizeRepeatedField names an argument two declarations rewrite,
	// whose result would be decided by declaration order.
	errNormalizeRepeatedField = errors.New("a normalize declaration names an argument another declaration already rewrites")

	// errNormalizeSecretField names an argument whose name marks it as the
	// credential itself, which a rewrite would change out from under the caller.
	errNormalizeSecretField = errors.New("a normalize declaration rewrites an argument named as a secret")

	// The normalize_fold refusals: a fold that cannot land would leave the
	// convenience argument traveling beside the shape it was meant to become.

	// errFoldEmptyKey names a fold with no member to write to.
	errFoldEmptyKey = errors.New("normalize_fold declares no target key")

	// errFoldSelfTarget names a fold whose source is its own target, which is
	// a rename, and the schema handles renames.
	errFoldSelfTarget = errors.New("normalize_fold folds an argument into itself")

	// errFoldSourceKind names a fold whose source is not the repeated integer
	// field the rendered helper reads.
	errFoldSourceKind = errors.New("normalize_fold's source is not a repeated integer BODY field")

	// errFoldTargetKind names a fold whose target is not an open-object BODY
	// field, which is the only shape with a member to receive the fold.
	errFoldTargetKind = errors.New("normalize_fold's target is not an open-object BODY field")

	// errUnsupportedTransform names a declared transform no arm renders, which
	// would reach one tree as a call and the other as silence.
	errUnsupportedTransform = errors.New("normalize_fields declares a transform the emitter does not render")

	// The object_walk refusals. Each names a declaration the walker would read
	// and drop, which is the one failure a declared check can have that nothing
	// downstream reports: the call goes through and the caller is told nothing.

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
	errNoNormalizePoint = errors.New("tier reads its arguments in the driver, ahead of any rewrite point")

	// errNotAWrapper means a response named an envelope is shaped otherwise.
	errNotAWrapper = errors.New("response is named a get envelope and is not shaped like one")

	// errUnknownPlaceholder names no field of the input message.
	errUnknownPlaceholder = errors.New("message template names an unknown field")

	// errRepeatedPlaceholder would render a list as one value, which the two
	// languages spell differently.
	errRepeatedPlaceholder = errors.New("message template names a repeated field without the count form")

	// errNotRepeated counts a field that carries nothing to count.
	errNotRepeated = errors.New("message template counts a field that is not repeated")

	// errUngatedExecute names a declared transport on a tier with no call to
	// hand over. The acknowledge tier and the read tier both assemble their
	// answer from the call rather than decoding it, so a transport there owes
	// back nothing or a few members. Any other tier would need it to answer with
	// the decoded message, which no arm can do.
	errUngatedExecute = errors.New("tool declares an execute_transport on a tier that decodes its answer, which no transport can supply")

	// errNotAnAssembledRead names a read whose call a transport makes but whose
	// response no arm can fill: LOCAL scalar members carry what the transfer
	// brought back, and every other member echoes an argument.
	errNotAnAssembledRead = errors.New("read declares an execute_transport and answers with a shape no transport can fill")

	// errNoTransportArm names an execute_transport carrying no arm, which
	// selects a transport and then says nothing about it.
	errNoTransportArm = errors.New("tool declares an execute_transport with no arm")

	// errTransportIncomplete names an arm missing a field its own shape needs,
	// which would emit a call with a blank where a name belongs.
	errTransportIncomplete = errors.New("execute_transport arm is missing a field its shape needs")

	// errTransportDirectionFields names an arm declaring the other direction's
	// fields. An upload has no answer member to fill, and a download reads no
	// source argument, so either one is a declaration nothing reads.
	errTransportDirectionFields = errors.New("execute_transport arm declares fields its direction does not read")

	// errNoTransferDirection names an arm whose direction is unset, which the
	// zero value would otherwise read as a download.
	errNoTransferDirection = errors.New("execute_transport arm declares no transfer direction")

	// errTransportUnknownArgument names an argument the input message does not
	// declare, which the transport would read as an empty string.
	errTransportUnknownArgument = errors.New("execute_transport reads an argument the input does not declare")

	// errTransportMembers names a transport and a response that disagree about
	// what is filled: an unfilled assembled member ships a zero the caller reads
	// as real, and a filled member the response does not assemble is dropped.
	errTransportMembers = errors.New("execute_transport fills members the response does not assemble")

	// errNotADeleteEnvelope means the answer could not be built from the call.
	errNotADeleteEnvelope = errors.New("delete response is not shaped like an id echo")

	// errNoStateRead would preview and hash a resource nothing read.
	errNoStateRead = errors.New("tool declares no state read to plan through")
	// errStateRouteWithNoReader names a declaration no step ever calls. A
	// removal reads state for its plan and its preview; every other tier reads
	// it for the dry run alone, so one without a dry run would fetch nothing.
	errStateRouteWithNoReader = errors.New("tool declares state_route but neither removes a resource nor previews one")
	// errStateRouteUnknownTool would resolve to no route at call time and answer
	// an empty preview.
	errStateRouteUnknownTool = errors.New("tool declares state_route naming no declared tool")

	// The scan, envelope, and composite state-form refusals: each declares a
	// fetch shape one detail short of renderable, which would otherwise ship
	// as a preview reading the wrong thing.

	// errScanWithResource names a match beside payload_member or body_key,
	// which describe a single-resource answer a scan does not read.
	errScanWithResource = errors.New("state_route match is declared beside payload_member or body_key")

	// errEnvelopeWithResource names envelope_state beside a single-resource
	// declaration or a match, each of which says the answer is one resource.
	errEnvelopeWithResource = errors.New("envelope_state is declared beside payload_member, body_key, or match")

	// errStateReadNotList names a scan or envelope read whose response is not
	// a page: there is no collection to scan or envelope to keep.
	errStateReadNotList = errors.New("the declared read answers no page")

	// errMatchUnknownField names a pair matching on a member the read's
	// element does not declare, which would compare against nothing.
	errMatchUnknownField = errors.New("state_route match names a field the read's element does not declare")

	// errCompositeUnstaged names a composite on a tool that advertises no
	// plan, which has nothing to hash the state for.
	errCompositeUnstaged = errors.New("state_composite is declared on a tool that advertises no plan")

	// errCompositeTooFew names a composite of fewer than two calls: one call
	// with a field subset is a state_route, not a composition.
	errCompositeTooFew = errors.New("state_composite declares fewer than two calls")

	// errCompositeMember names a call landing no member or one a second call
	// also lands, which would leave the state decided by call order.
	errCompositeMember = errors.New("state_composite lands a repeated or empty member")

	// errCompositeNoFields names a call keeping nothing, which reads nothing.
	errCompositeNoFields = errors.New("state_composite keeps no fields for a call")

	// The dependency_walk refusals: a walk that cannot land would preview a
	// blast radius with a hole in it.

	// errWalkSourceCount names a walk with no iteration source or two.
	errWalkSourceCount = errors.New("dependency_walk names no single iteration source")

	// errWalkNoEmit names a walk whose elements become nothing.
	errWalkNoEmit = errors.New("dependency_walk reads elements into no emission")

	// errWalkErrorWarning names a route walk without its failure sentence, or
	// the sentence without a route to fail.
	errWalkErrorWarning = errors.New("list_error_warning must pair with list_tool")

	// errWalkEmitKind names an emission spelling its kind both ways, neither
	// way, or a fallback with no field to fall back from.
	errWalkEmitKind = errors.New("a walk emission names its kind in exactly one way")

	// errWalkEmitEmpty names an emission with neither an action nor a note.
	errWalkEmitEmpty = errors.New("a walk emission carries no line worth saying")

	// errWalkFilterShape names a filter missing its field or comparing
	// against both a literal and an argument.
	errWalkFilterShape = errors.New("a walk filter compares one field against one source")

	// errWalkWarningShape names a warning with no template, half an equality
	// predicate, or two predicates.
	errWalkWarningShape = errors.New("a walk warning carries one predicate at most")

	// errWalkNoState names a state-fed walk on a tool with no declared read
	// to feed it.
	errWalkNoState = errors.New("dependency_walk needs a declared state read to feed it")

	// errWalkStateMember names a state member that is not a repeated member
	// the walk can iterate, or a scalar the state does not carry.
	errWalkStateMember = errors.New("dependency_walk walks a member the state does not carry")

	// errDepWalkUnknownField names an emitted or filtered field the element
	// does not declare.
	errDepWalkUnknownField = errors.New("dependency_walk reads a field the element does not declare")

	// errDepWalkPlaceholder names a template placeholder outside the declared
	// vocabulary.
	errDepWalkPlaceholder = errors.New("a dependency_walk template names a placeholder nothing fills")

	// errBillingShape names a billing estimate missing part of its flow.
	errBillingShape = errors.New("billing_delta names its tool, field, and both sentences")

	// errWalkEnrichShape names an enrichment reading a list, which has no
	// single object to decorate from.
	errWalkEnrichShape = errors.New("walk enrich reads one object")

	// errWalkEmitLabel names a label fallback with nothing to fall back from.
	errWalkEmitLabel = errors.New("walk emit label_fallback requires label_field")

	// errWalkSentence names a preview sentence a walk closure cannot seed.
	errWalkSentence = errors.New("preview sentences beside a declared walk are constant side-effect lines")

	// errCompositeUnknownField names a kept field the read does not declare.
	errCompositeUnknownField = errors.New("state_composite keeps a field the read does not declare")
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

	// errPreviewTransportNotUpload names a wording reading what a presigned
	// upload would send on a tool that declares no such transport, so nothing
	// would ever measure the value.
	errPreviewTransportNotUpload = errors.New("tool declares a preview_sentence reading a transport member and no presigned upload")
	// errPreviewTransportMember names a transport member the upload guard does
	// not measure: the size is the one reading a dry run takes before the call.
	errPreviewTransportMember = errors.New("tool declares a preview_sentence reading a transport member that is not the transport's size_field")
	// errPreviewTransportWithState names a transfer preview beside a state read.
	// The two word their lines against different subjects, and no arm renders a
	// closure over both.
	errPreviewTransportWithState = errors.New("tool declares a preview_sentence reading a transport member beside a declared state read")
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
	// errPreviewPresentWithChoice guards a line a flag already selects, where the
	// declaration's own absent arm is what says the flag was not carried.
	errPreviewPresentWithChoice = errors.New("tool declares a preview_sentence carrying both when_present and chosen_by")
	// errPreviewNumberGuardMixed reads one number two ways: guarded on one line,
	// where zero is a value, and unguarded on another, where zero is no value.
	errPreviewNumberGuardMixed = errors.New("tool declares a preview_sentence reading one number both guarded and unguarded")
	// errPreviewElementWithoutList reads an entry on a line written once, where
	// nothing would fill the placeholder.
	errPreviewElementWithoutList = errors.New("tool declares a preview_sentence reading {element} with no per_element list")
	// errPreviewElementUnread names a list a line is written over and never
	// reports, which is a line repeated with nothing to tell the copies apart.
	errPreviewElementUnread = errors.New("tool declares per_element on a line whose wording never reads {element}")
	// errPreviewElementNotRepeated names a value that is not a list, which has
	// no entries to write a line each over.
	errPreviewElementNotRepeated = errors.New("tool declares per_element over an argument that is not a repeated one the message declares")
	// errPreviewElementUnreportable names entries with no one spelling both
	// languages write them with.
	errPreviewElementUnreportable = errors.New("tool declares per_element over entries that are not text or whole numbers")
	// errPreviewElementBesideLine names a per-entry line sharing its half with
	// another, where one call answers with a list and the other with a string
	// and nothing here says how they join.
	errPreviewElementBesideLine = errors.New("tool declares a per-entry preview_sentence beside another line in the same half")
	// errPreviewMatchWithChoice and errPreviewMatchWithTemplate each name two
	// ways of saying what one line reports, where nothing says which would win.
	errPreviewMatchWithChoice   = errors.New("tool declares a preview_sentence carrying both matched_by and chosen_by")
	errPreviewMatchWithTemplate = errors.New("tool declares a preview_sentence carrying both matched_by and template")
	// errPreviewMatchArgument holds the selector to a value the tool carries and
	// may report, the same bar a placeholder answers to.
	errPreviewMatchArgument = errors.New("tool declares a preview_sentence matched_by a value the message does not declare")
	// errPreviewMatchNoArm names a match with nothing to select between, which
	// is a fixed wording spelled the long way.
	errPreviewMatchNoArm = errors.New("tool declares a preview_sentence matched_by nothing")
	// errPreviewMatchArmEmpty names an arm missing its value or its wording,
	// either of which leaves the arm unable to answer.
	errPreviewMatchArmEmpty = errors.New("tool declares a preview_sentence arm with no value or no wording")
	// errPreviewMatchArmRepeated names one value twice, where declaration order
	// rather than the contract would decide the answer.
	errPreviewMatchArmRepeated = errors.New("tool declares a preview_sentence naming one matched value twice")
	// errPreviewChangedUnknownArgument holds the pair's argument to a value the
	// tool carries and may report.
	errPreviewChangedUnknownArgument = errors.New("tool declares when_changed against an argument the message does not declare")
	// errPreviewChangedWithUnchanged reads one reading two ways: stepping a
	// wording aside, and dropping the line it sits on.
	errPreviewChangedWithUnchanged = errors.New("tool declares both when_changed and preview_unchanged over one state reading")
	// errPreviewUnchangedUnread names a pair no wording reads, which is a rule
	// that would never change an answer.
	errPreviewUnchangedUnread = errors.New("tool declares preview_unchanged for a state reading no wording names")
	// errPreviewUnchangedUnknownArgument holds the pair's argument to a value the
	// tool carries and may report, the same bar a placeholder answers to.
	errPreviewUnchangedUnknownArgument = errors.New("tool declares preview_unchanged against an argument the message does not declare")
	// errPreviewSentenceNoState names a wording reading the resource on a tool
	// that fetches none, which would report a gap on every call rather than the
	// missing declaration it is.
	errPreviewSentenceNoState = errors.New("tool declares a preview_sentence reading state with no declared state read")
	// errPreviewStateUnknownMember names a member the declared read's message
	// does not carry, or a repeated one a single placeholder cannot report.
	errPreviewStateUnknownMember = errors.New("tool declares a preview_sentence reading a state member the read does not answer with")
	// errPreviewStateDeepMember names a path past the one member level the
	// contract models, where every further level is a guess about a shape.
	errPreviewStateDeepMember = errors.New("tool declares a preview_sentence reading a state member more than one level deep")
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

	// errPreviewMatchSelector names a matched line selecting on both an
	// argument and a state reading, or on neither: nothing here would say which
	// value picks the wording.
	errPreviewMatchSelector = errors.New("tool declares a preview_sentence matched_by neither an argument nor a state, or by both")

	// errPreviewOmitsBodyWithoutBody names the flag on a tool whose preview
	// reports no body to begin with, which would read as a rule that never runs.
	errPreviewOmitsBodyWithoutBody = errors.New("tool declares preview_omits_body and sends no body")

	// errPreviewStandInNotItemList names an argument whose entries carry no
	// declared member: a scalar, a free-form list, or nothing the tool declares.
	errPreviewStandInNotItemList = errors.New("tool declares preview_stand_in over an argument that is not a repeated BODY message the tool declares")
	// errPreviewStandInUnknownMember names a member no entry carries. Nothing
	// would be stood in for, so the value the declaration is about would reach
	// the report intact.
	errPreviewStandInUnknownMember = errors.New("tool declares preview_stand_in over a member the entries do not declare")
	// errPreviewStandInNoText names a stand-in with nothing to report, which an
	// absent declaration already says.
	errPreviewStandInNoText = errors.New("tool declares preview_stand_in with no text")
	// errPreviewStandInRepeated names one member twice: two texts stand in for
	// one value and nothing here says which is reported.
	errPreviewStandInRepeated = errors.New("tool declares preview_stand_in twice over one member")
	// errPreviewStandInTier names a stand-in on a tier whose driver does not
	// carry one, where the reported body would keep the value in one language
	// and stand in for it in the other.
	errPreviewStandInTier = errors.New("tool declares preview_stand_in on a tier that reports no stood-in body")

	// errPartialTwoStage names half the plan/apply flow: a caller handed one of
	// the two arguments can reach neither stage.
	errPartialTwoStage = errors.New("tool advertises one of mode and plan_id without the other")

	// errNoTwoStageDriver names the plan/apply arguments on a tier with no flow
	// behind them, which is the shape the hole took before this tier had one:
	// the schema advertises the stages and the handler drops the arguments.
	errNoTwoStageDriver = errors.New("tool advertises mode and plan_id on a tier that emits no plan/apply flow")

	// errUnstagedWalk names a dependency walk on a tool that advertises no
	// plan, so the closure would be written and never reached.
	errUnstagedWalk = errors.New("tool declares a plan walk without advertising mode and plan_id")

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

	// errNoMetaAnswer names a meta tool that answers with none of the three
	// sources, which leaves the emitted handler with nothing to return.
	errNoMetaAnswer = errors.New("meta tool declares neither local_answer nor success_message")

	// errNoMetaMessageField names a meta tool whose declared sentence has
	// nowhere to be reported.
	errNoMetaMessageField = errors.New("meta tool declares a success_message its response has no message field for")

	// errNullsOverDecodedResponse would look the named keys up a level above the
	// resource they belong to: the raw body of a decoded envelope is the
	// envelope, not the resource inside it.
	errNullsOverDecodedResponse = errors.New("tool declares explicit_null_fields beside response_body_fields")

	// errNoScopes names a routed tool with no scope answer at all, which is
	// how a family used to go silently unrestricted in every language at once.
	errNoScopes = errors.New("tool declares neither tool_scopes.none nor a scope")

	// errScopesOnMeta names a scope declaration on a tool that reaches no
	// route, which has no documented security block to restate.
	errScopesOnMeta = errors.New("meta tool declares tool_scopes")

	// errScopesNoneBesideList names a declaration claiming both answers, and
	// nothing says which one the registries would render.
	errScopesNoneBesideList = errors.New("tool_scopes declares none beside a scope list")

	// errScopeRepeated names a scope declared twice, which would read as two
	// requirements where the API names one.
	errScopeRepeated = errors.New("tool_scopes declares one scope more than once")

	// errScopeUnrenderable names a member the emitter has no wire spelling
	// for, which would otherwise reach the registries as an empty string.
	errScopeUnrenderable = errors.New("tool_scopes declares a member with no wire spelling")

	// errNoCategories names a tool with no category answer at all. Required
	// on meta tools too: two core tools are meta, and a mutator declaring
	// nothing would be served by no category-scoped profile in silence.
	errNoCategories = errors.New("tool declares neither tool_categories.none nor a category")

	// errCategoriesNoneBesideList names a declaration claiming both answers,
	// and nothing says which one the registries would render.
	errCategoriesNoneBesideList = errors.New("tool_categories declares none beside a category list")

	// errCategoryRepeated names a category declared twice, which would file
	// the tool under one slice twice.
	errCategoryRepeated = errors.New("tool_categories declares one category more than once")

	// errCategoryUnrenderable names a member the emitter has no spelling for,
	// which would otherwise reach the registries as an empty string.
	errCategoryUnrenderable = errors.New("tool_categories declares a member with no spelling")

	// errLocalAnswerTier names a local answer on a tool that reaches a route.
	// The operation takes no client and builds the whole result, so a tool
	// that calls Linode has no point to hand over at.
	errLocalAnswerTier = errors.New("tool declares local_answer on a tier that reaches a route")

	// errLocalAnswerTwoAnswers names a tool declaring a local answer beside a
	// sentence, where the emitted handler would return one and drop the other.
	errLocalAnswerTwoAnswers = errors.New("tool declares local_answer beside a success_message")

	// errNoLocalCall names a local answer naming no operation, which would
	// emit a handler calling nothing.
	errNoLocalCall = errors.New("local_answer names no call")

	// errUnservedLocalCall names a declared member the emitter has no
	// operation for, which would otherwise render a call to nothing.
	errUnservedLocalCall = errors.New("local_answer names a call the engine does not serve")

	// errLocalStateMismatch names a declaration whose requires does not name
	// the state its operation reads, which would emit a handler resolving
	// state the operation never gets or skipping the one it needs.
	errLocalStateMismatch = errors.New("local_answer requires state the declared call does not read")

	// errLocalStateUnused names ambient state declared on an operation that
	// reads none, whose absence the tool would then answer for in vain.
	errLocalStateUnused = errors.New("local_answer requires state on a call that reads none")

	// errLocalDirectionUnserved names a direction on a one-sided operation or
	// an absent one on a two-sided operation, where unset reads as a direction
	// rather than as a missing answer.
	errLocalDirectionUnserved = errors.New("local_answer declares a direction its call cannot run")

	// errLocalBindNoInput names a binding filling nothing.
	errLocalBindNoInput = errors.New("local_answer binding names no input")

	// errLocalBindUnknownInput names a binding filling an input the declared
	// operation does not take.
	errLocalBindUnknownInput = errors.New("local_answer binding names an input its call does not take")

	// errLocalBindUnknownArgument names a binding reading an argument this
	// message does not declare, which would read as absent on every call.
	errLocalBindUnknownArgument = errors.New("local_answer binding names an argument the message does not declare")

	// errLocalBindRepeated names one input filled twice, where nothing says
	// which argument reaches the operation.
	errLocalBindRepeated = errors.New("local_answer binds one input more than once")

	// errLocalBindMissing names an input the operation requires that no
	// binding fills, which would reach it as a zero the caller never sent.
	errLocalBindMissing = errors.New("local_answer leaves a required input of its call unbound")

	// errLocalBindKind names an argument whose kind the input's reader cannot
	// build, which is the typed-parameter guard's own refusal: every value an
	// operation gets is read through the reader its input's type names, so an
	// argument that reader cannot read has no way in.
	errLocalBindKind = errors.New("local_answer binds an argument whose kind its input cannot take")

	// errLocalRefuseNoGuard names a ladder row answering no condition.
	errLocalRefuseNoGuard = errors.New("local_answer refusal names no guard")

	// errLocalRefuseSilent names a guard with no sentence, which would refuse
	// a caller without saying why.
	errLocalRefuseSilent = errors.New("local_answer refusal declares no message")

	// errLocalRefuseGuardArm names a condition the declared operation cannot
	// report, whose sentence nothing would ever answer.
	errLocalRefuseGuardArm = errors.New("local_answer refusal names a guard its call cannot report")

	// errLocalRefuseRepeated names one condition worded twice, where the
	// second sentence is unreachable behind the first.
	errLocalRefuseRepeated = errors.New("local_answer refuses one guard more than once")

	// errLocalRefuseUnworded names a condition the operation reports that the
	// ladder leaves unworded, which would refuse in silence at call time.
	errLocalRefuseUnworded = errors.New("local_answer leaves a guard its call reports unworded")

	// errLocalRefuseArgument names a guard reading an argument no binding
	// fills, or one of the store failures naming an argument it never reads.
	errLocalRefuseArgument = errors.New("local_answer refusal names the wrong argument for its guard")

	// errLocalRefusePlaceholder names a sentence naming a value the handler
	// cannot fill: only a bound argument and the reported cause are in scope,
	// which is what keeps a tool's own identity out of the operation.
	errLocalRefusePlaceholder = errors.New("local_answer refusal names a placeholder nothing fills")

	// errLocalArmUnnamed names a registered language the emitter has no
	// spelling rule for, which would render every call to an empty name. The
	// rule is what a tenth language is registered with, the way a renderer arm
	// is.
	errLocalArmUnnamed = errors.New("local operation names no function for a registered language")

	// errLocalArmUnstemmed names a declared member no function derives from.
	// The names are derived rather than stated, so a member the vocabulary's
	// own prefix does not open leaves every language with nothing to spell.
	errLocalArmUnstemmed = errors.New("local operation member derives no function name")

	// errLocalArmSpellingShared names two operations one language would spell
	// alike. A language running an operation's words together collides where an
	// underscore-keeping one does not, so the second engine to be written is
	// where a shared function surfaces.
	errLocalArmSpellingShared = errors.New("local operations share one function in a registered language")

	// errLocalArmInputKind names an operation input whose type is outside the
	// closed reader set. It is the other half of the typed-parameter guard:
	// the set holds no member that carries the request or the argument map, so
	// an operation cannot be given one.
	errLocalArmInputKind = errors.New("local operation declares an input outside the reader set")

	// errLocalArmInputRepeated names one input declared twice, which no
	// binding could fill unambiguously.
	errLocalArmInputRepeated = errors.New("local operation declares one input more than once")

	// errLocalArmInputUnnamed names an input a declaration left unnamed. A
	// binding fills an input by name, so a nameless one is a parameter nothing
	// could ever reach and every declaring tool would fail on instead.
	errLocalArmInputUnnamed = errors.New("local operation declares an input with no name")

	// errLocalArmAmbient names a reading outside the vocabulary, or one
	// declared twice. Each arm asks the declaration whether it reads a named
	// member, so either one is a declaration the emitter would drop in silence.
	errLocalArmAmbient = errors.New("local operation declares an ambient reading nothing renders")

	// errLocalArmToolNamed names an operation whose function is named after a
	// tool. An operation named for the tool it serves is per-tool code under
	// another heading, and the whole point of the vocabulary is that two tools
	// can share one operation.
	errLocalArmToolNamed = errors.New("local operation names a function after a tool")

	// errLocalArmUnshaped names an operation that shapes no answer for a
	// direction it runs in, shapes one for a direction it does not run in, or
	// names a shape no message declares. Held over the table rather than per
	// tool so the gap reads as the table's rather than as one tool's.
	errLocalArmUnshaped = errors.New("local operation shapes no answer for a direction it runs in")

	// errLocalShapeMemberKind names an answer member whose type the projection
	// carries no spelling for. It is the answer half of the typed-parameter
	// guard: a member outside the set would reach one language as a value the
	// other could not be held to, which is the drift the projection removes.
	errLocalShapeMemberKind = errors.New("local answer shape carries a member the projection cannot build")

	// errLocalRecordNested names a record shape carrying another declared shape.
	// A record's member order is the message's own, so a nested one would need
	// its order held too, in a serializer that recursed and a reader that knew
	// where to stop. Nothing declares one, so the run refuses instead.
	errLocalRecordNested = errors.New("local answer record carries another record")

	// errLocalRecordUnreached names a message declaring local_record that no
	// operation answers with. The option asks each language for a serializer and
	// a reader; outside the answer surface it gets neither, and the declaration
	// would read as met while the engine still spelled the record by hand.
	errLocalRecordUnreached = errors.New("local record is declared on a message no operation answers with")

	// errLocalShapeShared names two messages one language would spell alike, so
	// the emitted type would depend on which declaration was read first.
	errLocalShapeShared = errors.New("local answer shapes collide")

	// errLocalShapeCycle names shapes that reach each other, which no language
	// emitting value types in dependency order can write.
	errLocalShapeCycle = errors.New("local answer shapes reach each other")

	// errLocalAnswerShape names a tool declaring a response its operation
	// cannot build. The operation answers a plain body naming no message, so
	// nothing else would notice: a member the operation never fills would
	// answer a zero nothing computed, and one it does fill that the response
	// leaves out has nowhere to go.
	errLocalAnswerShape = errors.New("local_answer declares a response its call cannot fill")

	// errLocalArmSidedStateless names a two-sided operation holding no state. A
	// stateless side is served by the function's own type, and one name cannot
	// carry a signature per direction.
	errLocalArmSidedStateless = errors.New("local operation runs in two directions and holds no state to serve them")

	// errLocalArmReaderGuard names an operation reporting a condition its
	// reader answers instead. A reader refuses before the operation is reached,
	// so the generated arm would map a condition the subsystem cannot raise.
	errLocalArmReaderGuard = errors.New("local operation reports a condition its reader answers")

	// errLocalArmInputUntyped names a generated operation whose input the
	// answers tree carries no type for. The call list is the reader's own
	// record and lives with the readers, so a subsystem type naming it could
	// not be written where the engine implements it.
	errLocalArmInputUntyped = errors.New("local operation takes an input the answer tree carries no type for")

	// errLocalArmAmbientUnrendered names a generated operation reading
	// something a registered language states no rendering for. That language's
	// arm would hand its subsystem less than the declaration says while reading
	// as though it had handed over everything.
	errLocalArmAmbientUnrendered = errors.New("local operation reads ambient state a language renders nothing for")

	// errLocalArmAmbientUnhanded names a reading a language types but states no
	// expression for. Its subsystem would declare the parameter and its handler
	// would fill it from a name nothing defines, which Go answers as a build
	// failure in emitted code and Python only on the first call that arrives.
	errLocalArmAmbientUnhanded = errors.New("local operation reads ambient state a language states no expression for")

	// errLocalVerdictUnnamed names an operation carrying a wording row that
	// names no verdict. The row words something, and nothing says what, so
	// every language would emit a case no value reaches.
	errLocalVerdictUnnamed = errors.New("local operation words a verdict row naming no verdict")

	// errLocalVerdictRepeated names an operation wording one verdict twice.
	// Two rows for one member are two answers to one question, and which one
	// an engine emits would come down to the order the rows are read in.
	errLocalVerdictRepeated = errors.New("local operation words one verdict twice")

	// errLocalVerdictIncomplete names an operation wording some of the verdict
	// vocabulary and not the rest. The emitted type carries every member, so a
	// member left out is a verdict a subsystem can hand back and no language
	// can word.
	errLocalVerdictIncomplete = errors.New("local operation words part of the verdict vocabulary")

	// errLocalVerdictUnworded names a verdict that stops an entry and leaves
	// one of its three words off. A bucket with no sentence counts an entry the
	// caller is told nothing about, and a sentence with no bucket is a stopped
	// entry the summary never counts.
	errLocalVerdictUnworded = errors.New("local operation stops an entry without saying why, how, or where it counts")

	// errLocalVerdictWorded names the permitting verdict carrying words. It
	// stops nothing, so a bucket would count a permitted entry and a sentence
	// would explain a refusal that did not happen.
	errLocalVerdictWorded = errors.New("local operation words the verdict that permits an entry")

	// errLocalVerdictPlaceholder names a verdict sentence reading a value no
	// entry carries. The vocabulary is two members wide and neither reaches the
	// request, the argument map or a tool's name, which is what keeps a tool's
	// identity out of prose the operation's own answer carries.
	errLocalVerdictPlaceholder = errors.New("verdict sentence names a value outside the entry vocabulary")
)
