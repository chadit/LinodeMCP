package toolhooks_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	// redactedResponse is what a preview reports in place of a security answer.
	redactedResponse = "[redacted]"
	// keySecurityQuestions and keyQuestionID are the two members these previews
	// are read through, in the arguments and in the reported body alike.
	keySecurityQuestions = "security_questions"
	keyQuestionID        = "question_id"
	keyAnswerResponse    = "response"
)

// profilePreview is the slice of a dry-run report these hooks are judged on.
type profilePreview struct {
	CurrentState map[string]any `json:"current_state"`
	SideEffects  []string       `json:"side_effects"`
	Warnings     []string       `json:"warnings"`
}

// decodeProfilePreview reads the report a profile preview hook answered with.
func decodeProfilePreview(t *testing.T, text string) profilePreview {
	t.Helper()

	var preview profilePreview
	if err := json.Unmarshal([]byte(text), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	return preview
}

// silentServer fails the test if any request reaches it.
func silentServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("a request reached the server, want none: this preview fetches nothing")
	}))
	t.Cleanup(server.Close)

	return server
}

// previewBody reads the request body a dry-run report echoed.
func previewBody(t *testing.T, text string) map[string]any {
	t.Helper()

	var report struct {
		WouldExecute struct {
			Body map[string]any `json:"body"`
		} `json:"would_execute"`
	}

	if err := json.Unmarshal([]byte(text), &report); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	return report.WouldExecute.Body
}

// TestLinodeProfileSecurityQuestionAnswerPreviewRedactsEachAnswer: the ids stay
// readable because they are what a caller checks before confirming, and the
// answers do not, because they are the security material.
func TestLinodeProfileSecurityQuestionAnswerPreviewRedactsEachAnswer(t *testing.T) {
	t.Parallel()

	server := silentServer(t)
	request := requestWith(map[string]any{
		keyDryRun: true,
		keySecurityQuestions: []any{
			map[string]any{keyQuestionID: float64(1), keyAnswerResponse: "first answer"},
			map[string]any{keyQuestionID: float64(7), keyAnswerResponse: "second answer"},
		},
	})

	result, err := toolhooks.LinodeProfileSecurityQuestionAnswerPreview(
		t.Context(), &request, configFor(server.URL),
		http.MethodPost, "/profile/security-questions", "the built body, which this hook replaces",
	)
	if err != nil {
		t.Fatalf("LinodeProfileSecurityQuestionAnswerPreview: %v", err)
	}

	text := resultText(t, result)
	want := map[string]any{keySecurityQuestions: []any{
		map[string]any{keyQuestionID: float64(1), keyAnswerResponse: redactedResponse},
		map[string]any{keyQuestionID: float64(7), keyAnswerResponse: redactedResponse},
	}}

	if body := previewBody(t, text); !reflect.DeepEqual(body, want) {
		t.Errorf("body = %v, want %v", body, want)
	}

	preview := decodeProfilePreview(t, text)
	effect := "The profile's security question answers are saved."

	if len(preview.SideEffects) != 1 || preview.SideEffects[0] != effect {
		t.Errorf("side_effects = %v, want [%q]", preview.SideEffects, effect)
	}
}

// TestLinodeProfileSecurityQuestionAnswerPreviewSkipsWhatIsNotAnAnswer: the
// rules refuse both shapes before the preview runs, so the hook reports what it
// can read rather than inventing an entry for what it cannot.
func TestLinodeProfileSecurityQuestionAnswerPreviewSkipsWhatIsNotAnAnswer(t *testing.T) {
	t.Parallel()

	server := silentServer(t)
	request := requestWith(map[string]any{
		keyDryRun:            true,
		keySecurityQuestions: []any{"not an object", map[string]any{keyAnswerResponse: "no id here"}},
	})

	result, err := toolhooks.LinodeProfileSecurityQuestionAnswerPreview(
		t.Context(), &request, configFor(server.URL),
		http.MethodPost, "/profile/security-questions", nil,
	)
	if err != nil {
		t.Fatalf("LinodeProfileSecurityQuestionAnswerPreview: %v", err)
	}

	want := map[string]any{keySecurityQuestions: []any{
		map[string]any{keyQuestionID: float64(0), keyAnswerResponse: redactedResponse},
	}}

	if body := previewBody(t, resultText(t, result)); !reflect.DeepEqual(body, want) {
		t.Errorf("body = %v, want %v", body, want)
	}
}

// TestLinodeProfileSecurityQuestionAnswerPreviewReportsNoAnswersWhenGivenNone
// covers the argument arriving as something other than a list, which the rules
// refuse ahead of the preview.
func TestLinodeProfileSecurityQuestionAnswerPreviewReportsNoAnswersWhenGivenNone(t *testing.T) {
	t.Parallel()

	server := silentServer(t)
	request := requestWith(map[string]any{keyDryRun: true, keySecurityQuestions: "not a list"})

	result, err := toolhooks.LinodeProfileSecurityQuestionAnswerPreview(
		t.Context(), &request, configFor(server.URL),
		http.MethodPost, "/profile/security-questions", nil,
	)
	if err != nil {
		t.Fatalf("LinodeProfileSecurityQuestionAnswerPreview: %v", err)
	}

	want := map[string]any{keySecurityQuestions: []any{}}

	if body := previewBody(t, resultText(t, result)); !reflect.DeepEqual(body, want) {
		t.Errorf("body = %v, want %v", body, want)
	}
}
