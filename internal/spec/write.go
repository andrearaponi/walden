package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SaveDocument persists a document with stable frontmatter ordering.
func SaveDocument(document Document) error {
	if document.Path == "" {
		return fmt.Errorf("document path is required")
	}

	if err := os.MkdirAll(filepath.Dir(document.Path), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	orderedKeys, err := orderedFrontmatterKeys(document.Path)
	if err != nil {
		return err
	}

	var builder strings.Builder
	builder.WriteString("---\n")
	for _, key := range orderedKeys {
		builder.WriteString(key)
		builder.WriteString(": ")
		builder.WriteString(document.Fields[key])
		builder.WriteString("\n")
	}
	builder.WriteString("---\n\n")
	builder.WriteString(strings.TrimLeft(document.Body, "\n"))

	if !strings.HasSuffix(builder.String(), "\n") {
		builder.WriteString("\n")
	}

	return WriteFileAtomic(document.Path, []byte(builder.String()))
}

// WriteFileAtomic stages the content in a temp file inside the target's
// directory and renames it into place, so no failure path — interruption,
// full disk, permission error — can leave the target truncated or lost.
func WriteFileAtomic(path string, content []byte) error {
	staged, err := os.CreateTemp(filepath.Dir(path), ".walden-doc-*")
	if err != nil {
		return fmt.Errorf("stage document %s: %w", path, err)
	}
	stagedPath := staged.Name()
	cleanup := func() { _ = os.Remove(stagedPath) }

	if _, err := staged.Write(content); err != nil {
		_ = staged.Close()
		cleanup()
		return fmt.Errorf("stage document %s: %w", path, err)
	}
	if err := staged.Close(); err != nil {
		cleanup()
		return fmt.Errorf("stage document %s: %w", path, err)
	}
	if err := os.Chmod(stagedPath, 0o644); err != nil {
		cleanup()
		return fmt.Errorf("stage document %s: %w", path, err)
	}
	if err := os.Rename(stagedPath, path); err != nil {
		cleanup()
		return fmt.Errorf("write document %s: %w", path, err)
	}
	return nil
}

func orderedFrontmatterKeys(path string) ([]string, error) {
	switch filepath.Base(path) {
	case "requirements.md":
		return []string{"status", "approved_at", "last_modified", "approved_fingerprint"}, nil
	case "design.md":
		return []string{"status", "approved_at", "last_modified", "approved_fingerprint", "source_requirements_approved_at", "source_requirements_fingerprint"}, nil
	case "tasks.md":
		return []string{"status", "approved_at", "last_modified", "approved_fingerprint", "source_design_approved_at", "source_design_fingerprint"}, nil
	default:
		return nil, fmt.Errorf("unsupported document path %q", path)
	}
}
