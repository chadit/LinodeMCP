package toolhooks_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	// volumeIDArg is the argument the volume routes are addressed by.
	volumeIDArg = "volume_id"
	// labelArg is the argument a volume clone or update names the resource with.
	labelArg = "label"
	// dataLabel and sizeArg are the label and argument these tables reuse.
	dataLabel    = "data"
	sizeArg      = "size"
	cloneLabel   = "copy"
	renamedLabel = "renamed"
)

// volumePreview is the part of a detach preview these tests read.
type volumePreview struct {
	SideEffects []string `json:"side_effects"`
}

// detachPreview runs the hook against a stub API answering with body.
func detachPreview(t *testing.T, body string) volumePreview {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	request := requestWith(map[string]any{volumeIDArg: 123, keyDryRun: true})

	result, err := toolhooks.LinodeVolumeDetachPreview(
		t.Context(), &request, configFor(server.URL), http.MethodPost, "/volumes/123/detach", nil,
	)
	if err != nil {
		t.Fatalf("LinodeVolumeDetachPreview: %v", err)
	}

	var preview volumePreview

	if err := json.Unmarshal([]byte(resultText(t, result)), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	return preview
}

// TestLinodeVolumeDetachPreviewNamesTheInstance: the attachment is the whole
// reason to preview a detach, since the call names only the volume.
func TestLinodeVolumeDetachPreviewNamesTheInstance(t *testing.T) {
	t.Parallel()

	preview := detachPreview(t, `{"id":123,"label":"data","linode_id":456}`)

	want := "Volume 123 detaches from instance 456; its data is preserved and billing continues."
	if len(preview.SideEffects) != 1 || preview.SideEffects[0] != want {
		t.Errorf("side_effects = %v, want [%q]", preview.SideEffects, want)
	}
}

// TestLinodeVolumeDetachPreviewCallsAnUnattachedDetachANoOp: a caller looking
// at a volume attached to nothing is about to make a call that changes nothing,
// which is worth saying before the call rather than after.
func TestLinodeVolumeDetachPreviewCallsAnUnattachedDetachANoOp(t *testing.T) {
	t.Parallel()

	preview := detachPreview(t, `{"id":123,"label":"data"}`)

	want := "Volume is not attached to any instance; detach is a no-op."
	if len(preview.SideEffects) != 1 || preview.SideEffects[0] != want {
		t.Errorf("side_effects = %v, want [%q]", preview.SideEffects, want)
	}
}

// TestLinodeVolumeDetachPreviewReportsAFailedFetch: the preview is the caller's
// only look at the volume, so a fetch that failed must not read as a volume
// attached to nothing.
func TestLinodeVolumeDetachPreviewReportsAFailedFetch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"Not found"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	request := requestWith(map[string]any{volumeIDArg: 123, keyDryRun: true})

	result, err := toolhooks.LinodeVolumeDetachPreview(
		t.Context(), &request, configFor(server.URL), http.MethodPost, "/volumes/123/detach", nil,
	)
	if err != nil {
		t.Fatalf("LinodeVolumeDetachPreview: %v", err)
	}

	if !result.IsError {
		t.Errorf("result = %s, want the failed fetch reported", resultText(t, result))
	}
}

// TestLinodeVolumeDetachPreviewReadsAnEmptyAnswerAsUnattached: an answer with
// no attachment in it is the same call to make as one that named none, and both
// languages report it the same way.
func TestLinodeVolumeDetachPreviewReadsAnEmptyAnswerAsUnattached(t *testing.T) {
	t.Parallel()

	preview := detachPreview(t, `null`)

	want := "Volume is not attached to any instance; detach is a no-op."
	if len(preview.SideEffects) != 1 || preview.SideEffects[0] != want {
		t.Errorf("side_effects = %v, want [%q]", preview.SideEffects, want)
	}
}

// volumeStub answers every request with body, so a preview reads the volume it
// asked for without a live account.
func stubServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

