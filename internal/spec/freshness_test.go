package spec

import (
	"strings"
	"testing"
)

const (
	reqBody    = "# Requirements Document\n\n1. `R1.AC1` WHEN x, the system SHALL y.\n"
	designBody = "# Feature Design\n\nArchitecture.\n"
	tasksBody  = "# Implementation Plan\n\n- [ ] 1. Objective\n"
)

func approvedDocument(name, body string) Document {
	return Document{
		Path:                name,
		Status:              "approved",
		ApprovedAt:          "2026-07-02T08:00:00Z",
		LastModified:        "2026-07-02T08:00:00Z",
		ApprovedFingerprint: Fingerprint(name, body),
		Fields:              map[string]string{},
		Exists:              true,
		Body:                body,
	}
}

func approvedChain() Feature {
	requirements := approvedDocument("requirements.md", reqBody)
	design := approvedDocument("design.md", designBody)
	design.SourceRequirementsFingerprint = requirements.ApprovedFingerprint
	tasks := approvedDocument("tasks.md", tasksBody)
	tasks.SourceDesignFingerprint = design.ApprovedFingerprint

	return Feature{
		Name:         "example",
		Requirements: requirements,
		Design:       design,
		Tasks:        tasks,
	}
}

func TestEvaluateFreshnessApprovedChainIsFresh(t *testing.T) {
	report := EvaluateFreshness(approvedChain())

	for name, verdict := range map[string]DocumentFreshness{
		"requirements": report.Requirements,
		"design":       report.Design,
		"tasks":        report.Tasks,
	} {
		if !verdict.Intact || !verdict.Fresh || len(verdict.Causes) != 0 {
			t.Fatalf("%s: expected intact+fresh with no causes, got %+v", name, verdict)
		}
	}
}

func TestEvaluateFreshnessDraftsAreVacuouslyFresh(t *testing.T) {
	feature := approvedChain()
	feature.Design.Status = "draft"
	feature.Design.ApprovedFingerprint = ""
	feature.Tasks.Status = "draft"
	feature.Tasks.ApprovedFingerprint = ""

	report := EvaluateFreshness(feature)
	if !report.Design.Intact || !report.Design.Fresh {
		t.Fatalf("draft design should be vacuously fresh, got %+v", report.Design)
	}
	if !report.Tasks.Intact || !report.Tasks.Fresh {
		t.Fatalf("draft tasks should be vacuously fresh, got %+v", report.Tasks)
	}
}

func TestEvaluateFreshnessMissingFingerprint(t *testing.T) {
	feature := approvedChain()
	feature.Requirements.ApprovedFingerprint = ""

	report := EvaluateFreshness(feature)
	if report.Requirements.Intact || report.Requirements.Fresh {
		t.Fatalf("approved document without fingerprint must be stale, got %+v", report.Requirements)
	}
	if got := strings.Join(report.Requirements.Causes, ";"); got != CauseFingerprintMissing {
		t.Fatalf("causes = %q, want %q", got, CauseFingerprintMissing)
	}
	if report.Design.Fresh {
		t.Fatalf("design must not be fresh over a stale upstream, got %+v", report.Design)
	}
}

func TestEvaluateFreshnessMalformedFingerprint(t *testing.T) {
	feature := approvedChain()
	feature.Requirements.ApprovedFingerprint = "sha256:not-a-digest"

	report := EvaluateFreshness(feature)
	if report.Requirements.Intact {
		t.Fatalf("malformed fingerprint must fail closed, got %+v", report.Requirements)
	}
	if got := strings.Join(report.Requirements.Causes, ";"); got != CauseFingerprintMalformed {
		t.Fatalf("causes = %q, want %q", got, CauseFingerprintMalformed)
	}
}

