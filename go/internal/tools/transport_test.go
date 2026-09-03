package tools_test

import (
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/objectdata"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	transferRegion     = "us-east"
	transferBucket     = "artifacts"
	transferObjectKey  = "key"
	transferStoredETag = "abc123"
	transferContent    = "hello object storage"
	uploadTool         = "linode_object_storage_object_upload"
	downloadTool       = "linode_object_storage_object_download"
	uploadSubject      = "object storage object upload"
	downloadSubject    = "object storage object download"
	casePresignFails   = "the presign call fails"
	presignURLField    = "url"
	keySourcePath      = "source_path"
	keyDestPath        = "dest_path"
	thumbnailArgument  = "thumbnail_png_base64"
	caseTransferFails  = "the transfer fails"
)

// transferSource writes one file and answers its path.
func transferSource(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "upload.bin")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return path
}

// transferBody builds the presign body the generated handler hands the engine,
// so the cases below exercise the same shape production does.
func transferBody(request *mcp.CallToolRequest, verb string) *tools.WriteBody {
	body := tools.NewWriteBody(request, 3)

	body.PutString(keyName)
	body.SetString("content_type")
	body.SetInt("expires_in")
	body.Constant("method", verb)

	return body
}

// transferRequest is a call carrying the arguments a transfer reads.
func transferRequest(extra map[string]any) mcp.CallToolRequest {
	arguments := map[string]any{
		keyRegion: transferRegion,
		keyLabel:  transferBucket,
		keyName:   transferObjectKey,
	}
	maps.Copy(arguments, extra)

	request := mcp.CallToolRequest{}
	request.Params.Arguments = arguments

	return request
}

// transferClient answers a client with the data-plane budgets left at zero, so
// the engine fills its own defaults the way a live server does.
func transferClient(baseURL string, ceiling int64) *linode.Client {
	return linode.NewClient(baseURL, tokenTest,
		&config.Config{ObjectStorage: config.ObjectStorageConfig{MaxSinglePartBytes: ceiling}},
		linode.WithMaxRetries(0))
}

// uploadSpec and downloadSpec are the specs the emitter renders for the two
// declared presign tools, built here once so each case names only what it is
// varying.
func uploadSpec(localPath string, body any) *tools.PresignSpec {
	return &tools.PresignSpec{
		Tool:       uploadTool,
		Subject:    uploadSubject,
		Up:         true,
		URLField:   presignURLField,
		LocalPath:  localPath,
		PathValues: []any{transferRegion, transferBucket},
		Body:       body,
	}
}

func downloadSpec(localPath string, body any) *tools.PresignSpec {
	return &tools.PresignSpec{
		Tool:       downloadTool,
		Subject:    downloadSubject,
		URLField:   presignURLField,
		LocalPath:  localPath,
		PathValues: []any{transferRegion, transferBucket},
		Body:       body,
	}
}

// TestRunPresignTransferUpPresignsThenSends pins the two legs and their order:
// exactly one Linode call, then the transfer to the URL that call answered with.
func TestRunPresignTransferUpPresignsThenSends(t *testing.T) {
	t.Parallel()

	var (
		presignBody map[string]any
		putBody     string
		order       []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		order = append(order, r.Method+" "+r.URL.Path)

		if r.Method == http.MethodPost {
			_ = json.Unmarshal(body, &presignBody)

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"` + "http://" + r.Host + `/artifacts/key?sig=fixture"}`))

			return
		}

		putBody = string(body)

		w.Header().Set("ETag", `"`+transferStoredETag+`"`)
	}))
	t.Cleanup(server.Close)

	source := transferSource(t, transferContent)
	request := transferRequest(map[string]any{keySourcePath: source})

	transferred, err := tools.RunPresignTransfer(t.Context(), transferClient(server.URL, 0), uploadSpec(source, transferBody(&request, http.MethodPut)))
	if err != nil {
		t.Fatalf("RunPresignTransfer: %v", err)
	}

	if len(order) != 2 || !strings.HasPrefix(order[0], "POST") || !strings.HasPrefix(order[1], "PUT") {
		t.Fatalf("calls = %v, want the presign POST then the transfer PUT", order)
	}

	// The signature covers Content-Type, so the presign body has to carry the
	// default rather than the PUT inventing one the URL was not signed for.
	if presignBody["content_type"] != objectdata.DefaultContentType {
		t.Errorf("presigned content_type = %v, want the default filled in", presignBody["content_type"])
	}

	if presignBody["expires_in"] != float64(config.DefaultPresignTTLSeconds) {
		t.Errorf("presigned expires_in = %v, want the configured default", presignBody["expires_in"])
	}

	if putBody != transferContent {
		t.Errorf("transferred %q, want the file's bytes %q", putBody, transferContent)
	}

	if transferred.SizeBytes != int64(len(transferContent)) {
		t.Errorf("SizeBytes = %d, want %d", transferred.SizeBytes, len(transferContent))
	}

	if transferred.ETag != transferStoredETag {
		t.Errorf("ETag = %q, want the stored entity tag", transferred.ETag)
	}
}