// previewOf runs one preview hook against a stub and decodes what it reported.
func previewOf(
	t *testing.T,
	hook func(context.Context, *mcp.CallToolRequest, *config.Config, string, string, any) (*mcp.CallToolResult, error),
	args map[string]any,
	body, method, path string,
) writePreview {
	t.Helper()

	server := stubServer(t, body)
	args[keyDryRun] = true
	request := requestWith(args)

	result, err := hook(t.Context(), &request, configFor(server.URL), method, path, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	var preview writePreview

	if err := json.Unmarshal([]byte(resultText(t, result)), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	return preview
}

// writePreview is the part of a write preview these tests read.
type writePreview struct {
	CurrentState map[string]any `json:"current_state"`
	SideEffects  []string       `json:"side_effects"`
	Warnings     []string       `json:"warnings"`
}

// TestVolumeWritePreviewsReportWhatTheCallWouldDo: each preview is the only
// thing a caller sees before a billable or destructive change, so the prose is
// pinned rather than left to whatever the hook happens to say.
func TestVolumeWritePreviewsReportWhatTheCallWouldDo(t *testing.T) {
	t.Parallel()

	volume := `{"id":333,"label":"data","size":20,"region":"us-east"}`

	cases := []struct {
		hook        func(context.Context, *mcp.CallToolRequest, *config.Config, string, string, any) (*mcp.CallToolResult, error)
		args        map[string]any
		name        string
		body        string
		method      string
		path        string
		wantWarning string
		wantEffects []string
	}{
		{
			name:   "clone names the source it copies",
			hook:   toolhooks.LinodeVolumeClonePreview,
			args:   map[string]any{volumeIDArg: 333, labelArg: cloneLabel},
			body:   volume,
			method: http.MethodPost,
			path:   "/volumes/333/clone",
			wantEffects: []string{
				`Volume 333 ("data") will be cloned to a new volume labeled "copy".`,
			},
			wantWarning: "Billing for the cloned volume starts immediately on creation.",
		},
		{
			name:        "resize names the size it grows from",
			hook:        toolhooks.LinodeVolumeResizePreview,
			args:        map[string]any{volumeIDArg: 333, sizeArg: 40},
			body:        volume,
			method:      http.MethodPost,
			path:        "/volumes/333/resize",
			wantEffects: []string{"Volume resizes from 20 GB to 40 GB."},
			wantWarning: "A volume can only grow; the new size must be larger than the current size.",
		},
		{
			name:   "update names the label it replaces and the tags it clears",
			hook:   toolhooks.LinodeVolumeUpdatePreview,
			args:   map[string]any{volumeIDArg: 333, labelArg: "renamed", "tags": []any{tagLabelProd}},
			body:   volume,
			method: http.MethodPut,
			path:   "/volumes/333",
			wantEffects: []string{
				`Label changes from "data" to "renamed".`,
				"The volume's tag set is replaced with the provided tags.",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			preview := previewOf(t, testCase.hook, testCase.args, testCase.body, testCase.method, testCase.path)

			if !slices.Equal(preview.SideEffects, testCase.wantEffects) {
				t.Errorf("side_effects = %v, want %v", preview.SideEffects, testCase.wantEffects)
			}

			if testCase.wantWarning != "" && !slices.Contains(preview.Warnings, testCase.wantWarning) {
				t.Errorf("warnings = %v, want one of them %q", preview.Warnings, testCase.wantWarning)
			}
		})
	}
}

// TestLinodeVolumeClonePreviewDescribesACopyItCouldNotRead: the label the caller
// asked for is the part they are about to be billed for, so a source the fetch
// answered with nothing still earns the prose.
func TestLinodeVolumeClonePreviewDescribesACopyItCouldNotRead(t *testing.T) {
	t.Parallel()

	preview := previewOf(t, toolhooks.LinodeVolumeClonePreview,
		map[string]any{volumeIDArg: 333, labelArg: cloneLabel},
		emptyJSONBody, http.MethodPost, "/volumes/333/clone")

	want := `A new volume labeled "copy" will be created from the source volume.`
	if !slices.Equal(preview.SideEffects, []string{want}) {
		t.Errorf("side_effects = %v, want [%q]", preview.SideEffects, want)
	}
}

// TestLinodeVolumeResizePreviewOmitsASizeItCouldNotRead: a volume the fetch
// answered without a size reports the target alone rather than resizing from 0.
func TestLinodeVolumeResizePreviewOmitsASizeItCouldNotRead(t *testing.T) {
	t.Parallel()

	preview := previewOf(t, toolhooks.LinodeVolumeResizePreview,
		map[string]any{volumeIDArg: 333, sizeArg: 40},
		emptyJSONBody, http.MethodPost, "/volumes/333/resize")

	want := "Volume resizes to 40 GB."
	if !slices.Equal(preview.SideEffects, []string{want}) {
		t.Errorf("side_effects = %v, want [%q]", preview.SideEffects, want)
	}
}

// TestLinodeVolumeUpdatePreviewSetsALabelItCouldNotCompare: a volume the fetch
// answered without a label reads as a label being set rather than replaced.
func TestLinodeVolumeUpdatePreviewSetsALabelItCouldNotCompare(t *testing.T) {
	t.Parallel()

	preview := previewOf(t, toolhooks.LinodeVolumeUpdatePreview,
		map[string]any{volumeIDArg: 333, labelArg: "renamed"},
		emptyJSONBody, http.MethodPut, "/volumes/333")

	want := `Label is set to "renamed".`
	if !slices.Equal(preview.SideEffects, []string{want}) {
		t.Errorf("side_effects = %v, want [%q]", preview.SideEffects, want)
	}
}

// TestLinodeVolumeDeleteDependencyWalkNamesTheInstance: an attached volume comes
// off its instance before it is destroyed, which is the part of an irreversible
// delete a caller cannot see from the request.
func TestLinodeVolumeDeleteDependencyWalkNamesTheInstance(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		body       string
		wantLabel  string
		wantLength int
	}{
		{
			name:       "detached volume takes nothing with it",
			body:       `{"id":333}`,
			wantLength: 0,
		},
		{
			name:       "attached volume names the instance it carries",
			body:       `{"id":333,"linode_id":42,"linode_label":"web-1"}`,
			wantLabel:  "web-1",
			wantLength: 1,
		},
		{
			name:       "a volume the API sent no attachment for reads as detached",
			body:       `{"id":333,"linode_id":null}`,
			wantLength: 0,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := stubServer(t, `{"id":42,"label":"web-1"}`)
			client := linode.NewClient(server.URL, "token", configFor(server.URL))

			details, err := toolhooks.LinodeVolumeDeleteDependencyWalk(
				t.Context(), client, 333, declaredState(t, testCase.body, &linodev1.Volume{}))
			if err != nil {
				t.Fatalf("LinodeVolumeDeleteDependencyWalk: %v", err)
			}

			if len(details.Dependencies) != testCase.wantLength {
				t.Fatalf("dependencies = %v, want %d of them", details.Dependencies, testCase.wantLength)
			}

			if testCase.wantLength > 0 && details.Dependencies[0].Label != testCase.wantLabel {
				t.Errorf("label = %q, want %q", details.Dependencies[0].Label, testCase.wantLabel)
			}
		})
	}
}

