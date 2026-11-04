package workflow

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
)

const (
	gateFeature   = "gate-demo"
	markerRemedy  = "resolve it, or quote it in a code span"
	commentRemedy = "close it with -->, or quote it in a code span"
)

func draftRequirementsContent(body string) string {
	return "---\nstatus: draft\napproved_at:\nlast_modified: 2026-03-21T14:00:00Z\napproved_fingerprint:\n---\n\n" + body
}

func draftDesignContent(body string) string {
	return "---\nstatus: draft\napproved_at:\nlast_modified: 2026-03-21T14:10:00Z\napproved_fingerprint:\nsource_requirements_approved_at:\nsource_requirements_fingerprint:\n---\n\n" + body
}

func draftTasksContent(body string) string {
	return "---\nstatus: draft\napproved_at:\nlast_modified: 2026-03-21T14:20:00Z\napproved_fingerprint:\nsource_design_approved_at:\nsource_design_fingerprint:\n---\n\n" + body
}

// gateFixture writes approved requirements, a draft design holding body and
// a draft plan: the state in which the design review opens.
func gateFixture(t *testing.T, designBody string) string {
	t.Helper()
	root := t.TempDir()
	writeReviewFeatureDoc(t, root, gateFeature, "requirements.md", approvedRequirementsContent(approveReqBody, "2026-03-21T14:00:00Z"))
	writeReviewFeatureDoc(t, root, gateFeature, "design.md", draftDesignContent(designBody))
	writeReviewFeatureDoc(t, root, gateFeature, "tasks.md", draftTasksContent(approveTasksBody))
	return root
}

func gateDocPath(root, name string) string {
	return filepath.Join(root, ".walden", "specs", gateFeature, name)
}

