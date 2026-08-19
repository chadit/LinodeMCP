package tools_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/gentools"
)

// The declared previews whose wording a flag selects, and the one whose read is
// addressed by a query parameter. Each was hand-written per language until this
// slot, so the prose and the read are pinned here rather than left to the
// behavior fixtures alone: a fixture stub is matched on the path, so it cannot
// see the query the object ACL read is addressed by.

// The canned values these previews are driven with, each only ever this file's.
const (
	choiceClusterID    = float64(123)
	choiceTargetID     = float64(789)
	choiceObjectKey    = "photo.jpg"
	choiceBucketLabel  = "my-bucket"
	choiceBucketRegion = "us-east-1"
)

// declaredPreviewProse is both halves of the prose a generated dry run reported.
type declaredPreviewProse struct {
	SideEffects []string `json:"side_effects"`
	Warnings    []string `json:"warnings"`
}

// declaredPreviewLines reads both halves of the prose a generated dry run
// reported.
func declaredPreviewLines(t *testing.T, text string) declaredPreviewProse {
	t.Helper()

	var preview declaredPreviewProse

	if err := json.Unmarshal([]byte(text), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	return preview
}

// previewReadServer answers whatever read the preview fetches with one canned
// body, recording the URL so a test can pin what the fetch addressed.
func previewReadServer(t *testing.T, body string) (*httptest.Server, *string) {
	t.Helper()

	var addressed string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addressed = r.URL.RequestURI()

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server, &addressed
}

// TestGeneratedBackupRestorePreviewSplitsOnOverwrite: the flag decides what the
// restore does to the target and whether the destructive warning is reported at
// all. An absent overwrite reads as false, which is what both hand walks did.
func TestGeneratedBackupRestorePreviewSplitsOnOverwrite(t *testing.T) {
	t.Parallel()

	const (
		replaced = "All existing disks and configs on target instance 789 are destroyed and replaced by the backup."
		beside   = "The backup is restored onto target instance 789; the restore fails if its disks or configs collide."
		lost     = "overwrite=true: existing data on target instance 789 is permanently lost."
	)

	cases := []struct {
		overwrite    any
		name         string
		wantEffect   string
		wantWarnings []string
	}{
		{
			name:         "overwrite replaces the target and earns the warning",
			overwrite:    true,
			wantEffect:   replaced,
			wantWarnings: []string{lost},
		},
		{
			name:       "the restore lands beside what is there",
			overwrite:  false,
			wantEffect: beside,
		},
		{
			name:       "an absent overwrite reads as false",
			overwrite:  nil,
			wantEffect: beside,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server, _ := previewReadServer(t, `{"id":456,"label":"nightly","status":"successful"}`)

			args := map[string]any{
				keyManagedLinodeSettingsLinodeID: float64(123), "backup_id": float64(456),
				"target_linode_id": choiceTargetID, keyDryRun: true,
			}
			if testCase.overwrite != nil {
				args["overwrite"] = testCase.overwrite
			}

			_, _, handler := gentools.NewLinodeInstanceBackupRestoreTool(newTestConfig(server.URL))

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			prose := declaredPreviewLines(t, resultText(t, result))

			if !slices.Equal(prose.SideEffects, []string{testCase.wantEffect}) {
				t.Errorf("side_effects = %v, want [%q]", prose.SideEffects, testCase.wantEffect)
			}

			if !slices.Equal(prose.Warnings, testCase.wantWarnings) {
				t.Errorf("warnings = %v, want %v", prose.Warnings, testCase.wantWarnings)
			}
		})
	}
}

