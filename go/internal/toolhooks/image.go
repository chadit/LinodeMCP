package toolhooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// argImageID is the prefixed pair every image route is addressed by.
const argImageID = "image_id"

// argRegions is the slug list a replication is asked for.
const argRegions = "regions"

// LinodeImageCreatePreview names the disk the capture reads and the label it
// lands under, neither of which a declared sentence can interpolate. Both
// languages already answered this sentence; the request echo is new to both.
func LinodeImageCreatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	diskID := request.GetInt("disk_id", 0)
	label := request.GetString("label", "")

	result, err := tools.RunDryRunPreviewWithBodyDetailed(
		ctx, request, cfg, "linode_image_create", method, path, body, nil,
		func(ctx context.Context, _ *linode.Client, _ any) (tools.DryRunDetails, error) {
			return previewDetails(ctx, "linode_image_create", func() tools.DryRunDetails {
				return tools.DryRunDetails{SideEffects: []string{imageCreateSentence(diskID, label)}}
			})
		},
	)

	return wrapPreview("linode_image_create", result, err)
}

// imageCreateSentence spells the capture the same way the echoed body spells
// the label, so the prose and the request a caller reads together agree.
func imageCreateSentence(diskID int, label string) string {
	effect := fmt.Sprintf("A new image will be captured from disk %d", diskID)
	if label != "" {
		effect += fmt.Sprintf(" and labeled %q", label)
	}

	return effect + "."
}

// LinodeImageReplicateNormalize trims each region slug. Python has always sent
// them trimmed and Go refused a padded one, so both converge on trimming before
// the slug rule reads them.
func LinodeImageReplicateNormalize(request *mcp.CallToolRequest) {
	regions, supplied := request.GetArguments()[argRegions].([]any)
	if !supplied {
		return
	}

	for index, entry := range regions {
		if slug, isText := entry.(string); isText {
			regions[index] = strings.TrimSpace(slug)
		}
	}
}

// LinodeImageReplicatePreview reads the image so a caller sees what is being
// replicated, and names the target regions the way Python's twin sentence did.
// Each language carried one half of this before, the read or the sentence.
func LinodeImageReplicatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	imageID := request.GetString(argImageID, "")
	regions := replicationRegions(request)

	return statePreview(ctx, request, cfg, "linode_image_replicate", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return client.GetImage(ctx, imageID)
		},
		func(_ any) tools.DryRunDetails {
			return tools.DryRunDetails{
				SideEffects: []string{fmt.Sprintf("Image '%s' will be replicated to %s.",
					imageID, strings.Join(regions, ", "))},
			}
		})
}

// replicationRegions renders the slugs the sentence lists. Entries the rules
// already refused cannot reach here, so anything non-text is dropped rather
// than spelled out in Go's own rendering of an arbitrary value.
func replicationRegions(request *mcp.CallToolRequest) []string {
	entries, supplied := request.GetArguments()[argRegions].([]any)
	if !supplied {
		return nil
	}

	slugs := make([]string, 0, len(entries))

	for _, entry := range entries {
		if slug, isText := entry.(string); isText {
			slugs = append(slugs, slug)
		}
	}

	return slugs
}
