package spec

import (
	"fmt"
	"path/filepath"
)

// DowngradeDocumentToInReview clears invalid approval metadata on a document while preserving upstream source references.
func DowngradeDocumentToInReview(document Document, lastModified string) (Document, error) {
	if !document.Exists {
		return Document{}, fmt.Errorf("document does not exist")
	}
	if lastModified == "" {
		return Document{}, fmt.Errorf("last_modified is required")
	}

	updated, err := cloneDocument(document)
	if err != nil {
		return Document{}, err
	}

	updated.Status = "in-review"
	updated.ApprovedAt = ""
	updated.LastModified = lastModified
	updated.Fields["status"] = updated.Status
	updated.Fields["approved_at"] = updated.ApprovedAt
	updated.Fields["last_modified"] = updated.LastModified

	return updated, nil
}

// ResetDocumentToDraft clears invalid approval metadata and returns the document to draft.
func ResetDocumentToDraft(document Document, lastModified string) (Document, error) {
	if !document.Exists {
		return Document{}, fmt.Errorf("document does not exist")
	}
	if lastModified == "" {
		return Document{}, fmt.Errorf("last_modified is required")
	}

	updated, err := cloneDocument(document)
	if err != nil {
		return Document{}, err
	}

	updated.Status = "draft"
	updated.ApprovedAt = ""
	updated.LastModified = lastModified
	updated.Fields["status"] = updated.Status
	updated.Fields["approved_at"] = updated.ApprovedAt
	updated.Fields["last_modified"] = updated.LastModified

	switch filepath.Base(document.Path) {
	case "requirements.md":
		return updated, nil
	case "design.md":
		updated.SourceRequirementsApprovedAt = ""
		updated.Fields["source_requirements_approved_at"] = updated.SourceRequirementsApprovedAt
		return updated, nil
	case "tasks.md":
		updated.SourceDesignApprovedAt = ""
		updated.Fields["source_design_approved_at"] = updated.SourceDesignApprovedAt
		return updated, nil
	default:
		return Document{}, fmt.Errorf("unsupported document path %q", document.Path)
	}
}

func cloneDocument(document Document) (Document, error) {
	switch filepath.Base(document.Path) {
	case "requirements.md", "design.md", "tasks.md":
	default:
		return Document{}, fmt.Errorf("unsupported document path %q", document.Path)
	}

	updated := document
	updated.Fields = make(map[string]string, len(document.Fields))
	for key, value := range document.Fields {
		updated.Fields[key] = value
	}

	return updated, nil
}