// TestRunPresignTransferUpRefusesBeforeCalling pins that an unusable source
// costs no request: the file is checked ahead of the presign.
func TestRunPresignTransferUpRefusesBeforeCalling(t *testing.T) {
	t.Parallel()

	var calls int

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	missing := filepath.Join(t.TempDir(), "absent.bin")
	request := transferRequest(map[string]any{keySourcePath: missing})

	_, err := tools.RunPresignTransfer(t.Context(), transferClient(server.URL, 0), uploadSpec(missing, transferBody(&request, http.MethodPut)))
	if !errors.Is(err, objectdata.ErrSourceMissing) {
		t.Fatalf("error = %v, want ErrSourceMissing", err)
	}

	if calls != 0 {
		t.Errorf("made %d requests, want none before the source is readable", calls)
	}
}

// TestRunPresignTransferUpRefusesOverTheCeiling pins the other pre-request
// guard: a file above the configured single-part ceiling is refused rather than
// split, and it is refused before any call goes out.
func TestRunPresignTransferUpRefusesOverTheCeiling(t *testing.T) {
	t.Parallel()

	var calls int

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	source := transferSource(t, strings.Repeat("a", 2048))
	request := transferRequest(map[string]any{keySourcePath: source})

	_, err := tools.RunPresignTransfer(t.Context(), transferClient(server.URL, 1024), uploadSpec(source, transferBody(&request, http.MethodPut)))
	if !errors.Is(err, objectdata.ErrAboveSinglePartCeiling) {
		t.Fatalf("error = %v, want ErrAboveSinglePartCeiling", err)
	}

	if calls != 0 {
		t.Errorf("made %d requests, want none for a file over the ceiling", calls)
	}
}

// TestRunPresignTransferUpReportsEachLegsFailure pins that a failure in either
// leg reaches the caller rather than a partial answer.
func TestRunPresignTransferUpReportsEachLegsFailure(t *testing.T) {
	t.Parallel()

	for _, testCase := range legFailureCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(legFailureHandler(testCase.presignFail))
			t.Cleanup(server.Close)

			source := transferSource(t, "bytes")
			request := transferRequest(map[string]any{keySourcePath: source})

			_, err := tools.RunPresignTransfer(t.Context(), transferClient(server.URL, 0), uploadSpec(source, transferBody(&request, http.MethodPut)))
			if err == nil {
				t.Fatal("err = nil, want the failure the handler formats its sentence around")
			}
		})
	}
}

// TestRunPresignTransferDownPresignsThenFetches pins the two legs and their
// order, and that the object's bytes reach the named file.
func TestRunPresignTransferDownPresignsThenFetches(t *testing.T) {
	t.Parallel()

	var order []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, r.Method+" "+r.URL.Path)

		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"` + "http://" + r.Host + `/artifacts/key?sig=fixture"}`))

			return
		}

		w.Header().Set("ETag", `"`+transferStoredETag+`"`)
		_, _ = w.Write([]byte(transferContent))
	}))
	t.Cleanup(server.Close)

	destination := filepath.Join(t.TempDir(), "landed.bin")
	request := transferRequest(map[string]any{keyDestPath: destination})

	transferred, err := tools.RunPresignTransfer(t.Context(), transferClient(server.URL, 0), downloadSpec(destination, transferBody(&request, http.MethodGet)))
	if err != nil {
		t.Fatalf("RunPresignTransfer: %v", err)
	}

	if len(order) != 2 || !strings.HasPrefix(order[0], "POST") || !strings.HasPrefix(order[1], "GET") {
		t.Fatalf("calls = %v, want the presign POST then the transfer GET", order)
	}

	landed, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(landed) != transferContent {
		t.Errorf("file holds %q, want %q", landed, transferContent)
	}

	if transferred.SizeBytes != int64(len(transferContent)) {
		t.Errorf("SizeBytes = %d, want %d", transferred.SizeBytes, len(transferContent))
	}

	if transferred.ETag != transferStoredETag {
		t.Errorf("ETag = %q, want the stored entity tag", transferred.ETag)
	}
}

