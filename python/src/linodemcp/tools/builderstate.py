"""Ambient builder-state access for the profile-builder tools.

Python tool handlers take ``(arguments, config)`` with no context parameter, so
the server publishes the state the builder tools read through a ``ContextVar``
they read back. This mirrors the Go side's context injection
(``WithBuilderState`` / ``BuilderStateFromContext`` in ``internal/tools``) and
the two-stage plan store next to it.

The state carries the draft registry the tools mutate, the catalog they compose
a profile against, and the active profile a pre-check answers for. Catalog and
active_profile are callables read at call time rather than snapshots, so a
profile reload reaches an already-registered tool.
"""

from __future__ import annotations

from contextvars import ContextVar
from dataclasses import dataclass
from typing import TYPE_CHECKING

from linodemcp.config import (
    ConfigError,
    get_config_path,
    load_from_file,
    write_atomic,
)
from linodemcp.genlocal import (
    CallEntry,
    LocalAlreadyExistsError,
    LocalBuiltinProfileError,
    LocalDraftMissingError,
    LocalNotFoundError,
    LocalReadFailedError,
    LocalVerdict,
    LocalVerdictWords,
    LocalWriteFailedError,
    ProfileCanRunResponse,
    ProfileCanRunResult,
    ProfileCanRunSummary,
    ProfileCategoryItem,
    ProfileCategoryListResponse,
    ProfileDraftAddToolsResponse,
    ProfileDraftDiscardResponse,
    ProfileDraftRemoveToolsResponse,
    ProfileDraftResponse,
    ProfileDraftSaveResponse,
    ProfileDraftSetResponse,
    ProfileFieldDiff,
    ProfileToolCatalogItem,
    ProfileToolListResponse,
    catalog_can_run_buckets,
    catalog_can_run_words,
)
from linodemcp.profiles import CAPABILITY_PREFIX, Capability, capability_spelling
from linodemcp.profiles.builder import (
    DraftExistsError,
    DraftNotFoundError,
    compute_diff,
    draft_as_user_profile,
)
from linodemcp.profiles.builtin import builtin_profile_names
from linodemcp.profiles.loader import lookup_profile

if TYPE_CHECKING:
    from collections.abc import Callable
    from contextvars import Token
    from typing import Any

    from linodemcp.config import Config
    from linodemcp.profiles import Profile
    from linodemcp.profiles.builder import Diff, Draft, Registry
    from linodemcp.profiles.builtin import ToolDescriptor


# The sentence every builder tool answers when it was called with no state
# attached. Go answers the same words.
BUILDER_UNCONFIGURED = "draft registry not configured"


def draft_answer(draft: Draft) -> ProfileDraftResponse:
    """One draft as the answer a draft operation fills."""
    return ProfileDraftResponse(
        name=draft.name,
        description=draft.description,
        allowed_tools=list(draft.allowed_tools),
        allowed_environments=list(draft.allowed_environments),
        required_token_scopes=list(draft.required_token_scopes),
        allow_yolo=draft.allow_yolo,
    )


def _draft_save_answer(difference: Diff) -> ProfileDraftSaveResponse:
    """One save difference as the answer the operation fills.

    The changed members go over as the values the difference computed. Each is
    a string, a list of them, or a flag, which are the three forms the answer's
    own free-form member takes.
    """
    return ProfileDraftSaveResponse(
        name=difference.name,
        is_new=difference.is_new,
        added_tools=list(difference.added_tools),
        removed_tools=list(difference.removed_tools),
        changed_fields={
            member: ProfileFieldDiff(old=change.old, new=change.new)
            for member, change in difference.changed_fields.items()
        },
    )


def capability_matches(capability: Capability, wanted: str) -> bool:
    """Whether one capability tag answers to a filter.

    In either the long spelling the catalog carries or the short one without
    its prefix, case-insensitively, so a caller need not know which the tag
    uses.
    """
    long = capability_spelling(capability)
    short = long.removeprefix(CAPABILITY_PREFIX)

    return wanted.lower() in {short.lower(), long.lower()}


