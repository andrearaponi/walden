package spec

// Stale cause strings are stable identifiers shared by blockers, validation
// errors, and JSON output. Changing them is a contract change.
const (
	CauseFingerprintMissing   = "approval fingerprint missing"
	CauseFingerprintMalformed = "approval fingerprint malformed"
	CauseContentMismatch      = "content does not match approval fingerprint"
	CauseSourceMismatch       = "source fingerprint mismatch"
	CauseUpstreamStale        = "upstream document is stale"
	CauseUpstreamNotApproved  = "upstream document is not approved"
)

// DocumentFreshness is the fingerprint-based verdict for one document.
// Non-approved documents are vacuously intact and fresh: they are not yet
// bound to any approved content.
type DocumentFreshness struct {
	Intact bool
	Fresh  bool
	Causes []string
}

// FreshnessReport is the single verdict source for workflow state, review
// gates, validation, and reconciliation.
type FreshnessReport struct {
	Requirements DocumentFreshness
	Design       DocumentFreshness
	Tasks        DocumentFreshness
}

// EvaluateFreshness computes fingerprint-based integrity and chain freshness
// for a feature. Fingerprint comparison alone decides every verdict;
// timestamp fields are not consulted.
func EvaluateFreshness(feature Feature) FreshnessReport {
	report := FreshnessReport{
		Requirements: documentIntegrity(feature.Requirements),
		Design:       documentIntegrity(feature.Design),
		Tasks:        documentIntegrity(feature.Tasks),
	}

	evaluateChainLink(
		&report.Design,
		feature.Design,
		feature.Design.SourceRequirementsFingerprint,
		feature.Requirements,
		report.Requirements,
	)
	evaluateChainLink(
		&report.Tasks,
		feature.Tasks,
		feature.Tasks.SourceDesignFingerprint,
		feature.Design,
		report.Design,
	)

	// Cascade: a stale approved design invalidates tasks regardless of the
	// tasks document's own link.
	if feature.Design.Exists && feature.Design.Status == "approved" && !report.Design.Fresh {
		if feature.Tasks.Exists && report.Tasks.Fresh {
			report.Tasks.Fresh = false
			report.Tasks.Causes = append(report.Tasks.Causes, CauseUpstreamStale)
		}
	}

	return report
}

// documentIntegrity verifies an approved document's body against its own
// approval fingerprint. Missing, malformed, and mismatching fingerprints all
// fail closed with a distinct cause.
func documentIntegrity(document Document) DocumentFreshness {
	if !document.Exists || document.Status != "approved" {
		return DocumentFreshness{Intact: true, Fresh: true}
	}

	switch {
	case document.ApprovedFingerprint == "":
		return DocumentFreshness{Causes: []string{CauseFingerprintMissing}}
	case !ValidFingerprint(document.ApprovedFingerprint):
		return DocumentFreshness{Causes: []string{CauseFingerprintMalformed}}
	case !BodyMatchesFingerprint(document.Path, document.Body, document.ApprovedFingerprint):
		return DocumentFreshness{Causes: []string{CauseContentMismatch}}
	default:
		return DocumentFreshness{Intact: true, Fresh: true}
	}
}

// evaluateChainLink binds an approved downstream document to its upstream:
// the recorded source fingerprint must equal the upstream's current approval
// fingerprint, and the upstream itself must be approved and intact.
func evaluateChainLink(verdict *DocumentFreshness, downstream Document, sourceFingerprint string, upstream Document, upstreamVerdict DocumentFreshness) {
	if !downstream.Exists || downstream.Status != "approved" || !verdict.Intact {
		return
	}

	switch {
	case upstream.Status != "approved":
		verdict.Fresh = false
		verdict.Causes = append(verdict.Causes, CauseUpstreamNotApproved)
	case !upstreamVerdict.Intact:
		verdict.Fresh = false
		verdict.Causes = append(verdict.Causes, CauseUpstreamStale)
	case sourceFingerprint != upstream.ApprovedFingerprint:
		verdict.Fresh = false
		verdict.Causes = append(verdict.Causes, CauseSourceMismatch)
	}
}
