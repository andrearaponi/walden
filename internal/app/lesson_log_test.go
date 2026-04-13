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

func TestRunLessonLogPrintsHumanReadableSuccess(t *testing.T) {
	root := t.TempDir()

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected working directory lookup to succeed, got %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("expected chdir to succeed, got %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{
		"lesson", "log",
		"--feature", "Todo App Demo",
		"--phase", "execute",
		"--trigger", "missing lesson command",
		"--lesson", "deterministic helpers should replace legacy scripts incrementally",
		"--guardrail", "close each missing helper before exposing it in the skill",
	}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected lesson log to succeed, got %d and stderr %q", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	rendered := stdout.String()
	for _, want := range []string{
		"Summary: lesson logged for todo-app-demo",
		"Changed files:",
		".walden/lessons.md",
		"Next action: Review .walden/lessons.md before similar future work",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected output to contain %q, got %q", want, rendered)
		}
	}

	lessonsPath := filepath.Join(root, ".walden", "lessons.md")
	content, err := os.ReadFile(lessonsPath)
	if err != nil {
		t.Fatalf("expected lessons file to be readable, got %v", err)
	}
	for _, want := range []string{
		"# Walden Lessons",
		"### ",
		"| todo-app-demo | execute",
		"- Trigger: missing lesson command",
		"- Lesson: deterministic helpers should replace legacy scripts incrementally",
		"- Guardrail: close each missing helper before exposing it in the skill",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("expected lessons file to contain %q, got %q", want, string(content))
		}
	}
}

func TestRunLessonLogPrintsJSONSuccess(t *testing.T) {
	root := t.TempDir()

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected working directory lookup to succeed, got %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("expected chdir to succeed, got %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{
		"lesson", "log",
		"--feature", "full-deterministic-skill-coverage",
		"--phase", "tasks",
		"--trigger", "new CLI helper landed",
		"--lesson", "keep lesson logging in the CLI, not in ad hoc scripts",
		"--guardrail", "prefer one deterministic helper per repeated workflow mutation",
		"--json",
	}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected lesson log json mode to succeed, got %d and stderr %q", exitCode, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr for json mode, got %q", stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	result := envelope.Result
	if result.Summary != "lesson logged for full-deterministic-skill-coverage" {
		t.Fatalf("unexpected summary: %q", result.Summary)
	}
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != ".walden/lessons.md" {
		t.Fatalf("unexpected changed files: %#v", result.ChangedFiles)
	}
	if result.NextAction != "Review .walden/lessons.md before similar future work" {
		t.Fatalf("unexpected next action: %q", result.NextAction)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestRunLessonLogRejectsInvalidInputWithoutWritingFile(t *testing.T) {
	root := t.TempDir()

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected working directory lookup to succeed, got %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("expected chdir to succeed, got %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{
		"lesson", "log",
		"--feature", "todo-app-demo",
		"--phase", "planning",
		"--trigger", "bad phase",
		"--lesson", "lesson text",
		"--guardrail", "guardrail text",
		"--json",
	}, &stdout, &stderr)

	if exitCode != 1 {
		t.Fatalf("expected invalid lesson log to fail, got %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr for json mode, got %q", stderr.String())
	}

	var envelope output.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("expected valid json output, got %v", err)
	}
	result := envelope.Result
	if !strings.Contains(result.Summary, `phase must be one of "requirements", "design", "tasks", "execute", "release"`) {
		t.Fatalf("unexpected summary: %q", result.Summary)
	}
	if result.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", result.ExitCode)
	}
	if _, err := os.Stat(filepath.Join(root, ".walden", "lessons.md")); !os.IsNotExist(err) {
		t.Fatalf("expected lessons file to remain absent, got %v", err)
	}
}
