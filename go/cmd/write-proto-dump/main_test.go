package main_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"testing"
)

const (
	// repoRoot is where the dumper runs from: it walks the tool trees itself.
	repoRoot = "../.."
	// wantProto is the classification a handler routing its answer through the
	// proto marshaller carries.
	wantProto = "proto"
)

func TestReadDumpRecognizesNullPreservingProtoHandler(t *testing.T) {
	t.Parallel()

	cmd := exec.CommandContext(t.Context(), "go", "run", "./cmd/write-proto-dump", "-surface", "read")
	cmd.Dir = repoRoot

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("write-proto-dump failed: %v\nstderr: %s", err, stderr.Bytes())
	}

	var got map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout.Bytes())
	}

	const toolName = "linode_networking_reserved_ip_list"
	if got[toolName] != wantProto {
		t.Errorf("classification for %s = %q, want %s", toolName, got[toolName], wantProto)
	}
}

func TestWriteDumpRecognizesReservedIPDeleteProtoHandler(t *testing.T) {
	t.Parallel()

	cmd := exec.CommandContext(t.Context(), "go", "run", "./cmd/write-proto-dump", "-surface", "write")
	cmd.Dir = repoRoot

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("write-proto-dump failed: %v\nstderr: %s", err, stderr.Bytes())
	}

	var got map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout.Bytes())
	}

	const toolName = "linode_networking_reserved_ip_delete"
	if got[toolName] != wantProto {
		t.Errorf("classification for %s = %q, want %s", toolName, got[toolName], wantProto)
	}
}

// A mutation whose answer restores the explicit nulls its decode dropped still
// goes through the proto marshaller, so it classifies as proto rather than as a
// handler nothing recognizes.
func TestWriteDumpRecognizesTheNullRestoringProtoSink(t *testing.T) {
	t.Parallel()

	cmd := exec.CommandContext(t.Context(), "go", "run", "./cmd/write-proto-dump", "-surface", "write")
	cmd.Dir = repoRoot

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("write-proto-dump failed: %v\nstderr: %s", err, stderr.Bytes())
	}

	var got map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout.Bytes())
	}

	for _, toolName := range []string{
		"linode_networking_reserved_ip_create",
		"linode_networking_reserved_ip_update",
	} {
		if got[toolName] != wantProto {
			t.Errorf("classification for %s = %q, want %s", toolName, got[toolName], wantProto)
		}
	}
}
