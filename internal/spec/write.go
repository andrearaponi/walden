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

	if err := os.WriteFile(document.Path, []byte(builder.String()), 0o644); err != nil {
		return fmt.Errorf("write document: %w", err)
	}

	return nil
}

func orderedFrontmatterKeys(path string) ([]string, error) {
	switch filepath.Base(path) {
	case "requirements.md":
		return []string{"status", "approved_at", "last_modified"}, nil
	case "design.md":
		return []string{"status", "approved_at", "last_modified", "source_requirements_approved_at"}, nil
	case "tasks.md":
		return []string{"status", "approved_at", "last_modified", "source_design_approved_at"}, nil
	default:
		return nil, fmt.Errorf("unsupported document path %q", path)
	}
}
