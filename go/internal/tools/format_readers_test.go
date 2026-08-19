package tools_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The sentences the three named format readers answer without a declaration
// wording them, spelled once so a row reads as what the caller is told.
const (
	serviceTypeRefusal = "service_type must be a single non-empty service type slug"
	regionRefusal      = "region_id must be a lowercase region slug containing only letters, numbers, and hyphens"
	betaRefusal        = "beta_id must contain only letters, numbers, underscores, and hyphens"
	// The case names the three tables share.
	caseReaderAbsent = "absent"
	caseReaderNumber = "sent as a number"
	caseReaderBlank  = "blank"
	caseReaderPadded = "padded"
	caseReaderTaken  = "accepted"
	// goconst counts these across the package, so the two rows every table
	// carries name them once here.
	caseFormatTraversal = "traversal"
	caseFormatSeparator = "path separator"
	// The argument name the worded-arm cases probe under, short enough that no
	// row's sentence reads as one of the real tools'.
	wordedProbeName = "service"
)

// TestServiceTypeSlugArgumentTellsEveryUnusableSlugApart: the monitor routes
// splice this value into a path segment, so a caller reading the refusal is
// being told which of the three ways their value cannot address a route.
func TestServiceTypeSlugArgumentTellsEveryUnusableSlugApart(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want string
	}{
		{map[string]any{}, caseReaderAbsent, "service_type is required"},
		{map[string]any{monitorServiceTypeParam: float64(1)}, caseReaderNumber, "service_type must be a string"},
		{map[string]any{monitorServiceTypeParam: ""}, caseReaderBlank, serviceTypeRefusal},
		{map[string]any{monitorServiceTypeParam: " dbaas "}, caseReaderPadded, serviceTypeRefusal},
		{map[string]any{monitorServiceTypeParam: "DBaaS"}, "upper case", serviceTypeRefusal},
		{map[string]any{monitorServiceTypeParam: "-dbaas"}, "leading hyphen", serviceTypeRefusal},
		{map[string]any{monitorServiceTypeParam: "dbaas-"}, "trailing hyphen", serviceTypeRefusal},
		{map[string]any{monitorServiceTypeParam: "db/aas"}, caseFormatSeparator, serviceTypeRefusal},
		{map[string]any{monitorServiceTypeParam: "db_aas"}, "underscore", serviceTypeRefusal},
		{map[string]any{monitorServiceTypeParam: "db-aas"}, "interior hyphen", ""},
		{map[string]any{monitorServiceTypeParam: "dbaas"}, caseReaderTaken, ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(testCase.args)

			if _, got := tools.ServiceTypeSlugArgument(&request, monitorServiceTypeParam); got != testCase.want {
				t.Errorf("ServiceTypeSlugArgument = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestRegionSlugArgumentAcceptsExactlyWhatItsRefusalNames: the sentence names
// lowercase letters, numbers, and hyphens, so a hyphen at either end is
// accepted. Refusing one would tell a caller nothing they could read.
func TestRegionSlugArgumentAcceptsExactlyWhatItsRefusalNames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want string
	}{
		{map[string]any{}, caseReaderAbsent, "region_id is required"},
		{map[string]any{keyRegionID: ""}, caseReaderBlank, errRegionIDNonEmpty},
		{map[string]any{keyRegionID: " "}, "spaces", errRegionIDNonEmpty},
		{map[string]any{keyRegionID: 123}, caseReaderNumber, errRegionIDNonEmpty},
		{map[string]any{keyRegionID: "US-EAST"}, "upper case", regionRefusal},
		{map[string]any{keyRegionID: "us east"}, "inner space", regionRefusal},
		{map[string]any{keyRegionID: "us/east"}, caseFormatSeparator, regionRefusal},
		{map[string]any{keyRegionID: ".."}, caseFormatTraversal, regionRefusal},
		{map[string]any{keyRegionID: "us_east"}, "underscore", regionRefusal},
		{map[string]any{keyRegionID: "-us-east-"}, "edge hyphens", ""},
		{map[string]any{keyRegionID: placementGroupCreateRegion}, caseReaderTaken, ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(testCase.args)

			if _, got := tools.RegionSlugArgument(&request, keyRegionID); got != testCase.want {
				t.Errorf("RegionSlugArgument = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestBetaSlugArgumentIsWiderThanTheRegionMemberByCaseAndUnderscore: the two
// members exist separately for exactly this difference, so a row for each of
// the characters that separates them is what proves they are not one member.
func TestBetaSlugArgumentIsWiderThanTheRegionMemberByCaseAndUnderscore(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want string
	}{
		{map[string]any{}, caseReaderAbsent, "beta_id is required"},
		{map[string]any{keyBetaIDPath: " "}, caseReaderBlank, "beta_id must be a non-empty string"},
		{map[string]any{keyBetaIDPath: float64(3)}, caseReaderNumber, "beta_id must be a non-empty string"},
		{map[string]any{keyBetaIDPath: " cloud-firewall"}, caseReaderPadded, betaRefusal},
		{map[string]any{keyBetaIDPath: "beta/one"}, caseFormatSeparator, betaRefusal},
		{map[string]any{keyBetaIDPath: ".."}, caseFormatTraversal, betaRefusal},
		{map[string]any{keyBetaIDPath: "Cloud_Firewall"}, "case and underscore", ""},
		{map[string]any{keyBetaIDPath: "cloud-firewall"}, caseReaderTaken, ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(testCase.args)

			if _, got := tools.BetaSlugArgument(&request, keyBetaIDPath); got != testCase.want {
				t.Errorf("BetaSlugArgument = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestFormatReadersAnswerTheSentencesADeclarationWords: the member is what it
// accepts and the words are the declaration's, so a worded arm has to reach the
// caller in place of the member's own sentence.
func TestFormatReadersAnswerTheSentencesADeclarationWords(t *testing.T) {
	t.Parallel()

	request := requestFor(map[string]any{})
	if _, got := tools.DeclaredRegionSlug(&request, "region", "no region named", "", ""); got != "no region named" {
		t.Errorf("DeclaredRegionSlug absent = %q, want the worded arm", got)
	}

	request = requestFor(map[string]any{wordedProbeName: 7})
	if _, got := tools.DeclaredServiceTypeSlug(&request, wordedProbeName, "", "service is text", ""); got != "service is text" {
		t.Errorf("DeclaredServiceTypeSlug unusable = %q, want the worded arm", got)
	}

	request = requestFor(map[string]any{"program": "a b"})
	if _, got := tools.DeclaredBetaSlug(&request, "program", "", "", "program is a slug"); got != "program is a slug" {
		t.Errorf("DeclaredBetaSlug refused = %q, want the worded arm", got)
	}
}