// TestRunPresignTransferDownRefusesBeforeCalling pins that an occupied
// destination costs no request: the path is resolved before presigning.
func TestRunPresignTransferDownRefusesBeforeCalling(t *testing.T) {
	t.Parallel()

	var calls int

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		calls++
	}))
	t.Cleanup(server.Close)

	occupied := filepath.Join(t.TempDir(), "taken.bin")
	if err := os.WriteFile(occupied, []byte("already here"), 0o600); err != nil {
		t.Fatalf("seed the destination: %v", err)
	}

	request := transferRequest(map[string]any{keyDestPath: occupied})

	_, err := tools.RunPresignTransfer(t.Context(), transferClient(server.URL, 0), downloadSpec(occupied, transferBody(&request, http.MethodGet)))
	if !errors.Is(err, objectdata.ErrDestinationExists) {
		t.Fatalf("error = %v, want ErrDestinationExists", err)
	}

	if calls != 0 {
		t.Errorf("made %d requests, want none for an occupied destination", calls)
	}
}

// TestRunPresignTransferDownReportsEachLegsFailure pins that a failure in either
// leg reaches the caller rather than a partial answer.
func TestRunPresignTransferDownReportsEachLegsFailure(t *testing.T) {
	t.Parallel()

	for _, testCase := range legFailureCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(legFailureHandler(testCase.presignFail))
			t.Cleanup(server.Close)

			destination := filepath.Join(t.TempDir(), "landed.bin")
			request := transferRequest(map[string]any{keyDestPath: destination})

			_, err := tools.RunPresignTransfer(t.Context(), transferClient(server.URL, 0), downloadSpec(destination, transferBody(&request, http.MethodGet)))
			if err == nil {
				t.Fatal("err = nil, want the failure the handler formats its sentence around")
			}
		})
	}
}

// TestRunPresignTransferRefusesAPresignAnswerThatIsNotAnObject pins the
// sentence a bare array gets: the subject names which call answered badly,
// which is what Python's twin reports for the same body.
func TestRunPresignTransferRefusesAPresignAnswerThatIsNotAnObject(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	destination := filepath.Join(t.TempDir(), "landed.bin")
	request := transferRequest(map[string]any{keyDestPath: destination})

	_, err := tools.RunPresignTransfer(t.Context(), transferClient(server.URL, 0), downloadSpec(destination, transferBody(&request, http.MethodGet)))
	if !errors.Is(err, linode.ErrWriteResponseNotObject) {
		t.Fatalf("error = %v, want the non-object answer guard", err)
	}

	if err.Error() != downloadSubject+" response must be a JSON object" {
		t.Errorf("error = %q, want the subject to name the call", err)
	}
}

// legFailureCase names which of the two legs the server fails.
type legFailureCase struct {
	name        string
	presignFail bool
}

func legFailureCases() []legFailureCase {
	return []legFailureCase{
		{name: casePresignFails, presignFail: true},
		{name: caseTransferFails},
	}
}

// legFailureHandler answers a failure on the chosen leg and a usable presign on
// the other, so each case exercises one leg's refusal alone.
func legFailureHandler(presignFail bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && presignFail {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"` + "http://" + r.Host + `/artifacts/key"}`))

			return
		}

		w.WriteHeader(http.StatusForbidden)
	}
}

