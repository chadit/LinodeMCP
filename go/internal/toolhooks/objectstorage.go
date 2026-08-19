package toolhooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/objectdata"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// uploadModeSingle is the only mode Tier A answers with: a file over the
// ceiling is refused rather than split, because multipart is not built yet.
const uploadModeSingle = "single"

// LinodeObjectStorageKeyUpdatePreview reads the key so a caller sees the label
// the update replaces. The read is credential-safe: the key GET never carries
// the secret.
func LinodeObjectStorageKeyUpdatePreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	keyID := request.GetInt("key_id", 0)
	label := request.GetString("label", "")
	_, scopesSupplied := request.GetArguments()["bucket_access"]

	return statePreview(ctx, request, cfg, "linode_object_storage_key_update", method, path, body,
		func(ctx context.Context, client *linode.Client) (any, error) {
			return fetchedState(client.GetObjectStorageKey(ctx, keyID))
		},
		func(state any) tools.DryRunDetails {
			return keyUpdateSideEffects(state, label, scopesSupplied)
		})
}

// keyUpdateSideEffects is the Tier B preview prose for a key update: the label
// change read against the fetched key, and the scope replacement when the
// caller asked for one.
func keyUpdateSideEffects(state any, newLabel string, scopesSupplied bool) tools.DryRunDetails {
	var details tools.DryRunDetails

	var fromLabel string
	if key, ok := state.(*linode.ObjectStorageKey); ok && key != nil {
		fromLabel = key.Label
	}

	tools.LabelChangeSideEffect(&details, fromLabel, newLabel)

	if scopesSupplied {
		details.SideEffects = append(details.SideEffects,
			"The key's bucket access scopes are replaced.")
	}

	return details
}

// LinodeObjectStoragePresignedURLCreateNormalize folds the method to the
// canonical S3 verb, so a caller sending "get" reaches the enum rule as GET.
func LinodeObjectStoragePresignedURLCreateNormalize(request *mcp.CallToolRequest) {
	arguments := request.GetArguments()

	if method, supplied := arguments["method"].(string); supplied {
		arguments["method"] = strings.ToUpper(method)
	}
}

// objectUploadSettings resolves the data-plane budgets, filling any the config
// left at zero. Both hooks read them through here because a preview that
// described one presign lifetime while the transfer requested another would be
// reporting a call the tool does not make.
func objectUploadSettings(settings config.ObjectStorageConfig) config.ObjectStorageConfig {
	if settings.MaxSinglePartBytes == 0 {
		settings.MaxSinglePartBytes = config.DefaultMaxSinglePartBytes
	}

	if settings.TransferTimeout == 0 {
		settings.TransferTimeout = config.DefaultTransferTimeout
	}

	if settings.PresignTTLSeconds == 0 {
		settings.PresignTTLSeconds = config.DefaultPresignTTLSeconds
	}

	return settings
}

// fillPresignBody adds the members the caller may omit but the presign request
// has to carry. Content-Type especially: the signature covers it, so a URL
// signed without one and then PUT with one is refused by the endpoint.
func fillPresignBody(body any, settings config.ObjectStorageConfig) {
	writeBody, isWriteBody := body.(*tools.WriteBody)
	if !isWriteBody {
		return
	}

	writeBody.Default("content_type", objectdata.DefaultContentType)
	writeBody.Default("expires_in", settings.PresignTTLSeconds)
}

// LinodeObjectStorageObjectUploadPreview reports the file the upload would send
// without opening it or calling anything.
//
// It stats the source, which the support-ticket attachment's preview
// deliberately does not do, and the difference is the point: the attachment's
// size does not change what its call does, while here the size decides whether
// the call is legal at all. A preview that cannot say "this file is 8 GB and
// will be refused" has told the caller nothing they needed.
func LinodeObjectStorageObjectUploadPreview(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	method, path string,
	body any,
) (*mcp.CallToolResult, error) {
	settings := objectUploadSettings(objectStorageConfig(cfg))
	fillPresignBody(body, settings)

	result, err := tools.RunDryRunPreviewWithBodyDetailed(
		ctx, request, cfg, "linode_object_storage_object_upload", method, path, body, nil,
		func(ctx context.Context, _ *linode.Client, _ any) (tools.DryRunDetails, error) {
			return previewDetails(ctx, "linode_object_storage_object_upload", func() tools.DryRunDetails {
				return objectUploadSideEffects(request, settings)
			})
		},
	)

	return wrapPreview("linode_object_storage_object_upload", result, err)
}

