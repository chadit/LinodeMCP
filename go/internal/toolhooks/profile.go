package toolhooks

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// securityQuestionAnswerPreview is one answer as the preview reports it: the
// question still readable, the answer replaced. A struct rather than a map so
// the two members keep the order the live request sends them in.
type securityQuestionAnswerPreview struct {
	Response   string `json:"response"`
	QuestionID int64  `json:"question_id"`
}

// redactedAnswer is what stands in for an answer in the preview. The field-wide
// redaction marker would take the question ids with it, and those are the half
// of the call a caller checks before confirming.
const redactedAnswer = "[redacted]"

// LinodeProfileSecurityQuestionAnswerPreview reports the call with every answer
// replaced and the question ids left alone, which is the shape Go's hand-written
// preview reported and the shape Python reported nothing at all in.
//
// The reported body is rebuilt rather than passed through: the answers are
// security material, and the built body is the one the live call sends.
// Everything it is rebuilt from has already been through the rules the message
// declares, so the ids are present and positive by the time this runs.
func LinodeProfileSecurityQuestionAnswerPreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	_ any,
) (*mcp.CallToolResult, error) {
	reported := map[string]any{
		"security_questions": redactedSecurityQuestions(request),
	}

	return statePreview(ctx, request, cfg, "linode_profile_security_question_answer", method, path, reported, nil,
		func(_ any) tools.DryRunDetails {
			return tools.DryRunDetails{SideEffects: []string{
				"The profile's security question answers are saved.",
			}}
		})
}

// redactedSecurityQuestions reads the answers off the call and hands back the
// ids alone.
func redactedSecurityQuestions(request *mcp.CallToolRequest) []securityQuestionAnswerPreview {
	raw, _ := request.GetArguments()["security_questions"].([]any)
	answers := make([]securityQuestionAnswerPreview, 0, len(raw))

	for _, entry := range raw {
		item, isObject := entry.(map[string]any)
		if !isObject {
			continue
		}

		id, _ := item["question_id"].(float64)
		answers = append(answers, securityQuestionAnswerPreview{
			QuestionID: int64(id),
			Response:   redactedAnswer,
		})
	}

	return answers
}
