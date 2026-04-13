package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendLessonBootstrapsLessonsFileAndAppendsCanonicalEntry(t *testing.T) {
	root := t.TempDir()

	entry, lessonsPath, err := AppendLesson(root, LessonEntry{
		Feature:   "Todo App Demo",
		Phase:     "design",
		Trigger:   " design constraints added after approval request ",
		Lesson:    " capture workflow constraints before approval ",
		Guardrail: " confirm toolchain and testing constraints before sign-off ",
		LoggedAt:  "2026-03-22T08:00:00Z",
	})
	if err != nil {
		t.Fatalf("expected append lesson to succeed, got %v", err)
	}

	if entry.Feature != "todo-app-demo" {
		t.Fatalf("expected normalized feature name, got %q", entry.Feature)
	}
	if lessonsPath != filepath.Join(root, ".walden", "lessons.md") {
		t.Fatalf("unexpected lessons path: %q", lessonsPath)
	}

	content, err := os.ReadFile(lessonsPath)
	if err != nil {
		t.Fatalf("expected lessons file to be readable, got %v", err)
	}

	expected := `# Walden Lessons

Review this file before non-trivial work when the current request matches past mistakes, rejections, or validation failures.

## Lessons

<!-- Append entries with scripts/log_walden_lesson.py -->
### 2026-03-22T08:00:00Z | todo-app-demo | design
- Trigger: design constraints added after approval request
- Lesson: capture workflow constraints before approval
- Guardrail: confirm toolchain and testing constraints before sign-off

`
	if string(content) != expected {
		t.Fatalf("unexpected lessons content:\n%s", string(content))
	}
}

func TestAppendLessonPreservesExistingEntriesAndAppendsNewOne(t *testing.T) {
	root := t.TempDir()
	lessonsPath := filepath.Join(root, ".walden", "lessons.md")
	if err := os.MkdirAll(filepath.Dir(lessonsPath), 0o755); err != nil {
		t.Fatalf("expected lessons directory creation to succeed, got %v", err)
	}
	existing := `# Walden Lessons

Review this file before non-trivial work when the current request matches past mistakes, rejections, or validation failures.

## Lessons

<!-- Append entries with scripts/log_walden_lesson.py -->
### 2026-03-21T14:07:51Z | repo-init-and-review-flow | design
- Trigger: existing trigger
- Lesson: existing lesson
- Guardrail: existing guardrail

`
	if err := os.WriteFile(lessonsPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("expected existing lessons file write to succeed, got %v", err)
	}

	_, _, err := AppendLesson(root, LessonEntry{
		Feature:   "full-deterministic-skill-coverage",
		Phase:     "execute",
		Trigger:   "missing lesson command",
		Lesson:    "deterministic helpers must replace legacy scripts incrementally",
		Guardrail: "close the next missing helper before exposing the command in the skill",
		LoggedAt:  "2026-03-22T08:05:00Z",
	})
	if err != nil {
		t.Fatalf("expected append lesson to succeed, got %v", err)
	}

	content, err := os.ReadFile(lessonsPath)
	if err != nil {
		t.Fatalf("expected lessons file to be readable, got %v", err)
	}

	rendered := string(content)
	if strings.Count(rendered, "# Walden Lessons") != 1 {
		t.Fatalf("expected header to appear once, got:\n%s", rendered)
	}
	for _, want := range []string{
		"### 2026-03-21T14:07:51Z | repo-init-and-review-flow | design",
		"### 2026-03-22T08:05:00Z | full-deterministic-skill-coverage | execute",
		"- Guardrail: close the next missing helper before exposing the command in the skill",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected content to contain %q, got:\n%s", want, rendered)
		}
	}
}

func TestAppendLessonRejectsIncompleteEntryWithoutWritingFile(t *testing.T) {
	root := t.TempDir()
	lessonsPath := filepath.Join(root, ".walden", "lessons.md")

	_, _, err := AppendLesson(root, LessonEntry{
		Feature:  "todo-app-demo",
		Phase:    "tasks",
		Trigger:  "missing guardrail",
		Lesson:   "lesson text",
		LoggedAt: "2026-03-22T08:10:00Z",
	})
	if err == nil {
		t.Fatal("expected append lesson to fail on missing guardrail")
	}
	if !strings.Contains(err.Error(), "guardrail is required") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(lessonsPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected lessons file to remain absent, got %v", statErr)
	}
}

func TestAppendLessonRejectsInvalidPhaseWithoutWritingFile(t *testing.T) {
	root := t.TempDir()
	lessonsPath := filepath.Join(root, ".walden", "lessons.md")

	_, _, err := AppendLesson(root, LessonEntry{
		Feature:   "todo-app-demo",
		Phase:     "planning",
		Trigger:   "bad phase",
		Lesson:    "lesson text",
		Guardrail: "guardrail text",
		LoggedAt:  "2026-03-22T08:10:00Z",
	})
	if err == nil {
		t.Fatal("expected append lesson to fail on invalid phase")
	}
	if !strings.Contains(err.Error(), `phase must be one of "requirements", "design", "tasks", "execute", "release"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(lessonsPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected lessons file to remain absent, got %v", statErr)
	}
}
