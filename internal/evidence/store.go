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

	document, err := Decode(data, feature)
	if err != nil {
		return Document{}, fmt.Errorf("evidence document %s: %w", DocumentPath(root, feature), err)
	}
	return document, nil
}

// Decode judges captured bytes without reading a second version of the file.
// Legacy entries keep their original fields; a new container grants no trust.
func Decode(data []byte, feature string) (Document, error) {
	var document Document
	if err := json.Unmarshal(data, &document); err != nil {
		return Document{}, fmt.Errorf("parse: %w; retain the ledger for diagnosis", err)
	}
	switch document.SchemaVersion {
	case "", "v1alpha1", SchemaVersion, AttestedSchemaVersion:
	default:
		return Document{}, fmt.Errorf("schema %s is unsupported; this binary writes %s, and %s for a ledger that holds an attestation — retain the ledger and use a compatible walden reader", document.SchemaVersion, SchemaVersion, AttestedSchemaVersion)
	}
	if document.Feature != "" && document.Feature != feature {
		return Document{}, fmt.Errorf("ledger belongs to feature %q, not %q — retain it and inspect its provenance", document.Feature, feature)
	}
	if document.Tasks == nil {
		document.Tasks = map[string]Record{}
	}
	return document, nil
}

// Save persists the document atomically, stamping the schema version its
// content calls for: the version a document carries is never written back.
func Save(root string, document Document) error {
	document.SchemaVersion = document.schemaVersion()
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

// schemaVersion follows the content: a ledger in which a record holds an
// attestation is written in the format earlier releases refuse, and any other
// in the format they read. A ledger whose last attestation is gone goes back
// to it: there is nothing left for an earlier release to replace.
func (d Document) schemaVersion() string {
	for _, record := range d.Tasks {
		if record.Attestation != nil {
			return AttestedSchemaVersion
		}
	}
	return SchemaVersion
}
