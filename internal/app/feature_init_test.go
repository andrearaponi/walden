package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/output"
)

func TestRunFeatureInitPrintsNormalizedFeatureAndCurrentPhase(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("expected git dir creation to succeed, got %v", err)
	}

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected working directory lookup to succeed, got %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	if err := os.Chdir(root); err != nil {
		t.Fatalf("expected chdir to temp repo to succeed, got %v", err)
	}

	if exitCode := Run([]string{"repo", "init"}, ioDiscard{}, ioDiscard{}); exitCode != 0 {
		t.Fatalf("expected repo init to succeed before feature init, got exit code %d", exitCode)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"feature", "init", "Todo App Demo"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected feature init to succeed, got exit code %d and stderr %q", exitCode, stderr.String())
	}

	rendered := stdout.String()
	for _, want := range []string{
		"feature scaffold initialized for todo-app-demo",
		".walden/specs/todo-app-demo/requirements.md",
		"Current phase: requirements",
		"Next action: Edit .walden/specs/todo-app-demo/requirements.md and move it to in-review",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected command output to contain %q, got %q", want, rendered)
		}
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunFeatureInitPrintsJSON(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("expected git dir creation to succeed, got %v", err)
	}

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected working directory lookup to succeed, got %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	if err := os.Chdir(root); err != nil {
		t.Fatalf("expected chdir to temp repo to succeed, got %v", err)
	}

	if exitCode := Run([]string{"repo", "init"}, ioDiscard{}, ioDiscard{}); exitCode != 0 {
		t.Fatalf("expected repo init to succeed before feature init, got exit code %d", exitCode)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"feature", "init", "test-json", "--json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected feature init --json to succeed, got exit code %d and stderr %q", exitCode, stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}
	if envelope.Command != "feature-init" {
		t.Fatalf("expected command feature-init, got %q", envelope.Command)
	}
	if !envelope.OK {
		t.Fatalf("expected ok=true, got false")
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
