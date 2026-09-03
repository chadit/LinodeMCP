package main

import (
	"slices"
	"strings"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The Python arm of the declared walks: the same WalkSpec literals the Go arm
// renders, interpreted by linodemcp.tools.walk_spec.

// The closure's own lines. A parameter nothing in the body reads gets the
// underscore the linter exempts: the client on a walkless plan, the state on a
// closure with no walk, no estimate and no wording naming a member of it.
const (
	pyWalkDefLine  = "    async def dependency_walk("
	pyWalkTypeLine = "    ) -> DryRunDetails:"
)

// Keyword-argument indents inside the rendered closure, and the extra step a
// wrapped sentence takes.
const (
	pyWalkSpecIndent    = 20
	pyWalkNestedIndent  = 24
	pyWalkWarningIndent = 28
	pyWalkBillingIndent = 16
	pyWalkSeedIndent    = 12
	pyWalkWrapStep      = 4
)

// pyDeclaredWalks renders the dependency_walk closure over the declared walks.
func pyDeclaredWalks(tool *pyTool) []string {
	client := "client"
	if len(tool.c.DepWalks) == 0 && tool.c.BillingDecl == nil {
		client = "_client"
	}

	state := "state"
	if !walkReadsState(tool.c) {
		state = "_state"
	}

	lines := []string{
		pyWalkDefLine,
		"        " + client + ": RetryableClient, " + state + ": Any",
		pyWalkTypeLine,
	}

	if walkReadsState(tool.c) {
		lines = append(lines, "        declared = declared_state_of(state)")
	}

	lines = append(lines, pyWalkProseValues(tool)...)

	// A tool with nothing to iterate says its declared lines and stops: running
	// the engine over no walks would be a call whose answer is always empty.
	if len(tool.c.DepWalks) == 0 {
		return append(lines, pyWalkSeededOnly(tool)...)
	}

	seeds := pySentenceSeeds(tool, false)
	cautions := pySentenceSeeds(tool, true)
	assembled := len(seeds) > 0 || len(cautions) > 0 || tool.c.BillingDecl != nil

	call := "        return await run_dependency_walks("
	if assembled {
		call = "        details = await run_dependency_walks("
	}

	lines = append(lines, call, "            client,", "            [")

	for index := range tool.c.DepWalks {
		lines = append(lines, pyWalkSpec(tool, &tool.c.DepWalks[index])...)
	}

	lines = append(lines,
		"            ],",
		"            declared,",
		"            arguments,",
		"        )",
	)

	if !assembled {
		return append(lines, "")
	}

	lines = append(lines, pySeededHalf(seeds, "side_effects", true)...)
	lines = append(lines, pySeededHalf(cautions, "warnings", true)...)
	lines = append(lines, pyBillingLiteral(tool)...)

	return append(lines,
		"        return details",
		"",
	)
}

// pyWalkSeededOnly is the body of a walk that iterates nothing: the declared
// lines and the estimate beside them, with no engine call between.
func pyWalkSeededOnly(tool *pyTool) []string {
	lines := []string{"        details: DryRunDetails = {}"}
	lines = append(lines, pySeededHalf(pySentenceSeeds(tool, false), "side_effects", false)...)
	lines = append(lines, pySeededHalf(pySentenceSeeds(tool, true), "warnings", false)...)
	lines = append(lines, pyBillingLiteral(tool)...)

	return append(lines, "        return details", "")
}

// pySeededHalf writes one half of the seeded prose ahead of the walk's own.
func pySeededHalf(seeds []string, keyword string, walked bool) []string {
	if len(seeds) == 0 {
		return nil
	}

	lines := []string{`        details["` + keyword + `"] = [`}
	lines = append(lines, seeds...)

	if walked {
		lines = append(lines, `            *details.get("`+keyword+`", []),`)
	}

	return append(lines, "        ]")
}

// pySentenceSeeds is one half of the lines the preview family declares, worded
// the way the preview words them: a wording with nothing to fill is its own
// text, and one that names a value reads the plan's own state.
func pySentenceSeeds(tool *pyTool, warnings bool) []string {
	seeds := make([]string, 0, len(tool.c.PreviewSentences))

	for index := range tool.c.PreviewSentences {
		sentence := &tool.c.PreviewSentences[index]
		if sentence.Warning != warnings {
			continue
		}

		if !previewSentenceReads(sentence) {
			// The seed stands directly in the details list, so a split wording
			// carries the parentheses the linter asks of a collection entry.
			seeds = append(seeds, pyWrappedGroupEntry(sentence.Templates[0], pyWalkSeedIndent)...)

			continue
		}

		seeds = append(seeds, pyPreviewSentence(tool, sentence)...)
	}

	return seeds
}

// pyWalkProseValues is the values the seeded wordings read, rooted at the
// composite member the prose is declared against so one wording serves the
// preview and the plan.
func pyWalkProseValues(tool *pyTool) []string {
	if !walkSeedsRead(tool.c) {
		return nil
	}

	if tool.c.previewReadsStateProse() {
		return []string{
			"        " + pyPreviewValuesLocal + " = preview_state_values(" + pyWalkProseState(tool.c) + ")",
		}
	}

	// A wording naming only arguments has nothing to read off the projection,
	// so the map is built here rather than through the state closure the
	// handler renders for the tools that do read it.
	names := tool.c.previewValueArguments()
	if len(names) == 0 {
		return nil
	}

	lines := make([]string, 0, len(names)+3)
	lines = append(lines, "        "+pyPreviewValuesLocal+" = {")

	for _, name := range names {
		lines = append(lines, "            "+pyQuote(name)+": "+pyPreviewValueRead(tool, name)+",")
	}

	return append(lines, "        }", "")
}

// pyWalkProseState is the state a seeded wording reads through.
func pyWalkProseState(tool *contract) string {
	if tool.ProseMember == "" {
		return "declared"
	}

	return "declared.object(" + pyQuote(tool.ProseMember) + ")"
}

// pyBillingLiteral renders the declared price estimate beside the walks.
func pyBillingLiteral(tool *pyTool) []string {
	billing := tool.c.BillingDecl
	if billing == nil {
		return nil
	}

	lines := []string{
		"        await run_billing_delta(",
		"            client,",
		"            BillingSpec(",
		"                price_tool=" + pyQuote(billing.GetPriceTool()) + ",",
		"                type_field=" + pyQuote(billing.GetTypeField()) + ",",
		"                amount=" + pyQuote(billing.GetAmount()) + ",",
	}
	lines = pyWalkString(lines, pyWalkBillingIndent, "sentence", billing.GetSentence())
	lines = pyWalkString(lines, pyWalkBillingIndent, "unknown_sentence", billing.GetUnknownSentence())
	lines = pyWalkString(lines, pyWalkBillingIndent, "absent_type_sentence", billing.GetAbsentTypeSentence())

	return append(lines,
		"            ),",
		"            declared,",
		"            details,",
		"        )",
	)
}

// pyWalkSpec renders one declared walk as its engine literal.
func pyWalkSpec(tool *pyTool, walk *resolvedWalk) []string {
	decl := walk.Decl
	lines := []string{"                WalkSpec("}

	if decl.GetListTool() != "" {
		values := make([]string, 0, len(walk.Slots))
		for _, slot := range walk.Slots {
			values = append(values, pyWalkResolve(tool, slot))
		}

		lines = append(lines,
			"                    list_tool="+pyQuote(decl.GetListTool())+",",
			"                    values=("+strings.Join(values, ", ")+",),")
		lines = pyWalkString(lines, pyWalkSpecIndent, "list_member", decl.GetListMember())
		lines = pyWalkString(lines, pyWalkSpecIndent, "list_error_warning", decl.GetListErrorWarning())
	}

	lines = pyWalkString(lines, pyWalkSpecIndent, "state_member", decl.GetStateMember())
	lines = pyWalkString(lines, pyWalkSpecIndent, "state_scalar", decl.GetStateScalar())
	lines = append(lines, pyWalkFilter(decl.GetFilter())...)
	lines = append(lines, pyWalkEnrich(tool, walk)...)
	lines = append(lines, pyWalkEmit(decl.GetEmit())...)
	lines = append(lines, pyWalkWarnings(decl.GetWarnings())...)

	return append(lines, "                ),")
}

// pyWalkResolve maps one argument name to the expression the seam reads it
// through: the coerced id on the destroy tier, the parsed local elsewhere.
func pyWalkResolve(tool *pyTool, name string) string {
	if tool.c.Tier == tierDestroy && slices.Contains(tool.c.Slots, name) {
		return pyDestroyID(tool, name)
	}

	return pyLocal(name)
}

// pyWalkFilter renders the declared element filter, when one is.
func pyWalkFilter(filter *linodev1.WalkFilter) []string {
	if filter == nil {
		return nil
	}

	lines := []string{"                    filter=WalkFilter("}
	lines = pyWalkString(lines, pyWalkNestedIndent, "field", filter.GetField())
	lines = pyWalkString(lines, pyWalkNestedIndent, "value", filter.GetValue())
	lines = pyWalkString(lines, pyWalkNestedIndent, "argument", filter.GetArgument())

	if filter.GetFold() {
		lines = append(lines, "                        fold=True,")
	}

	return append(lines, "                    ),")
}

// pyWalkEnrich renders the enrichment read, its slots filled from the
// arguments and off the declared state for the fetch-only ids.
func pyWalkEnrich(tool *pyTool, walk *resolvedWalk) []string {
	enrich := walk.Decl.GetEnrich()
	if enrich == nil {
		return nil
	}

	values := make([]string, 0, len(walk.EnrichSlots))

	for _, slot := range walk.EnrichSlots {
		if slot.Argument != "" {
			values = append(values, pyWalkResolve(tool, slot.Argument))

			continue
		}

		values = append(values, "declared.fields.get("+pyQuote(slot.StateField)+")")
	}

	return []string{
		"                    enrich=WalkEnrich(",
		"                        tool=" + pyQuote(enrich.GetTool()) + ",",
		"                        fields=(" + pyQuotedJoin(enrich.GetFields()) + ",),",
		"                        values=(" + strings.Join(values, ", ") + ",),",
		"                    ),",
	}
}

// pyWalkEmit renders what each element becomes.
func pyWalkEmit(emit *linodev1.WalkEmit) []string {
	if emit == nil {
		return nil
	}

	lines := []string{"                    emit=WalkEmit("}
	lines = pyWalkString(lines, pyWalkNestedIndent, "kind", emit.GetKind())
	lines = pyWalkString(lines, pyWalkNestedIndent, "kind_field", emit.GetKindField())
	lines = pyWalkString(lines, pyWalkNestedIndent, "kind_fallback", emit.GetKindFallback())
	lines = pyWalkString(lines, pyWalkNestedIndent, "id_field", emit.GetIdField())
	lines = pyWalkString(lines, pyWalkNestedIndent, "label_field", emit.GetLabelField())
	lines = pyWalkString(lines, pyWalkNestedIndent, "label_fallback", emit.GetLabelFallback())
	lines = pyWalkString(lines, pyWalkNestedIndent, "action", emit.GetAction())
	lines = pyWalkString(lines, pyWalkNestedIndent, "note", emit.GetNote())

	if emit.GetTarget() == linodev1.WalkTarget_WALK_TARGET_SIDE_EFFECTS {
		lines = append(lines, "                        side_effects=True,")
	}

	return append(lines, "                    ),")
}

// pyWalkWarnings renders the whole-walk sentences.
func pyWalkWarnings(warnings []*linodev1.WalkWarning) []string {
	if len(warnings) == 0 {
		return nil
	}

	lines := []string{"                    warnings=("}

	for _, warning := range warnings {
		lines = append(lines, "                        WalkWarning(")
		lines = pyWalkString(lines, pyWalkWarningIndent, "template", warning.GetTemplate())
		lines = pyWalkString(lines, pyWalkWarningIndent, "when_field", warning.GetWhenField())
		lines = pyWalkString(lines, pyWalkWarningIndent, "when_equals", warning.GetWhenEquals())
		lines = pyWalkString(lines, pyWalkWarningIndent, "when_positive", warning.GetWhenPositive())
		lines = pyWalkString(lines, pyWalkWarningIndent, "when_present", warning.GetWhenPresent())
		lines = pyWalkString(lines, pyWalkWarningIndent, "when_absent", warning.GetWhenAbsent())
		lines = pyWalkBool(lines, pyWalkWarningIndent, "when_truncated", warning.GetWhenTruncated())
		lines = pyWalkBool(lines, pyWalkWarningIndent, "when_any", warning.GetWhenAny())
		lines = append(lines, "                        ),")
	}

	return append(lines, "                    ),")
}

// pyWalkString renders one keyword argument, parenthesized across lines when
// the sentence cannot fit the budget.
func pyWalkString(lines []string, indent int, name, value string) []string {
	if value == "" {
		return lines
	}

	pad := strings.Repeat(" ", indent)

	single := pad + name + "=" + pyQuote(value) + ","
	if len(single) <= pyLineBudget {
		return append(lines, single)
	}

	lines = append(lines, pad+name+"=(")
	lines = append(lines, pyQuoteWrapped(value, indent+pyWalkWrapStep)...)

	return append(lines, pad+"),")
}

// pyWalkBool renders one bool keyword argument, omitting false.
func pyWalkBool(lines []string, indent int, name string, value bool) []string {
	if !value {
		return lines
	}

	return append(lines, strings.Repeat(" ", indent)+name+"=True,")
}

// pyWalkImports is the walk-engine names a module's declared walks spell.
func pyWalkImports(tools []*pyTool) string {
	named := make(map[string]bool)
	for _, tool := range tools {
		collectPyWalkImports(tool.c, named)
	}

	return strings.Join(pySortedKeys(named), ", ")
}

// collectPyWalkImports records the literal types one tool's walks render.
func collectPyWalkImports(tool *contract, named map[string]bool) {
	// The deliberately-unknown estimate is a literal in the preview's own
	// details, so it names the sentinel and none of the pricing machinery.
	switch {
	case billingUnpriced(tool.BillingDecl):
		named["BILLING_UNKNOWN"] = true
	case tool.BillingDecl != nil:
		named["BillingSpec"] = true
		named["run_billing_delta"] = true
	}

	for index := range tool.DepWalks {
		decl := tool.DepWalks[index].Decl
		named["WalkSpec"] = true
		named["run_dependency_walks"] = true

		if decl.GetEmit() != nil {
			named["WalkEmit"] = true
		}

		if decl.GetFilter() != nil {
			named["WalkFilter"] = true
		}

		if len(decl.GetWarnings()) > 0 {
			named["WalkWarning"] = true
		}

		if decl.GetEnrich() != nil {
			named["WalkEnrich"] = true
		}
	}
}
