package builder

import (
	"slices"

	"github.com/chadit/LinodeMCP/internal/config"
)

// FieldDiff carries an old/new value pair for one scalar or list field
// in a draft save. JSON-marshaled as “{"old": ..., "new": ...}“ so
// the tool response can carry typed-but-flexible field changes
// without a per-field response shape.
type FieldDiff struct {
	Old any `json:"old"`
	New any `json:"new"`
}

// Diff is the change set a save produces. “IsNew“ true means the
// profile didn't exist before; “AddedTools“/“RemovedTools“
// describe the AllowedTools delta; “ChangedFields“ carries the
// scalar/list fields that differ (description, allow_yolo,
// allowed_environments, required_token_scopes).
//
// For a new profile, “ChangedFields“ includes every non-zero field
// with “Old“ as the zero value. For an update, only differing
// fields appear. Either way the response is small and easy for the
// model to summarize.
type Diff struct {
	Name          string               `json:"name"`
	IsNew         bool                 `json:"is_new"`
	AddedTools    []string             `json:"added_tools"`
	RemovedTools  []string             `json:"removed_tools"`
	ChangedFields map[string]FieldDiff `json:"changed_fields"`
}

// DraftAsUserProfile converts a Draft into the config-file shape so
// callers can compare against and store into “Config.Profiles“.
// Slices are copied so later mutation of the draft doesn't propagate
// into the saved config.
func DraftAsUserProfile(draft *Draft) config.UserProfileConfig {
	return config.UserProfileConfig{
		Description:         draft.Description,
		AllowedTools:        slices.Clone(draft.AllowedTools),
		AllowedEnvironments: slices.Clone(draft.AllowedEnvironments),
		RequiredTokenScopes: slices.Clone(draft.RequiredTokenScopes),
		AllowYolo:           draft.AllowYolo,
	}
}

// ComputeDiff produces a Diff comparing “draft“ against “existing“.
// When “existing“ is nil, every non-zero field of the draft shows up
// in “ChangedFields“ (with “Old“ as the zero value) and IsNew is
// true. AddedTools/RemovedTools always reflect the AllowedTools delta;
// for a new profile that means AddedTools is the draft's full
// AllowedTools and RemovedTools is empty.
//
// The returned slices are sorted ascending so the response is
// reproducible regardless of map iteration order.
func ComputeDiff(
	name string,
	draftCfg *config.UserProfileConfig,
	existing *config.UserProfileConfig,
) *Diff {
	diff := &Diff{
		Name:          name,
		ChangedFields: make(map[string]FieldDiff),
	}

	diff.IsNew = existing == nil

	var prev config.UserProfileConfig
	if existing != nil {
		prev = *existing
	}

	diff.AddedTools = subtractSorted(draftCfg.AllowedTools, prev.AllowedTools)
	diff.RemovedTools = subtractSorted(prev.AllowedTools, draftCfg.AllowedTools)

	if draftCfg.Description != prev.Description {
		diff.ChangedFields["description"] = FieldDiff{Old: prev.Description, New: draftCfg.Description}
	}

	if !slices.Equal(draftCfg.AllowedEnvironments, prev.AllowedEnvironments) {
		diff.ChangedFields["allowed_environments"] = FieldDiff{
			Old: emptyIfNilSlice(prev.AllowedEnvironments),
			New: emptyIfNilSlice(draftCfg.AllowedEnvironments),
		}
	}

	if !slices.Equal(draftCfg.RequiredTokenScopes, prev.RequiredTokenScopes) {
		diff.ChangedFields["required_token_scopes"] = FieldDiff{
			Old: emptyIfNilSlice(prev.RequiredTokenScopes),
			New: emptyIfNilSlice(draftCfg.RequiredTokenScopes),
		}
	}

	if draftCfg.AllowYolo != prev.AllowYolo {
		diff.ChangedFields["allow_yolo"] = FieldDiff{Old: prev.AllowYolo, New: draftCfg.AllowYolo}
	}

	return diff
}

// subtractSorted returns the elements of `source` that are not in
// `minus`, sorted ascending. Used to compute AllowedTools deltas
// for the save diff.
func subtractSorted(source, minus []string) []string {
	if len(source) == 0 {
		return []string{}
	}

	minusSet := make(map[string]struct{}, len(minus))
	for _, item := range minus {
		minusSet[item] = struct{}{}
	}

	out := make([]string, 0, len(source))

	for _, item := range source {
		if _, drop := minusSet[item]; drop {
			continue
		}

		out = append(out, item)
	}

	slices.Sort(out)

	return out
}

// emptyIfNilSlice ensures JSON marshaling produces “[]“ instead of
// “null“ for empty list-valued fields.
func emptyIfNilSlice(s []string) []string {
	if s == nil {
		return []string{}
	}

	return s
}
