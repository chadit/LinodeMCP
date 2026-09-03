package main

import (
	"maps"
	"slices"
	"strings"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The Go arm of the declared walks: each resolvedWalk renders as one
// tools.WalkSpec literal the shared engine interprets, the CompositeCall
// pattern applied to the blast radius.

// emitDeclaredWalks writes the DependencyWalk closure over the declared
// walks. The parsed parameter carries the single-id wrapper's own parameter,
// empty where the closure closes over the handler's locals instead.
func emitDeclaredWalks(out *source, tool *contract, parsed string, ordered []field) {
	out.need(importProto)

	signature := "ctx context.Context, client *linode.Client, state any"
	if parsed != "" {
		signature = "ctx context.Context, client *linode.Client, " + parsed + ", state any"
	}

	out.writef("\t\tDependencyWalk: func(%s) (tools.DryRunDetails, error) {", signature)

	// The projection is read by the walks, by a wording naming a state member,
	// and by the billing estimate. A closure with none of the three has nothing
	// to read it, and binding it there would not compile.
	if walkReadsState(tool) {
		out.writef("\t\t\tdeclared, err := tools.DeclaredStateOf(state)")
		out.writef("\t\t\tif err != nil {")
		out.writef("\t\t\t\treturn tools.DryRunDetails{}, err")
		out.writef("\t\t\t}")
		out.writef("")
	}

	emitWalkProseValues(out, tool)

	seeds := walkSentenceSeeds(out, tool, false)
	cautions := walkSentenceSeeds(out, tool, true)
	assembled := len(seeds) > 0 || len(cautions) > 0 || tool.BillingDecl != nil

	// A tool with nothing to iterate says its declared lines and stops: running
	// the engine over no walks would be a call whose answer is always empty.
	if len(tool.DepWalks) == 0 {
		out.writef("\t\t\tdetails := tools.DryRunDetails{}")
	}

	if len(tool.DepWalks) > 0 {
		call := "return tools.RunDependencyWalks(ctx, client, []tools.WalkSpec{"
		if assembled {
			call = "details, err := tools.RunDependencyWalks(ctx, client, []tools.WalkSpec{"
		}

		out.writef("\t\t\t%s", call)

		for index := range tool.DepWalks {
			emitWalkSpec(out, &tool.DepWalks[index], ordered)
		}

		out.writef("\t\t\t}, declared, %s)", walkArgumentsLiteral(tool, ordered))

		if assembled {
			out.writef("\t\t\tif err != nil {")
			out.writef("\t\t\t\treturn details, err")
			out.writef("\t\t\t}")
		}
	}

	emitSeededHalf(out, seeds, "SideEffects", len(tool.DepWalks) > 0)
	emitSeededHalf(out, cautions, "Warnings", len(tool.DepWalks) > 0)

	emitBillingLiteral(out, tool)

	if assembled {
		out.writef("")
		out.writef("\t\t\treturn details, nil")
	}

	out.writef("\t\t},")
}

// walkReadsState reports whether the rendered closure has a consumer for the
// projected state: a walk to run it against, a billing estimate, or a wording
// naming a member of it. A closure with none of the three would bind a local
// nothing reads, which does not compile.
func walkReadsState(tool *contract) bool {
	return len(tool.DepWalks) > 0 || tool.BillingDecl != nil || tool.previewReadsStateProse()
}

// emitSeededHalf writes one half of the seeded prose. A closure that ran walks
// says its lines ahead of theirs; one that ran none states them, since merging
// with an answer that is always empty reads as a walk that never happened.
func emitSeededHalf(out *source, seeds []string, member string, walked bool) {
	if len(seeds) == 0 {
		return
	}

	out.writef("")

	if !walked {
		out.writef("\t\t\tdetails.%s = []string{%s}", member, strings.Join(seeds, ", "))

		return
	}

	out.writef("\t\t\tdetails.%s = append([]string{%s}, details.%s...)",
		member, strings.Join(seeds, ", "), member)
}

// emitWalkProseValues writes the value map the seeded wordings read, for the
// prose that names what the plan's own state carries. The reads are rooted at
// the composite member the prose is declared against, so one wording serves the
// preview and the plan.
func emitWalkProseValues(out *source, tool *contract) {
	if !walkSeedsRead(tool) {
		return
	}

	emitPreviewValuesFrom(out, tool, "\t\t\t", walkProseState(tool))
}

// walkProseState is the state a seeded wording reads through: the declared
// prose member of the composite, or the whole projection when the tool names
// none.
func walkProseState(tool *contract) string {
	if tool.ProseMember == "" {
		return "declared"
	}

	return "declared.Object(" + goStringLiteral(tool.ProseMember) + ")"
}

// emitBillingLiteral writes the declared price estimate beside the walks.
func emitBillingLiteral(out *source, tool *contract) {
	billing := tool.BillingDecl
	if billing == nil {
		return
	}

	out.writef("")
	out.writef("\t\t\ttools.RunBillingDelta(ctx, client, &tools.BillingSpec{")
	out.writef("\t\t\t\tNew: func() proto.Message { return &linodev1.%s{} },", tool.BillingRead.TypeName)
	out.writef("\t\t\t\tPriceTool: %s,", goStringLiteral(billing.GetPriceTool()))
	out.writef("\t\t\t\tTypeField: %s,", goStringLiteral(billing.GetTypeField()))
	out.writef("\t\t\t\tAmount: %s,", goStringLiteral(billing.GetAmount()))
	out.writef("\t\t\t\tSentence: %s,", goStringLiteral(billing.GetSentence()))
	out.writef("\t\t\t\tUnknownSentence: %s,", goStringLiteral(billing.GetUnknownSentence()))
	walkGoString(out, walkBillingTabs, "AbsentTypeSentence", billing.GetAbsentTypeSentence())
	out.writef("\t\t\t}, declared, &details)")
}

// walkSentenceSeeds is one half of the lines the preview family declares, said
// ahead of the walk's own.
//
// A wording with nothing to fill is its own text. One that names a value is
// worded here the way the preview words it, against the plan's state, so a
// staged tool states its prose once and both paths report the same sentence.
func walkSentenceSeeds(out *source, tool *contract, warnings bool) []string {
	seeds := make([]string, 0, len(tool.PreviewSentences))

	for index := range tool.PreviewSentences {
		sentence := &tool.PreviewSentences[index]
		if sentence.Warning != warnings {
			continue
		}

		if !previewSentenceReads(sentence) {
			seeds = append(seeds, goStringLiteral(sentence.Templates[0]))

			continue
		}

		out.need(importTools)

		seeds = append(seeds, previewSentenceExpression(tool, sentence))
	}

	return seeds
}

// walkSeedsRead reports whether any seeded wording names a value, which is what
// puts a value map inside the emitted walk.
func walkSeedsRead(tool *contract) bool {
	for index := range tool.PreviewSentences {
		if previewSentenceReads(&tool.PreviewSentences[index]) {
			return true
		}
	}

	return false
}

// Tab depths of the rendered literal's fields inside the DestructiveAction.
const (
	walkFieldTabs   = 5
	walkNestedTabs  = 6
	walkWarningTabs = 7
	walkBillingTabs = 4
)

// emitWalkSpec writes one declared walk as its engine literal.
func emitWalkSpec(out *source, walk *resolvedWalk, ordered []field) {
	decl := walk.Decl

	out.writef("\t\t\t\t{")

	if decl.GetListTool() != "" {
		values := make([]string, 0, len(walk.Slots))
		for _, slot := range walk.Slots {
			values = append(values, matchLocal(slot, ordered))
		}

		// A member walk decodes the whole response; a page walk decodes
		// each element.
		decoded := walk.Element.TypeName
		if decl.GetListMember() != "" {
			decoded = walk.Response.TypeName
		}

		out.writef("\t\t\t\t\tListTool: %s,", goStringLiteral(decl.GetListTool()))
		walkGoString(out, walkFieldTabs, "ListMember", decl.GetListMember())
		out.writef("\t\t\t\t\tNew: func() proto.Message { return &linodev1.%s{} },", decoded)
		out.writef("\t\t\t\t\tValues: []any{%s},", strings.Join(values, ", "))
		out.writef("\t\t\t\t\tListErrorWarning: %s,", goStringLiteral(decl.GetListErrorWarning()))
	}

	walkGoString(out, walkFieldTabs, "StateMember", decl.GetStateMember())
	walkGoString(out, walkFieldTabs, "StateScalar", decl.GetStateScalar())
	emitWalkFilterLiteral(out, decl.GetFilter())
	emitWalkEnrichLiteral(out, walk, ordered)
	emitWalkEmitLiteral(out, decl.GetEmit())
	emitWalkWarningLiterals(out, decl.GetWarnings())

	out.writef("\t\t\t\t},")
}

// emitWalkFilterLiteral writes the declared element filter, when one is.
func emitWalkFilterLiteral(out *source, filter *linodev1.WalkFilter) {
	if filter == nil {
		return
	}

	out.writef("\t\t\t\t\tFilter: &tools.WalkSpecFilter{")
	walkGoString(out, walkNestedTabs, "Field", filter.GetField())
	walkGoString(out, walkNestedTabs, "Value", filter.GetValue())
	walkGoString(out, walkNestedTabs, "Argument", filter.GetArgument())
	walkGoBool(out, walkNestedTabs, "Fold", filter.GetFold())
	out.writef("\t\t\t\t\t},")
}

// emitWalkEnrichLiteral writes the enrichment read, its slots filled from the
// locals for arguments and off the declared state for the fetch-only ids.
func emitWalkEnrichLiteral(out *source, walk *resolvedWalk, ordered []field) {
	enrich := walk.Decl.GetEnrich()
	if enrich == nil {
		return
	}

	values := make([]string, 0, len(walk.EnrichSlots))

	for _, slot := range walk.EnrichSlots {
		if slot.Argument != "" {
			values = append(values, matchLocal(slot.Argument, ordered))

			continue
		}

		values = append(values, "declared["+goStringLiteral(slot.StateField)+"]")
	}

	out.writef("\t\t\t\t\tEnrich: &tools.WalkSpecEnrich{")
	out.writef("\t\t\t\t\t\tNew: func() proto.Message { return &linodev1.%s{} },", walk.EnrichMessage.TypeName)
	out.writef("\t\t\t\t\t\tTool: %s,", goStringLiteral(enrich.GetTool()))
	out.writef("\t\t\t\t\t\tFields: []string{%s},", goNameList(enrich.GetFields()))
	out.writef("\t\t\t\t\t\tValues: []any{%s},", strings.Join(values, ", "))
	out.writef("\t\t\t\t\t},")
}

// emitWalkEmitLiteral writes what each element becomes.
func emitWalkEmitLiteral(out *source, emit *linodev1.WalkEmit) {
	if emit == nil {
		return
	}

	out.writef("\t\t\t\t\tEmit: tools.WalkSpecEmit{")
	walkGoString(out, walkNestedTabs, "Kind", emit.GetKind())
	walkGoString(out, walkNestedTabs, "KindField", emit.GetKindField())
	walkGoString(out, walkNestedTabs, "KindFallback", emit.GetKindFallback())
	walkGoString(out, walkNestedTabs, "IDField", emit.GetIdField())
	walkGoString(out, walkNestedTabs, "LabelField", emit.GetLabelField())
	walkGoString(out, walkNestedTabs, "LabelFallback", emit.GetLabelFallback())
	walkGoString(out, walkNestedTabs, "Action", emit.GetAction())
	walkGoString(out, walkNestedTabs, "Note", emit.GetNote())
	walkGoBool(out, walkNestedTabs, "SideEffects",
		emit.GetTarget() == linodev1.WalkTarget_WALK_TARGET_SIDE_EFFECTS)
	out.writef("\t\t\t\t\t},")
}

// emitWalkWarningLiterals writes the whole-walk sentences.
func emitWalkWarningLiterals(out *source, warnings []*linodev1.WalkWarning) {
	if len(warnings) == 0 {
		return
	}

	out.writef("\t\t\t\t\tWarnings: []tools.WalkSpecWarning{")

	for _, warning := range warnings {
		out.writef("\t\t\t\t\t\t{")
		walkGoString(out, walkWarningTabs, "Template", warning.GetTemplate())
		walkGoString(out, walkWarningTabs, "WhenField", warning.GetWhenField())
		walkGoString(out, walkWarningTabs, "WhenEquals", warning.GetWhenEquals())
		walkGoString(out, walkWarningTabs, "WhenPositive", warning.GetWhenPositive())
		walkGoString(out, walkWarningTabs, "WhenPresent", warning.GetWhenPresent())
		walkGoString(out, walkWarningTabs, "WhenAbsent", warning.GetWhenAbsent())
		walkGoBool(out, walkWarningTabs, "WhenTruncated", warning.GetWhenTruncated())
		walkGoBool(out, walkWarningTabs, "WhenAny", warning.GetWhenAny())
		out.writef("\t\t\t\t\t\t},")
	}

	out.writef("\t\t\t\t\t},")
}

// walkGoString writes one string field of a literal, omitting the zero value
// so the rendered spec reads as sparsely as the declaration.
func walkGoString(out *source, depth int, name, value string) {
	if value == "" {
		return
	}

	out.writef("%s%s: %s,", strings.Repeat("\t", depth), name, goStringLiteral(value))
}

// walkGoBool writes one bool field of a literal, omitting false.
func walkGoBool(out *source, depth int, name string, value bool) {
	if !value {
		return
	}

	out.writef("%s%s: true,", strings.Repeat("\t", depth), name)
}

// walkArgumentsLiteral renders the map of arguments the walks interpolate or
// filter on, nil when they name none.
func walkArgumentsLiteral(tool *contract, ordered []field) string {
	names := walkArgumentNames(tool)
	if len(names) == 0 {
		return "nil"
	}

	entries := make([]string, 0, len(names))
	for _, name := range names {
		entries = append(entries, goStringLiteral(name)+": "+matchLocal(name, ordered))
	}

	return "map[string]any{" + strings.Join(entries, ", ") + "}"
}

// walkArgumentNames collects every argument name the walks reference, sorted
// for a stable rendering.
func walkArgumentNames(tool *contract) []string {
	seen := make(map[string]bool)

	for index := range tool.DepWalks {
		collectWalkArguments(tool.DepWalks[index].Decl, seen)
	}

	names := slices.Collect(maps.Keys(seen))
	slices.Sort(names)

	return names
}

// collectWalkArguments records the argument names one walk reads: its
// filter's, and every {arg:name} its templates interpolate.
func collectWalkArguments(decl *linodev1.DependencyWalk, seen map[string]bool) {
	if name := decl.GetFilter().GetArgument(); name != "" {
		seen[name] = true
	}

	for _, template := range walkTemplates(decl) {
		collectTemplateArguments(template, seen)
	}
}

// walkTemplates is every template one walk carries.
func walkTemplates(decl *linodev1.DependencyWalk) []string {
	templates := make([]string, 0, 2+len(decl.GetWarnings()))
	templates = append(templates, decl.GetEmit().GetNote(), decl.GetListErrorWarning())

	for _, warning := range decl.GetWarnings() {
		templates = append(templates, warning.GetTemplate())
	}

	return templates
}

// collectTemplateArguments records one template's {arg:name} references.
func collectTemplateArguments(template string, seen map[string]bool) {
	for _, match := range depWalkPlaceholder.FindAllStringSubmatch(template, -1) {
		if name, found := strings.CutPrefix(match[1], "arg:"); found {
			seen[name] = true
		}
	}
}
