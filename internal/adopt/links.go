package adopt

import (
	"fmt"

	"github.com/andrearaponi/walden/internal/spec"
)

// chainLink is one link of the approval chain as the documents record it.
type chainLink struct {
	downstream string // file name of the document that was approved later
	upstream   string // file name of the document it was approved against
	sealed     bool   // the downstream records a source fingerprint
	recordedAt string // the upstream approval time the downstream records
	approvedAt string // the approval time the upstream carries
}

// chainLinks returns the links of a feature whose two documents are both
// approved: design to requirements, then plan to design.
func chainLinks(feature spec.Feature) []chainLink {
	approved := func(document spec.Document) bool {
		return document.Exists && document.Status == "approved"
	}

	var links []chainLink
	if approved(feature.Design) && approved(feature.Requirements) {
		links = append(links, chainLink{
			downstream: "design.md",
			upstream:   "requirements.md",
			sealed:     feature.Design.SourceRequirementsFingerprint != "",
			recordedAt: feature.Design.SourceRequirementsApprovedAt,
			approvedAt: feature.Requirements.ApprovedAt,
		})
	}
	if approved(feature.Tasks) && approved(feature.Design) {
		links = append(links, chainLink{
			downstream: "tasks.md",
			upstream:   "design.md",
			sealed:     feature.Tasks.SourceDesignFingerprint != "",
			recordedAt: feature.Tasks.SourceDesignApprovedAt,
			approvedAt: feature.Design.ApprovedAt,
		})
	}
	return links
}

// timesDisagree compares the two values as text, as the releases that judged
// a chain by them did: every approval copies the upstream's value into the
// link, so any difference means the downstream followed another approval.
func (l chainLink) timesDisagree() bool {
	return l.recordedAt != l.approvedAt
}

// times says what the two documents of a link record about the approval of
// the upstream.
func (l chainLink) times() string {
	recorded := fmt.Sprintf("%s records %s approved at %s", l.downstream, l.upstream, l.recordedAt)
	if l.recordedAt == "" {
		recorded = fmt.Sprintf("%s records no approval time for %s", l.downstream, l.upstream)
	}
	carried := fmt.Sprintf("%s was approved at %s", l.upstream, l.approvedAt)
	if l.approvedAt == "" {
		carried = fmt.Sprintf("%s carries no approval time", l.upstream)
	}
	return recorded + ", but " + carried
}

// legacyLinkDisagreement returns the reason that blocks a feature: its first
// link without a fingerprint whose times disagree. It returns "" when no
// link is in that state.
//
// A link without a fingerprint still records which approval of its upstream
// the downstream followed. When that is not the approval the upstream
// carries, the releases that judged a chain by these times called it stale,
// and joining the two documents would seal a review that never happened.
func legacyLinkDisagreement(feature spec.Feature) string {
	for _, link := range chainLinks(feature) {
		if link.sealed || !link.timesDisagree() {
			continue
		}
		return fmt.Sprintf("%s: %s was approved against another approval — run walden reconcile %s and re-approve",
			link.times(), link.downstream, feature.Name)
	}
	return ""
}

// sealedLinkWarnings returns one warning for each link that has a
// fingerprint and whose times disagree.
//
// Two histories leave that record. A backfill by v0.10.0 to v0.15.2 joined
// every empty link without reading its time: on a chain the times called
// stale the fingerprint is new and the time is the old one, and a review is
// missing. From v0.4.0 to v0.9.0 an upstream could be approved again while
// its downstream stayed approved: with unchanged content the fingerprint the
// downstream recorded still matches, and nothing is missing. The documents
// do not say which history is theirs, and the seal binds the content as it
// is, so the class does not change: the plan reports the link, and a person
// tells the two apart and gives the review where it is missing.
func sealedLinkWarnings(feature spec.Feature) []string {
	var warnings []string
	for _, link := range chainLinks(feature) {
		if !link.sealed || !link.timesDisagree() {
			continue
		}
		recorded := fmt.Sprintf("records it approved at %s", link.recordedAt)
		if link.recordedAt == "" {
			recorded = "records no approval time for it"
		}
		carried := fmt.Sprintf("%s was approved at %s", link.upstream, link.approvedAt)
		if link.approvedAt == "" {
			carried = fmt.Sprintf("%s carries no approval time", link.upstream)
		}
		warnings = append(warnings, fmt.Sprintf(
			"%s is sealed to %s but %s, and %s: either %s was approved again without a change, or a backfill by v0.10.0 to v0.15.2 joined them without a review — the adoption guide says how to tell; to clear this, open for review the approved documents below %s, the last one first, and approve them again, the first one first",
			link.downstream, link.upstream, recorded, carried, link.upstream, link.upstream))
	}
	return warnings
}
