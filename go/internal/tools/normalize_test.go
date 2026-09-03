package tools_test

import (
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The declared transforms, held to what the per-tool bodies they replaced did:
// text is rewritten in place, anything else is left for the check that words it
// by type, and an argument nobody sent stays absent.

const (
	keyNormalizeLabel   = "label"
	keyNormalizeImages  = "images"
	keyNormalizeSummary = "summary"
	paddedDebian12      = "  linode/debian12  "
	keyFoldSource       = "linode_ids"
	keyFoldTarget       = "entities"
	caseAbsentArgument  = "an absent argument is not invented"
)

func TestTrimArgumentsRewritesEveryNamedTextValue(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arguments map[string]any
		want      map[string]any
	}{
		"padded values": {
			arguments: map[string]any{keyNormalizeLabel: "  web  ", managedServiceAddressParam: "\t10.0.0.1\n"},
			want:      map[string]any{keyNormalizeLabel: tagWeb, managedServiceAddressParam: "10.0.0.1"},
		},
		"spaces alone become the empty string the required rule refuses": {
			arguments: map[string]any{keyNormalizeLabel: blankString},
			want:      map[string]any{keyNormalizeLabel: ""},
		},
		"a value that is not text is left for the type refusal": {
			arguments: map[string]any{keyNormalizeLabel: 5, managedServiceAddressParam: nil},
			want:      map[string]any{keyNormalizeLabel: 5, managedServiceAddressParam: nil},
		},
		caseAbsentArgument: {
			arguments: map[string]any{"weight": 100},
			want:      map[string]any{"weight": 100},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(test.arguments)
			tools.TrimArguments(&request, keyNormalizeLabel, managedServiceAddressParam)

			if got := request.GetArguments(); !reflect.DeepEqual(got, test.want) {
				t.Errorf("arguments = %v, want %v", got, test.want)
			}
		})
	}
}

func TestTrimListDropBlankRewritesOnlyAllTextLists(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arguments map[string]any
		want      map[string]any
	}{
		"entries are trimmed and the blanks dropped": {
			arguments: map[string]any{keyNormalizeImages: []any{paddedDebian12, blankString, "\tprivate/1"}},
			want:      map[string]any{keyNormalizeImages: []any{testDebian12Image, "private/1"}},
		},
		"a list of blanks answers the empty list": {
			arguments: map[string]any{keyNormalizeImages: []any{blankWhitespace}},
			want:      map[string]any{keyNormalizeImages: []any{}},
		},
		"one entry that is not text leaves the whole list alone": {
			arguments: map[string]any{keyNormalizeImages: []any{paddedDebian12, 5}},
			want:      map[string]any{keyNormalizeImages: []any{paddedDebian12, 5}},
		},
		"a value that is not a list is left for the type refusal": {
			arguments: map[string]any{keyNormalizeImages: testDebian12Image},
			want:      map[string]any{keyNormalizeImages: testDebian12Image},
		},
		caseAbsentArgument: {
			arguments: map[string]any{keyNormalizeLabel: tagWeb},
			want:      map[string]any{keyNormalizeLabel: tagWeb},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(test.arguments)
			tools.TrimListDropBlank(&request, keyNormalizeImages)

			if got := request.GetArguments(); !reflect.DeepEqual(got, test.want) {
				t.Errorf("arguments = %v, want %v", got, test.want)
			}
		})
	}
}

// Both transforms take a name list, so a tool declaring one over several
// arguments gets one call rather than one per argument.
func TestTransformsRewriteEveryNameTheyAreGiven(t *testing.T) {
	t.Parallel()

	request := requestFor(map[string]any{
		keyNormalizeSummary: "  down  ",
		keyDescription:      "  since 3am  ",
		keyNormalizeImages:  []any{"  a  "},
		keyReservedIPTags:   []any{"  b  "},
	})

	tools.TrimArguments(&request, keyNormalizeSummary, keyDescription)
	tools.TrimListDropBlank(&request, keyNormalizeImages, keyReservedIPTags)

	want := map[string]any{
		keyNormalizeSummary: "down",
		keyDescription:      "since 3am",
		keyNormalizeImages:  []any{"a"},
		keyReservedIPTags:   []any{"b"},
	}

	if got := request.GetArguments(); !reflect.DeepEqual(got, want) {
		t.Errorf("arguments = %v, want %v", got, want)
	}
}

func TestTrimListTrimsEntriesAndKeepsBlanks(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arguments map[string]any
		want      map[string]any
	}{
		"padded entries are trimmed in place": {
			arguments: map[string]any{keyNormalizeImages: []any{"  us-east  ", "eu-west"}},
			want:      map[string]any{keyNormalizeImages: []any{placementGroupCreateRegion, "eu-west"}},
		},
		"an entry that was only padding stays as the blank the rule refuses": {
			arguments: map[string]any{keyNormalizeImages: []any{blankString, placementGroupCreateRegion}},
			want:      map[string]any{keyNormalizeImages: []any{"", placementGroupCreateRegion}},
		},
		"non-text entries are left for the type refusal": {
			arguments: map[string]any{keyNormalizeImages: []any{" us-east ", 3}},
			want:      map[string]any{keyNormalizeImages: []any{placementGroupCreateRegion, 3}},
		},
		"a value that is not a list is left alone": {
			arguments: map[string]any{keyNormalizeImages: placementGroupCreateRegion},
			want:      map[string]any{keyNormalizeImages: placementGroupCreateRegion},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(test.arguments)
			tools.TrimList(&request, keyNormalizeImages)

			if got := request.GetArguments(); !reflect.DeepEqual(got, test.want) {
				t.Errorf("arguments = %v, want %v", got, test.want)
			}
		})
	}
}

func TestFoldIntListFoldsDropsAndRefusesLikeTheHookItReplaced(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		arguments map[string]any
		want      map[string]any
	}{
		"the convenience form folds into the target member and leaves": {
			arguments: map[string]any{keyFoldSource: []any{float64(123), float64(456)}},
			want:      map[string]any{keyFoldTarget: map[string]any{keyPlacementGroupLinodes: []int{123, 456}}},
		},
		"a caller-supplied target wins and the source is dropped": {
			arguments: map[string]any{
				keyFoldSource: []any{float64(123)},
				keyFoldTarget: map[string]any{"volumes": []any{float64(9)}},
			},
			want: map[string]any{keyFoldTarget: map[string]any{"volumes": []any{float64(9)}}},
		},
		"an unusable source is left alone so validate names it": {
			arguments: map[string]any{keyFoldSource: "123"},
			want:      map[string]any{keyFoldSource: "123"},
		},
		"an unusable target is left alone so the body builder names it": {
			arguments: map[string]any{keyFoldSource: []any{float64(1)}, keyFoldTarget: "linodes"},
			want:      map[string]any{keyFoldSource: []any{float64(1)}, keyFoldTarget: "linodes"},
		},
		"nothing supplied means nothing invented": {
			arguments: map[string]any{},
			want:      map[string]any{},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := requestFor(test.arguments)
			tools.FoldIntList(&request, keyFoldSource, keyFoldTarget, "linodes")

			if got := request.GetArguments(); !reflect.DeepEqual(got, test.want) {
				t.Errorf("arguments = %v, want %v", got, test.want)
			}
		})
	}
}