func TestEvaluateFreshnessTamperedBody(t *testing.T) {
	feature := approvedChain()
	feature.Requirements.Body += "\nInjected requirement.\n"

	report := EvaluateFreshness(feature)
	if report.Requirements.Intact || report.Requirements.Fresh {
		t.Fatalf("tampered approved document must be stale, got %+v", report.Requirements)
	}
	if got := strings.Join(report.Requirements.Causes, ";"); got != CauseContentMismatch {
		t.Fatalf("causes = %q, want %q", got, CauseContentMismatch)
	}
	if report.Design.Fresh || report.Tasks.Fresh {
		t.Fatalf("downstream must not stay fresh over a tampered upstream: design=%+v tasks=%+v", report.Design, report.Tasks)
	}
}

func TestEvaluateFreshnessChainMismatch(t *testing.T) {
	feature := approvedChain()
	feature.Design.SourceRequirementsFingerprint = Fingerprint("requirements.md", "some other requirements content")

	report := EvaluateFreshness(feature)
	if !report.Design.Intact {
		t.Fatalf("design integrity should hold, got %+v", report.Design)
	}
	if report.Design.Fresh {
		t.Fatalf("design must be stale on source fingerprint mismatch, got %+v", report.Design)
	}
	if got := strings.Join(report.Design.Causes, ";"); got != CauseSourceMismatch {
		t.Fatalf("causes = %q, want %q", got, CauseSourceMismatch)
	}
	if report.Tasks.Fresh {
		t.Fatalf("tasks must cascade stale when design is stale, got %+v", report.Tasks)
	}
	if got := strings.Join(report.Tasks.Causes, ";"); got != CauseUpstreamStale {
		t.Fatalf("tasks causes = %q, want %q", got, CauseUpstreamStale)
	}
}

func TestEvaluateFreshnessFingerprintComparisonAloneDecides(t *testing.T) {
	// Content-identical re-approval: approved_at diverges from the recorded
	// source timestamp, but fingerprints match. The chain must stay fresh.
	feature := approvedChain()
	feature.Requirements.ApprovedAt = "2026-07-09T10:00:00Z"
	feature.Requirements.LastModified = "2026-07-09T10:00:00Z"
	feature.Design.SourceRequirementsApprovedAt = "2026-07-02T08:00:00Z"

	report := EvaluateFreshness(feature)
	if !report.Design.Fresh {
		t.Fatalf("fingerprint match must keep design fresh regardless of timestamps, got %+v", report.Design)
	}
	if !report.Tasks.Fresh {
		t.Fatalf("fingerprint match must keep tasks fresh regardless of timestamps, got %+v", report.Tasks)
	}
}

func TestEvaluateFreshnessUpstreamNotApproved(t *testing.T) {
	feature := approvedChain()
	feature.Requirements.Status = "draft"
	feature.Requirements.ApprovedFingerprint = ""

	report := EvaluateFreshness(feature)
	if report.Design.Fresh {
		t.Fatalf("approved design over unapproved requirements must be stale, got %+v", report.Design)
	}
	if got := strings.Join(report.Design.Causes, ";"); got != CauseUpstreamNotApproved {
		t.Fatalf("causes = %q, want %q", got, CauseUpstreamNotApproved)
	}
}

func TestEvaluateFreshnessLegacyChainFullyStale(t *testing.T) {
	// A pre-fingerprint feature: everything approved, no fingerprints anywhere.
	feature := approvedChain()
	feature.Requirements.ApprovedFingerprint = ""
	feature.Design.ApprovedFingerprint = ""
	feature.Design.SourceRequirementsFingerprint = ""
	feature.Tasks.ApprovedFingerprint = ""
	feature.Tasks.SourceDesignFingerprint = ""

	report := EvaluateFreshness(feature)
	for name, verdict := range map[string]DocumentFreshness{
		"requirements": report.Requirements,
		"design":       report.Design,
		"tasks":        report.Tasks,
	} {
		if verdict.Fresh {
			t.Fatalf("%s: legacy approved document must be stale under strict migration, got %+v", name, verdict)
		}
		if len(verdict.Causes) == 0 || verdict.Causes[0] != CauseFingerprintMissing {
			t.Fatalf("%s: first cause must be %q, got %v", name, CauseFingerprintMissing, verdict.Causes)
		}
	}
}
