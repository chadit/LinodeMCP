package tools_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/genlocal"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The last arm of a generated ladder: an operation failing in a way its own
// declaration does not name.
//
// Every declared condition is worded by the tool and reached through the
// behavior corpus. This one is not declared, so nothing words it: what the arm
// has to do is refuse to project, because the answer beside an unnamed failure
// is nothing and projecting nothing would read a value the subsystem never
// built.

// errUnnamedFailure stands for whatever a subsystem could fail with that its
// declaration does not list.
var errUnnamedFailure = errors.New("the store went away")

// unnamedDraftRead is a draft-read subsystem reporting a condition
// LOCAL_CALL_DRAFT_READ does not declare.
type unnamedDraftRead struct{}

func (unnamedDraftRead) DraftRead(string) (*genlocal.ProfileDraftResponse, error) {
	return nil, errUnnamedFailure
}

// TestGeneratedArmRefusesToProjectAnUnnamedFailure is the case that says the
// ladder's last arm does something. Without it the arm would fall through to
// the projection with no answer to project.
func TestGeneratedArmRefusesToProjectAnUnnamedFailure(t *testing.T) {
	t.Parallel()

	outcome := gentools.RunDraftRead(unnamedDraftRead{}, "any-draft")

	if outcome.Body != nil {
		t.Errorf("outcome.Body = %v, want none", outcome.Body)
	}

	if outcome.Refusal != tools.LocalRefusalNone {
		t.Errorf("outcome.Refusal = %v, want none: no tool words this one", outcome.Refusal)
	}

	if !strings.Contains(outcome.Cause, errUnnamedFailure.Error()) {
		t.Errorf("outcome.Cause = %q, want it to carry %q",
			outcome.Cause, errUnnamedFailure.Error())
	}
}

// TestAnUnnamedFailureAnswersTheDisagreementRatherThanAHalfBody carries the
// same outcome the rest of the way, since what a caller sees is what makes the
// refusal safe: the response check finds the missing members and says so.
func TestAnUnnamedFailureAnswersTheDisagreementRatherThanAHalfBody(t *testing.T) {
	t.Parallel()

	outcome := gentools.RunDraftRead(unnamedDraftRead{}, "any-draft")

	result, err := tools.LocalResponse(&linodev1.ProfileDraftResponse{}, outcome.Body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Fatal("the answer is not an error, so an unnamed failure answered a body")
	}

	if got := resultText(t, result); !strings.Contains(got, "unfilled") {
		t.Errorf("answer = %q, want it to name the members the body leaves unfilled", got)
	}
}

// TestTheDeclaredConditionStillWordsItsOwnRefusal is the control beside the two
// above: the arm's declared arm is unchanged, so an unnamed failure is the only
// thing that reaches the last one.
func TestTheDeclaredConditionStillWordsItsOwnRefusal(t *testing.T) {
	t.Parallel()

	outcome := gentools.RunDraftRead(missingDraftRead{}, "any-draft")

	if outcome.Refusal != tools.LocalRefusalDraftMissing {
		t.Errorf("outcome.Refusal = %v, want the declared draft-missing condition",
			outcome.Refusal)
	}
}

// missingDraftRead reports the one condition LOCAL_CALL_DRAFT_READ declares.
type missingDraftRead struct{}

func (missingDraftRead) DraftRead(string) (*genlocal.ProfileDraftResponse, error) {
	return nil, genlocal.ErrLocalDraftMissing
}
