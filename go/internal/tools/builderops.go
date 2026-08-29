package tools

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/genlocal"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
)

// The builder state's own local operations: the draft registry, the tool
// catalog and the configuration file as the declared operations reach them.
//
// Each takes the values its declaration names and answers the shape it
// declares, so none of them can tell which tool called it. No sentence is
// written here: a reported condition is worded by the generated arm beside the
// handlers, and a per-entry verdict is worded by the generated lookup the
// contract fills, which is why the same words reach every language.

// capabilityPrefix is what every capability tag's long spelling opens with, so
// trimming it gives the short form a caller may filter on instead.
const capabilityPrefix = "Cap"

// DraftRead answers one draft's current state, reporting
// genlocal.ErrLocalDraftMissing where the registry holds none under the name.
func (s *BuilderState) DraftRead(draft string) (*genlocal.ProfileDraftResponse, error) {
	found, held := s.Drafts.Get(draft)
	if !held {
		return nil, genlocal.ErrLocalDraftMissing
	}

	return draftAnswer(found), nil
}

// DraftCreate files a new draft, seeded from the profile a source names.
//
// It reports genlocal.ErrLocalNotFound where the source resolves to no profile
// and genlocal.ErrLocalAlreadyExists where the name is already filed. The seed
// is looked up across both the configuration's own profiles and the built-ins,
// which is why the state carries the configuration rather than taking it per
// call.
func (s *BuilderState) DraftCreate(draft, source string) (*genlocal.ProfileDraftResponse, error) {
	var seed *profiles.Profile

	if source != "" {
		resolved, found := profiles.LookupProfile(source, s.Config, s.Catalog())
		if !found {
			return nil, genlocal.ErrLocalNotFound
		}

		seed = &resolved
	}

	// Create reports an empty name or a name already filed, and the contract's
	// own rule refuses the empty one before the call, so a failure here is the
	// name being taken.
	created, err := s.Drafts.Create(draft, seed)
	if err != nil {
		return nil, genlocal.ErrLocalAlreadyExists
	}

	return draftAnswer(created), nil
}

// DraftSet writes every setting the call sent and answers which ones moved,
// leaving a setting the call did not send alone.
//
// The pointers are what carry that difference: a list sent empty clears a
// setting, and an absent one is not an instruction at all.
func (s *BuilderState) DraftSet(
	draft string, environments, scopes *[]string, yolo *bool,
) (*genlocal.ProfileDraftSetResponse, error) {
	changes := make(map[string]any, 3)

	if environments != nil {
		if s.Drafts.SetAllowedEnvironments(draft, *environments) != nil {
			return nil, genlocal.ErrLocalDraftMissing
		}

		changes[argAllowedEnvironments] = *environments
	}

	if scopes != nil {
		if s.Drafts.SetRequiredTokenScopes(draft, *scopes) != nil {
			return nil, genlocal.ErrLocalDraftMissing
		}

		changes[argRequiredTokenScopes] = *scopes
	}

	if yolo != nil {
		if s.Drafts.SetAllowYolo(draft, *yolo) != nil {
			return nil, genlocal.ErrLocalDraftMissing
		}

		changes[argAllowYolo] = *yolo
	}

	return genlocal.NewProfileDraftSetResponse(draft, changes), nil
}

// DraftToolsAdd writes the tool names a draft allows and answers the ones that
// moved, reporting genlocal.ErrLocalDraftMissing where the registry holds no
// draft under the name.
//
// The patterns expand against the live catalog, so a wildcard picks up tools
// registered after the draft was made. That is what separates this side from
// the removing one, which matches the draft's own list.
func (s *BuilderState) DraftToolsAdd(
	draft string, patterns []string,
) (*genlocal.ProfileDraftAddToolsResponse, error) {
	added, err := s.Drafts.AddTools(draft, patterns, s.Catalog())
	if err != nil {
		return nil, genlocal.ErrLocalDraftMissing
	}

	return genlocal.NewProfileDraftAddToolsResponse(draft, added), nil
}

// DraftToolsRemove takes tool names off a draft and answers the ones that
// moved, reporting genlocal.ErrLocalDraftMissing the same way.
func (s *BuilderState) DraftToolsRemove(
	draft string, patterns []string,
) (*genlocal.ProfileDraftRemoveToolsResponse, error) {
	removed, err := s.Drafts.RemoveTools(draft, patterns)
	if err != nil {
		return nil, genlocal.ErrLocalDraftMissing
	}

	return genlocal.NewProfileDraftRemoveToolsResponse(draft, removed), nil
}

// DraftDiscard removes a draft and answers whether one was there, which is what
// lets a caller run it from a cleanup path without checking first.
func (s *BuilderState) DraftDiscard(draft string) *genlocal.ProfileDraftDiscardResponse {
	return genlocal.NewProfileDraftDiscardResponse(draft, s.Drafts.Discard(draft))
}