// TestGeneratedBucketAccessUpdatePreviewNamesOnlyWhatWasAsked: an omitted acl or
// cors_enabled leaves that setting as it stands, so its whole line is dropped
// rather than reported with a gap in it.
func TestGeneratedBucketAccessUpdatePreviewNamesOnlyWhatWasAsked(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want []string
	}{
		{
			name: "both settings named",
			args: map[string]any{keyACL: "private", "cors_enabled": true},
			want: []string{
				`Bucket access control is set to "private".`,
				"CORS is enabled for the bucket.",
			},
		},
		{
			name: "cors turned off with no acl to report",
			args: map[string]any{"cors_enabled": false},
			want: []string{"CORS is disabled for the bucket."},
		},
		{
			name: "an acl alone leaves the cors line out",
			args: map[string]any{keyACL: "private"},
			want: []string{`Bucket access control is set to "private".`},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server, _ := previewReadServer(t, `{"acl":"public-read","cors_enabled":true}`)

			args := map[string]any{
				keyRegion: choiceBucketRegion, managedServiceLabelParam: choiceBucketLabel, keyDryRun: true,
			}
			maps.Copy(args, testCase.args)

			_, _, handler := gentools.NewLinodeObjectStorageBucketAccessUpdateTool(newTestConfig(server.URL))

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			prose := declaredPreviewLines(t, resultText(t, result))
			if !slices.Equal(prose.SideEffects, testCase.want) {
				t.Errorf("side_effects = %v, want %v", prose.SideEffects, testCase.want)
			}
		})
	}
}

// TestGeneratedLkeACLUpdatePreviewNamesWhatTheEditDoes: the flag rides inside
// the object the call sends, and an absent enabled is neither an open nor a
// close, which is the third answer Go's hand walk once reported as a disable.
func TestGeneratedLkeACLUpdatePreviewNamesWhatTheEditDoes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		acl  map[string]any
		name string
		want string
	}{
		{
			name: "enabling closes the API to everything unlisted",
			acl:  map[string]any{statusEnabled: true},
			want: "The control-plane ACL is enabled; only the listed addresses may reach the Kubernetes API.",
		},
		{
			name: "disabling opens the API to any address",
			acl:  map[string]any{statusEnabled: false},
			want: "The control-plane ACL is disabled; the Kubernetes API becomes reachable from any address.",
		},
		{
			name: "an address-only edit is neither",
			acl:  map[string]any{"addresses": map[string]any{"ipv4": []any{"203.0.113.1/32"}}},
			want: "The cluster control-plane ACL address list is updated.",
		},
		{
			name: "an enabled that is not a flag reads as absent",
			acl:  map[string]any{statusEnabled: boolStringTrue},
			want: "The cluster control-plane ACL address list is updated.",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// The API wraps the ACL under a top-level acl key, which is what the
			// declaration reads it out of.
			server, addressed := previewReadServer(t,
				`{"acl":{"enabled":false,"addresses":{"ipv4":[],"ipv6":[]}}}`)

			args := map[string]any{
				"cluster_id": choiceClusterID, keyACL: testCase.acl, keyDryRun: true,
			}

			_, _, handler := gentools.NewLinodeLkeACLUpdateTool(newTestConfig(server.URL))

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			if want := "/lke/clusters/123/control_plane_acl"; *addressed != want {
				t.Errorf("the fetch addressed %q, want %q", *addressed, want)
			}

			prose := declaredPreviewLines(t, resultText(t, result))
			if !slices.Equal(prose.SideEffects, []string{testCase.want}) {
				t.Errorf("side_effects = %v, want [%q]", prose.SideEffects, testCase.want)
			}
		})
	}
}

// TestGeneratedObjectACLUpdatePreviewAddressesTheObject: the read is addressed
// by the object key in its query as well as by the bucket in its path, and no
// behavior fixture can see that, since a fixture stub is matched on the path
// alone. Without the query this fetch answers about the bucket.
func TestGeneratedObjectACLUpdatePreviewAddressesTheObject(t *testing.T) {
	t.Parallel()

	server, addressed := previewReadServer(t, `{"acl":"private","acl_xml":"<AccessControlPolicy/>"}`)

	args := map[string]any{
		keyRegion: choiceBucketRegion, managedServiceLabelParam: choiceBucketLabel,
		managedContactNameParam: choiceObjectKey, keyACL: "public-read", keyDryRun: true,
	}

	_, _, handler := gentools.NewLinodeObjectStorageObjectACLUpdateTool(newTestConfig(server.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, args))
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	want := "/object-storage/buckets/us-east-1/my-bucket/object-acl?name=photo.jpg"
	if *addressed != want {
		t.Errorf("the fetch addressed %q, want %q", *addressed, want)
	}

	prose := declaredPreviewLines(t, resultText(t, result))

	wantEffect := `Object access control is set to "public-read".`
	if !slices.Equal(prose.SideEffects, []string{wantEffect}) {
		t.Errorf("side_effects = %v, want [%q]", prose.SideEffects, wantEffect)
	}
}
