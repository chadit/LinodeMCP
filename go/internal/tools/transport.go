package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/objectdata"
)

// The declared transports' engine: the steps a tool's live call takes when the
// bytes on the wire are not the JSON request the emitter derives. The emitter
// renders one call per declared arm and this runs it, so both languages move
// one file from one declaration. Mirrors Python's tools/transport.py.

// Base64Argument decodes a base64 argument into the bytes a raw-body transfer
// sends. Text the decoder refuses answers no bytes, because the message rules
// have already accepted this value by here and a second sentence from the
// transport would be one the caller never reads.
func Base64Argument(request *mcp.CallToolRequest, name string) []byte {
	decoded, err := base64.StdEncoding.DecodeString(request.GetString(name, ""))
	if err != nil {
		return nil
	}

	return decoded
}

// Base64Text encodes the bytes a raw-body transfer read into the text its
// answer member carries, since a JSON answer has no other way to hold an image.
func Base64Text(payload []byte) string {
	return base64.StdEncoding.EncodeToString(payload)
}

// ObjectTransferSettings fills any data-plane budget the config left at zero.
// Both the transfer and the preview describing it resolve through here, because
// a preview naming one presign lifetime while the transfer requested another
// would report a call the tool does not make.
func ObjectTransferSettings(settings config.ObjectStorageConfig) config.ObjectStorageConfig {
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

// FillPresignBody adds the members the caller may omit but the presign request
// has to carry. Content-Type especially: the signature covers it, so a URL
// signed without one and then used with one is refused by the endpoint.
func FillPresignBody(body any, settings config.ObjectStorageConfig) {
	writeBody, isWriteBody := body.(*WriteBody)
	if !isWriteBody {
		return
	}

	writeBody.Default("content_type", objectdata.DefaultContentType)
	writeBody.Default("expires_in", settings.PresignTTLSeconds)
}

// PresignPreview is what a dry run measured off the local end of a presigned
// upload. Exactly one member is filled: the size the transfer would send, or
// the sentence the guard refuses it with. That is what lets a declared wording
// read the size and drop itself when the guard spoke instead.
type PresignPreview struct {
	// SizeBytes is the inspected source's size in decimal, "" when refused.
	SizeBytes string
	// Refusal is the guard's sentence, "" when the transfer would go ahead.
	Refusal string
}

// PreviewPresignSource runs the upload guard the live transfer runs, against
// the same settings and before any call, and fills the presign body the way
// the live call fills it, so the preview describes the request the tool would
// make rather than the one the caller spelled. Nothing is opened: a preview
// that cannot say "this file will be refused" has told the caller nothing they
// needed, and one that streams the file has made half the call.
func PreviewPresignSource(
	request *mcp.CallToolRequest, cfg *config.Config, body any, localPathArgument string,
) PresignPreview {
	settings := ObjectTransferSettings(objectStorageSettings(cfg))
	FillPresignBody(body, settings)

	source, err := objectdata.Inspect(request.GetString(localPathArgument, ""), settings.FilesystemRoot)
	if err != nil {
		return PresignPreview{Refusal: err.Error()}
	}

	if err := objectdata.CheckSinglePart(source.SizeBytes, settings.MaxSinglePartBytes); err != nil {
		return PresignPreview{Refusal: err.Error()}
	}

	return PresignPreview{SizeBytes: strconv.FormatInt(source.SizeBytes, 10)}
}

// objectStorageSettings reads the data-plane block off a config that may be
// nil, which is what a tool registered before any config was loaded receives.
func objectStorageSettings(cfg *config.Config) config.ObjectStorageConfig {
	if cfg == nil {
		return config.ObjectStorageConfig{}
	}

	return cfg.ObjectStorage
}

// PresignSpec is one declared presigned transfer, rendered by the emitter from
// the tool's execute_transport. Members are ordered for alignment rather than
// for reading, which is what the field-alignment check asks of a struct this
// wide.
type PresignSpec struct {
	// Body is the request the shared builder assembled for the presign call.
	Body any
	// Tool names the route that mints the URL, and Subject names that call in
	// the report a malformed answer gets.
	Tool    string
	Subject string
	// URLField is the member of the presign answer carrying the minted URL.
	URLField string
	// LocalPath is the source going up, the destination coming down.
	LocalPath string
	// ContentType is the media type an upload sends under, "" for the default.
	ContentType string
	// PathValues fill the presign route's slots.
	PathValues []any
	// Up is whether the local file is sent or written.
	Up bool
	// Overwrite permits a download to replace an occupied destination.
	Overwrite bool
}

// TransferResult is what a transfer measured off its own stream: the bytes that
// actually moved and the entity tag the endpoint reported for them.
type TransferResult struct {
	ETag      string
	SizeBytes int64
}

// RunPresignTransfer asks the API for a presigned URL and then moves the local
// file through it.
//
// The transfer is not the derived request because it addresses a URL the API
// just minted rather than a route the client builds. Only one Linode operation
// happens here, the presign, which is why that is the route the tool declares.
//
// The local end is resolved before the presign call, so an oversized source or
// an occupied destination costs no request at all.
func RunPresignTransfer(
	ctx context.Context, client *linode.Client, spec *PresignSpec,
) (TransferResult, error) {
	settings := ObjectTransferSettings(client.ObjectStorage())

	source, destination, err := resolveTransferEnds(spec, settings)
	if err != nil {
		return TransferResult{}, err
	}

	url, err := mintPresignedURL(ctx, client, spec, settings)
	if err != nil {
		return TransferResult{}, err
	}

	if spec.Up {
		return uploadThrough(ctx, spec, settings, url, source)
	}

	return downloadThrough(ctx, settings, url, destination)
}

// RunPresignRemove asks the API for a URL signed for DELETE and then sends that
// DELETE.
//
// Nothing local is resolved and nothing is measured, because a removal moves no
// bytes: the presign is the only Linode operation, the same as it is for a
// transfer, and the DELETE that follows addresses the URL it answered with.
func RunPresignRemove(ctx context.Context, client *linode.Client, spec *PresignSpec) error {
	settings := ObjectTransferSettings(client.ObjectStorage())

	url, err := mintPresignedURL(ctx, client, spec, settings)
	if err != nil {
		return err
	}

	if removeErr := objectdata.Remove(ctx, objectdata.RemoveRequest{
		URL:     url,
		Timeout: settings.TransferTimeout,
	}); removeErr != nil {
		return fmt.Errorf("%w", removeErr)
	}

	return nil
}

// mintPresignedURL makes the presign call both arms share and answers with the
// URL it minted. The body is filled first, so a caller that omitted the signed
// content type or the lifetime still sends the request the endpoint accepts.
func mintPresignedURL(
	ctx context.Context, client *linode.Client, spec *PresignSpec, settings config.ObjectStorageConfig,
) (string, error) {
	FillPresignBody(spec.Body, settings)

	minted := &structpb.Struct{}
	if err := client.CallProtoRouteBody(
		ctx, spec.Tool, spec.PathValues, spec.Body, spec.Subject, minted,
	); err != nil {
		return "", fmt.Errorf("%w", err)
	}

	return minted.GetFields()[spec.URLField].GetStringValue(), nil
}

// resolveTransferEnds runs the guard the direction implies, answering the
// inspected source going up and the resolved destination coming down.
func resolveTransferEnds(
	spec *PresignSpec, settings config.ObjectStorageConfig,
) (objectdata.LocalFile, string, error) {
	if !spec.Up {
		destination, err := objectdata.ResolveDestination(
			spec.LocalPath, settings.FilesystemRoot, spec.Overwrite,
		)
		if err != nil {
			return objectdata.LocalFile{}, "", fmt.Errorf("%w", err)
		}

		return objectdata.LocalFile{}, destination, nil
	}

	source, err := objectdata.Inspect(spec.LocalPath, settings.FilesystemRoot)
	if err != nil {
		return objectdata.LocalFile{}, "", fmt.Errorf("%w", err)
	}

	if ceilingErr := objectdata.CheckSinglePart(
		source.SizeBytes, settings.MaxSinglePartBytes,
	); ceilingErr != nil {
		return objectdata.LocalFile{}, "", fmt.Errorf("%w", ceilingErr)
	}

	return source, "", nil
}

// uploadThrough sends the inspected file to the minted URL.
func uploadThrough(
	ctx context.Context,
	spec *PresignSpec,
	settings config.ObjectStorageConfig,
	url string,
	source objectdata.LocalFile,
) (TransferResult, error) {
	contentType := spec.ContentType
	if contentType == "" {
		contentType = objectdata.DefaultContentType
	}

	uploaded, err := objectdata.Upload(ctx, objectdata.UploadRequest{
		URL:         url,
		ContentType: contentType,
		Source:      source,
		Timeout:     settings.TransferTimeout,
	})
	if err != nil {
		return TransferResult{}, fmt.Errorf("%w", err)
	}

	return TransferResult{SizeBytes: uploaded.SizeBytes, ETag: uploaded.ETag}, nil
}

// downloadThrough streams the minted URL to the resolved destination.
func downloadThrough(
	ctx context.Context, settings config.ObjectStorageConfig, url, destination string,
) (TransferResult, error) {
	fetched, err := objectdata.Download(ctx, objectdata.DownloadRequest{
		URL:      url,
		DestPath: destination,
		Timeout:  settings.TransferTimeout,
	})
	if err != nil {
		return TransferResult{}, fmt.Errorf("%w", err)
	}

	return TransferResult{SizeBytes: fetched.SizeBytes, ETag: fetched.ETag}, nil
}