// TestBase64ArgumentAnswersTheDecodedBytes pins both halves of the raw-body
// encoding: usable text decodes, and text the rules would have refused answers
// no bytes rather than a second sentence the caller never reads.
func TestBase64ArgumentAnswersTheDecodedBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "standard base64", value: "cG5nLWJ5dGVz", want: "png-bytes"},
		{name: "unusable text", value: "not base64!", want: ""},
		{name: "absent argument", want: ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			arguments := map[string]any{}
			if testCase.value != nil {
				arguments[thumbnailArgument] = testCase.value
			}

			request := mcp.CallToolRequest{}
			request.Params.Arguments = arguments

			if got := string(tools.Base64Argument(&request, thumbnailArgument)); got != testCase.want {
				t.Errorf("Base64Argument = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestBase64TextEncodesWhatTheTransferRead pins the downward half: the bytes
// the route answered with become the text the response member carries.
func TestBase64TextEncodesWhatTheTransferRead(t *testing.T) {
	t.Parallel()

	if got := tools.Base64Text([]byte("png-bytes")); got != "cG5nLWJ5dGVz" {
		t.Errorf("Base64Text = %q, want cG5nLWJ5dGVz", got)
	}
}

// A body the presign builder never assembled has nothing to default into, which
// is what a tool registered before its config was loaded hands over. Filling it
// anyway would put members on a request the caller never built.
func TestFillPresignBodyLeavesWhatItCannotDefaultInto(t *testing.T) {
	t.Parallel()

	settings := config.ObjectStorageConfig{PresignTTLSeconds: 900}

	// Neither reaches the defaults below, so the case is that both return
	// without touching anything.
	tools.FillPresignBody(nil, settings)
	tools.FillPresignBody(map[string]any{}, settings)

	request := transferRequest(nil)
	body := transferBody(&request, http.MethodPut)

	tools.FillPresignBody(body, settings)

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var filled map[string]any
	if err := json.Unmarshal(raw, &filled); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if filled["expires_in"] != float64(900) {
		t.Errorf("expires_in = %v, want the configured lifetime", filled["expires_in"])
	}

	if filled["content_type"] != objectdata.DefaultContentType {
		t.Errorf("content_type = %v, want the default filled in", filled["content_type"])
	}
}

// The removal arm's own constants and body: the generated builder declares no
// content_type field, since the delete tool advertises none.
const (
	deleteTool    = "linode_object_storage_object_delete"
	deleteSubject = "object storage object delete"
)

// removeBody builds the presign body the generated destroy handler hands the
// engine, so the cases below exercise the same shape production does.
func removeBody(request *mcp.CallToolRequest) *tools.WriteBody {
	body := tools.NewWriteBody(request, 2)

	body.PutString(keyName)
	body.SetInt("expires_in")
	body.Constant("method", http.MethodDelete)

	return body
}

func removeSpec(body any) *tools.PresignSpec {
	return &tools.PresignSpec{
		Tool:       deleteTool,
		Subject:    deleteSubject,
		URLField:   presignURLField,
		PathValues: []any{transferRegion, transferBucket},
		Body:       body,
	}
}

// TestRunPresignRemovePresignsThenDeletes pins the two legs and their order:
// exactly one Linode call, then the DELETE to the URL that call answered with.
func TestRunPresignRemovePresignsThenDeletes(t *testing.T) {
	t.Parallel()

	var (
		presignBody map[string]any
		order       []string
		deletePath  string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		order = append(order, r.Method+" "+r.URL.Path)

		if r.Method == http.MethodPost {
			_ = json.Unmarshal(body, &presignBody)

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"` + "http://" + r.Host + `/artifacts/key?sig=fixture"}`))

			return
		}

		deletePath = r.URL.Path

		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	request := transferRequest(nil)

	if err := tools.RunPresignRemove(
		t.Context(), transferClient(server.URL, 0), removeSpec(removeBody(&request)),
	); err != nil {
		t.Fatalf("RunPresignRemove: %v", err)
	}

	if len(order) != 2 || !strings.HasPrefix(order[0], "POST") || !strings.HasPrefix(order[1], "DELETE") {
		t.Fatalf("calls = %v, want the presign POST then the DELETE", order)
	}

	if deletePath != "/artifacts/key" {
		t.Errorf("delete path = %q, want the minted URL's own path", deletePath)
	}

	// The verb is the tool's, and the two members the caller may omit are filled
	// by the engine so the signed request is the one the endpoint accepts.
	if presignBody["method"] != http.MethodDelete {
		t.Errorf("presign method = %v, want DELETE", presignBody["method"])
	}

	if presignBody["expires_in"] != float64(config.DefaultPresignTTLSeconds) {
		t.Errorf("presign expires_in = %v, want the configured default", presignBody["expires_in"])
	}

	if presignBody["content_type"] != objectdata.DefaultContentType {
		t.Errorf("presign content_type = %v, want the shared default", presignBody["content_type"])
	}
}

// TestRunPresignRemoveReportsEitherLeg pins that neither half fails quietly: a
// refused presign never reaches the DELETE, and a refused DELETE is reported
// rather than read as a removal that happened.
func TestRunPresignRemoveReportsEitherLeg(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		presignStatus int
		deleteStatus  int
		wantDeletes   int
	}{
		casePresignFails: {presignStatus: http.StatusInternalServerError, wantDeletes: 0},
		"the delete fails": {
			presignStatus: http.StatusOK,
			deleteStatus:  http.StatusForbidden,
			wantDeletes:   1,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var deletes int

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					if test.presignStatus != http.StatusOK {
						w.WriteHeader(test.presignStatus)

						return
					}

					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"url":"` + "http://" + r.Host + `/artifacts/key?sig=secret-token"}`))

					return
				}

				deletes++

				w.WriteHeader(test.deleteStatus)
			}))
			t.Cleanup(server.Close)

			request := transferRequest(nil)

			err := tools.RunPresignRemove(
				t.Context(), transferClient(server.URL, 0), removeSpec(removeBody(&request)),
			)
			if err == nil {
				t.Fatal("RunPresignRemove = nil, want the leg's own failure")
			}

			if deletes != test.wantDeletes {
				t.Errorf("delete calls = %d, want %d", deletes, test.wantDeletes)
			}

			// The text is read into a local first: this asserts on what the
			// caller sees, not on which error it is.
			if reported := err.Error(); strings.Contains(reported, "secret-token") {
				t.Errorf("error text leaks the minted URL: %s", reported)
			}
		})
	}
}
