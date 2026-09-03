// Package objectdata moves object bytes to the presigned URLs the Linode API
// mints.
//
// It sits outside internal/linode deliberately. cmd/route-dump resolves the
// requests built in that package against the route contract, and a PUT to a URL
// the API handed back is not a route this client builds: it would surface as
// unresolved route evidence with no contract line able to match it. Keeping the
// transfer here leaves the client package saying only what it declares.
package objectdata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultContentType is what an object is stored under when the caller names
// none. It is filled into the presign body rather than onto the PUT, because
// the signature covers the header and a PUT carrying a type the presign request
// never named is refused.
const DefaultContentType = "application/octet-stream"

// LocalFile is one resolved upload source: the path after symlink resolution
// and the size the transfer expects to send.
type LocalFile struct {
	Path      string
	SizeBytes int64
}

// Inspect resolves sourcePath and reports what a transfer would send, opening
// nothing and calling nothing. The preview and the transfer both read it, so a
// dry run describes the same file the live call would send.
//
// The path is resolved through symlinks before the root comparison, so a link
// inside the root pointing out of it is refused rather than followed. An empty
// filesystemRoot confines nothing: the stdio deployment this feature exists for
// reads the operator's own paths, and a default root would refuse them.
func Inspect(sourcePath, filesystemRoot string) (LocalFile, error) {
	resolved, err := filepath.EvalSymlinks(sourcePath)
	if err != nil {
		return LocalFile{}, fmt.Errorf("%w: %s", ErrSourceMissing, sourcePath)
	}

	if confineErr := confine(resolved, filesystemRoot); confineErr != nil {
		return LocalFile{}, confineErr
	}

	// A stat failure and a non-regular mode answer together: EvalSymlinks above
	// already proved the path resolves, so anything left is "this is not a file
	// the transfer can send", which is the one thing the caller has to fix.
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return LocalFile{}, fmt.Errorf("%w: %s", ErrSourceNotRegular, sourcePath)
	}

	return LocalFile{Path: resolved, SizeBytes: info.Size()}, nil
}

// confine refuses a resolved path outside root. An empty root confines nothing.
func confine(resolved, root string) error {
	if root == "" {
		return nil
	}

	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrSourceOutsideRoot, resolved)
	}

	// filepath.Rel is the comparison a string prefix gets wrong: /srv/data-other
	// carries /srv/data as a text prefix and is not inside it. Anything above the
	// root answers with a leading parent element.
	relative, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: %s", ErrSourceOutsideRoot, resolved)
	}

	return nil
}

// CheckSinglePart refuses a file larger than one presigned PUT carries. The
// sentence names the ceiling and the size the caller has, and says what is
// missing rather than naming a tool that does not exist: an object this large
// needs multipart, and Tier A does not implement it.
func CheckSinglePart(sizeBytes, maxBytes int64) error {
	if sizeBytes <= maxBytes {
		return nil
	}

	return fmt.Errorf("%w: %d bytes exceeds the %d byte ceiling, and multipart upload is not implemented yet",
		ErrAboveSinglePartCeiling, sizeBytes, maxBytes)
}

// UploadRequest is one transfer against an already-minted presigned URL.
type UploadRequest struct {
	// URL is what the API answered with. It is a bearer credential for its
	// lifetime, which is why no error below ever reports it.
	URL string
	// ContentType must be the value the presign request carried, because the
	// signature covers it.
	ContentType string
	Source      LocalFile
	Timeout     time.Duration
}

// UploadResult is what the transfer produced. SizeBytes is measured off the
// stream rather than restated from the request, so a file that changed size
// mid-read reports what actually reached the bucket.
type UploadResult struct {
	ETag      string
	SizeBytes int64
}

