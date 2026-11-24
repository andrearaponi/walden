package release

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/spec"
)

// FeatureEvidence is one feature as the release gate reads it, with the
// evidence of each leaf of its plan and no verdict.
type FeatureEvidence struct {
	Feature  spec.Feature
	Tree     spec.TaskTree
	Evidence []evidence.TaskEvidence
}

// ReadFeatureEvidence reads the documents and the ledger of one feature, each
// file once, and derives the evidence of each leaf as ReleaseCheck does. It
// judges nothing, executes no proof and no environment probe, and writes
// nothing in the repository: the only processes it starts are the git
// commands that read the code identity and the history of the plan.
//
// It fails, and derives nothing, when the feature has no directory, when a
// document or the ledger cannot be read, when the requirements or the plan
// are missing, and when the plan does not parse. Each message names the
// feature or the file from the repository root, then the cause.
func ReadFeatureEvidence(ctx context.Context, root, featureName string) (FeatureEvidence, error) {
	root, err := resolveRoot(root)
	if err != nil {
		return FeatureEvidence{}, err
	}
	name, err := spec.NormalizeFeatureName(featureName)
	if err != nil {
		return FeatureEvidence{}, err
	}
	if _, err := os.Stat(filepath.Join(root, ".walden", "specs", name)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FeatureEvidence{}, fmt.Errorf("feature %q %w", name, spec.ErrFeatureNotFound)
		}
		return FeatureEvidence{}, fmt.Errorf("inspect feature directory: %w", err)
	}

	snapshot := captureFeature(root, name, false)
	if snapshot.LoadErr != nil {
		return FeatureEvidence{}, snapshot.LoadErr
	}
	for _, required := range []struct {
		name     string
		document spec.Document
	}{
		{"requirements.md", snapshot.Feature.Requirements},
		{"tasks.md", snapshot.Feature.Tasks},
	} {
		if !required.document.Exists {
			return FeatureEvidence{}, fmt.Errorf(".walden/specs/%s/%s does not exist", name, required.name)
		}
	}
	tree, err := spec.ParseTaskTree(snapshot.Feature.Tasks)
	if err != nil {
		return FeatureEvidence{}, fmt.Errorf(".walden/specs/%s/tasks.md: %w", name, err)
	}
	if snapshot.LedgerErr != nil {
		return FeatureEvidence{}, snapshot.LedgerErr
	}

	code := captureCodeIdentity(ctx, root)
	return FeatureEvidence{
		Feature:  snapshot.Feature,
		Tree:     tree,
		Evidence: deriveLeafEvidence(ctx, root, snapshot.Feature, tree, snapshot.Ledger, code.digest, code.ok, code.commit),
	}, nil
}
