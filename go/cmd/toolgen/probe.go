package main

import (
	"maps"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// The seams this command's own tests drive it through.
//
// Every refusal in errors.go answers a declaration, and this repo's contract
// declares none of them: the proto is the surface that ships, so a message
// shaped to trip a check cannot live there. The tests synthesize those
// declarations instead and run them through here.
//
// Exported because a main package is importable only from the external test
// package beside it, so nothing outside this command can reach these. That is
// what makes them narrower than the alternative: a flag on the binary would be
// real surface added for one caller that never ships.

// ProbeRun is one synthesized declaration, run the way `make proto` runs the
// real ones: the contract build, then a renderer arm per language.
type ProbeRun struct {
	// Message is the synthesized input message, carrying the tool's whole
	// declaration the same way every *Input message in the contract does.
	Message protoreflect.MessageDescriptor
	// Beside is the rest of the surface this run sees, which is where a
	// removal finds its sibling state read. Usually empty.
	Beside map[string]protoreflect.MessageDescriptor
	// Name is the tool the message declares, which the route or the meta
	// marker on it names.
	Name string
	// Schemas is the directory the argument descriptions are read from. A
	// probe usually points it nowhere, since a missing schema is not an error
	// and an undescribed argument renders fine.
	Schemas string
	// PyOut and Ruff turn the Python arm on. It renders through the repo's
	// formatter, so a probe whose subject is a contract or Go refusal leaves
	// them empty and runs the Go arm alone.
	PyOut string
	Ruff  string
}

// Emit answers the files the declaration renders to, keyed by language and file
// name, or the refusal it raised on the way there.
func (p *ProbeRun) Emit() (map[string]string, error) {
	declared := map[string]protoreflect.MessageDescriptor{p.Name: p.Message}
	maps.Copy(declared, p.Beside)

	built, err := buildContract(p.Name, p.Message, newSchemaDocs(p.Schemas))
	if err != nil {
		return nil, err
	}

	if readErr := built.resolveStateRead(declared); readErr != nil {
		return nil, readErr
	}

	return p.render([]contract{built})
}

// render runs the built contract through each arm this probe turned on.
func (p *ProbeRun) render(contracts []contract) (map[string]string, error) {
	arms := []renderer{goRenderer{}}
	if p.Ruff != "" {
		arms = append(arms, pyRenderer{outDir: p.PyOut, ruff: p.Ruff})
	}

	files := make(map[string]string, len(arms)*2)

	for _, arm := range arms {
		rendered, err := renderTree(arm, contracts)
		if err != nil {
			return nil, err
		}

		for name, text := range rendered {
			files[arm.language()+"/"+name] = text
		}
	}

	return files, nil
}

// ProbeClaim is one claim a probe arm makes about one contract option, spelled
// in plain strings so a test can state what no real arm would.
type ProbeClaim struct {
	Option  string
	Home    string
	Emitted bool
}

// ProbeCoverage holds one probe arm's claims to the options the contract
// declares, which is the check every real arm passes on every run.
func ProbeCoverage(language string, claims []ProbeClaim) error {
	options, err := contractOptions()
	if err != nil {
		return err
	}

	stated := make([]optionClaim, 0, len(claims))
	for _, claim := range claims {
		stated = append(stated, optionClaim{
			option:  protoreflect.Name(claim.Option),
			home:    claim.Home,
			emitted: claim.Emitted,
		})
	}

	return checkArmCoverage(language, stated, options)
}

// ProbeArmClaims is what one registered language answers for, so a test can
// hold each claim to the tree the arm renders and to the file it names.
func ProbeArmClaims(language string) ([]ProbeClaim, error) {
	arm, err := armFor(language, &runPaths{goOut: ".", pyOut: ".", ruff: "."})
	if err != nil {
		return nil, err
	}

	claims := make([]ProbeClaim, 0, len(arm.lang.acts()))
	for _, claim := range arm.lang.acts() {
		claims = append(claims, ProbeClaim{
			Option:  string(claim.option),
			Home:    claim.home,
			Emitted: claim.emitted,
		})
	}

	return claims, nil
}

// ProbeNormalizeCalls is the support-layer function each declared transform
// renders to, keyed by the enum member it belongs to and holding the Go name
// beside the Python one. A test holds this to the enum the contract declares,
// which is what keeps a member from shipping with an arm missing.
func ProbeNormalizeCalls() map[string][]string {
	rendering := normalizeRendering()

	calls := make(map[string][]string, len(rendering))
	for transform, call := range rendering {
		calls[transform.String()] = []string{call.Go, call.Python}
	}

	return calls
}

// ProbeRefusals is every refusal this emitter can raise, by the name it is
// declared under. A test holds a synthesized declaration to the error itself
// rather than to its wording, and the accounting test holds this table to
// errors.go in both directions, so a refusal cannot be added or removed without
// this list following it.
func ProbeRefusals() map[string]error {
	return map[string]error{
		"errToolNotDeclared":               errToolNotDeclared,
		"errHandwrittenNotDeclared":        errHandwrittenNotDeclared,
		"errNoLanguages":                   errNoLanguages,
		"errNoRendererArm":                 errNoRendererArm,
		"errNoOutputDir":                   errNoOutputDir,
		"errArmMisnamed":                   errArmMisnamed,
		"errNoContractOptions":             errNoContractOptions,
		"errUnclaimedOption":               errUnclaimedOption,
		"errUnknownClaim":                  errUnknownClaim,
		"errRepeatedClaim":                 errRepeatedClaim,
		"errSilentClaim":                   errSilentClaim,
		"errEmptyCohort":                   errEmptyCohort,
		"errMetaNotEmitted":                errMetaNotEmitted,
		"errNoDescription":                 errNoDescription,
		"errNoResponse":                    errNoResponse,
		"errNoErrorMessage":                errNoErrorMessage,
		"errNoSuccessMessage":              errNoSuccessMessage,
		"errNoConfirmMessage":              errNoConfirmMessage,
		"errUnusedSuccessMessage":          errUnusedSuccessMessage,
		"errNoWarningMessage":              errNoWarningMessage,
		"errUnusedWarningMessage":          errUnusedWarningMessage,
		"errNotAWriteEnvelope":             errNotAWriteEnvelope,
		"errNotAWritePage":                 errNotAWritePage,
		"errNotABodyRead":                  errNotABodyRead,
		"errNoMarkerCursor":                errNoMarkerCursor,
		"errGatedBodyRead":                 errGatedBodyRead,
		"errUnsupportedBodyKind":           errUnsupportedBodyKind,
		"errUnsupportedItemKind":           errUnsupportedItemKind,
		"errRecursiveItemMessage":          errRecursiveItemMessage,
		"errNoFieldLocation":               errNoFieldLocation,
		"errFoldNotPlaced":                 errFoldNotPlaced,
		"errRoutedToolArgument":            errRoutedToolArgument,
		"errRedactNotBody":                 errRedactNotBody,
		"errBodyNameNotBody":               errBodyNameNotBody,
		"errBodyNameNoop":                  errBodyNameNoop,
		"errNullableNotBody":               errNullableNotBody,
		"errNullableNotObject":             errNullableNotObject,
		"errBodyRootNotObject":             errBodyRootNotObject,
		"errBodyRootNotAlone":              errBodyRootNotAlone,
		"errReaderOnDestroy":               errReaderOnDestroy,
		"errUnsupportedReader":             errUnsupportedReader,
		"errPresentNotBody":                errPresentNotBody,
		"errReaderNotPath":                 errReaderNotPath,
		"errReaderNotPathText":             errReaderNotPathText,
		"errEnumMemberPlacement":           errEnumMemberPlacement,
		"errEnumMemberRepeated":            errEnumMemberRepeated,
		"errEnumMemberKind":                errEnumMemberKind,
		"errEnumMemberNoValues":            errEnumMemberNoValues,
		"errPresentTextShape":              errPresentTextShape,
		"errPresentBoolShape":              errPresentBoolShape,
		"errPresentStringShape":            errPresentStringShape,
		"errIDListShape":                   errIDListShape,
		"errReaderWithValidate":            errReaderWithValidate,
		"errRefuseArguments":               errRefuseArguments,
		"errRefuseWithValidate":            errRefuseWithValidate,
		"errRefuseUnknownWithValidate":     errRefuseUnknownWithValidate,
		"errRefuseDeclaredField":           errRefuseDeclaredField,
		"errAnyOfTooFew":                   errAnyOfTooFew,
		"errAnyOfUnknownField":             errAnyOfUnknownField,
		"errAnyOfDuplicateField":           errAnyOfDuplicateField,
		"errAnyOfWithValidate":             errAnyOfWithValidate,
		"errNormalizeWithHook":             errNormalizeWithHook,
		"errNormalizeNoField":              errNormalizeNoField,
		"errNormalizeUnknownField":         errNormalizeUnknownField,
		"errNormalizeRepeatedField":        errNormalizeRepeatedField,
		"errNormalizeSecretField":          errNormalizeSecretField,
		"errUnsupportedTransform":          errUnsupportedTransform,
		"errWalkWithValidate":              errWalkWithValidate,
		"errWalkTier":                      errWalkTier,
		"errWalkNoField":                   errWalkNoField,
		"errWalkUnknownField":              errWalkUnknownField,
		"errWalkRepeatedField":             errWalkRepeatedField,
		"errWalkSilent":                    errWalkSilent,
		"errWalkElementOnMap":              errWalkElementOnMap,
		"errWalkEmptyOnList":               errWalkEmptyOnList,
		"errWalkValueWithoutKey":           errWalkValueWithoutKey,
		"errWalkDuplicateKey":              errWalkDuplicateKey,
		"errWalkUnknownWithoutVocabulary":  errWalkUnknownWithoutVocabulary,
		"errWalkRequireUnknownName":        errWalkRequireUnknownName,
		"errWalkBoundKind":                 errWalkBoundKind,
		"errWalkMembersKind":               errWalkMembersKind,
		"errWalkUnknownEnum":               errWalkUnknownEnum,
		"errWalkPlaceholder":               errWalkPlaceholder,
		"errAnyOfTier":                     errAnyOfTier,
		"errReaderValuesOnEnum":            errReaderValuesOnEnum,
		"errReaderValuesAlone":             errReaderValuesAlone,
		"errReaderMessageAlone":            errReaderMessageAlone,
		"errReaderMessageArm":              errReaderMessageArm,
		"errReaderNotPathInteger":          errReaderNotPathInteger,
		"errCommaListNotBody":              errCommaListNotBody,
		"errCommaListNotString":            errCommaListNotString,
		"errEchoArgumentShape":             errEchoArgumentShape,
		"errEchoArgumentNoop":              errEchoArgumentNoop,
		"errNoGoType":                      errNoGoType,
		"errAmbiguousEnvelope":             errAmbiguousEnvelope,
		"errUnmatchedFilter":               errUnmatchedFilter,
		"errUnsupportedTier":               errUnsupportedTier,
		"errPyRender":                      errPyRender,
		"errPyFormat":                      errPyFormat,
		"errPyLineBudget":                  errPyLineBudget,
		"errPathArity":                     errPathArity,
		"errUnsupportedPathKind":           errUnsupportedPathKind,
		"errUnsupportedQuery":              errUnsupportedQuery,
		"errNoReadPreview":                 errNoReadPreview,
		"errNoStructFailPrefix":            errNoStructFailPrefix,
		"errNoNormalizePoint":              errNoNormalizePoint,
		"errNotAWrapper":                   errNotAWrapper,
		"errUnknownPlaceholder":            errUnknownPlaceholder,
		"errRepeatedPlaceholder":           errRepeatedPlaceholder,
		"errNotRepeated":                   errNotRepeated,
		"errUngatedExecute":                errUngatedExecute,
		"errNotAnAssembledRead":            errNotAnAssembledRead,
		"errUnknownHookKind":               errUnknownHookKind,
		"errRepeatedHookKind":              errRepeatedHookKind,
		"errNotADeleteEnvelope":            errNotADeleteEnvelope,
		"errNoFetchState":                  errNoFetchState,
		"errStateRouteWithHook":            errStateRouteWithHook,
		"errStateRouteWithPreviewHook":     errStateRouteWithPreviewHook,
		"errStateRouteWithNoReader":        errStateRouteWithNoReader,
		"errStateRouteUnknownTool":         errStateRouteUnknownTool,
		"errStateRouteNotRead":             errStateRouteNotRead,
		"errStateRouteNoResponse":          errStateRouteNoResponse,
		"errStateRouteUnknownSlot":         errStateRouteUnknownSlot,
		"errStateRouteSlotUnknownRead":     errStateRouteSlotUnknownRead,
		"errStateRouteSlotUnknownArgument": errStateRouteSlotUnknownArgument,
		"errStateRouteSlotSameName":        errStateRouteSlotSameName,
		"errStateRouteUnfilledQuery":       errStateRouteUnfilledQuery,
		"errStateRouteQueryAmbiguous":      errStateRouteQueryAmbiguous,
		"errStateRouteQuerySecret":         errStateRouteQuerySecret,
		"errStateRoutePayloadMember":       errStateRoutePayloadMember,
		"errStateRoutePayloadWithBodyKey":  errStateRoutePayloadWithBodyKey,

		"errPreviewSentenceWithHook":        errPreviewSentenceWithHook,
		"errPreviewSentenceNoTemplate":      errPreviewSentenceNoTemplate,
		"errPreviewSentenceNoLine":          errPreviewSentenceNoLine,
		"errPreviewSentenceUnclosed":        errPreviewSentenceUnclosed,
		"errPreviewSentenceUnknownArgument": errPreviewSentenceUnknownArgument,
		"errPreviewSentenceUnreportable":    errPreviewSentenceUnreportable,
		"errPreviewSentenceSecretArgument":  errPreviewSentenceSecretArgument,
		"errPreviewSentenceUnreachable":     errPreviewSentenceUnreachable,
		"errPreviewChoiceWithTemplate":      errPreviewChoiceWithTemplate,
		"errPreviewChoiceNoArgument":        errPreviewChoiceNoArgument,
		"errPreviewChoiceNoWording":         errPreviewChoiceNoWording,
		"errPreviewChoiceUnknownArgument":   errPreviewChoiceUnknownArgument,
		"errPreviewChoiceNotFlag":           errPreviewChoiceNotFlag,
		"errPreviewChoiceNotObject":         errPreviewChoiceNotObject,
		"errPreviewChoiceDeepMember":        errPreviewChoiceDeepMember,
		"errPreviewOmitsBodyWithHook":       errPreviewOmitsBodyWithHook,
		"errPreviewOmitsBodyWithoutBody":    errPreviewOmitsBodyWithoutBody,
		"errPartialTwoStage":                errPartialTwoStage,
		"errNoTwoStageDriver":               errNoTwoStageDriver,
		"errUnstagedHook":                   errUnstagedHook,
		"errNoNullPayload":                  errNoNullPayload,
		"errUnknownNullField":               errUnknownNullField,
		"errUnsupportedDestroyShape":        errUnsupportedDestroyShape,
		"errUnplacedLocalMember":            errUnplacedLocalMember,
		"errUnknownResponseBody":            errUnknownResponseBody,
		"errUnusedResponseBody":             errUnusedResponseBody,
		"errEmptyDefault":                   errEmptyDefault,
		"errModifiedDefault":                errModifiedDefault,
		"errDefaultNotToolArg":              errDefaultNotToolArg,
		"errUnsupportedToolArg":             errUnsupportedToolArg,
		"errUnreadableDefault":              errUnreadableDefault,
		"errMetaDryRun":                     errMetaDryRun,
		"errNoMetaAnswer":                   errNoMetaAnswer,
		"errMetaTwoAnswers":                 errMetaTwoAnswers,
		"errNoMetaMessageField":             errNoMetaMessageField,
		"errUngatedAnswer":                  errUngatedAnswer,
		"errUnservedMetaHook":               errUnservedMetaHook,
		"errNullsOverDecodedResponse":       errNullsOverDecodedResponse,
	}
}