// Upload streams the source file to the presigned URL and reports what landed.
//
// The ETag comes back verbatim with its quotes stripped and is not checked here.
// For a single PUT the endpoint's ETag is the stored object's MD5, so the
// caller verifies by hashing their own file and comparing, which tests the round
// trip end to end instead of repeating the server's arithmetic back at it.
func Upload(ctx context.Context, request UploadRequest) (UploadResult, error) {
	// #nosec G304 -- path is the user-selected local file to upload; reading it is the tool's purpose
	file, err := os.Open(request.Source.Path)
	if err != nil {
		return UploadResult{}, fmt.Errorf("%w: %s", ErrSourceMissing, request.Source.Path)
	}

	defer func() { _ = file.Close() }()

	counter := &countingWriter{}

	response, err := send(ctx, request, io.TeeReader(file, counter))
	if err != nil {
		return UploadResult{}, err
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return UploadResult{}, fmt.Errorf("%w: the bucket answered HTTP %d", ErrTransfer, response.StatusCode)
	}

	return UploadResult{
		SizeBytes: counter.count,
		ETag:      strings.Trim(response.Header.Get("ETag"), `"`),
	}, nil
}

// send performs the PUT with its own client rather than the API client's: the
// presigned URL carries its authorization in the query string, so attaching the
// Linode token would put an account credential on a host that never needed one.
func send(ctx context.Context, request UploadRequest, body io.Reader) (*http.Response, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPut, request.URL, body)
	if err != nil {
		return nil, fmt.Errorf("%w: the presigned URL could not be used", ErrTransfer)
	}

	httpRequest.Header.Set("Content-Type", request.ContentType)
	// Set explicitly because the body is a reader with no length of its own, and
	// a presigned PUT is signed for a fixed length rather than a chunked one.
	httpRequest.ContentLength = request.Source.SizeBytes

	// Timeout bounds the whole exchange, body included, which is the budget a
	// multi-gigabyte transfer needs and the API client's per-request one cannot
	// give. The context still carries the caller's cancellation.
	response, err := (&http.Client{Timeout: request.Timeout}).Do(httpRequest)
	if err != nil {
		return nil, transferError(ErrTransfer, err)
	}

	return response, nil
}

// transferError strips the URL out of a transport failure and reports it under
// the sentinel the caller's own verb owns. net/http wraps every failure in a
// *url.Error that prints the full URL, and that URL is a bearer credential:
// reporting it would put a transfer token in the tool's error text and from
// there into the audit record.
func transferError(kind, err error) error {
	if urlErr, isURLErr := errors.AsType[*url.Error](err); isURLErr {
		err = urlErr.Err
	}

	return fmt.Errorf("%w: %w", kind, err)
}

// countingWriter counts the bytes the transport actually read off the file,
// which is what the result reports. It is the write half of a tee rather than a
// wrapping reader, so counting costs no extra pass and it has no borrowed error
// to hand back: an io.EOF wrapped on its way to the transport would read as a
// short body and fail the PUT.
type countingWriter struct {
	count int64
}

func (c *countingWriter) Write(data []byte) (int, error) {
	c.count += int64(len(data))

	return len(data), nil
}

// DownloadRequest is one transfer from an already-minted presigned URL to a
// local path.
type DownloadRequest struct {
	// URL is what the API answered with, and a bearer credential for its
	// lifetime, so no error below reports it.
	URL string
	// DestPath is where the object lands. It is confined to FilesystemRoot the
	// same way an upload source is, because a write outside the root is the
	// case a root exists to prevent.
	DestPath       string
	FilesystemRoot string
	Overwrite      bool
	Timeout        time.Duration
}

// DownloadResult is what the transfer produced, counted off the stream that was
// written rather than taken from a header the endpoint sent.
type DownloadResult struct {
	ETag      string
	SizeBytes int64
}

// Download streams the presigned URL into DestPath.
//
// The bytes land in a temporary file beside the destination and are renamed
// into place only after the whole body has been written, so a transfer that
// fails midway leaves no truncated file at the path the caller named. The
// existence check runs before the request, so a refused download costs no call.
func Download(ctx context.Context, request DownloadRequest) (DownloadResult, error) {
	destination, err := ResolveDestination(
		request.DestPath, request.FilesystemRoot, request.Overwrite,
	)
	if err != nil {
		return DownloadResult{}, err
	}

	response, err := fetch(ctx, request)
	if err != nil {
		return DownloadResult{}, err
	}

	// Close drains the unread body itself from Go 1.27, bounded, so a transfer
	// that fails midway drops one pooled connection instead of pulling the
	// discarded remainder of a large object back across the network.
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return DownloadResult{}, fmt.Errorf("%w: the bucket answered HTTP %d", ErrTransfer, response.StatusCode)
	}

	written, err := writeThroughTemp(destination, response.Body)
	if err != nil {
		return DownloadResult{}, err
	}

	return DownloadResult{
		SizeBytes: written,
		ETag:      strings.Trim(response.Header.Get("ETag"), `"`),
	}, nil
}