@dataclass(frozen=True)
class BuilderState:
    """What the profile-builder tools read off the call.

    The methods below are the local operations the builder state serves. Each
    takes the values its declaration names and answers the shape it declares,
    so none of them can tell which tool called it.
    """

    drafts: Registry
    catalog: Callable[[], list[ToolDescriptor]]
    active_profile: Callable[[], Profile]
    config: Config

    def draft_create(self, draft: str, source: str) -> ProfileDraftResponse:
        """File a new draft, seeded from the profile a source names.

        Raises LocalNotFoundError where the source resolves to no profile and
        LocalAlreadyExistsError where the name is already filed. The seed is
        looked up across both the configuration's own profiles and the
        built-ins, which is why the state carries the configuration rather than
        taking it per call.
        """
        seed: Profile | None = None

        if source:
            seed = lookup_profile(source, self.config, self.catalog())
            if seed is None:
                raise LocalNotFoundError

        try:
            created = self.drafts.create(draft, seed)
        except DraftExistsError as exc:
            raise LocalAlreadyExistsError from exc

        return draft_answer(created)

    def draft_set(
        self,
        draft: str,
        environments: list[str] | None,
        scopes: list[str] | None,
        yolo: bool | None,
    ) -> ProfileDraftSetResponse:
        """Write every setting the call sent and answer which ones moved.

        ``None`` is what carries the difference: a list sent empty clears a
        setting, and an absent one is not an instruction at all.
        """
        changes: dict[str, Any] = {}

        try:
            if environments is not None:
                self.drafts.set_allowed_environments(draft, environments)
                changes["allowed_environments"] = environments

            if scopes is not None:
                self.drafts.set_required_token_scopes(draft, scopes)
                changes["required_token_scopes"] = scopes

            if yolo is not None:
                self.drafts.set_allow_yolo(draft, yolo)
                changes["allow_yolo"] = yolo
        except DraftNotFoundError as exc:
            raise LocalDraftMissingError from exc

        return ProfileDraftSetResponse(name=draft, changes=changes)

    def draft_tools_add(
        self, draft: str, patterns: list[str]
    ) -> ProfileDraftAddToolsResponse:
        """Write the tool names a draft allows and answer the ones that moved.

        Raises LocalDraftMissingError where the registry holds no draft under
        the name. The patterns expand against the live catalog, so a wildcard
        picks up tools registered after the draft was made. That is what
        separates this side from the removing one, which matches the draft's
        own list.
        """
        try:
            added = self.drafts.add_tools(draft, patterns, self.catalog())
        except DraftNotFoundError as exc:
            raise LocalDraftMissingError from exc

        return ProfileDraftAddToolsResponse(name=draft, added=added)

    def draft_tools_remove(
        self, draft: str, patterns: list[str]
    ) -> ProfileDraftRemoveToolsResponse:
        """Take tool names off a draft and answer the ones that moved.

        Raises LocalDraftMissingError the same way the adding side does.
        """
        try:
            removed = self.drafts.remove_tools(draft, patterns)
        except DraftNotFoundError as exc:
            raise LocalDraftMissingError from exc

        return ProfileDraftRemoveToolsResponse(name=draft, removed=removed)

    def draft_read(self, draft: str) -> ProfileDraftResponse:
        """Answer one draft's current state, raising LocalDraftMissingError."""
        found = self.drafts.get(draft)
        if found is None:
            raise LocalDraftMissingError

        return draft_answer(found)

    def draft_discard(self, draft: str) -> ProfileDraftDiscardResponse:
        """Remove a draft and answer whether one was there.

        Answering the miss rather than reporting it is what lets a caller run
        this from a cleanup path without checking first.
        """
        return ProfileDraftDiscardResponse(
            name=draft, discarded=self.drafts.discard(draft)
        )

    def draft_save(self, draft: str) -> ProfileDraftSaveResponse:
        """Write one draft into the configuration file and answer the difference.

        Raises LocalBuiltinProfileError for a name a built-in already holds and
        LocalDraftMissingError where the registry carries none under the name.
        The read and write conditions carry the path that failed, since the
        sentence a caller reads names it.

        The file is re-read on every call so a concurrent edit is not stomped,
        and the path is read at call time so an override applies to the call
        rather than to the process. That is why this reads the file for itself
        instead of the configuration the state carries, which the server loaded
        once at startup. The active profile is left alone: switching to what was
        saved is a later step the operator takes.
        """
        if draft in builtin_profile_names():
            raise LocalBuiltinProfileError

        held = self.drafts.get(draft)
        if held is None:
            raise LocalDraftMissingError

        file = get_config_path()

        try:
            cfg = load_from_file(file)
        except (ConfigError, OSError) as exc:
            raise LocalReadFailedError(f"{file}: {exc}") from exc

        saved = draft_as_user_profile(held)
        difference = compute_diff(draft, saved, cfg.profiles.get(draft))
        cfg.profiles[draft] = saved

        try:
            write_atomic(file, cfg)
        except (ConfigError, OSError) as exc:
            raise LocalWriteFailedError(f"{file}: {exc}") from exc

        return _draft_save_answer(difference)

    def catalog_categories(self) -> ProfileCategoryListResponse:
        """Answer every category the catalog carries with the tools it covers.

        Sorted by name so two languages answer one order.
        """
        counts: dict[str, int] = {}

        for entry in self.catalog():
            for category in entry.categories:
                counts[category] = counts.get(category, 0) + 1

        items = [
            ProfileCategoryItem(name=name, tool_count=counts[name])
            for name in sorted(counts)
        ]

        return ProfileCategoryListResponse(count=len(items), categories=items)

    def catalog_tools(self, category: str, capability: str) -> ProfileToolListResponse:
        """Answer the tools the catalog carries that the filters admit.

        Name-sorted so two languages answer one order whichever binary served
        the call, and over the whole registerable surface rather than the active
        profile's own list: the answer is what a profile could be composed from,
        not what this one already permits.
        """
        admitted = [
            entry
            for entry in self.catalog()
            if (not category or category in entry.categories)
            and (not capability or capability_matches(entry.capability, capability))
        ]
        admitted.sort(key=lambda entry: entry.name)

        items = [
            ProfileToolCatalogItem(
                name=entry.name,
                capability=capability_spelling(entry.capability),
                categories=list(entry.categories),
            )
            for entry in admitted
        ]

        return ProfileToolListResponse(count=len(items), tools=items)

    def catalog_can_run(self, calls: list[CallEntry]) -> ProfileCanRunResponse:
        """Answer whether the active profile would permit each call in a sequence.

        A caller can stop before a partial run strands its user. Each entry's
        tool name and the environment it would target are all that is read: no
        resource identity, no token scope, no rate limit. The answer is advice
        about the profile, not a plan.
        """
        profile = self.active_profile()
        registered = {entry.name: entry.capability for entry in self.catalog()}
        permitted = set(profile.allowed_tools)
        environments = list(profile.allowed_environments)
        every_environment = _permits_every_environment(environments)

        results: list[ProfileCanRunResult] = []
        blocked = catalog_can_run_buckets()
        allowed = 0

        for call in calls:
            verdict = _can_run_verdict(
                call,
                registered=registered,
                permitted=permitted,
                environments=environments,
                every_environment=every_environment,
            )
            spelled = registered.get(call.tool)
            words = catalog_can_run_words(
                verdict,
                call.tool,
                "" if spelled is None else capability_spelling(spelled),
            )
            results.append(_can_run_result(call.tool, verdict, words))

            if verdict is LocalVerdict.PERMITTED:
                allowed += 1
            else:
                blocked[words.bucket] += 1

        return ProfileCanRunResponse(
            active_profile=profile.name,
            results=results,
            summary=ProfileCanRunSummary(
                total=len(results),
                allowed=allowed,
                blocked=len(results) - allowed,
                blocked_by_reason=blocked,
            ),
        )


