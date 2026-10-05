package consolidate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/andrearaponi/walden/internal/spec"
)

// FeatureSnapshot is the consolidation-relevant state of one feature's
// requirements. A load error leaves the feature visible but never approved.
type FeatureSnapshot struct {
	Name                string
	HasRequirements     bool
	Status              string
	ApprovedFingerprint string
	Fresh               bool
	Definitions         Definitions
	Statements          map[string]string
	Citations           Citations
	LoadError           string
}

// Approved reports whether the feature has approved, fresh requirements.
func (f FeatureSnapshot) Approved() bool {
	return f.HasRequirements && f.Status == "approved" && f.Fresh
}

// Snapshot is one read-only pass over the portfolio's requirements, sorted
// by feature name.
type Snapshot struct {
	Features []FeatureSnapshot
}

// Feature returns the named feature.
func (s Snapshot) Feature(name string) (FeatureSnapshot, bool) {
	for _, feature := range s.Features {
		if feature.Name == name {
			return feature, true
		}
	}
	return FeatureSnapshot{}, false
}

// LoadSnapshot reads every feature's requirements without writing anything.
// A repository without specifications yields an empty snapshot.
func LoadSnapshot(root string) (Snapshot, error) {
	specsDir := filepath.Join(root, ".walden", "specs")
	entries, err := os.ReadDir(specsDir)
	if errors.Is(err, os.ErrNotExist) {
		return Snapshot{}, nil
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("read feature specs: %w", err)
	}

	snapshot := Snapshot{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		feature := FeatureSnapshot{Name: entry.Name()}
		path := filepath.Join(specsDir, entry.Name(), "requirements.md")
		text, err := os.ReadFile(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			feature.HasRequirements = true
			feature.LoadError = err.Error()
		default:
			feature.HasRequirements = true
			document, parseErr := spec.ParseDocument(path, text)
			if parseErr != nil {
				feature.LoadError = parseErr.Error()
				break
			}
			feature.Status = document.Status
			feature.ApprovedFingerprint = document.ApprovedFingerprint
			feature.Fresh = document.Status == "approved" && spec.BodyMatchesFingerprint(path, document.Body, document.ApprovedFingerprint)
			feature.Definitions = DefinedIdentifiers(document.Body)
			feature.Statements = StatementTexts(document.Body)
			feature.Citations = ParseCitations(document.Body)
		}
		snapshot.Features = append(snapshot.Features, feature)
	}
	return snapshot, nil
}