// ResolveDestination refuses the two cases the caller has to fix: a file
// already there without overwrite, and a path outside the configured root. The
// parent directory is what gets confined, because the file does not exist yet.
//
// Exported because the download hook calls it before asking for a presigned
// URL: a refused download should cost no request. Download calls it again, so
// the guarantee does not depend on the caller having done so.
func ResolveDestination(destPath, filesystemRoot string, overwrite bool) (string, error) {
	info, err := os.Stat(destPath)
	if err == nil && !overwrite {
		return "", fmt.Errorf("%w: %s", ErrDestinationExists, destPath)
	}

	if err == nil && info.IsDir() {
		return "", fmt.Errorf("%w: %s is a directory", ErrDestinationUnwritable, destPath)
	}

	if confineErr := confine(filepath.Dir(destPath), filesystemRoot); confineErr != nil {
		return "", fmt.Errorf("%w: %s", ErrSourceOutsideRoot, destPath)
	}

	return destPath, nil
}

// fetch performs the GET with its own client, for the reason the upload's PUT
// uses one: the presigned URL carries its authorization in the query string, so
// attaching the Linode token would put an account credential on a host that
// never needed one.
func fetch(ctx context.Context, request DownloadRequest) (*http.Response, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, request.URL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("%w: the presigned URL could not be used", ErrTransfer)
	}

	response, err := (&http.Client{Timeout: request.Timeout}).Do(httpRequest)
	if err != nil {
		return nil, transferError(ErrTransfer, err)
	}

	return response, nil
}

// writeThroughTemp writes the body to a temporary file in the destination's own
// directory and renames it into place, so the rename is atomic on the same
// filesystem and no partial file is ever visible at the destination.
func writeThroughTemp(destination string, body io.Reader) (int64, error) {
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".download-*")
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrDestinationUnwritable, destination)
	}

	written, copyErr := io.Copy(temporary, body)
	closeErr := temporary.Close()

	renameErr := error(nil)
	if copyErr == nil && closeErr == nil {
		renameErr = os.Rename(temporary.Name(), destination)
	}

	// One branch because the three failures are one fact for the caller: nothing
	// landed at the destination. The temporary file goes either way, which is
	// what keeps a failed transfer from leaving a partial file behind.
	if copyErr != nil || closeErr != nil || renameErr != nil {
		_ = os.Remove(temporary.Name())

		return 0, fmt.Errorf("%w: nothing landed at %s", ErrTransfer, destination)
	}

	return written, nil
}

// RemoveRequest is one DELETE against an already-minted presigned URL.
type RemoveRequest struct {
	// URL is what the API answered with, and a bearer credential for its
	// lifetime, so no error below reports it.
	URL string
	// Timeout bounds the whole exchange.
	Timeout time.Duration
}

// Remove sends the DELETE the minted URL authorizes.
//
// Nothing is measured because nothing moves: the object is either gone
// afterwards or the bucket said why it is not. The request carries no body and
// no Content-Type, the same shape the download's GET sends.
func Remove(ctx context.Context, request RemoveRequest) error {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodDelete, request.URL, http.NoBody)
	if err != nil {
		return fmt.Errorf("%w: the presigned URL could not be used", ErrRemove)
	}

	// Its own client rather than the API client's, for the reason the upload's
	// PUT gives: the presigned URL carries its authorization in the query
	// string, so attaching the Linode token would put an account credential on
	// a host that never needed one.
	response, err := (&http.Client{Timeout: request.Timeout}).Do(httpRequest)
	if err != nil {
		return transferError(ErrRemove, err)
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: the bucket answered HTTP %d", ErrRemove, response.StatusCode)
	}

	return nil
}
