package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

const (
	profileSecurityQuestionsPath  = "/profile/security-questions"
	profileSecurityQuestionsParam = "security_questions"

	profileSecurityQuestionsCount    = 3
	profileSecurityResponseMinLength = 3
	profileSecurityResponseMaxLength = 17
)

// NewLinodeProfileSecurityQuestionsAnswerTool creates a tool for answering profile security questions.
func NewLinodeProfileSecurityQuestionsAnswerTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_profile_security_question_answer",
		"Answers security questions for the authenticated profile. Pass dry_run=true to preview without submitting.",
		toolschemas.Schema("linode.mcp.v1.SecurityQuestionAnswerInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeProfileSecurityQuestionsAnswerRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapAdmin, handler
}

func handleLinodeProfileSecurityQuestionsAnswerRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := answerProfileSecurityQuestionsRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		redactedReq := redactSecurityQuestionsRequest(req)

		return RunDryRunPreviewWithBodyDetailed(ctx, request, cfg, "linode_profile_security_question_answer", httpMethodPost,
			profileSecurityQuestionsPath, redactedReq, nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return profileSecurityQuestionsAnswerSideEffects(ctx)
			})
	}

	if result := RequireConfirm(request, "This submits profile security question answers. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if errorMessage := answerProfileSecurityQuestionsErrorMessage(ctx, client, req); errorMessage != "" {
		return mcp.NewToolResultError(errorMessage), nil
	}

	return MarshalProtoToolResponse(&linodev1.MessageResponse{
		Message: "Profile security questions answered successfully",
	})
}

func answerProfileSecurityQuestionsRequestFromTool(request *mcp.CallToolRequest) (*linode.AnswerProfileSecurityQuestionsRequest, string) {
	args := request.GetArguments()

	raw, exists := args[profileSecurityQuestionsParam]
	if !exists {
		return nil, profileSecurityQuestionsParam + " is required"
	}

	answers, validationMessage := objectSliceFromToolArg[linode.SecurityQuestionAnswer](raw, profileSecurityQuestionsParam)
	if validationMessage != "" {
		return nil, validationMessage
	}

	if validationMessage := validateSecurityQuestionAnswers(answers); validationMessage != "" {
		return nil, validationMessage
	}

	return &linode.AnswerProfileSecurityQuestionsRequest{SecurityQuestions: answers}, ""
}

// validateSecurityQuestionAnswers enforces the same shape the Linode API and the
// Python implementation require: exactly three answers, each with a positive,
// unique question_id and a response between 3 and 17 characters.
func validateSecurityQuestionAnswers(answers []linode.SecurityQuestionAnswer) string {
	if len(answers) != profileSecurityQuestionsCount {
		return profileSecurityQuestionsParam + " must contain exactly 3 answers"
	}

	seen := make(map[int]struct{}, profileSecurityQuestionsCount)

	for _, answer := range answers {
		if answer.QuestionID < 1 {
			return "question_id must be a positive integer"
		}

		if _, duplicate := seen[answer.QuestionID]; duplicate {
			return "question_id values must be unique"
		}

		seen[answer.QuestionID] = struct{}{}

		if len(answer.Response) < profileSecurityResponseMinLength || len(answer.Response) > profileSecurityResponseMaxLength {
			return "response length must be between 3 and 17 characters"
		}
	}

	return ""
}

// redactSecurityQuestionsRequest replaces every plaintext response with a
// placeholder so the dry-run preview body never echoes the answers.
func redactSecurityQuestionsRequest(req *linode.AnswerProfileSecurityQuestionsRequest) *linode.AnswerProfileSecurityQuestionsRequest {
	redacted := make([]linode.SecurityQuestionAnswer, len(req.SecurityQuestions))
	for i, answer := range req.SecurityQuestions {
		redacted[i] = linode.SecurityQuestionAnswer{QuestionID: answer.QuestionID, Response: "[redacted]"}
	}

	return &linode.AnswerProfileSecurityQuestionsRequest{SecurityQuestions: redacted}
}

func answerProfileSecurityQuestionsErrorMessage(ctx context.Context, client *linode.Client, req *linode.AnswerProfileSecurityQuestionsRequest) string {
	if err := client.AnswerProfileSecurityQuestions(ctx, req); err != nil {
		return "Failed to answer profile security questions: " + err.Error()
	}

	return ""
}
