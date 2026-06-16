package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

const (
	profileSecurityQuestionsPath  = "/profile/security-questions"
	profileSecurityQuestionsParam = "security_questions"
)

// NewLinodeProfileSecurityQuestionsAnswerTool creates a tool for answering profile security questions.
func NewLinodeProfileSecurityQuestionsAnswerTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_profile_security_question_answer",
		"Answers security questions for the authenticated profile. Pass dry_run=true to preview without submitting.",
		[]mcp.ToolOption{
			mcp.WithString(profileSecurityQuestionsParam, mcp.Required(),
				mcp.Description("Security question answers payload to submit.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm submitting profile security question answers. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeProfileSecurityQuestionsAnswerRequest,
	)

	return tool, profiles.CapAdmin, handler
}

func handleLinodeProfileSecurityQuestionsAnswerRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := answerProfileSecurityQuestionsRequestFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		redactedReq := &linode.AnswerProfileSecurityQuestionsRequest{SecurityQuestions: "[redacted]"}

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

	return MarshalToolResponse(struct {
		Message string `json:"message"`
	}{Message: "Profile security questions answered successfully"})
}

func answerProfileSecurityQuestionsRequestFromTool(request *mcp.CallToolRequest) (*linode.AnswerProfileSecurityQuestionsRequest, string) {
	args := request.GetArguments()
	req := linode.AnswerProfileSecurityQuestionsRequest{}

	raw, exists := args[profileSecurityQuestionsParam]
	if !exists {
		return nil, profileSecurityQuestionsParam + " is required"
	}

	value, ok := raw.(string)
	if !ok {
		return nil, profileSecurityQuestionsParam + " must be a string"
	}

	if value == "" {
		return nil, profileSecurityQuestionsParam + " must not be empty"
	}

	req.SecurityQuestions = value

	return &req, ""
}

func answerProfileSecurityQuestionsErrorMessage(ctx context.Context, client *linode.Client, req *linode.AnswerProfileSecurityQuestionsRequest) string {
	if err := client.AnswerProfileSecurityQuestions(ctx, req); err != nil {
		return "Failed to answer profile security questions: " + err.Error()
	}

	return ""
}
