package app

import (
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/release"
)

func TestReleaseEnvelopeCarriesCompletionAndCommit(t *testing.T) {
	report := release.ReleaseReport{
		Features: []release.FeatureCertification{
			{Feature: "demo", Pending: []string{"1.3"}},
		},
		CertifiedCommit: "0123456789abcdef0123456789abcdef01234567",
	}

	result := releaseCheckResult(report)

	if result.Completion != release.CompletionWithPending {
		t.Fatalf("completion = %q, want %q", result.Completion, release.CompletionWithPending)
	}
	if result.CertifiedCommit != report.CertifiedCommit {
		t.Fatalf("certified_commit = %q, want %q", result.CertifiedCommit, report.CertifiedCommit)
	}
	if !strings.Contains(result.Summary, "1 task(s) still planned") {
		t.Fatalf("summary does not count planned tasks: %q", result.Summary)
	}

	executed := release.ReleaseReport{Features: []release.FeatureCertification{{Feature: "demo"}}}
	if got := releaseCheckResult(executed).Completion; got != release.CompletionComplete {
		t.Fatalf("completion = %q, want %q", got, release.CompletionComplete)
	}
}

func TestReleasableSummaryNamesCommit(t *testing.T) {
	report := release.ReleaseReport{
		Features:        []release.FeatureCertification{{Feature: "demo"}},
		CertifiedCommit: "0123456789abcdef0123456789abcdef01234567",
	}

	result := releaseCheckResult(report)
	if !strings.Contains(result.Summary, "commit 0123456789ab") {
		t.Fatalf("releasable summary does not name the short commit: %q", result.Summary)
	}

	// Blocked verdicts keep their summary untouched; the facts stay in JSON.
	blocked := release.ReleaseReport{
		Features:         []release.FeatureCertification{{Feature: "demo"}},
		WorktreeBlockers: []string{"uncommitted: src.txt — commit it before certifying"},
		CertifiedCommit:  "0123456789abcdef0123456789abcdef01234567",
	}
	blockedResult := releaseCheckResult(blocked)
	if strings.Contains(blockedResult.Summary, "commit 0123") {
		t.Fatalf("blocked summary must not carry the commit clause: %q", blockedResult.Summary)
	}
	if blockedResult.CertifiedCommit == "" {
		t.Fatal("blocked result must still expose the commit fact in JSON")
	}
}
