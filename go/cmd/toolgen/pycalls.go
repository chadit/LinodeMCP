package main

import (
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// pyDriverCall is the driver the tier runs through, named with what it cannot
// derive.
func pyDriverCall(tool *pyTool) ([]string, error) {
	if tool.assembledRead() {
		return pyAssembledReadCall(tool)
	}

	driver := pyListDriver
	if tool.c.Tier == tierGet {
		driver = pyGetDriver
	}

	lines := append(pyPreviewStateRead(tool), pyPreviewValues(tool)...)
	lines = append(lines,
		"    return await "+driver+"(",
		pyDriverConfigLine,
		pyDriverArgumentsLine,
		"        tool="+pyQuote(tool.c.Name)+",",
	)

	if len(tool.c.Slots) > 0 {
		// Already read in its declared type above, so no second coercion here.
		values := make([]string, 0, len(tool.c.Slots))
		for _, slot := range tool.c.Slots {
			values = append(values, pyQuote(slot)+": "+pyLocal(slot))
		}

		lines = append(lines, "        path_values={"+strings.Join(values, ", ")+"},")
	}

	if tool.c.Tier == tierGet {
		extras, err := pyGetExtras(tool)
		if err != nil {
			return nil, err
		}

		lines = append(lines, extras...)
	}

	if len(tool.c.Filters) > 0 {
		lines = append(lines, "        filters=(")
		for _, entry := range tool.c.Filters {
			lines = append(lines, "            "+pyFilterLiteral(entry)+",")
		}

		lines = append(lines, "        ),")
	}

	if len(tool.c.Forwarded) > 0 {
		named := make([]string, 0, len(tool.c.Forwarded))
		for i := range tool.c.Forwarded {
			named = append(named, pyQuote(tool.c.Forwarded[i].ProtoName))
		}

		lines = append(lines, "        forwarded=("+strings.Join(named, ", ")+",),")
	}

	if tool.c.Tier == tierMarkerList {
		echoed := make([]string, 0, len(tool.c.Echoed))
		for i := range tool.c.Echoed {
			echoed = append(echoed, pyQuote(tool.c.Echoed[i].ProtoName))
		}

		lines = append(lines, "        echoes=("+strings.Join(echoed, ", ")+",),")
		if len(echoed) == 0 {
			lines[len(lines)-1] = "        echoes=(),"
		}

		// The page controls take no part: this route pages by the marker its
		// previous answer handed out, and its bound travels forwarded as
		// declared, which is why the shape may publish it as text.
		lines = append(lines, "        pagination=None,")
	}

	return append(lines, "    )"), nil
}

// pyGetExtras is what a single-resource read passes beyond its path values. A
// read carries a query only where its route publishes one, and answers under an
// envelope member only where the tool has always done so. A gated read adds the
// verdict the driver times and, where one is declared, the hand-written preview
// that answers its dry run.
func pyGetExtras(tool *pyTool) ([]string, error) {
	lines := make([]string, 0, 6)

	if len(tool.c.Query) > 0 {
		lines = append(lines, "        query=query,")
	}

	wrapper, err := tool.wrapperMember()
	if err != nil {
		return nil, err
	}

	if wrapper != "" {
		lines = append(lines, "        wrapper_field="+pyQuote(wrapper)+",")
	}

	if tool.c.StructResponse {
		lines = append(lines, "        struct_response=True,")
	}

	if !tool.gatedRead() {
		return lines, nil
	}

	lines = append(lines,
		"        gated=True,",
		"        error=failure,",
		"        preview_error=failure,",
	)

	lines = append(lines, pyDeclaredPreview(tool)...)

	return lines, nil
}

// pyAssembledReadCall is the assembled-read driver call: the transport that
// fetches, and what it fills. The route answers with something no decode can
// place, so the declared transport owns the transfer and the driver places its
// one member beside the ids the call was addressed by.
func pyAssembledReadCall(tool *pyTool) ([]string, error) {
	members := make([]string, 0, len(tool.c.Assembled))
	for _, name := range tool.assembledNames() {
		members = append(members, pyQuote(name))
	}

	lines := []string{
		"    return await run_assembled_read_tool(",
		pyDriverConfigLine,
		pyDriverArgumentsLine,
		"        tool=" + pyQuote(tool.c.Name) + ",",
		pyTransportLine(tool),
		"        assembled=(" + strings.Join(members, ", ") + ",),",
	}

	if len(tool.echoes) > 0 {
		echo := pyAssembledEchoArguments(tool)

		lines = append(lines, "        echo_members="+echo+",")
	}

	if len(tool.c.Slots) > 0 {
		values := make([]string, 0, len(tool.c.Slots))
		for _, slot := range tool.c.Slots {
			values = append(values, pyQuote(slot)+": "+pyLocal(slot))
		}

		lines = append(lines, "        path_values={"+strings.Join(values, ", ")+"},")
	}

	return append(lines, "    )"), nil
}

// pyWriteCall is the write driver call, named with what the contract cannot
// answer.
func pyWriteCall(tool *pyTool) ([]string, error) {
	lines := append(pyPreviewStateRead(tool), pyPreviewValues(tool)...)
	lines = append(lines,
		"    return await run_write_tool(",
		pyDriverConfigLine,
		pyDriverArgumentsLine,
		"        tool="+pyQuote(tool.c.Name)+",",
		// Left empty on purpose: the driver then reports the failure sentence
		// the contract declares, which is the only one that can name the ids the
		// call was addressed by.
		`        error_action="",`,
		"        body=body,",
	)

	if len(tool.echoes) > 0 {
		echo := pyWriteEchoArguments(tool)

		lines = append(lines, "        echo_args="+echo+",")
	}

	if len(tool.c.Slots) > 0 {
		lines = append(lines, "        path_values={"+pyPathValuesLiteral(tool)+"},")
	}

	lines = append(lines, "        error=failure,", "        preview_error=failure,")

	if redacted := pyRedactedMembers(tool); redacted != "" {
		lines = append(lines, "        redact_preview=("+redacted+",),")
	}

	lines = append(lines, pyDeclaredPreview(tool)...)

	if len(tool.c.Query) > 0 {
		lines = append(lines, "        query=query,")
	}

	// A paged answer is decoded through the list envelope, which reports a body
	// that is not a page in its own words, so there is no subject to name.
	if tool.c.PagedWrite {
		return append(lines, "        paged=True,", "    )"), nil
	}

	return append(lines,
		"        invalid_response_subject="+pyQuote(tool.responseSubject())+",",
		"    )",
	), nil
}

// pyBodyReadCall is the body-read driver call: the tool, the body, and the ids
// it addresses. No error_action, so the driver reports the sentence the
// contract declares, which is the only one that can name the bucket and object
// the call carried.
func pyBodyReadCall(tool *pyTool) ([]string, error) {
	lines := []string{
		"    return await run_body_read_tool(",
		pyDriverConfigLine,
		pyDriverArgumentsLine,
		"        tool=" + pyQuote(tool.c.Name) + ",",
		"        body=body,",
	}

	if len(tool.echoes) > 0 {
		echo := pyWriteEchoArguments(tool)

		lines = append(lines, "        echo_args="+echo+",")
	}

	if len(tool.c.Slots) > 0 {
		lines = append(lines, "        path_values={"+pyPathValuesLiteral(tool)+"},")
	}

	// The transport stands in for the routed call, the same way it does on the
	// acknowledge tier; the driver keeps everything around it.
	if tool.c.transported() {
		lines = append(lines, pyTransportLine(tool))

		if len(tool.c.Assembled) > 0 {
			lines = append(lines, "        assembled=("+pyQuotedJoin(tool.assembledNames())+",),")
		}
	}

	return append(lines, "    )"), nil
}

// pyAcknowledgeCall is the acknowledge driver call, named with what the
// contract cannot answer.
func pyAcknowledgeCall(tool *pyTool) ([]string, error) {
	lines := append(pyPreviewStateRead(tool), pyPreviewValues(tool)...)
	lines = append(lines,
		"    return await run_acknowledge_tool(",
		pyDriverConfigLine,
		pyDriverArgumentsLine,
		"        tool="+pyQuote(tool.c.Name)+",",
		"        echo_args="+pyEchoArguments(tool)+",",
	)

	if tool.buildsBody() {
		lines = append(lines, "        body=body,")
	}

	if len(tool.c.Slots) > 0 {
		lines = append(lines, "        path_values={"+pyPathValuesLiteral(tool)+"},")
	}

	lines = append(lines, "        error=failure,", "        preview_error=failure,")

	// A declared redaction is honored here for the reason the write tier honors
	// it: the tier a tool lands on is decided by the shape of its answer, and a
	// secret in the request does not stop being one because the API answers with
	// nothing.
	if redacted := pyRedactedMembers(tool); redacted != "" {
		lines = append(lines, "        redact_preview=("+redacted+",),")
	}

	lines = append(lines, pyStandInMembers(tool)...)
	lines = append(lines, pyDeclaredPreview(tool)...)

	// The transport stands in for the live call alone: everything the driver
	// does around it, the gate included, is what the tier already emits.
	if tool.c.transported() {
		lines = append(lines, pyTransportLine(tool))

		if len(tool.c.Assembled) > 0 {
			lines = append(lines, "        assembled=("+pyQuotedJoin(tool.assembledNames())+",),")
		}
	}

	if tool.c.Mode {
		lines = append(lines, "        fetch_state=fetch_state,")

		if tool.walksDependencies() {
			lines = append(lines, "        dependency_walk=dependency_walk,")
		}

		capability, capErr := tool.capabilityMember()
		if capErr != nil {
			return nil, capErr
		}

		// Named rather than defaulted: the flow's opt-in gate reads it, and a
		// Write staying opt-in by config is the whole difference from a delete.
		lines = append(lines, "        capability=Capability."+capability+",")
	}

	return append(lines, "    )"), nil
}

// pyDestroyArguments reads the ids, notes a missing one, and hands the driver
// its closures.
//
// The missing-id sentence is noted rather than answered: when it surfaces is
// the driver's job and differs by branch, ahead of a plan or a preview and
// behind the confirm gate on a live call.
//
// An integer id is read through destroy_id, which carries its own sentence
// because this tier refuses more than an absent value: a fractional or
// string-typed id is refused rather than truncated onto the path.
func pyDestroyArguments(tool *pyTool) ([]string, error) {
	lines := make([]string, 0, len(tool.c.Slots)*3+12)
	for _, slot := range tool.c.Slots {
		lines = append(lines, pyDestroyPathRead(tool, slot))
	}

	lines = append(lines, "    error = "+pyConstraintCall(tool)+" or None")
	lines = append(lines, pyDestroySlotChecks(tool)...)

	// A removal that is a rebuild rather than a delete sends what it was given,
	// and an unusable member is noted beside a missing id so both surface under
	// the one order this tier's branches follow.
	if tool.buildsBody() {
		lines = append(lines,
			"",
			"    body, body_message = "+pyBodyName(tool.c.Name)+"(arguments)",
			"    error = error or body_message or None",
		)
	}

	ids := pyDestroyIDs(tool)

	fetch := pyStateFetch(tool, ids)

	lines = append(lines,
		"",
		"    async def fetch_state(client: RetryableClient) -> Any:",
		"        return "+fetch,
		"",
	)

	lines = append(lines, pyDependencyWalk(tool)...)

	return lines, nil
}

// pyDependencyWalk is the walk the removal declares, reading the state the
// fetch it is paired with produces.
//
// A declared fetch reports the projected state, so the walk is handed it as the
// type that carries it: the annotation stops type-checking if the pair ever
// falls out of step, where the shape check it replaces went on reporting an
// empty walk.
func pyDependencyWalk(tool *pyTool) []string {
	if len(tool.c.DepWalks) > 0 || len(tool.c.PreviewSentences) > 0 || tool.c.BillingDecl != nil {
		return pyDeclaredWalks(tool)
	}

	return nil
}

// pyStateReadIDs is the removal's path arguments in the order the state read
// fills its own template. A derived sibling shares the removal's template, so
// the ids reach it as they come; a declared route names its own slots.
func pyStateReadIDs(tool *pyTool, ids string) string {
	if len(tool.c.StateRead.Slots) == 0 {
		return ids
	}

	values := make([]string, 0, len(tool.c.StateRead.Slots))
	for _, slot := range tool.c.StateRead.Slots {
		values = append(values, pyDestroyID(tool, slot))
	}

	return strings.Join(values, ", ")
}

// pyStateMember names the raw-body key the resource sits under, for the reads
// that wrap it. The nulls the same read restores are looked up off the
// descriptor by the driver, the way this arm resolves every other declaration.
func pyStateMember(tool *pyTool) string {
	if key := tool.c.StateRead.BodyKey; key == "" {
		return ""
	}

	return ", member=" + pyQuote(tool.c.StateRead.BodyKey)
}

// pyStatePayload names the response member the resource itself is, for a read
// that answers a wrapper around it.
func pyStatePayload(tool *pyTool) string {
	if tool.c.StateRead.Payload == "" {
		return ""
	}

	return ", payload=" + pyQuote(tool.c.StateRead.Payload)
}

// pyDestroySlotChecks is the id chain a removal refuses on. A lone slot folds
// into the error guard, since a single nested if under it is the shape the
// linter refuses; several keep the guarded if/elif chain.
func pyDestroySlotChecks(tool *pyTool) []string {
	if len(tool.c.Slots) == 1 {
		return pyDestroySoleIDCheck(tool, tool.c.Slots[0])
	}

	lines := []string{"    if not error:"}
	branch := pyBranchIf

	for _, slot := range tool.c.Slots {
		lines = append(lines, pyDestroyIDBranch(tool, slot, branch)...)

		branch = pyBranchElif
	}

	return lines
}

// pyDestroySoleIDCheck is the one-slot chain with the error guard folded in.
func pyDestroySoleIDCheck(tool *pyTool, slot string) []string {
	local := pyLocal(slot)

	if tool.pathArg(slot).numeric {
		return []string{
			"    if not error and " + local + "_error:",
			"        error = " + local + "_error",
		}
	}

	return []string{
		"    if not error and not " + local + ":",
		"        error = " + pyQuote(slot+" is required"),
	}
}

// pyDestroyIDBranch is one arm of the chain a removal notes its first unusable
// id in. An integer id carries the sentence its reader already answered; a
// label reads as blank when absent, which is the only thing there is to say.
func pyDestroyIDBranch(tool *pyTool, slot, branch string) []string {
	local := pyLocal(slot)

	if tool.pathArg(slot).numeric {
		return []string{
			"        " + branch + " " + local + "_error:",
			"            error = " + local + "_error",
		}
	}

	return []string{
		"        " + branch + " not " + local + ":",
		"            error = " + pyQuote(slot+" is required"),
	}
}

// pyDestroyIDs is the value each path id reaches the route, the echo, and the
// closures under. All three read the same expression, so a delete cannot
// address one resource and report another.
func pyDestroyIDs(tool *pyTool) string {
	values := make([]string, 0, len(tool.c.Slots))
	for _, slot := range tool.c.Slots {
		values = append(values, pyDestroyID(tool, slot))
	}

	return strings.Join(values, ", ")
}

// pyDestroyID is one path id as the expression every branch reads it through.
func pyDestroyID(tool *pyTool, slot string) string {
	argument := tool.pathArg(slot)

	return argument.coerce + "(" + pyLocal(slot) + " or " + argument.absent + ")"
}

// pyDestroyPathRead reads one of a removal's path slots, in the reader that
// slot's type needs.
func pyDestroyPathRead(tool *pyTool, slot string) string {
	local := pyLocal(slot)
	if !tool.pathArg(slot).numeric {
		return "    " + local + " = " + pyPathRead(tool, slot)
	}

	return "    " + local + ", " + local + "_error = destroy_id(arguments, " + pyQuote(slot) + ")"
}

// pyStateFetch is the synthesized state fetch: the sibling read, or a selection
// out of the collection the resource belongs to. A collection sibling is
// addressed by the removal's ids with the trailing one dropped, because that id
// names the element rather than the collection, and the same id is what selects
// the element out of the page.
func pyStateFetch(tool *pyTool, ids string) string {
	if tool.c.StateRead.scan() {
		return pyScanFetch(tool)
	}

	if tool.c.StateRead.Envelope {
		return "await read_envelope_state(client" + pyStateFormValues(tool) +
			", tool=" + pyQuote(tool.c.StateRead.Tool) + ")"
	}

	// The query rides along for the reason the Go arm gives: the object ACL
	// names the bucket in its path and the object in its query, so a fetch that
	// dropped it would preview the bucket while the removal took one object out
	// of it.
	if !tool.c.StateRead.collection() {
		return "await read_route_state(client, " + pyStateReadIDs(tool, ids) +
			", tool=" + pyQuote(tool.c.StateRead.Tool) + pyStateMember(tool) +
			pyStatePayload(tool) + pyStateQuery(tool) + ")"
	}

	parents := make([]string, 0, len(tool.c.Slots))
	for _, slot := range tool.c.Slots[:len(tool.c.Slots)-1] {
		parents = append(parents, pyDestroyID(tool, slot))
	}

	trailing := pyDestroyID(tool, tool.c.Slots[len(tool.c.Slots)-1])

	return "await read_collection_state(client, " + strings.Join(parents, ", ") +
		", tool=" + pyQuote(tool.c.StateRead.Tool) +
		", member=" + pyQuote(elementIDField) + ", value=" + trailing + ")"
}

// pyScanFetch is the whole-collection scan a declared match resolves to, with
// the pairs handed over in declaration order for the not-found sentence.
func pyScanFetch(tool *pyTool) string {
	pairs := make([]string, 0, len(tool.c.StateRead.Matches))
	for _, match := range tool.c.StateRead.Matches {
		pairs = append(pairs, pyQuote(match.Field)+": "+pyLocal(match.Argument))
	}

	return "await read_collection_scan(client" + pyStateFormValues(tool) +
		", tool=" + pyQuote(tool.c.StateRead.Tool) +
		", matches={" + strings.Join(pairs, ", ") + "})"
}

// pyStateFormValues is the positional values that fill a scan or envelope
// read's own slots, empty when the list takes none.
func pyStateFormValues(tool *pyTool) string {
	// A slot renders as ", " plus a short local name, comfortably under this.
	const slotRenderBytes = 16

	var values strings.Builder

	values.Grow(len(tool.c.StateRead.Slots) * slotRenderBytes)

	for _, slot := range tool.c.StateRead.Slots {
		values.WriteString(", ")
		values.WriteString(pyDestroyID(tool, slot))
	}

	return values.String()
}

// pyCompositeFetch is the declared multi-call state read: each call in
// declaration order, keeping the declared field subset.
func pyCompositeFetch(tool *pyTool) []string {
	lines := []string{"        return await read_composite_state(client, ["}

	for index := range tool.c.Composite {
		call := &tool.c.Composite[index]

		values := make([]string, 0, len(call.Slots))
		for _, slot := range call.Slots {
			values = append(values, pyLocal(slot))
		}

		fields := make([]string, 0, len(call.Fields))
		for _, name := range call.Fields {
			fields = append(fields, pyQuote(name))
		}

		lines = append(lines,
			"            CompositeCall(",
			"                tool="+pyQuote(call.Tool)+",",
			"                member="+pyQuote(call.Member)+",",
			"                fields=("+strings.Join(fields, ", ")+",),",
			"                values=("+strings.Join(values, ", ")+",),",
			"                is_list="+pyLiteralBool(call.List)+",",
			"            ),",
		)
	}

	return append(lines, "        ])")
}

// pyLiteralBool is one Go bool spelled the way Python reads it.
func pyLiteralBool(value bool) string {
	if value {
		return pyTrue
	}

	return pyFalse
}

// pyDestroyCall is the destroy driver call, named with what the contract cannot
// answer. No execute: the driver removes the resource through the routed
// primitive, since a delete sends no body and reads nothing back. A declared
// transport replaces that primitive, which is how the removal arm sends its
// DELETE to a URL the declared route just minted.
func pyDestroyCall(tool *pyTool) ([]string, error) {
	values := make([]string, 0, len(tool.c.Slots))
	for _, slot := range tool.c.Slots {
		values = append(values, pyQuote(slot)+": "+pyDestroyID(tool, slot))
	}

	lines := []string{
		"    return await run_destructive_tool(",
		pyDriverConfigLine,
		pyDriverArgumentsLine,
		"        tool=" + pyQuote(tool.c.Name) + ",",
		// Left empty on purpose: the driver then reports the sentence the tool
		// declares, or the tier's shared one where it declares none.
		`        error_action="",`,
		"        id_args={" + strings.Join(values, ", ") + "},",
		"        fetch_state=fetch_state,",
	}

	if tool.buildsBody() {
		lines = append(lines, "        body=body,")

		// A declared redaction is honored for the reason the write tier honors
		// it: a secret in the request does not stop being one because the tool
		// that carries it removes a resource instead of creating one.
		if redacted := pyRedactedMembers(tool); redacted != "" {
			lines = append(lines, "        redact_preview=("+redacted+",),")
		}
	}

	if tool.walksDependencies() {
		lines = append(lines, "        dependency_walk=dependency_walk,")
	}

	if tool.c.transported() {
		lines = append(lines, pyTransportLine(tool))
	}

	return append(lines, "        error=error,", "    )"), nil
}

// walksDependencies reports whether the tool hands the driver a walk closure:
// one it declares, or the seeded prose and estimate a plan reports through the
// same closure.
func (t *pyTool) walksDependencies() bool {
	return len(t.c.DepWalks) > 0 ||
		len(t.c.PreviewSentences) > 0 || t.c.BillingDecl != nil
}

// pyRedactedMembers is the body members a preview stands in for rather than
// echoing, "" when the tool declares none.
func pyRedactedMembers(tool *pyTool) string {
	named := make([]string, 0, len(tool.c.Body))

	for _, entry := range tool.bodyFields() {
		if redactsPreview(entry) {
			named = append(named, pyQuote(wireNameOf(entry)))
		}
	}

	return strings.Join(named, ", ")
}

// pyFilterLiteral is one ListFilter literal. The substring match is the
// default, so only the other two name a mode.
func pyFilterLiteral(entry listFilter) string {
	var mode string
	if !pyFilterContains(entry) {
		mode = ", MatchMode." + pyFilterMode(entry)
	}

	return "ListFilter(" + pyQuote(entry.Param) + ", " + pyQuote(entry.Field) + mode + ")"
}

// pyFilterMode is the MatchMode member one filter names. Only text and flags
// are compared, so a flag matched for equality names the mode written for it
// rather than the shared one.
func pyFilterMode(entry listFilter) string {
	if entry.Match == linodev1.ListFilterSpec_MATCH_MEMBER {
		return "MEMBER"
	}

	if entry.Kind == protoreflect.BoolKind {
		return "BOOL"
	}

	return "EQUALS"
}

// pyQuotedJoin is a name list as comma-separated Python literals.
func pyQuotedJoin(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, pyQuote(name))
	}

	return strings.Join(quoted, ", ")
}
