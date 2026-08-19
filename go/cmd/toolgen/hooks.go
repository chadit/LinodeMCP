package main

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	// hookKindValidate names a hook that checks a tool's arguments before
	// anything else runs, replacing the required-field checks the emitter would
	// derive.
	hookKindValidate = "validate"
	// hookKindNormalize names a hook that rewrites a tool's arguments before
	// anything reads them, which is where a family's own reading of a value
	// lives: a trimmed label, a decoded list.
	hookKindNormalize = "normalize"
	// hookKindPreview names a hook that answers a mutating tool's dry run,
	// replacing the request-only preview the emitter would derive.
	hookKindPreview = "preview"
	// hookKindFetchState names a hook that reads the resource a destroy would
	// remove. Every destroy declares one: the shape a preview reports and a
	// plan hashes is the typed resource a family's own client method answers
	// with, and no descriptor says which method that is.
	hookKindFetchState = "fetch_state"
	// hookKindDependencyWalk names a hook that says what else a destroy takes
	// with it. A destroy without one previews the call and the state alone.
	hookKindDependencyWalk = "dependency_walk"
	// hookKindExecute names a hook that makes the live call itself, for a route
	// whose request is not the JSON body every generated mutation sends.
	hookKindExecute = "execute"
	// hookKindAnswer names a hook that produces a meta tool's whole result from
	// local state, for the tools that reach no Linode route at all.
	hookKindAnswer = "answer"
)

// toolHooks is the hand-written steps one tool's *Input message declares
// through `tool_hooks`. The emitter writes a direct call to each, so a kind the
// language does not implement breaks the build rather than the first tool call.
type toolHooks struct {
	// Normalize is the Go function rewriting the tool's arguments before
	// anything reads them, or "" when the arguments are read as they arrived.
	Normalize string
	// Validate is the Go function checking the tool's arguments, or "" when
	// the emitter derives the checks from the contract instead.
	Validate string
	// Preview is the Go function answering the tool's dry run, or "" when the
	// emitter previews the request alone.
	Preview string
	// FetchState is the Go function reading the resource a destroy removes.
	FetchState string
	// DependencyWalk is the Go function naming what else the destroy touches,
	// or "" when the preview carries the call and the state alone.
	DependencyWalk string
	// Execute is the Go function making the tool's live call, or "" when the
	// routed JSON request the emitter derives is the call.
	Execute string
	// Answer is the Go function producing a meta tool's whole result, or ""
	// when the declared success_message is the answer.
	Answer string
}

// slot returns where one declared kind's function name is kept, and whether the
// kind is one this emitter knows.
func (h *toolHooks) slot(kind string) (*string, bool) {
	switch kind {
	case hookKindNormalize:
		return &h.Normalize, true
	case hookKindValidate:
		return &h.Validate, true
	case hookKindPreview:
		return &h.Preview, true
	case hookKindFetchState:
		return &h.FetchState, true
	case hookKindDependencyWalk:
		return &h.DependencyWalk, true
	case hookKindExecute:
		return &h.Execute, true
	case hookKindAnswer:
		return &h.Answer, true
	}

	return nil, false
}

// readHooks reads the hook kinds one tool declares and resolves each to the
// function this repo's toolhooks package must export for it.
//
// The name is derived rather than declared because a declared spelling is a
// second fact to keep in step with the first: the contract already says which
// tool needs which kind, and a function named anything else could not be found
// from that. An unknown kind stops the run, since a kind read and then ignored
// would leave the tool served by derived checks its contract says it does not
// use.
func readHooks(tool string, options protoreflect.ProtoMessage) (toolHooks, error) {
	kinds, ok := proto.GetExtension(options, linodev1.E_ToolHooks).([]string)
	if !ok {
		return toolHooks{}, nil
	}

	var declared toolHooks

	for _, kind := range kinds {
		slot, known := declared.slot(kind)
		if !known {
			return toolHooks{}, fmt.Errorf("%w: %s declares %q", errUnknownHookKind, tool, kind)
		}

		if *slot != "" {
			return toolHooks{}, fmt.Errorf("%w: %s declares %s twice", errRepeatedHookKind, tool, kind)
		}

		*slot = toolhooks.FunctionName(tool, kind)
	}

	return declared, nil
}