// DraftSave writes one draft into the configuration file as a user profile and
// answers the difference it made.
//
// The file is re-read on every call so a concurrent edit is not stomped, and the
// path is read at call time so an override applies to the call rather than to
// the process. That is why this reads the file for itself instead of the
// configuration the state carries, which the server loaded once at startup. The
// active profile is left alone: switching to what was saved is a later step the
// operator takes.
//
// It reports genlocal.ErrLocalBuiltinProfile for a name a built-in already
// holds and genlocal.ErrLocalDraftMissing where the registry carries no draft
// under the name. The read and write conditions carry the path that failed,
// since the sentence a caller reads names it.
func (s *BuilderState) DraftSave(draft string) (*genlocal.ProfileDraftSaveResponse, error) {
	if slices.Contains(profiles.BuiltinProfileNames(), draft) {
		return nil, genlocal.ErrLocalBuiltinProfile
	}

	held, found := s.Drafts.Get(draft)
	if !found {
		return nil, genlocal.ErrLocalDraftMissing
	}

	file := config.Path()

	cfg, err := config.Load(file)
	if err != nil {
		return nil, genlocal.NewLocalCause(genlocal.ErrLocalReadFailed,
			fmt.Sprintf("%s: %v", file, err))
	}

	difference := mergeDraftIntoConfig(cfg, draft, held)

	if writeErr := config.WriteAtomic(file, cfg); writeErr != nil {
		return nil, genlocal.NewLocalCause(genlocal.ErrLocalWriteFailed,
			fmt.Sprintf("%s: %v", file, writeErr))
	}

	return draftSaveAnswer(difference), nil
}

// mergeDraftIntoConfig files one draft into a loaded configuration as a user
// profile and answers what it changed against whatever it replaced.
//
// The difference is computed before the write so it describes the profile that
// was there, which is what the caller is shown.
func mergeDraftIntoConfig(
	cfg *config.Config, draft string, held *builder.Draft,
) *builder.Diff {
	saved := builder.DraftAsUserProfile(held)

	var prior *config.UserProfileConfig

	if existing, had := cfg.Profiles[draft]; had {
		prior = &existing
	}

	difference := builder.ComputeDiff(draft, &saved, prior)

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]config.UserProfileConfig, 1)
	}

	cfg.Profiles[draft] = saved

	return difference
}

// draftSaveAnswer is one save difference as the answer the operation fills.
//
// The changed members go in as the values the difference computed. Each is a
// string, a list of them, or a flag, which are the three forms the answer's own
// free-form member takes, so nothing here has a conversion to fail at.
func draftSaveAnswer(difference *builder.Diff) *genlocal.ProfileDraftSaveResponse {
	changed := make(map[string]*genlocal.ProfileFieldDiff, len(difference.ChangedFields))
	for member, change := range difference.ChangedFields {
		changed[member] = genlocal.NewProfileFieldDiff(change.Old, change.New)
	}

	return genlocal.NewProfileDraftSaveResponse(difference.Name, difference.IsNew,
		difference.AddedTools, difference.RemovedTools, changed)
}

// CatalogCategories answers every category the catalog carries with the number
// of tools it covers, sorted by name so two languages answer one order.
func (s *BuilderState) CatalogCategories() *genlocal.ProfileCategoryListResponse {
	entries := s.Catalog()
	counts := make(map[string]int, len(entries))

	for _, entry := range entries {
		for _, category := range entry.Categories {
			counts[category]++
		}
	}

	names := slices.Sorted(maps.Keys(counts))

	items := make([]*genlocal.ProfileCategoryItem, len(names))
	for index, name := range names {
		items[index] = genlocal.NewProfileCategoryItem(name, counts[name])
	}

	return genlocal.NewProfileCategoryListResponse(len(items), items)
}

// CatalogTools answers the tools the catalog carries that the filters admit,
// name-sorted so two languages answer one order whichever binary served the
// call.
//
// The whole registerable surface rather than the active profile's own list: the
// answer is what a profile could be composed from, not what this one already
// permits.
func (s *BuilderState) CatalogTools(category, capability string) *genlocal.ProfileToolListResponse {
	admitted := make([]profiles.ToolDescriptor, 0, len(s.Catalog()))

	for _, entry := range s.Catalog() {
		if catalogAdmits(&entry, category, capability) {
			admitted = append(admitted, entry)
		}
	}

	slices.SortFunc(admitted, func(left, right profiles.ToolDescriptor) int {
		return strings.Compare(left.Name, right.Name)
	})

	items := make([]*genlocal.ProfileToolCatalogItem, len(admitted))
	for index, entry := range admitted {
		items[index] = genlocal.NewProfileToolCatalogItem(
			entry.Name, entry.Capability.String(), entry.Categories,
		)
	}

	return genlocal.NewProfileToolListResponse(len(items), items)
}

