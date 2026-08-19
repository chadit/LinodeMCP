package tools_test

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// thumbnailHelloBase64 decodes to toolHello, so a passing case can prove the
// helper hands back the decoded bytes rather than the text it was given.
const (
	thumbnailHelloBase64 = "aGVsbG8="
	errThumbnailNonEmpty = "thumbnail_png_base64 must be a non-empty string"
)

// TestOAuthClientThumbnailPNGAnswersEachShape pins the helper the generated
// thumbnail update reaches through its validate hook and takes its bytes from.
// Absent is answered apart from present-but-unusable because Go has always
// separated the two, and the contract cannot express base64 at all, which is
// why this stayed a hook rather than becoming a rule.
func TestOAuthClientThumbnailPNGAnswersEachShape(t *testing.T) {
	t.Parallel()

	cases := []struct {
		arguments map[string]any
		name      string
		want      string
		wantBytes string
	}{
		{
			name:      "no image at all",
			arguments: map[string]any{},
			want:      "thumbnail_png_base64 is required",
		},
		{
			name:      "an image as a number",
			arguments: map[string]any{keyThumbnailPNGBase64: 7},
			want:      errThumbnailNonEmpty,
		},
		{
			name:      "an empty image",
			arguments: map[string]any{keyThumbnailPNGBase64: ""},
			want:      errThumbnailNonEmpty,
		},
		{
			name:      "a blank image",
			arguments: map[string]any{keyThumbnailPNGBase64: blankString},
			want:      errThumbnailNonEmpty,
		},
		{
			name:      "an image outside the base64 alphabet",
			arguments: map[string]any{keyThumbnailPNGBase64: "not!!base64"},
			want:      "thumbnail_png_base64 must be valid standard base64",
		},
		{
			name:      "an image the caller sent unpadded",
			arguments: map[string]any{keyThumbnailPNGBase64: "aGVsbG8"},
			want:      "thumbnail_png_base64 must be valid standard base64",
		},
		{
			name:      "a decodable image",
			arguments: map[string]any{keyThumbnailPNGBase64: thumbnailHelloBase64},
			want:      "",
			wantBytes: toolHello,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var request mcp.CallToolRequest

			request.Params.Arguments = testCase.arguments

			decoded, got := tools.OAuthClientThumbnailPNG(&request)
			if got != testCase.want {
				t.Errorf("OAuthClientThumbnailPNG message = %q, want %q", got, testCase.want)
			}

			if string(decoded) != testCase.wantBytes {
				t.Errorf("OAuthClientThumbnailPNG bytes = %q, want %q", decoded, testCase.wantBytes)
			}
		})
	}
}
