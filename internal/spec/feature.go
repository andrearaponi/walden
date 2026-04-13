package spec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var featureNamePattern = regexp.MustCompile(`[^a-z0-9]+`)

// Document models a spec document loaded from disk.
type Document struct {
	Path                         string
	Status                       string
	ApprovedAt                   string
	LastModified                 string
	SourceRequirementsApprovedAt string
	SourceDesignApprovedAt       string
	Fields                       map[string]string
	Exists                       bool
	Body                         string
}

// Feature contains the document set for one Walden feature.
type Feature struct {
	Name         string
	Root         string
	Requirements Document
	Design       Document
	Tasks        Document
}

// NormalizeFeatureName canonicalizes a feature name to kebab-case.
func NormalizeFeatureName(raw string) (string, error) {
	normalized := featureNamePattern.ReplaceAllString(strings.ToLower(strings.TrimSpace(raw)), "-")
	normalized = strings.Trim(normalized, "-")
	normalized = strings.Join(strings.FieldsFunc(normalized, func(r rune) bool {
		return r == '-'
	}), "-")
	if normalized == "" {
		return "", errors.New("feature name cannot be empty")
	}

	return normalized, nil
}

// LoadFeature loads the three Walden documents for a feature from disk.
func LoadFeature(root, rawName string) (Feature, error) {
	name, err := NormalizeFeatureName(rawName)
	if err != nil {
		return Feature{}, err
	}

	featureRoot := filepath.Join(root, ".walden", "specs", name)
	if _, err := os.Stat(featureRoot); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Feature{}, fmt.Errorf("feature %q does not exist", name)
		}

		return Feature{}, fmt.Errorf("inspect feature directory: %w", err)
	}

	feature := Feature{
		Name: name,
		Root: featureRoot,
	}

	requirements, err := loadDocument(filepath.Join(featureRoot, "requirements.md"))
	if err != nil {
		return Feature{}, fmt.Errorf("load requirements.md: %w", err)
	}
	design, err := loadDocument(filepath.Join(featureRoot, "design.md"))
	if err != nil {
		return Feature{}, fmt.Errorf("load design.md: %w", err)
	}
	tasks, err := loadDocument(filepath.Join(featureRoot, "tasks.md"))
	if err != nil {
		return Feature{}, fmt.Errorf("load tasks.md: %w", err)
	}

	feature.Requirements = requirements
	feature.Design = design
	feature.Tasks = tasks

	return feature, nil
}

func loadDocument(path string) (Document, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Document{Path: path}, nil
		}

		return Document{}, err
	}

	values, body, err := parseFrontmatter(string(text))
	if err != nil {
		return Document{}, err
	}

	return Document{
		Path:                         path,
		Status:                       values.Status,
		ApprovedAt:                   values.ApprovedAt,
		LastModified:                 values.LastModified,
		SourceRequirementsApprovedAt: values.SourceRequirementsApprovedAt,
		SourceDesignApprovedAt:       values.SourceDesignApprovedAt,
		Fields:                       values.Fields,
		Exists:                       true,
		Body:                         body,
	}, nil
}
