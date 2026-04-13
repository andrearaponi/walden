package repo

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/templates"
)

// FeatureInitReport describes the outcome of scaffolding a feature spec.
type FeatureInitReport struct {
	FeatureName   string
	CurrentPhase  string
	CreatedFiles  []string
	SkippedFiles  []string
	AlreadyExists bool
}

type specTemplate struct {
	Source string
	Target string
}

var specTemplates = []specTemplate{
	{Source: "requirements.md.tmpl", Target: "requirements.md"},
	{Source: "design.md.tmpl", Target: "design.md"},
	{Source: "tasks.md.tmpl", Target: "tasks.md"},
}

// InitFeature creates the baseline Walden document set for a new feature.
func InitFeature(root, rawName string) (FeatureInitReport, error) {
	if err := ensureWaldenInitialized(root); err != nil {
		return FeatureInitReport{}, err
	}

	featureName, err := spec.NormalizeFeatureName(rawName)
	if err != nil {
		return FeatureInitReport{}, err
	}

	featureDir := filepath.Join(root, ".walden", "specs", featureName)
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		return FeatureInitReport{}, fmt.Errorf("create feature directory: %w", err)
	}

	report := FeatureInitReport{
		FeatureName:  featureName,
		CurrentPhase: "requirements",
	}

	specFS := templates.SpecFS()
	timestamp := utcNow()

	for _, specFile := range specTemplates {
		rendered, renderErr := renderSpecTemplate(specFS, specFile.Source, timestamp)
		if renderErr != nil {
			return FeatureInitReport{}, renderErr
		}

		target := filepath.Join(".walden", "specs", featureName, specFile.Target)
		state, writeErr := writeFeatureFile(root, target, rendered)
		if writeErr != nil {
			return FeatureInitReport{}, writeErr
		}

		switch state {
		case "created":
			report.CreatedFiles = append(report.CreatedFiles, target)
		case "skipped":
			report.SkippedFiles = append(report.SkippedFiles, target)
			report.AlreadyExists = true
		}
	}

	return report, nil
}

func ensureWaldenInitialized(root string) error {
	if err := requireGitRepository(root); err != nil {
		return err
	}

	specRoot := filepath.Join(root, ".walden", "specs")
	if _, err := os.Stat(specRoot); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("walden is not initialized: run `walden repo init` first")
		}

		return fmt.Errorf("inspect Walden repository state: %w", err)
	}

	return nil
}

func requireGitRepository(root string) error {
	gitPath := filepath.Join(root, ".git")
	if _, err := os.Stat(gitPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("walden is not initialized: run `walden repo init` first")
		}

		return fmt.Errorf("inspect git metadata: %w", err)
	}

	return nil
}

func utcNow() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func renderSpecTemplate(templateFS fs.FS, name, timestamp string) ([]byte, error) {
	raw, err := fs.ReadFile(templateFS, name)
	if err != nil {
		return nil, fmt.Errorf("read spec template %q: %w", name, err)
	}

	tmpl, err := template.New(name).Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse spec template %q: %w", name, err)
	}

	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, struct{ Timestamp string }{Timestamp: timestamp}); err != nil {
		return nil, fmt.Errorf("render spec template %q: %w", name, err)
	}

	return rendered.Bytes(), nil
}

func writeFeatureFile(root, target string, content []byte) (string, error) {
	targetPath := filepath.Join(root, target)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", fmt.Errorf("create parent directory for %q: %w", target, err)
	}

	if _, err := os.Stat(targetPath); err == nil {
		return "skipped", nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect %q: %w", target, err)
	}

	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		return "", fmt.Errorf("create %q: %w", target, err)
	}

	return "created", nil
}