// objectStorageConfig reads the data-plane block off a config that may be nil,
// which is what a tool registered before any config was loaded receives.
func objectStorageConfig(cfg *config.Config) config.ObjectStorageConfig {
	if cfg == nil {
		return config.ObjectStorageConfig{}
	}

	return cfg.ObjectStorage
}

// objectUploadSideEffects describes the transfer, naming the refusal up front
// when the file is already over the ceiling.
func objectUploadSideEffects(
	request *mcp.CallToolRequest, settings config.ObjectStorageConfig,
) tools.DryRunDetails {
	var details tools.DryRunDetails

	source, err := objectdata.Inspect(request.GetString("source_path", ""), settings.FilesystemRoot)
	if err != nil {
		details.Warnings = append(details.Warnings, err.Error())

		return details
	}

	if err := objectdata.CheckSinglePart(source.SizeBytes, settings.MaxSinglePartBytes); err != nil {
		details.Warnings = append(details.Warnings, err.Error())

		return details
	}

	details.SideEffects = append(details.SideEffects,
		fmt.Sprintf("%d bytes will be uploaded to '%s' in bucket '%s' as a single part.",
			source.SizeBytes, request.GetString("name", ""), request.GetString("label", "")))

	return details
}

// LinodeObjectStorageObjectUploadExecute asks the API for a presigned URL and
// then sends the file to it.
//
// The transfer is the hook's rather than the generated handler's because the
// request is a file body against a URL the API just minted, not the JSON body
// every generated mutation sends. Only one Linode operation happens here, the
// presign, which is why that is the route the tool declares.
//
// The file is checked before the presign call, so an oversized or unreadable
// source costs no request at all.
func LinodeObjectStorageObjectUploadExecute(
	ctx context.Context,
	client *linode.Client,
	request *mcp.CallToolRequest,
	pathValues []any,
	body any,
) (*linodev1.ObjectStorageObjectUploadResponse, error) {
	settings := objectUploadSettings(client.ObjectStorage())

	source, err := objectdata.Inspect(request.GetString("source_path", ""), settings.FilesystemRoot)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	if ceilingErr := objectdata.CheckSinglePart(
		source.SizeBytes, settings.MaxSinglePartBytes,
	); ceilingErr != nil {
		return nil, fmt.Errorf("%w", ceilingErr)
	}

	fillPresignBody(body, settings)

	presigned := &linodev1.PresignedURLResponse{}
	if presignErr := client.CallProtoRouteBody(ctx, "linode_object_storage_object_upload",
		pathValues, body, "object storage object upload", presigned); presignErr != nil {
		return nil, fmt.Errorf("%w", presignErr)
	}

	uploaded, err := objectdata.Upload(ctx, objectdata.UploadRequest{
		URL:         presigned.GetUrl(),
		ContentType: request.GetString("content_type", objectdata.DefaultContentType),
		Source:      source,
		Timeout:     settings.TransferTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return &linodev1.ObjectStorageObjectUploadResponse{
		SizeBytes:  uploaded.SizeBytes,
		Etag:       uploaded.ETag,
		UploadMode: uploadModeSingle,
	}, nil
}

// LinodeObjectStorageObjectDownloadExecute asks the API for a presigned URL and
// then follows it to the local file.
//
// The transfer is the hook's for the reason the upload's is: the response is a
// stream of object bytes rather than the JSON body a generated read decodes.
// Only one Linode operation happens here, the presign, which is the route the
// tool declares.
//
// The destination is resolved before the presign call, so a refused download
// (a file already there, or a path outside the configured root) costs no
// request at all.
func LinodeObjectStorageObjectDownloadExecute(
	ctx context.Context,
	client *linode.Client,
	request *mcp.CallToolRequest,
	pathValues []any,
	body any,
) (*linodev1.ObjectStorageObjectDownloadResponse, error) {
	settings := objectUploadSettings(client.ObjectStorage())

	destination, err := objectdata.ResolveDestination(
		request.GetString("dest_path", ""), settings.FilesystemRoot,
		request.GetBool("overwrite", false),
	)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	fillPresignBody(body, settings)

	presigned := &linodev1.PresignedURLResponse{}
	if presignErr := client.CallProtoRouteBody(ctx, "linode_object_storage_object_download",
		pathValues, body, "object storage object download", presigned); presignErr != nil {
		return nil, fmt.Errorf("%w", presignErr)
	}

	fetched, err := objectdata.Download(ctx, objectdata.DownloadRequest{
		URL:      presigned.GetUrl(),
		DestPath: destination,
		Timeout:  settings.TransferTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return &linodev1.ObjectStorageObjectDownloadResponse{
		SizeBytes: fetched.SizeBytes,
		Etag:      fetched.ETag,
	}, nil
}
