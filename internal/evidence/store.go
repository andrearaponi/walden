package evidence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/andrearaponi/walden/internal/spec"
)

// DocumentPath returns the evidence document location for a feature. It
// lives inside the committed .walden/ tree deliberately: evidence is shared
// repository state, not a local cache.
func DocumentPath(root, feature string) string {
	return filepath.Join(root, ".walden", "evidence", feature+".json")
}

// Load reads the feature's evidence document. An absent file is an empty
// ledger, not an error.
func Load(root, feature string) (Document, error) {
	data, err := os.ReadFile(DocumentPath(root, feature))
	if errors.Is(err, os.ErrNotExist) {
		return Document{SchemaVersion: SchemaVersion, Feature: feature, Tasks: map[string]Record{}}, nil
	}
	if err != nil {
		return Document{}, fmt.Errorf("read evidence document: %w", err)
	}

	var document Document
	if err := json.Unmarshal(data, &document); err != nil {
		return Document{}, fmt.Errorf("parse evidence document %s: %w", DocumentPath(root, feature), err)
	}
	if document.Tasks == nil {
		document.Tasks = map[string]Record{}
	}
	return document, nil
}

// Save persists the document atomically, stamping the schema version.
func Save(root string, document Document) error {
	document.SchemaVersion = SchemaVersion
	path := DocumentPath(root, document.Feature)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create evidence directory: %w", err)
	}

	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode evidence document: %w", err)
	}
	return spec.WriteFileAtomic(path, append(data, '\n'))
}
