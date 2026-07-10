package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
)

func TestEmitResultJSONErrorEnvelope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	result := errorResult(errors.New("requirements.md is stale"))

	exitCode := emitResult("review-open", result, true, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v (got %q)", err, stdout.String())
	}
	if envelope.Command != "review-open" {
		t.Fatalf("command = %q, want review-open", envelope.Command)
	}
	if envelope.OK {
		t.Fatal("ok = true for a failing result")
	}
	if envelope.Result.Summary != "requirements.md is stale" {
		t.Fatalf("summary = %q, want the error text", envelope.Result.Summary)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr not empty in JSON mode: %q", stderr.String())
	}
}

func TestEmitResultTextErrorGoesToStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	result := errorResult(errors.New("boom happened"))
	result.NextAction = "Run walden reconcile"

	exitCode := emitResult("status", result, false, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout not empty for a text-mode error: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "boom happened") {
		t.Fatalf("stderr %q does not carry the error text", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Run walden reconcile") {
		t.Fatalf("stderr %q does not carry the next action", stderr.String())
	}
	if strings.Contains(stderr.String(), "{") {
		t.Fatalf("text mode leaked JSON: %q", stderr.String())
	}
}

func TestEmitResultTextSuccessUsesPrintText(t *testing.T) {
	var stdout, stderr bytes.Buffer
	result := output.Result{Summary: "all good", ExitCode: 0}

	exitCode := emitResult("status", result, false, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}
	if !strings.Contains(stdout.String(), "all good") {
		t.Fatalf("stdout %q does not carry the summary", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr not empty on success: %q", stderr.String())
	}
}

func TestEmitResultJSONSuccessEnvelope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	result := output.Result{Summary: "all good", ExitCode: 0}

	exitCode := emitResult("status", result, true, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}
	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v", err)
	}
	if !envelope.OK {
		t.Fatal("ok = false for a succeeding result")
	}
}
