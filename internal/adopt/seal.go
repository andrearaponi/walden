package adopt

import (
	"errors"

	"github.com/andrearaponi/walden/internal/spec"
)

// sealFeature stamps absent approval fingerprints from current bodies and
// repairs empty downstream source links to the upstream seal, writing only
// through spec.SaveDocument. Bodies, statuses, and approval timestamps stay
// untouched: the seal records what was already approved, nothing more.
// Idempotent — a sealed chain yields zero writes — and inert on documents
// whose fingerprints are present, wrong included: those are blocked upstream
// and never reach here with intent to write. A link without a fingerprint
// whose times disagree is refused with an error, before anything is written.
func sealFeature(root, name string) ([]string, error) {
	feature, err := spec.LoadFeature(root, name)
	if err != nil {
		return nil, err
	}

	// The classifier keeps such a feature away from here. This second
	// reading holds when the documents changed since the plan, or when the
	// sealer is reached by another path: nothing is stamped or saved.
	if reason := legacyLinkDisagreement(feature); reason != "" {
		return nil, errors.New(reason)
	}

	requirements, design, tasks := feature.Requirements, feature.Design, feature.Tasks
	requirementsChanged := sealDocument(&requirements)
	designChanged := sealDocument(&design)
	tasksChanged := sealDocument(&tasks)

	// Chain repair: an empty link joins the (possibly fresh) upstream seal.
	// Links that are present were vetted by the classifier before this runs.
	if design.Exists && design.Status == "approved" && requirements.Status == "approved" &&
		design.Fields["source_requirements_fingerprint"] == "" && requirements.Fields["approved_fingerprint"] != "" {
		design.Fields["source_requirements_fingerprint"] = requirements.Fields["approved_fingerprint"]
		design.SourceRequirementsFingerprint = requirements.Fields["approved_fingerprint"]
		designChanged = true
	}
	if tasks.Exists && tasks.Status == "approved" && design.Status == "approved" &&
		tasks.Fields["source_design_fingerprint"] == "" && design.Fields["approved_fingerprint"] != "" {
		tasks.Fields["source_design_fingerprint"] = design.Fields["approved_fingerprint"]
		tasks.SourceDesignFingerprint = design.Fields["approved_fingerprint"]
		tasksChanged = true
	}

	sealed := []string{}
	for _, entry := range []struct {
		label    string
		changed  bool
		document spec.Document
	}{
		{"requirements.md", requirementsChanged, requirements},
		{"design.md", designChanged, design},
		{"tasks.md", tasksChanged, tasks},
	} {
		if !entry.changed {
			continue
		}
		if err := spec.SaveDocument(entry.document); err != nil {
			return sealed, err
		}
		sealed = append(sealed, entry.label)
	}
	return sealed, nil
}

// sealDocument stamps the current body's fingerprint on an approved document
// that has none. Everything else — drafts, in-review documents, documents
// already carrying a fingerprint — is left exactly as it is.
func sealDocument(document *spec.Document) bool {
	if !document.Exists || document.Status != "approved" || document.ApprovedFingerprint != "" {
		return false
	}
	fingerprint := spec.Fingerprint(document.Path, document.Body)
	document.ApprovedFingerprint = fingerprint
	document.Fields["approved_fingerprint"] = fingerprint
	return true
}
