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

func TestRunRepoInitPrintsBootstrapSummary(t *testing.T) {
	root := t.TempDir()

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

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"repo", "init"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected repo init to succeed, got exit code %d and stderr %q", exitCode, stderr.String())
	}

	rendered := stdout.String()
	for _, want := range []string{
		"repository initialized",
		"Git: initialized new repository",
		"Created files:",
		".walden/lessons.md",
		"Next action: Run walden feature init <name>",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected command output to contain %q, got %q", want, rendered)
		}
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Fatalf("expected repo init to bootstrap git metadata, got %v", err)
	}
}

func TestRunRepoInitPrintsJSON(t *testing.T) {
	root := t.TempDir()

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

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"repo", "init", "--json"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected repo init --json to succeed, got exit code %d and stderr %q", exitCode, stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}
	if envelope.SchemaVersion != "v0alpha1" {
		t.Fatalf("expected schema_version v0alpha1, got %q", envelope.SchemaVersion)
	}
	if envelope.Command != "repo-init" {
		t.Fatalf("expected command repo-init, got %q", envelope.Command)
	}
	if !envelope.OK {
		t.Fatalf("expected ok=true, got false")
	}
}

func TestRunRepoInitReportsExistingGitRepository(t *testing.T) {
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

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"repo", "init"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected repo init to succeed, got exit code %d and stderr %q", exitCode, stderr.String())
	}

	rendered := stdout.String()
	if !strings.Contains(rendered, "Git: repository already initialized") {
		t.Fatalf("expected command output to report existing git metadata, got %q", rendered)
	}
}