def _can_run_verdict(
    call: CallEntry,
    *,
    registered: dict[str, Capability],
    permitted: set[str],
    environments: list[str],
    every_environment: bool,
) -> LocalVerdict:
    """The verdict one entry meets.

    The order mirrors real dispatch: the name first, then the profile's tool
    list, then its environments.
    """
    capability = registered.get(call.tool)
    if capability is None:
        return LocalVerdict.UNREGISTERED

    if call.tool not in permitted:
        # A destroy the profile leaves out counts apart from every other
        # omission, because listing the tool alone cannot unblock it.
        if capability is Capability.Destroy:
            return LocalVerdict.CAPABILITY_BLOCK

        return LocalVerdict.PROFILE_BLOCK

    unlisted = call.environment not in environments
    if call.environment and not every_environment and unlisted:
        return LocalVerdict.ENVIRONMENT_BLOCK

    return LocalVerdict.PERMITTED


def _can_run_result(
    tool: str, verdict: LocalVerdict, words: LocalVerdictWords
) -> ProfileCanRunResult:
    """One entry's verdict as the answer carries it.

    A permitted entry declares both sentences with presence and omits them,
    since there is nothing to say.
    """
    if verdict is LocalVerdict.PERMITTED:
        return ProfileCanRunResult(tool=tool, allowed=True, reason=None, remedy=None)

    return ProfileCanRunResult(
        tool=tool, allowed=False, reason=words.reason, remedy=words.remedy
    )


def _permits_every_environment(environments: list[str]) -> bool:
    """Whether a profile imposes no environment restriction.

    No list at all, or one whose only entry is the wildcard.
    """
    return len(environments) == 0 or environments == [_ENVIRONMENT_WILDCARD]


# The entry that permits every configured environment. One entry spelled this
# way and an empty list mean the same thing.
_ENVIRONMENT_WILDCARD = "*"


_BUILDER_STATE: ContextVar[BuilderState | None] = ContextVar(
    "builder_state", default=None
)


def set_builder_state(state: BuilderState | None) -> Token[BuilderState | None]:
    """Publish the state the builder tools read, returning a resetting token."""
    return _BUILDER_STATE.set(state)


def reset_builder_state(token: Token[BuilderState | None]) -> None:
    """Restore the state the matching set_builder_state call replaced."""
    _BUILDER_STATE.reset(token)


def builder_state_from_context() -> BuilderState | None:
    """Return the state the server published, or None when unset.

    None means no server is behind the handler (a unit test calling it
    directly), so the tool refuses rather than answering from nothing.
    """
    return _BUILDER_STATE.get()
