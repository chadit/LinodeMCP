package objectdata

import "errors"

// The failures a transfer answers with. Each is a sentinel so the hooks and the
// tests match by identity rather than by matching prose the behavior fixtures
// also pin.
var (
	// ErrSourceMissing reports a source path with no readable file behind it.
	ErrSourceMissing = errors.New("source_path names no readable file")

	// ErrSourceNotRegular reports a directory, device, or socket named as a
	// source. Streaming one would send bytes the caller did not mean to store.
	ErrSourceNotRegular = errors.New("source_path is not a regular file")

	// ErrSourceOutsideRoot reports a source resolving outside the configured
	// filesystem root, which is the whole reason for setting one.
	ErrSourceOutsideRoot = errors.New("source_path resolves outside the configured filesystem root")

	// ErrAboveSinglePartCeiling reports a file larger than one presigned PUT
	// carries.
	ErrAboveSinglePartCeiling = errors.New("source_path is above the single-part upload ceiling")

	// ErrTransfer reports a failed PUT against the presigned URL.
	ErrTransfer = errors.New("presigned upload failed")
)

// The failures the download half answers with.
var (
	// ErrDestinationExists reports a destination already holding a file when
	// the caller did not ask for a replacement.
	ErrDestinationExists = errors.New("dest_path already exists and overwrite is false")

	// ErrDestinationUnwritable reports a destination the transfer cannot create
	// or write, which is a local problem rather than a bucket one.
	ErrDestinationUnwritable = errors.New("dest_path cannot be written")
)

// ErrRemove reports a failed DELETE against the presigned URL. Separate from
// ErrTransfer so the sentence names what the call was doing: a removal reported
// as an upload failure sends the caller looking for bytes that never moved.
var ErrRemove = errors.New("presigned removal failed")