// CatalogCanRun answers whether the active profile would permit each call in a
// sequence, so a caller can stop before a partial run strands its user.
//
// It reads each entry's tool name and the environment it would target and
// nothing else: no resource identity, no token scope, no rate limit. The answer
// is advice about the profile, not a plan.
func (s *BuilderState) CatalogCanRun(calls []genlocal.CallEntry) *genlocal.ProfileCanRunResponse {
	profile := s.ActiveProfile()
	registered := catalogCapabilities(s.Catalog())
	permitted := make(map[string]struct{}, len(profile.AllowedTools))

	for _, name := range profile.AllowedTools {
		permitted[name] = struct{}{}
	}

	everyEnvironment := permitsEveryEnvironment(profile.AllowedEnvironments)
	results := make([]*genlocal.ProfileCanRunResult, 0, len(calls))
	blocked := genlocal.CatalogCanRunBuckets()

	var allowed int

	for _, call := range calls {
		verdict := canRunVerdict(&call, registered, permitted,
			profile.AllowedEnvironments, everyEnvironment)
		words := genlocal.CatalogCanRunWords(verdict, call.Tool, registered[call.Tool].String())

		results = append(results, canRunResult(call.Tool, verdict, &words))

		if verdict == genlocal.LocalVerdictPermitted {
			allowed++

			continue
		}

		blocked[words.Bucket]++
	}

	return genlocal.NewProfileCanRunResponse(profile.Name, results,
		genlocal.NewProfileCanRunSummary(
			len(results), allowed, len(results)-allowed, blocked,
		))
}

// canRunVerdict is the verdict one entry meets. The order mirrors real
// dispatch: the name first, then the profile's tool list, then its
// environments.
func canRunVerdict(
	call *genlocal.CallEntry,
	registered map[string]profiles.Capability,
	permitted map[string]struct{},
	environments []string,
	everyEnvironment bool,
) genlocal.LocalVerdict {
	capability, known := registered[call.Tool]
	if !known {
		return genlocal.LocalVerdictUnregistered
	}

	if _, listed := permitted[call.Tool]; !listed {
		// A destroy the profile leaves out counts apart from every other
		// omission, because listing the tool alone cannot unblock it.
		if capability == profiles.CapDestroy {
			return genlocal.LocalVerdictCapabilityBlock
		}

		return genlocal.LocalVerdictProfileBlock
	}

	if call.Environment != "" && !everyEnvironment &&
		!slices.Contains(environments, call.Environment) {
		return genlocal.LocalVerdictEnvironmentBlock
	}

	return genlocal.LocalVerdictPermitted
}

// canRunResult is one entry's verdict as the answer carries it. A permitted
// entry declares both sentences with presence and omits them, since there is
// nothing to say.
func canRunResult(
	tool string, verdict genlocal.LocalVerdict, words *genlocal.LocalVerdictWords,
) *genlocal.ProfileCanRunResult {
	if verdict == genlocal.LocalVerdictPermitted {
		return genlocal.NewProfileCanRunResult(tool, true, nil, nil)
	}

	return genlocal.NewProfileCanRunResult(tool, false, &words.Reason, &words.Remedy)
}

// catalogCapabilities maps every registered tool to its capability, which is
// what lets one pass both spot an unregistered name and annotate a destroy the
// profile leaves out.
func catalogCapabilities(catalog []profiles.ToolDescriptor) map[string]profiles.Capability {
	found := make(map[string]profiles.Capability, len(catalog))
	for _, entry := range catalog {
		found[entry.Name] = entry.Capability
	}

	return found
}

// permitsEveryEnvironment is whether a profile imposes no environment
// restriction: no list at all, or one whose only entry is the wildcard.
func permitsEveryEnvironment(environments []string) bool {
	return len(environments) == 0 ||
		(len(environments) == 1 && environments[0] == environmentWildcard)
}

// environmentWildcard is the entry that permits every configured environment.
// One entry spelled this way and an empty list mean the same thing, which is
// what Profile.AllowedEnvironments already means by them.
const environmentWildcard = "*"

// catalogAdmits is whether one catalog entry answers to both filters, each
// empty filter admitting everything.
func catalogAdmits(entry *profiles.ToolDescriptor, category, capability string) bool {
	if category != "" && !slices.Contains(entry.Categories, category) {
		return false
	}

	return capability == "" || capabilityMatches(entry.Capability, capability)
}

// capabilityMatches is whether one capability tag answers to a filter, in
// either the long spelling the catalog carries or the short one without its
// prefix, case-insensitively, so a caller need not know which the tag uses.
func capabilityMatches(capability profiles.Capability, filter string) bool {
	long := capability.String()
	if strings.EqualFold(long, filter) {
		return true
	}

	short, trimmed := strings.CutPrefix(long, capabilityPrefix)

	return trimmed && strings.EqualFold(short, filter)
}
