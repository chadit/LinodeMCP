package tools_test

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// errTypeMembers is the plural refusal the vocabulary below composes.
const errTypeMembers = "type must be one of: master, slave"

// TestMemberChoiceArgumentHoldsTheRawArgumentToItsVocabulary covers every
// sentence the membership reader answers: the required split on an absent,
// non-string, or empty argument, the plural and single-value refusals, and the
// sentinel a real member name never spells.
func TestMemberChoiceArgumentHoldsTheRawArgumentToItsVocabulary(t *testing.T) {
	t.Parallel()

	members := []string{keyMaster, domainSlave}

	cases := []struct {
		args      map[string]any
		name      string
		wantValue string
		wantWords string
		members   []string
		required  bool
	}{
		{
			name:      "absent required argument",
			args:      map[string]any{},
			required:  true,
			members:   members,
			wantWords: errTypeRequired,
		},
		{
			name:      "absent optional argument passes",
			args:      map[string]any{},
			members:   members,
			wantWords: "",
		},
		{
			name:      "non-string required argument reads as absent",
			args:      map[string]any{keyType: 5},
			required:  true,
			members:   members,
			wantWords: errTypeRequired,
		},
		{
			name:      "empty required argument reads as absent",
			args:      map[string]any{keyType: ""},
			required:  true,
			members:   members,
			wantWords: errTypeRequired,
		},
		{
			name:      "empty optional argument passes",
			args:      map[string]any{keyType: ""},
			members:   members,
			wantWords: "",
		},
		{
			name:      "a name outside the vocabulary",
			args:      map[string]any{keyType: keyPrimary},
			required:  true,
			members:   members,
			wantWords: errTypeMembers,
		},
		{
			name:      "the sentinel is no member",
			args:      map[string]any{keyType: "unspecified"},
			members:   members,
			wantWords: errTypeMembers,
		},
		{
			name:      "an optional non-member is still refused",
			args:      map[string]any{keyType: keyPrimary},
			members:   members,
			wantWords: errTypeMembers,
		},
		{
			name:      "a one-value vocabulary names the value alone",
			args:      map[string]any{keyType: "affinity"},
			required:  true,
			members:   []string{"anti_affinity:local"},
			wantWords: "type must be anti_affinity:local",
		},
		{
			name:      "a member passes and answers the value",
			args:      map[string]any{keyType: domainSlave},
			required:  true,
			members:   members,
			wantValue: domainSlave,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := mcp.CallToolRequest{
				Params: mcp.CallToolParams{Arguments: testCase.args},
			}

			value, words := tools.MemberChoiceArgument(
				&request, keyType, testCase.required, testCase.members...,
			)

			if words != testCase.wantWords {
				t.Errorf("message = %q, want %q", words, testCase.wantWords)
			}

			if value != testCase.wantValue {
				t.Errorf("value = %q, want %q", value, testCase.wantValue)
			}
		})
	}
}
