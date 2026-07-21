package main_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"testing"
)

func TestReadDumpRecognizesNullPreservingProtoHandler(t *testing.T) {
	t.Parallel()

	cmd := exec.CommandContext(t.Context(), "go", "run", "./cmd/write-proto-dump", "-surface", "read")
	cmd.Dir = "../.."

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
	if got[toolName] != "proto" {
		t.Errorf("classification for %s = %q, want proto", toolName, got[toolName])
	}
}
