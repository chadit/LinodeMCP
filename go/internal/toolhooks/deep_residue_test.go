package toolhooks_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	keyLabel      = "label"
	keyTarget     = "target"
	targetPublic  = "8.8.8.8"
	tagLabelProd  = "prod"
	recordNameNew = "new"
)

// residuePreview is the part of a dry run these tests read: the state the hook
// fetched, the body it echoed, and the prose it owns.
type residuePreview struct {
	CurrentState any `json:"current_state"`
	WouldExecute struct {
		Body any `json:"body"`
	} `json:"would_execute"`
	SideEffects []string `json:"side_effects"`
}

// residuePreviewOf decodes one dry-run answer.
func residuePreviewOf(t *testing.T, text string) residuePreview {
	t.Helper()

	var preview residuePreview
	if err := json.Unmarshal([]byte(text), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	return preview
}

// wantSideEffects holds a preview to the exact sentences it should carry, in
// order: the prose is the whole of what a preview hook owns.
func wantSideEffects(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("side_effects = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("side_effects[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestLinodeStackscriptUpdatePreviewDiffsAgainstTheFetchedScript covers the
// walk and the body echo together: the label sentence is the fetched value
// against the asked-for one, and the body is what Python's preview has always
// carried.
func TestLinodeStackscriptUpdatePreviewDiffsAgainstTheFetchedScript(t *testing.T) {
	t.Parallel()

	server := stubServer(t, `{"id":123,"label":"before","script":"#!/bin/sh"}`)
	request := requestWith(map[string]any{
		keyLabel: labelAfter, "script": "#!/bin/bash", keyDescription: recordNameNew,
	})

	result, err := toolhooks.LinodeStackscriptUpdatePreview(
		t.Context(), &request, configFor(server.URL), http.MethodPut,
		"/linode/stackscripts/77",
		map[string]any{keyLabel: labelAfter, "script": "#!/bin/bash", keyDescription: recordNameNew},
	)
	if err != nil {
		t.Fatalf("LinodeStackscriptUpdatePreview: %v", err)
	}

	preview := residuePreviewOf(t, resultText(t, result))

	if preview.CurrentState == nil {
		t.Error("current_state is absent, want the fetched StackScript")
	}

	if preview.WouldExecute.Body == nil {
		t.Error("would_execute.body is absent, want the body the call would send")
	}

	wantSideEffects(t, preview.SideEffects, []string{
		`Label changes from "before" to "after".`,
		"The StackScript body is replaced.",
		"The StackScript description is updated.",
	})
}

// TestLinodeDomainRecordCreatePreviewNamesTheRecord covers the sentence's three
// shapes: the record alone, with a host, and with a target.
func TestLinodeDomainRecordCreatePreviewNamesTheRecord(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want string
	}{
		{
			name: "the record alone",
			args: map[string]any{domainIDArg: 333, keyType: "A", keyDryRun: true},
			want: "A new A record will be created in domain 333.",
		},
		{
			name: "with a host",
			args: map[string]any{domainIDArg: 333, keyType: "A", argName: "www", keyDryRun: true},
			want: `A new A record will be created in domain 333 for host "www".`,
		},
		{
			name: "with a host and a target",
			args: map[string]any{
				domainIDArg: 333, keyType: "A", argName: "www",
				keyTarget: targetPublic, keyDryRun: true,
			},
			want: `A new A record will be created in domain 333 for host "www" targeting "8.8.8.8".`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := requestWith(testCase.args)

			result, err := toolhooks.LinodeDomainRecordCreatePreview(
				t.Context(), &request, configFor(""), http.MethodPost,
				"/domains/333/records", map[string]any{keyType: "A"},
			)
			if err != nil {
				t.Fatalf("LinodeDomainRecordCreatePreview: %v", err)
			}

			preview := residuePreviewOf(t, resultText(t, result))

			if preview.CurrentState != nil {
				t.Errorf("current_state = %v, want none", preview.CurrentState)
			}

			wantSideEffects(t, preview.SideEffects, []string{testCase.want})
		})
	}
}

// TestLinodeDomainRecordUpdatePreviewDiffsAgainstTheFetchedRecord: the walk
// reports what changes rather than what was asked for, so a value equal to the
// one already stored says nothing.
func TestLinodeDomainRecordUpdatePreviewDiffsAgainstTheFetchedRecord(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want []string
	}{
		{
			name: "a changed name and target both report",
			args: map[string]any{
				domainIDArg: 333, "record_id": 555, keyDryRun: true,
				argName: recordNameNew, keyTarget: targetPublic,
			},
			want: []string{
				`Record name changes from "old" to "new".`,
				`Record target changes from "1.1.1.1" to "8.8.8.8".`,
			},
		},
		{
			name: "a value equal to the stored one says nothing",
			args: map[string]any{
				domainIDArg: 333, "record_id": 555, keyDryRun: true,
				argName: "old", keyTarget: "1.1.1.1",
			},
			want: nil,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := stubServer(t, `{"id":555,"name":"old","target":"1.1.1.1","type":"A"}`)
			request := requestWith(testCase.args)

			result, err := toolhooks.LinodeDomainRecordUpdatePreview(
				t.Context(), &request, configFor(server.URL), http.MethodPut,
				"/domains/333/records/555", nil,
			)
			if err != nil {
				t.Fatalf("LinodeDomainRecordUpdatePreview: %v", err)
			}

			preview := residuePreviewOf(t, resultText(t, result))

			if preview.CurrentState == nil {
				t.Error("current_state is absent, want the record as it stands")
			}

			wantSideEffects(t, preview.SideEffects, testCase.want)
		})
	}
}
