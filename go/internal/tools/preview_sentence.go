package tools

import (
	"context"
	"fmt"
	"maps"
	"math"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// The support the generated handlers call to report the prose their contract
// declares. Python's twin lives in linodemcp/tools/preview.py and answers the
// same way, so one declared sentence reaches both languages as one sentence.

// previewOpen and previewClose bracket an argument name inside a wording.
const (
	previewOpen  = "{"
	previewClose = "}"
)

// PreviewText is a text argument as a declared preview sentence reports it,
// and "" when the call carries none or carries something that is not text.
//
// Empty is what lets a wording naming the argument step aside for the next one,
// so a caller who supplied no label reads the sentence that names none rather
// than one with a gap in it.
func PreviewText(request *mcp.CallToolRequest, name string) string {
	return request.GetString(name, "")
}

// PreviewNumber is a whole-number argument as a declared preview sentence
// reports it, and "" when the call carries none.
//
// Zero reads as none for the same reason the hooks this replaces read it that
// way: the ids a sentence names are resource ids, and no resource has id zero.
func PreviewNumber(request *mcp.CallToolRequest, name string) string {
	// Read raw rather than through the request's coercing getter: Python's twin
	// never coerced, and the checks have already held the argument to its
	// declared type by the time a preview reads it.
	whole, ok := previewWholeNumber(request.GetArguments()[name])
	if !ok || whole == 0 {
		return ""
	}

	return strconv.FormatInt(whole, 10)
}

// previewWholeNumber is a JSON number as a whole one, and false for anything
// else. A number arrives as a float once it has been through a decoder.
func previewWholeNumber(raw any) (int64, bool) {
	switch value := raw.(type) {
	case float64:
		if value != math.Trunc(value) {
			return 0, false
		}

		return int64(value), true
	case int:
		return int64(value), true
	case int64:
		return value, true
	}

	return 0, false
}

// previewStateSeparator divides a state member from the member under it, which
// is as deep as a declared path goes.
const previewStateSeparator = "."

// previewStateAt walks a declared state down to the object holding the last name
// in a path, and that name.
//
// A state some other fetch produced reads as carrying nothing, so a wording over
// it steps aside rather than reporting a gap.
func previewStateAt(state any, path string) (DeclaredState, string) {
	fields, isDeclared := state.(DeclaredState)
	if !isDeclared {
		return DeclaredState{}, ""
	}

	head, member, dotted := strings.Cut(path, previewStateSeparator)
	if !dotted {
		return fields, head
	}

	return fields.Object(head), member
}

// PreviewStateText is a text member of the fetched resource as a declared
// preview sentence reports it, and "" when the resource carries none.
func PreviewStateText(state any, path string) string {
	fields, name := previewStateAt(state, path)

	return fields.Text(name)
}

// PreviewStateNumber is a whole-number member of the fetched resource as a
// declared preview sentence reports it, and "" when the resource carries none.
//
// Zero reads as none for the reason PreviewNumber reads it that way: the sizes
// and ids a sentence names are absent at zero, which is what lets the wording
// naming a starting size give way to the one that names only the target.
func PreviewStateNumber(state any, path string) string {
	fields, name := previewStateAt(state, path)

	whole, carried := fields.Number(name)
	if !carried || whole == 0 {
		return ""
	}

	return strconv.Itoa(whole)
}

// PreviewChanged is a state reading as a wording that only names a change
// reports it: the reading itself, and "" where it already matches the argument
// the call would set.
//
// Matching reads as absent so the wording naming the reading steps aside for the
// next one, which is what turns "Label changes from X to Y" into "Label is set
// to Y" without a second rule for choosing between them.
func PreviewChanged(reading, argument string) string {
	if reading == argument {
		return ""
	}

	return reading
}

// PreviewCarried reports whether the call sent an argument at all, which is what
// a line guarded on a value it never names asks about.
//
// Presence rather than emptiness is the question here: a number, a list and an
// object each have a real zero value a caller can mean, so a throttle of 0 and
// an empty tag list are both carried. A text argument is guarded on its own
// reader instead, which already collapses empty onto absent.
func PreviewCarried(request *mcp.CallToolRequest, name string) bool {
	_, carried := request.GetArguments()[name]

	return carried
}

// PreviewCarriedNumber is a whole-number argument a line has already guarded on
// being carried, so zero reports as the value it is rather than as no value.
func PreviewCarriedNumber(request *mcp.CallToolRequest, name string) string {
	whole, isWhole := previewWholeNumber(request.GetArguments()[name])
	if !isWhole {
		return ""
	}

	return strconv.FormatInt(whole, 10)
}

// PreviewGuarded is a line the call has to have asked for: the wording when it
// did, and "" when it did not, which drops the line.
func PreviewGuarded(carried bool, line string) string {
	if !carried {
		return ""
	}

	return line
}

// PreviewDiffers reports whether a line naming only the new value has a change
// to report: the call carries one, and the resource does not already hold it.
//
// A wording that names both values needs none of this, since the reading it
// names steps aside on its own. This is for the lines that name neither the
// old value nor the comparison.
func PreviewDiffers(reading, argument string) bool {
	return argument != "" && reading != argument
}

// PreviewOr is a folded argument as a wording reports it: the value the call
// carried, or the one that argument's own fold sends in its place. The default
// is declared once, in the fold, so the sentence and the request it describes
// name the same value.
func PreviewOr(value, folded string) string {
	if value == "" {
		return folded
	}

	return value
}

// PreviewFolded is a state reading as a matched line compares it. A resource
// spells its own vocabularies, so a status the API sends as "Running" selects
// the arm declared for "running"; an argument's values are the ones the tool's
// schema publishes and are compared exactly.
func PreviewFolded(reading string) string {
	return strings.ToLower(reading)
}

// PreviewMatched is the wording a value selects, filled in the way an ordered
// wording is, and "" for a value the declaration answers with no wording.
func PreviewMatched(values map[string]string, value string, arms map[string]string, otherwise string) string {
	wording, named := arms[value]
	if !named {
		wording = otherwise
	}

	if wording == "" {
		return ""
	}

	return PreviewSentence(values, wording)
}

// previewElementValue is one entry of a repeated argument as a wording reports
// it, and "" for an entry neither language spells the same way.
func previewElementValue(raw any) string {
	if text, isText := raw.(string); isText {
		return text
	}

	whole, isWhole := previewWholeNumber(raw)
	if !isWhole {
		return ""
	}

	return strconv.FormatInt(whole, 10)
}

// previewElements is the entries of a repeated argument, the ones neither
// language can spell dropped.
func previewElements(request *mcp.CallToolRequest, name string) []string {
	raw, sent := request.GetArguments()[name].([]any)
	if !sent {
		return nil
	}

	entries := make([]string, 0, len(raw))

	for _, item := range raw {
		if value := previewElementValue(item); value != "" {
			entries = append(entries, value)
		}
	}

	return entries
}

// PreviewJoined is the entries of a repeated argument as one value, for the
// line that names them all rather than one each.
func PreviewJoined(request *mcp.CallToolRequest, name, separator string) string {
	return strings.Join(previewElements(request, name), separator)
}

// PreviewPerElement is one line per entry of a repeated argument, in the order
// the call sent them.
//
// A list the call did not send writes no lines at all, which is what a preview
// owes a caller who asked for no memberships.
func PreviewPerElement(
	request *mcp.CallToolRequest, name string, values map[string]string, template string,
) []string {
	entries := previewElements(request, name)
	lines := make([]string, 0, len(entries))

	for _, entry := range entries {
		filled := make(map[string]string, len(values)+1)
		maps.Copy(filled, values)
		filled[PreviewElementName] = entry

		lines = append(lines, PreviewSentence(filled, template))
	}

	return lines
}

// PreviewElementName is the placeholder an entry fills, which is the one name a
// wording may read that is not an argument.
const PreviewElementName = "element"

// PreviewFlagState is a flag's three states as a declared preview reads it: a// PreviewFlagState is a flag's three states as a declared preview reads it: a// PreviewFlagState is a flag's three states as a declared preview reads it: a// PreviewFlagState is a flag's three states as a declared preview reads it: a
// declaration answers each of them with a wording of its own, so absent is not
// folded onto false here and each family says which it means.
type PreviewFlagState int

// The states a chosen line selects its wording by.
const (
	// PreviewFlagAbsent is a flag the call did not carry, or carried as
	// something that is not a bool.
	PreviewFlagAbsent PreviewFlagState = iota
	PreviewFlagTrue
	PreviewFlagFalse
)

// PreviewFlag is the state of a bool argument a declared preview line selects
// its wording by.
func PreviewFlag(request *mcp.CallToolRequest, name string) PreviewFlagState {
	return previewFlagOf(request.GetArguments()[name])
}

// PreviewMemberFlag is the state of a bool sitting inside an open-object
// argument, which is where the families that send their flag inside the payload
// carry it.
func PreviewMemberFlag(request *mcp.CallToolRequest, name, member string) PreviewFlagState {
	object, ok := request.GetArguments()[name].(map[string]any)
	if !ok {
		return PreviewFlagAbsent
	}

	return previewFlagOf(object[member])
}

// previewFlagOf reads one value as a flag. Anything that is not a bool reads as
// absent, which is what both hand walks this replaces did with a flag arriving
// as text.
func previewFlagOf(raw any) PreviewFlagState {
	flag, ok := raw.(bool)
	if !ok {
		return PreviewFlagAbsent
	}

	if flag {
		return PreviewFlagTrue
	}

	return PreviewFlagFalse
}

// PreviewChosen is the wording a flag selects, filled in the way an ordered
// wording is, and "" for a state the declaration answers with no wording.
func PreviewChosen(
	values map[string]string, state PreviewFlagState, whenTrue, whenFalse, whenAbsent string,
) string {
	wording := whenAbsent

	switch state {
	case PreviewFlagTrue:
		wording = whenTrue
	case PreviewFlagFalse:
		wording = whenFalse
	case PreviewFlagAbsent:
	}

	if wording == "" {
		return ""
	}

	return PreviewSentence(values, wording)
}

// PreviewSentence is the wording a declared preview line reports: the first one
// whose placeholders all carry a value, and "" when none is complete.
//
// A line with no wording to report is dropped rather than reported with a gap
// in it, which is what lets a whole sentence be conditional: the volume create
// names the instance it attaches to only when the call names one.
func PreviewSentence(values map[string]string, templates ...string) string {
	for _, template := range templates {
		if previewFilled(values, template) {
			return previewExpand(values, template)
		}
	}

	return ""
}

// previewReportedLines is the declared lines that have something to say, which
// is every one whose wording could be filled.
func previewReportedLines(lines []string) []string {
	reported := make([]string, 0, len(lines))

	for _, line := range lines {
		if line != "" {
			reported = append(reported, line)
		}
	}

	if len(reported) == 0 {
		return nil
	}

	return reported
}

// previewFilled reports whether every placeholder in a wording has a value.
func previewFilled(values map[string]string, template string) bool {
	rest := template

	for {
		_, after, found := strings.Cut(rest, previewOpen)
		if !found {
			return true
		}

		name, remainder, closed := strings.Cut(after, previewClose)
		if !closed {
			return true
		}

		if values[name] == "" {
			return false
		}

		rest = remainder
	}
}

// previewExpand writes each placeholder's value into the wording.
func previewExpand(values map[string]string, template string) string {
	var out strings.Builder

	out.Grow(len(template))

	rest := template

	for {
		before, after, found := strings.Cut(rest, previewOpen)
		if !found {
			out.WriteString(rest)

			return out.String()
		}

		out.WriteString(before)

		name, remainder, closed := strings.Cut(after, previewClose)
		if !closed {
			out.WriteString(previewOpen)
			out.WriteString(after)

			return out.String()
		}

		out.WriteString(values[name])

		rest = remainder
	}
}

// RunDeclaredPreview answers the dry run of a tool whose prose the contract
// declares: the call reported rather than made, and the consequences beside it.
//
// fetchState is the read the contract declares its state through, nil for a
// tool that describes a call against no existing resource.
//
// The walk runs under the caller's context, so a preview nobody is waiting for
// stops rather than describing a call into a closed connection, and a failure
// names the tool because a preview fails when the call it was asked to describe
// has no JSON form, which is a defect in the tool rather than in the resource.
func RunDeclaredPreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, method, path string,
	body any,
	fetchState func(ctx context.Context, client *linode.Client) (any, error),
	details *DryRunDetails,
) (*mcp.CallToolResult, error) {
	worded := *details

	return RunDeclaredStatePreview(ctx, request, cfg, toolName, method, path, body, fetchState,
		func(_ any) DryRunDetails { return worded })
}

