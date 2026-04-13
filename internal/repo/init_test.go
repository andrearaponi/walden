package repo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesBaselineWaldenFiles(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git"))

	report, err := Init(root)
	if err != nil {
		t.Fatalf("expected init to succeed, got %v", err)
	}

	wantCreated := []string{
		".walden/constitution.md",
		".walden/lessons.md",
		".github/pull_request_template.md",
		".github/workflows/validate-walden.yml",
	}

	for _, want := range wantCreated {
		if !contains(report.CreatedFiles, want) {
			t.Fatalf("expected created files to include %q, got %#v", want, report.CreatedFiles)
		}
		if _, err := os.Stat(filepath.Join(root, want)); err != nil {
			t.Fatalf("expected file %q to exist, got %v", want, err)
		}
	}

	if len(report.UpdatedFiles) != 0 {
		t.Fatalf("expected no updated files on first init, got %#v", report.UpdatedFiles)
	}
	if len(report.SkippedFiles) != 0 {
		t.Fatalf("expected no skipped files on first init, got %#v", report.SkippedFiles)
	}
	if !report.GitAlreadyInitialized {
		t.Fatalf("expected repo init to report an existing git repository, got %#v", report)
	}
	if report.GitInitialized {
		t.Fatalf("expected repo init not to create git metadata when .git already exists, got %#v", report)
	}
}

func TestInitAutoInitializesGitBeforeWritingFiles(t *testing.T) {
	root := t.TempDir()

	previousRunner := gitInitRunner
	gitInitRunner = func(root string) error {
		return os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	}
	t.Cleanup(func() {
		gitInitRunner = previousRunner
	})

	report, err := Init(root)
	if err != nil {
		t.Fatalf("expected init to auto-bootstrap git, got %v", err)
	}

	if !report.GitInitialized {
		t.Fatalf("expected init to report git bootstrap, got %#v", report)
	}
	if report.GitAlreadyInitialized {
		t.Fatalf("expected init not to report a pre-existing git repository, got %#v", report)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Fatalf("expected git metadata to exist after bootstrap, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".walden", "lessons.md")); err != nil {
		t.Fatalf("expected Walden files to be written after git bootstrap, got %v", err)
	}
}

func TestInitFailsWithoutPartialWritesWhenGitBootstrapFails(t *testing.T) {
	root := t.TempDir()

	previousRunner := gitInitRunner
	gitInitRunner = func(string) error {
		return errors.New("git unavailable")
	}
	t.Cleanup(func() {
		gitInitRunner = previousRunner
	})

	_, err := Init(root)
	if err == nil {
		t.Fatal("expected init to fail when git bootstrap fails")
	}

	for _, path := range []string{
		".walden",
		".github",
	} {
		if _, statErr := os.Stat(filepath.Join(root, path)); !os.IsNotExist(statErr) {
			t.Fatalf("expected init to avoid partial writes for %q, got stat error %v", path, statErr)
		}
	}
}

func TestInitIsIdempotentOnSecondRun(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".git"))

	if _, err := Init(root); err != nil {
		t.Fatalf("expected first init to succeed, got %v", err)
	}

	report, err := Init(root)
	if err != nil {
		t.Fatalf("expected second init to succeed, got %v", err)
	}

	if len(report.CreatedFiles) != 0 {
		t.Fatalf("expected no created files on second init, got %#v", report.CreatedFiles)
	}
	if len(report.UpdatedFiles) != 0 {
		t.Fatalf("expected no updated files on second init, got %#v", report.UpdatedFiles)
	}
	if len(report.SkippedFiles) == 0 {
		t.Fatal("expected second init to report skipped files")
	}
	if !contains(report.SkippedFiles, ".walden/lessons.md") {
		t.Fatalf("expected skipped files to include lessons file, got %#v", report.SkippedFiles)
	}
	if !report.GitAlreadyInitialized {
		t.Fatalf("expected second init to detect existing git metadata, got %#v", report)
	}
	if report.GitInitialized {
		t.Fatalf("expected second init not to bootstrap git, got %#v", report)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("expected mkdir %q to succeed, got %v", path, err)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}