func readGateDoc(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(gateDocPath(root, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

// featureDigests hashes the three documents of the fixture, frontmatter
// included.
func featureDigests(t *testing.T, root string) map[string][sha256.Size]byte {
	t.Helper()
	digests := map[string][sha256.Size]byte{}
	for _, name := range []string{"requirements.md", "design.md", "tasks.md"} {
		digests[name] = sha256.Sum256([]byte(readGateDoc(t, root, name)))
	}
	return digests
}

func requireDigestsUnchanged(t *testing.T, root string, before map[string][sha256.Size]byte) {
	t.Helper()
	for name, digest := range featureDigests(t, root) {
		if digest != before[name] {
			t.Fatalf("%s changed although the review command was refused", name)
		}
	}
}

// gateBodyLine returns the body line, counted from 1, of the first line of
// the named document holding fragment.
func gateBodyLine(t *testing.T, root, name, fragment string) int {
	t.Helper()
	feature, err := spec.LoadFeature(root, gateFeature)
	if err != nil {
		t.Fatalf("load feature: %v", err)
	}
	document := fixtureDocumentByName(&feature, name)
	for index, line := range strings.Split(strings.ReplaceAll(document.Body, "\r\n", "\n"), "\n") {
		if strings.Contains(line, fragment) {
			return index + 1
		}
	}
	t.Fatalf("no body line of %s holds %q", name, fragment)
	return 0
}

func requireGateStatus(t *testing.T, root, name, want string) {
	t.Helper()
	feature, err := spec.LoadFeature(root, gateFeature)
	if err != nil {
		t.Fatalf("load feature: %v", err)
	}
	if got := fixtureDocumentByName(&feature, name).Status; got != want {
		t.Fatalf("%s status = %q, want %q", name, got, want)
	}
}

func TestOpenReviewRefusesUnresolvedContent(t *testing.T) {
	root := gateFixture(t, "# Feature Design\n\nFirst [decision: which store backs this?] stays open.\n\n<!-- assumed: a comment that never closes\n\nThen [decision: which queue?] too.\n")
	lines := []int{
		gateBodyLine(t, root, "design.md", "First"),
		gateBodyLine(t, root, "design.md", "assumed: a comment"),
		gateBodyLine(t, root, "design.md", "Then"),
	}
	before := featureDigests(t, root)

	_, err := OpenReview(root, gateFeature, PhaseDesign)
	if err == nil {
		t.Fatal("review open accepted a design holding unresolved content")
	}
	requireDigestsUnchanged(t, root, before)
	message := err.Error()
	if !strings.Contains(message, "design.md") {
		t.Fatalf("refusal does not name the document: %s", message)
	}
	previous := -1
	for _, line := range lines {
		at := strings.Index(message, fmt.Sprintf("body line %d (", line))
		if at < 0 || at < previous {
			t.Fatalf("refusal does not name body line %d in document order: %s", line, message)
		}
		previous = at
	}

	// The same document without its unresolved content opens.
	writeReviewFeatureDoc(t, root, gateFeature, "design.md", draftDesignContent("# Feature Design\n\nFirst the store stays local.\n\nThen the queue stays in memory.\n"))
	if _, err := OpenReview(root, gateFeature, PhaseDesign); err != nil {
		t.Fatalf("review open refused a design without unresolved content: %v", err)
	}
	requireGateStatus(t, root, "design.md", "in-review")
}

func TestApproveReviewRefusesUnresolvedContent(t *testing.T) {
	root := gateFixture(t, "# Feature Design\n\nNothing is open.\n")
	if _, err := OpenReview(root, gateFeature, PhaseDesign); err != nil {
		t.Fatalf("open design: %v", err)
	}
	inReview := readGateDoc(t, root, "design.md")

	// The document in review gains a bare marker.
	if err := os.WriteFile(gateDocPath(root, "design.md"), []byte(inReview+"\nA late [decision: which store backs this?]\n"), 0o644); err != nil {
		t.Fatalf("write design: %v", err)
	}
	before := featureDigests(t, root)
	_, err := ApproveReview(root, gateFeature, PhaseDesign)
	if err == nil {
		t.Fatal("review approve accepted a design holding unresolved content")
	}
	requireDigestsUnchanged(t, root, before)
	if !strings.Contains(err.Error(), "design.md") || !strings.Contains(err.Error(), markerRemedy) {
		t.Fatalf("refusal does not name the document and the remedy: %v", err)
	}

	// Once the marker is gone, the same command approves it, with no new
	// review open.
	if err := os.WriteFile(gateDocPath(root, "design.md"), []byte(inReview), 0o644); err != nil {
		t.Fatalf("restore design: %v", err)
	}
	if _, err := ApproveReview(root, gateFeature, PhaseDesign); err != nil {
		t.Fatalf("review approve refused a design without unresolved content: %v", err)
	}
	requireGateStatus(t, root, "design.md", "approved")
}

func TestReviewGateTargetsPhaseDocument(t *testing.T) {
	// The targeted document is refused.
	root := gateFixture(t, "# Feature Design\n\nA bare [decision: which store backs this?]\n")
	if _, err := OpenReview(root, gateFeature, PhaseDesign); err == nil {
		t.Fatal("review open accepted the targeted design holding a bare marker")
	}

	// A draft design does not stop the requirements review.
	root = t.TempDir()
	writeReviewFeatureDoc(t, root, gateFeature, "requirements.md", draftRequirementsContent(approveReqBody))
	writeReviewFeatureDoc(t, root, gateFeature, "design.md", draftDesignContent("# Feature Design\n\nA bare [decision: which store backs this?]\n"))
	if _, err := OpenReview(root, gateFeature, PhaseRequirements); err != nil {
		t.Fatalf("a draft design stopped review open --phase requirements: %v", err)
	}
	if _, err := ApproveReview(root, gateFeature, PhaseRequirements); err != nil {
		t.Fatalf("a draft design stopped review approve --phase requirements: %v", err)
	}

	// Approved requirements sealed by an earlier release do not stop the
	// design review.
	root = t.TempDir()
	writeReviewFeatureDoc(t, root, gateFeature, "requirements.md", approvedRequirementsContent("# Requirements Document\n\nA bare [decision: sealed by an earlier release]\n", "2026-03-21T14:00:00Z"))
	writeReviewFeatureDoc(t, root, gateFeature, "design.md", draftDesignContent(approveDesignBody))
	if _, err := OpenReview(root, gateFeature, PhaseDesign); err != nil {
		t.Fatalf("sealed requirements stopped review open --phase design: %v", err)
	}
	if _, err := ApproveReview(root, gateFeature, PhaseDesign); err != nil {
		t.Fatalf("sealed requirements stopped review approve --phase design: %v", err)
	}
}

func TestReviewGateCleanDocumentUnchanged(t *testing.T) {
	type outcome struct {
		opened   ReviewContext
		approved ApprovalResult
	}
	quoting := "# Feature Design\n\nA checkpoint reads `[decision: which store?]` and an assumption opens with ``<!--``.\n\n```markdown\n[decision: inside a fence]\n<!-- inside a fence\n```\n\n<!-- assumed: no [decision: stays open -->\n"
	plain := "# Feature Design\n\nNothing to quote.\n"

	var outcomes []outcome
	for _, body := range []string{plain, quoting} {
		root := gateFixture(t, body)
		requirementsBefore := readGateDoc(t, root, "requirements.md")
		tasksBefore := readGateDoc(t, root, "tasks.md")

		opened, err := OpenReview(root, gateFeature, PhaseDesign)
		if err != nil {
			t.Fatalf("review open refused a design without unresolved content: %v", err)
		}
		requireGateStatus(t, root, "design.md", "in-review")
		approved, err := ApproveReview(root, gateFeature, PhaseDesign)
		if err != nil {
			t.Fatalf("review approve refused a design without unresolved content: %v", err)
		}

		feature, err := spec.LoadFeature(root, gateFeature)
		if err != nil {
			t.Fatalf("load feature: %v", err)
		}
		design := feature.Design
		if design.Status != "approved" || design.ApprovedAt != approved.ApprovedAt {
			t.Fatalf("design = %q approved at %q, result approved at %q", design.Status, design.ApprovedAt, approved.ApprovedAt)
		}
		if strings.TrimSpace(design.Body) != strings.TrimSpace(body) {
			t.Fatalf("the review commands changed the body:\n%s", design.Body)
		}
		if design.ApprovedFingerprint != spec.Fingerprint(design.Path, body) {
			t.Fatalf("approved fingerprint %q does not seal the body", design.ApprovedFingerprint)
		}
		if readGateDoc(t, root, "requirements.md") != requirementsBefore || readGateDoc(t, root, "tasks.md") != tasksBefore {
			t.Fatal("the design review changed another document")
		}
		approved.ApprovedAt = ""
		outcomes = append(outcomes, outcome{opened: opened, approved: approved})
	}
	if outcomes[0] != outcomes[1] {
		t.Fatalf("a document quoting both texts went through the review commands differently: %+v then %+v", outcomes[0], outcomes[1])
	}
}

func TestReviewGateRemediesClearRefusal(t *testing.T) {
	// Every kind present states its remedy once, however many findings it has.
	root := gateFixture(t, "# Feature Design\n\n[decision: one]\n\n[decision: two]\n\n<!-- never closed\n")
	_, err := OpenReview(root, gateFeature, PhaseDesign)
	if err == nil {
		t.Fatal("review open accepted a design holding unresolved content")
	}
	if strings.Count(err.Error(), markerRemedy) != 1 || strings.Count(err.Error(), commentRemedy) != 1 {
		t.Fatalf("refusal does not state each remedy exactly once: %v", err)
	}

	for _, tc := range []struct {
		name, refused, remedied, remedy string
	}{
		{"marker_resolved", "Which store? [decision: which store backs this?]", "The store stays local.", markerRemedy},
		{"marker_quoted", "Which store? [decision: which store backs this?]", "A checkpoint reads `[decision: which store backs this?]`.", markerRemedy},
		{"comment_closed", "<!-- assumed: the store stays local", "<!-- assumed: the store stays local -->", commentRemedy},
		{"opener_quoted", "An assumption opens with <!-- and nothing more", "An assumption opens with `<!--` and nothing more", commentRemedy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gateFixture(t, "# Feature Design\n\n"+tc.refused+"\n")
			_, err := OpenReview(root, gateFeature, PhaseDesign)
			if err == nil || !strings.Contains(err.Error(), tc.remedy) {
				t.Fatalf("refusal does not state %q: %v", tc.remedy, err)
			}
			writeReviewFeatureDoc(t, root, gateFeature, "design.md", draftDesignContent("# Feature Design\n\n"+tc.remedied+"\n"))
			if _, err := OpenReview(root, gateFeature, PhaseDesign); err != nil {
				t.Fatalf("the remedy did not clear the refusal: %v", err)
			}
		})
	}
}
