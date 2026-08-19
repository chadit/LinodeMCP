package tools

import (
	"context"
	"fmt"
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

// PreviewFlagState is a flag's three states as a declared preview reads it: a
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
	reported := DryRunDetails{
		SideEffects: previewReportedLines(details.SideEffects),
		Warnings:    previewReportedLines(details.Warnings),
	}

	result, err := RunDryRunPreviewWithBodyDetailed(
		ctx, request, cfg, toolName, method, path, body, fetchState,
		func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
			if err := ctx.Err(); err != nil {
				return DryRunDetails{}, fmt.Errorf("%s side-effect walk canceled: %w", toolName, err)
			}

			return reported, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("%s preview: %w", toolName, err)
	}

	return result, nil
}
