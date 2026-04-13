package repo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitFeatureNormalizesNameAndCreatesSpecDocuments(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git"))

	if _, err := Init(root); err != nil {
		t.Fatalf("expected repo init to succeed, got %v", err)
	}

	report, err := InitFeature(root, "Todo App Demo")
	if err != nil {
		t.Fatalf("expected feature init to succeed, got %v", err)
	}

	if report.FeatureName != "todo-app-demo" {
		t.Fatalf("expected normalized feature name, got %q", report.FeatureName)
	}
	if report.CurrentPhase != "requirements" {
		t.Fatalf("expected current phase requirements, got %q", report.CurrentPhase)
	}

	wantCreated := []string{
		".walden/specs/todo-app-demo/requirements.md",
		".walden/specs/todo-app-demo/design.md",
		".walden/specs/todo-app-demo/tasks.md",
	}

	for _, want := range wantCreated {
		if !contains(report.CreatedFiles, want) {
			t.Fatalf("expected created files to include %q, got %#v", want, report.CreatedFiles)
		}
		if _, err := os.Stat(filepath.Join(root, want)); err != nil {
			t.Fatalf("expected file %q to exist, got %v", want, err)
		}
	}

	if len(report.SkippedFiles) != 0 {
		t.Fatalf("expected no skipped files on first run, got %#v", report.SkippedFiles)
	}
}

func TestInitFeatureSkipsExistingScaffoldWithoutOverwriting(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git"))

	if _, err := Init(root); err != nil {
		t.Fatalf("expected repo init to succeed, got %v", err)
	}
	if _, err := InitFeature(root, "todo-app-demo"); err != nil {
		t.Fatalf("expected first feature init to succeed, got %v", err)
	}

	requirementsPath := filepath.Join(root, ".walden/specs/todo-app-demo/requirements.md")
	before, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("expected requirements file to exist, got %v", err)
	}

	report, err := InitFeature(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("expected second feature init to succeed, got %v", err)
	}

	after, err := os.ReadFile(requirementsPath)
	if err != nil {
		t.Fatalf("expected requirements file to still exist, got %v", err)
	}

	if string(before) != string(after) {
		t.Fatal("expected existing requirements scaffold to remain unchanged")
	}
	if len(report.CreatedFiles) != 0 {
		t.Fatalf("expected no created files on second run, got %#v", report.CreatedFiles)
	}
	if !contains(report.SkippedFiles, ".walden/specs/todo-app-demo/requirements.md") {
		t.Fatalf("expected skipped files to include requirements.md, got %#v", report.SkippedFiles)
	}
	if !report.AlreadyExists {
		t.Fatal("expected second run to report existing feature")
	}
}

func TestInitFeatureFailsFastWhenRepoIsNotInitialized(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git"))

	if _, err := InitFeature(root, "todo-app-demo"); err == nil {
		t.Fatal("expected feature init to fail before repo init")
	}
}