// TestLinodeVolumeDeleteDependencyWalkResolvesAMissingLabel: a volume record
// naming the instance by id alone still reports a label, because the id is not
// what a caller recognizes the instance by.
func TestLinodeVolumeDeleteDependencyWalkResolvesAMissingLabel(t *testing.T) {
	t.Parallel()

	server := stubServer(t, `{"id":42,"label":"web-1"}`)
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	details, err := toolhooks.LinodeVolumeDeleteDependencyWalk(
		t.Context(), client, 333, declaredState(t, `{"id":333,"linode_id":42}`, &linodev1.Volume{}),
	)
	if err != nil {
		t.Fatalf("LinodeVolumeDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 1 || details.Dependencies[0].Label != "web-1" {
		t.Errorf("dependencies = %v, want the instance resolved to web-1", details.Dependencies)
	}
}

// TestSSHKeyUpdatePreviewNamesTheLabelChange: the public half stays out of the
// prose, so what a caller reads before the call is the label it replaces.
func TestSSHKeyUpdatePreviewNamesTheLabelChange(t *testing.T) {
	t.Parallel()

	updated := previewOf(t, toolhooks.LinodeSshkeyUpdatePreview,
		map[string]any{argSSHKeyID: 5, labelArg: renamedLabel},
		`{"id":5,"label":"laptop","ssh_key":"ssh-rsa AAAA"}`,
		http.MethodPut, "/profile/sshkeys/5")

	wantUpdate := `Label changes from "laptop" to "renamed".`
	if !slices.Equal(updated.SideEffects, []string{wantUpdate}) {
		t.Errorf("update side_effects = %v, want [%q]", updated.SideEffects, wantUpdate)
	}
}
