package main

import (
	"fmt"
	"slices"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The declared transports: how a tool's live call travels when the derived JSON
// request is not what reaches the API. Each arm is read here and held to the
// fields its own shape needs, so an arm missing half its declaration fails the
// run rather than emitting a call with a blank in it.

// transportKind names which arm of execute_transport a tool declared.
type transportKind int

const (
	transportNone transportKind = iota
	transportMultipart
	transportRawBody
	transportPresign
	transportPresignRemove
)

// transportSpec is one declared transport, flattened out of the oneof: the
// emitters read fields rather than re-walking the arm, and the reader below is
// the only place an arm's fields are matched to its kind.
type transportSpec struct {
	// ContentType is the media type raw bytes travel under.
	ContentType string
	// FileArgument and PartName are the multipart arm's: the local file whose
	// contents become the part, and the form field it is filed under.
	FileArgument string
	PartName     string
	// SourceArgument and AnswerField are the raw-body arm's base64 ends.
	SourceArgument string
	AnswerField    string
	// The presign arm's own fields.
	URLField            string
	LocalPathArgument   string
	ContentTypeArgument string
	OverwriteArgument   string
	SizeField           string
	ETagField           string
	// Constants are the presign arm's fixed answer members.
	Constants []*linodev1.TransferConstant
	Kind      transportKind
	// Up is whether the bytes travel toward the API. Meaningless on the
	// multipart arm, which only ever sends.
	Up bool
}

// readExecuteTransport records the declared transport, holding each arm to the
// fields it needs before anything reads it.
func (c *contract) readExecuteTransport(options protoreflect.ProtoMessage) error {
	declared, _ := proto.GetExtension(options, linodev1.E_ExecuteTransport).(*linodev1.ExecuteTransport)
	if declared == nil {
		return nil
	}

	spec, err := c.transportArm(declared)
	if err != nil {
		return err
	}

	c.Transport = &spec

	return nil
}

// transportArm reads whichever arm the oneof carries.
func (c *contract) transportArm(declared *linodev1.ExecuteTransport) (transportSpec, error) {
	switch {
	case declared.GetMultipart() != nil:
		return c.multipartArm(declared.GetMultipart())
	case declared.GetRawBody() != nil:
		return c.rawBodyArm(declared.GetRawBody())
	case declared.GetPresign() != nil:
		return c.presignArm(declared.GetPresign())
	case declared.GetPresignRemove() != nil:
		return c.presignRemoveArm(declared.GetPresignRemove())
	}

	return transportSpec{}, fmt.Errorf("%w: %s", errNoTransportArm, c.Name)
}

// multipartArm reads a form upload: the file whose contents travel, and the
// field they are filed under.
func (c *contract) multipartArm(declared *linodev1.MultipartUpload) (transportSpec, error) {
	if declared.GetFileArgument() == "" || declared.GetPartName() == "" {
		return transportSpec{}, fmt.Errorf("%w: %s multipart", errTransportIncomplete, c.Name)
	}

	return transportSpec{
		Kind:         transportMultipart,
		Up:           true,
		FileArgument: declared.GetFileArgument(),
		PartName:     declared.GetPartName(),
	}, nil
}

// rawBodyArm reads a bare-bytes transfer. The direction decides which end of
// the base64 is declared, and declaring the other one is a field nothing reads.
func (c *contract) rawBodyArm(declared *linodev1.RawBody) (transportSpec, error) {
	upward, err := c.transportDirection(declared.GetDirection())
	if err != nil {
		return transportSpec{}, err
	}

	if declared.GetContentType() == "" {
		return transportSpec{}, fmt.Errorf("%w: %s raw_body", errTransportIncomplete, c.Name)
	}

	if upward != (declared.GetSourceArgument() != "") || upward == (declared.GetAnswerField() != "") {
		return transportSpec{}, fmt.Errorf("%w: %s raw_body", errTransportDirectionFields, c.Name)
	}

	return transportSpec{
		Kind:           transportRawBody,
		Up:             upward,
		ContentType:    declared.GetContentType(),
		SourceArgument: declared.GetSourceArgument(),
		AnswerField:    declared.GetAnswerField(),
	}, nil
}

// presignArm reads a minted-URL transfer. Both directions report the same two
// measurements, and each declares only the argument its own guard reads.
func (c *contract) presignArm(declared *linodev1.PresignTransfer) (transportSpec, error) {
	upward, err := c.transportDirection(declared.GetDirection())
	if err != nil {
		return transportSpec{}, err
	}

	if declared.GetUrlField() == "" || declared.GetLocalPathArgument() == "" ||
		declared.GetSizeField() == "" || declared.GetEtagField() == "" {
		return transportSpec{}, fmt.Errorf("%w: %s presign", errTransportIncomplete, c.Name)
	}

	if upward != (declared.GetContentTypeArgument() != "") || upward == (declared.GetOverwriteArgument() != "") {
		return transportSpec{}, fmt.Errorf("%w: %s presign", errTransportDirectionFields, c.Name)
	}

	for _, constant := range declared.GetAnswerConstant() {
		if constant.GetField() == "" || constant.GetText() == "" {
			return transportSpec{}, fmt.Errorf("%w: %s presign", errTransportIncomplete, c.Name)
		}
	}

	return transportSpec{
		Kind:                transportPresign,
		Up:                  upward,
		URLField:            declared.GetUrlField(),
		LocalPathArgument:   declared.GetLocalPathArgument(),
		ContentTypeArgument: declared.GetContentTypeArgument(),
		OverwriteArgument:   declared.GetOverwriteArgument(),
		SizeField:           declared.GetSizeField(),
		ETagField:           declared.GetEtagField(),
		Constants:           declared.GetAnswerConstant(),
	}, nil
}

// presignRemoveArm reads a minted-URL removal. The URL is the whole
// declaration: no bytes move, so there is no local end to guard, no direction
// to pick, and nothing measured for a response member to carry.
func (c *contract) presignRemoveArm(declared *linodev1.PresignRemove) (transportSpec, error) {
	if declared.GetUrlField() == "" {
		return transportSpec{}, fmt.Errorf("%w: %s presign_remove", errTransportIncomplete, c.Name)
	}

	return transportSpec{
		Kind:     transportPresignRemove,
		URLField: declared.GetUrlField(),
	}, nil
}

// transportDirection refuses the unset direction, which would otherwise read as
// a download on every arm that has one.
func (c *contract) transportDirection(direction linodev1.TransferDirection) (bool, error) {
	switch direction {
	case linodev1.TransferDirection_TRANSFER_DIRECTION_UP:
		return true, nil
	case linodev1.TransferDirection_TRANSFER_DIRECTION_DOWN:
		return false, nil
	case linodev1.TransferDirection_TRANSFER_DIRECTION_UNSPECIFIED:
	}

	return false, fmt.Errorf("%w: %s", errNoTransferDirection, c.Name)
}

// transported is whether the tool's live call is a declared transport rather
// than the derived JSON request.
func (c *contract) transported() bool {
	return c.Transport != nil
}

// transportDropsBody is whether the live call leaves the derived JSON body
// behind. Both presign arms send it, since the body is what the presign request
// asks with; the other two frame their own request out of a local file or a
// decoded argument, so the built body is advertised by the schema and never
// travels.
func (c *contract) transportDropsBody() bool {
	return c.transported() &&
		c.Transport.Kind != transportPresign && c.Transport.Kind != transportPresignRemove
}

// checkTransportNames holds the declared transport to the message it annotates:
// every argument it reads is one the input declares, and every member it fills
// is one the response assembles. Run after the fields and the response are
// read, since neither is in scope while the option itself is.
func (c *contract) checkTransportNames() error {
	if !c.transported() {
		return nil
	}

	spec := c.Transport

	for _, argument := range []string{
		spec.FileArgument, spec.SourceArgument,
		spec.LocalPathArgument, spec.ContentTypeArgument, spec.OverwriteArgument,
	} {
		if argument == "" {
			continue
		}

		if !slices.Contains(c.AllArguments, argument) {
			return fmt.Errorf("%w: %s reads %s", errTransportUnknownArgument, c.Name, argument)
		}
	}

	return c.checkTransportMembers()
}

// checkTransportMembers holds the members a transport fills to the ones the
// response declares assembled, and refuses an assembled member the transport
// leaves unfilled: the emitter copies member by member, so either mistake ships
// a zero the caller reads as real.
func (c *contract) checkTransportMembers() error {
	filled := c.transportFilled()

	assembled := make([]string, 0, len(c.Assembled))
	for _, member := range c.Assembled {
		assembled = append(assembled, member.ProtoName)
	}

	slices.Sort(filled)
	slices.Sort(assembled)

	if !slices.Equal(filled, assembled) {
		return fmt.Errorf("%w: %s fills %v, and %s assembles %v",
			errTransportMembers, c.Name, filled, c.ResponseGo.FullName, assembled)
	}

	return nil
}

// transportFilled names every response member the declared transport answers
// with, in no particular order.
func (c *contract) transportFilled() []string {
	spec := c.Transport

	filled := make([]string, 0, len(spec.Constants)+2)

	for _, name := range []string{spec.AnswerField, spec.SizeField, spec.ETagField} {
		if name != "" {
			filled = append(filled, name)
		}
	}

	for _, constant := range spec.Constants {
		filled = append(filled, constant.GetField())
	}

	return filled
}
