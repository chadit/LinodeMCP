package main_test

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The transfer-preview refusals. A wording reading `{transport:size_bytes}` is
// what makes a dry run measure a presigned upload's source, so one declared
// where nothing can measure would report a gap on every call, or read a member
// only the transfer itself fills.

const (
	uploadProbePath     = "/probes/{region}/{label}/object-url"
	uploadProbeReadPath = "/probes/{region}/{label}"
	uploadProbeResponse = "linode.mcp.v1.ObjectStorageObjectUploadResponse"
	uploadProbeRegion   = "region"
	uploadProbeLabel    = "label"
	uploadProbeSource   = "source_path"
	uploadProbeSizeLine = "{transport:size_bytes} bytes will be uploaded to '{name}'."
	uploadProbeETagLine = "{transport:etag} is the tag '{name}' will carry."
)

func TestRefusesATransferPreviewThatCannotMeasure(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "transport member on a tool with no presigned upload",
			refusal: "errPreviewTransportNotUpload",
			build: previewProbe("ProbeTransferPreviewNoUploadInput", nil,
				&linodev1.PreviewSentence{Template: []string{uploadProbeSizeLine}, Line: previewEffect}),
		},
		{
			name:    "transport member the guard never measures",
			refusal: "errPreviewTransportMember",
			build:   uploadProbe("ProbeTransferPreviewETagInput", uploadProbeETagLine, nil),
		},
		{
			name:    "transfer preview beside a declared state read",
			refusal: "errPreviewTransportWithState",
			build: uploadProbe("ProbeTransferPreviewWithStateInput", uploadProbeSizeLine,
				withStateRoute(&linodev1.StateRoute{Tool: probeStateReadTool})),
		},
	})
}

// uploadProbe is a presigned upload on the acknowledge tier, the shape the
// object upload declares, carrying one wording over its transfer. The read the
// state case names is declared beside it, so that refusal comes from the two
// declarations meeting rather than from a read that does not exist.
func uploadProbe(
	message, template string, extra func(*descriptorpb.MessageOptions),
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		sets := []func(*descriptorpb.MessageOptions){
			withRoute("POST", uploadProbePath),
			withCapability(writeCapability),
			withResponse(uploadProbeResponse),
			withDescription("Uploads a probe."),
			withErrorMessage("Failed to upload '{name}': {error}"),
			withConfirmMessage("This uploads a probe. Set confirm=true to proceed."),
			withSuccessMessage("Object '{name}' uploaded to bucket '{label}'"),
			probeScopes(),
			probeCategories(),
			withTransport(presignTransport(&linodev1.PresignTransfer{
				Direction:           linodev1.TransferDirection_TRANSFER_DIRECTION_UP,
				UrlField:            presignURLMember,
				LocalPathArgument:   uploadProbeSource,
				ContentTypeArgument: "content_type",
				SizeField:           "size_bytes",
				EtagField:           "etag",
				AnswerConstant:      []*linodev1.TransferConstant{{Field: "upload_mode", Text: "single"}},
			})),
			withPreview(&linodev1.PreviewSentence{Template: []string{template}, Line: previewEffect}),
		}
		if extra != nil {
			sets = append(sets, extra)
		}

		run := goProbe(probeMessage(t, message, messageOptions(sets...),
			pathString(uploadProbeRegion, 1),
			pathString(uploadProbeLabel, 2),
			bodyString(argName, 3),
			probeField(uploadProbeSource, 4, descriptorpb.FieldDescriptorProto_TYPE_STRING,
				fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_TOOL))),
			bodyString("content_type", 5),
			probeField(dryRunArgumentName, 6, descriptorpb.FieldDescriptorProto_TYPE_BOOL,
				fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_LOCAL))),
		))

		run.Beside = map[string]protoreflect.MessageDescriptor{
			probeStateReadTool: probeMessage(t, message+"Read",
				messageOptions(withRoute("GET", uploadProbeReadPath),
					withCapability(readCapability),
					withResponse(probeResource),
					withDescription("Reads a probe.")),
				pathString(uploadProbeRegion, 1),
				pathString(uploadProbeLabel, 2)),
		}

		return run
	}
}

// pathString is a text argument a route slot is filled from.
func pathString(name string, number int32) *descriptorpb.FieldDescriptorProto {
	return probeField(name, number, descriptorpb.FieldDescriptorProto_TYPE_STRING,
		fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_PATH)))
}

const (
	presignURLMember   = "url"
	argName            = "name"
	dryRunArgumentName = "dry_run"
)