// RunDeclaredTransportPreview is RunDeclaredPreview for a tool whose prose reads
// what its presigned upload would send. The source is measured inside the walk,
// so a preview nobody is waiting for opens nothing, and a source the guard
// refuses is reported as a warning ahead of the declared lines.
func RunDeclaredTransportPreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, method, path string,
	body any,
	localPathArgument string,
	lines func(transfer PresignPreview) DryRunDetails,
) (*mcp.CallToolResult, error) {
	return RunDeclaredStatePreview(ctx, request, cfg, toolName, method, path, body, nil,
		func(_ any) DryRunDetails {
			transfer := PreviewPresignSource(request, cfg, body, localPathArgument)
			worded := lines(transfer)
			worded.Warnings = append([]string{transfer.Refusal}, worded.Warnings...)

			return worded
		})
}

// RunDeclaredStatePreview is RunDeclaredPreview for the tools whose prose names
// the resource the fetch read, so the lines are worded after the read rather
// than before it.
//
// lines receives whatever the declared fetch answered, which is the projected
// state for a tool that declares one and nil for a tool that declares none.
func RunDeclaredStatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, method, path string,
	body any,
	fetchState func(ctx context.Context, client *linode.Client) (any, error),
	lines func(state any) DryRunDetails,
) (*mcp.CallToolResult, error) {
	result, err := RunDryRunPreviewWithBodyDetailed(
		ctx, request, cfg, toolName, method, path, body, fetchState,
		func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
			if err := ctx.Err(); err != nil {
				return DryRunDetails{}, fmt.Errorf("%s side-effect walk canceled: %w", toolName, err)
			}

			worded := lines(state)

			// The estimate rides through as declared: a line is dropped when
			// nothing filled it, where an estimate is the whole answer to a
			// question the caller asked.
			return DryRunDetails{
				SideEffects:  previewReportedLines(worded.SideEffects),
				Warnings:     previewReportedLines(worded.Warnings),
				BillingDelta: worded.BillingDelta,
			}, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("%s preview: %w", toolName, err)
	}

	return result, nil
}
