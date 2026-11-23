package adopt

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

// The three sets of real documents. The first was approved with the v0.1.0
// binary, after which the requirements were revised and approved again:
// that release reports the design and the plan as stale. The second is the
// first after `walden adopt --apply` of v0.15.2, which sealed it. The third
// was approved with the v0.4.0 binary, which records fingerprints, after
// which the requirements were approved again, unchanged, while the design
// and the plan stayed approved.
const (
	approvedByOldRelease = "approved-by-v0.1.0"
	sealedByBackfill     = "sealed-by-v0.15.2"
	approvedAgain        = "reapproved-by-v0.4.0"

	// The times the first two sets hold.
	designRecordsAt       = "2026-11-23T11:50:38Z"
	requirementsApproveAt = "2026-11-23T11:50:39Z"

	// The times the third set holds.
	approvedAgainDesignRecordsAt = "2026-11-23T12:41:06Z"
	approvedAgainRequirementsAt  = "2026-11-23T12:41:09Z"
)

var linkDocuments = []string{"requirements.md", "design.md", "tasks.md"}

// copyLinkDocs writes one set of the real documents as a feature of root and
// returns what it wrote, by file name.
func copyLinkDocs(t *testing.T, root, feature, set string) map[string]string {
	t.Helper()
	written := map[string]string{}
	for _, name := range linkDocuments {
		content, err := os.ReadFile(filepath.Join("testdata", "legacy-link", set, name))
		if err != nil {
			t.Fatalf("read test data: %v", err)
		}
		writeDoc(t, root, feature, name, string(content))
		written[name] = string(content)
	}
	return written
}

// rewriteDoc replaces one occurrence of old in a document of a feature.
func rewriteDoc(t *testing.T, root, feature, name, old, replacement string) {
	t.Helper()
	path := filepath.Join(root, ".walden", "specs", feature, name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if strings.Count(string(content), old) != 1 {
		t.Fatalf("%s holds %q %d time(s), want one", name, old, strings.Count(string(content), old))
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(content), old, replacement, 1)), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func readDoc(t *testing.T, root, feature, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, ".walden", "specs", feature, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}

func requireFragments(t *testing.T, what, text string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(text, fragment) {
			t.Errorf("%s does not mention %q: %s", what, fragment, text)
		}
	}
}

func TestPlanBlocksLegacyLinkWhoseTimesDisagree(t *testing.T) {
	cases := []struct {
		name    string
		feature string
		arrange func(t *testing.T, root string)
		reason  []string
	}{
		{
			name:    "documents approved by v0.1.0",
			feature: "greeting",
			arrange: func(t *testing.T, root string) {
				copyLinkDocs(t, root, "greeting", approvedByOldRelease)
			},
			reason: []string{"design.md", "requirements.md", designRecordsAt, requirementsApproveAt, "walden reconcile greeting"},
		},
		{
			name:    "requirements approved again after the design",
			feature: "old-era",
			arrange: func(t *testing.T, root string) {
				preFingerprintDocs(t, root, "old-era")
				rewriteDoc(t, root, "old-era", "requirements.md", "approved_at: 2026-06-09T14:00:00Z", "approved_at: 2026-06-09T15:00:00Z")
			},
			reason: []string{"design.md", "requirements.md", "2026-06-09T14:00:00Z", "2026-06-09T15:00:00Z", "walden reconcile old-era"},
		},
		{
			name:    "design approved again after the plan",
			feature: "old-era",
			arrange: func(t *testing.T, root string) {
				preFingerprintDocs(t, root, "old-era")
				rewriteDoc(t, root, "old-era", "design.md", "approved_at: 2026-06-09T14:10:00Z", "approved_at: 2026-06-09T16:00:00Z")
			},
			reason: []string{"tasks.md", "design.md", "2026-06-09T14:10:00Z", "2026-06-09T16:00:00Z", "walden reconcile old-era"},
		},
		{
			name:    "link that records no time",
			feature: "old-era",
			arrange: func(t *testing.T, root string) {
				preFingerprintDocs(t, root, "old-era")
				rewriteDoc(t, root, "old-era", "design.md", "source_requirements_approved_at: 2026-06-09T14:00:00Z", "source_requirements_approved_at:")
			},
			reason: []string{"design.md", "records no approval time", "requirements.md", "2026-06-09T14:00:00Z", "walden reconcile old-era"},
		},
		{
			name:    "upstream that carries a fingerprint",
			feature: "old-era",
			arrange: func(t *testing.T, root string) {
				preFingerprintDocs(t, root, "old-era")
				seal := spec.Fingerprint("requirements.md", "# Requirements Document\n")
				rewriteDoc(t, root, "old-era", "requirements.md",
					"approved_at: 2026-06-09T14:00:00Z\nlast_modified: 2026-06-09T14:00:00Z\n",
					"approved_at: 2026-06-09T15:00:00Z\nlast_modified: 2026-06-09T15:00:00Z\napproved_fingerprint: "+seal+"\n")
			},
			reason: []string{"design.md", "requirements.md", "2026-06-09T14:00:00Z", "2026-06-09T15:00:00Z", "walden reconcile old-era"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.arrange(t, root)
			before := hashWaldenTree(t, root)

			report, plan := planOf(t, root, tc.feature)

			if plan.Class != ClassBlocked {
				t.Fatalf("class = %q, want %q: %+v", plan.Class, ClassBlocked, plan)
			}
			requireFragments(t, "the reason", plan.BlockReason, tc.reason...)
			if len(plan.SealableDocs) != 0 || report.Totals.SealableDocs != 0 {
				t.Errorf("a blocked feature promises documents to seal: %v, total %d", plan.SealableDocs, report.Totals.SealableDocs)
			}
			if report.Totals.Blocked != 1 || report.Totals.Backfill != 0 {
				t.Errorf("totals = %+v, want one blocked feature and no backfill", report.Totals)
			}
			if after := hashWaldenTree(t, root); after != before {
				t.Error("the plan wrote under .walden")
			}
		})
	}
}

func TestPlanKeepsLegacyLinkWhoseTimesAgree(t *testing.T) {
	root := t.TempDir()
	preFingerprintDocs(t, root, "old-era")

	_, plan := planOf(t, root, "old-era")
	if plan.Class != ClassBackfill || len(plan.SealableDocs) != 3 {
		t.Fatalf("plan = %+v, want backfill with three documents to seal", plan)
	}
	if len(plan.Warnings) != 0 {
		t.Errorf("a chain whose times agree carries warnings: %v", plan.Warnings)
	}

	// The fixture has no git repository around it, so the proof run that
	// follows the sealing has no code identity to work with. The test reads
	// only what the sealing did.
	if _, err := Apply(context.Background(), root, "old-era", testutil.NewFakeRunner(), nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	_, sealed := planOf(t, root, "old-era")
	if sealed.Class == ClassBlocked || len(sealed.SealableDocs) != 0 {
		t.Errorf("after apply the plan = %+v, want no document to seal and no block", sealed)
	}
	if len(sealed.Warnings) != 0 {
		t.Errorf("a chain sealed from times that agree carries warnings: %v", sealed.Warnings)
	}
}

func TestApplyLeavesBlockedLegacyLinkUnsealed(t *testing.T) {
	root := t.TempDir()
	written := copyLinkDocs(t, root, "greeting", approvedByOldRelease)
	preFingerprintDocs(t, root, "old-era")
	_, planned := planOf(t, root, "greeting")

	report, err := Apply(context.Background(), root, "", testutil.NewFakeRunner(), nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	features := map[string]FeatureAdoption{}
	for _, feature := range report.Features {
		features[feature.Name] = feature
	}

	blocked := features["greeting"]
	if blocked.Class != ClassBlocked || len(blocked.SealedDocs) != 0 {
		t.Errorf("greeting = %+v, want blocked with nothing sealed", blocked)
	}
	if blocked.Error == "" || blocked.Error != planned.BlockReason {
		t.Errorf("the run reports %q, want the reason of the plan %q", blocked.Error, planned.BlockReason)
	}
	for _, name := range linkDocuments {
		if got := readDoc(t, root, "greeting", name); got != written[name] {
			t.Errorf("apply changed %s of the blocked feature:\n got %q\nwant %q", name, got, written[name])
		}
	}

	if adopted := features["old-era"]; len(adopted.SealedDocs) != 3 {
		t.Errorf("old-era = %+v, want its three documents sealed in the same run", adopted)
	}
	if report.Totals.Blocked != 1 || report.Totals.SealedDocs != 3 {
		t.Errorf("totals = %+v, want one blocked feature and three sealed documents", report.Totals)
	}
}

func TestSealFeatureRefusesLegacyLinkWhoseTimesDisagree(t *testing.T) {
	root := t.TempDir()
	copyLinkDocs(t, root, "greeting", approvedByOldRelease)
	before := hashWaldenTree(t, root)

	sealed, err := sealFeature(root, "greeting")

	if err == nil {
		t.Fatalf("sealFeature sealed %v on a legacy link whose times disagree", sealed)
	}
	requireFragments(t, "the error", err.Error(), "design.md", "requirements.md", designRecordsAt, requirementsApproveAt)
	if len(sealed) != 0 {
		t.Errorf("sealFeature reports sealed documents on a refusal: %v", sealed)
	}
	if after := hashWaldenTree(t, root); after != before {
		t.Error("sealFeature wrote under .walden on a refusal")
	}
}

func TestPlanWarnsAboutSealedLinkWhoseTimesDisagree(t *testing.T) {
	t.Run("documents sealed by v0.15.2", func(t *testing.T) {
		root := t.TempDir()
		copyLinkDocs(t, root, "greeting", sealedByBackfill)
		before := hashWaldenTree(t, root)

		_, plan := planOf(t, root, "greeting")

		// v0.15.2 plans these documents as complete: the seal binds them.
		if plan.Class != ClassComplete {
			t.Errorf("class = %q, want %q, the class the previous release gives", plan.Class, ClassComplete)
		}
		if len(plan.Warnings) != 1 {
			t.Fatalf("warnings = %v, want exactly one", plan.Warnings)
		}
		requireFragments(t, "the warning", plan.Warnings[0],
			"design.md", "requirements.md", designRecordsAt, requirementsApproveAt,
			"without a review", "the last one first", "approve them again")
		if after := hashWaldenTree(t, root); after != before {
			t.Error("the plan wrote under .walden")
		}

		// An apply run names the same link, in front of what its proofs report.
		report, err := Apply(context.Background(), root, "greeting", testutil.NewFakeRunner(), nil)
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		if len(report.Features) != 1 || len(report.Features[0].Warnings) == 0 || report.Features[0].Warnings[0] != plan.Warnings[0] {
			t.Errorf("the apply run does not carry the warning of the plan first: %+v", report.Features)
		}
	})

	t.Run("chain approved under fingerprints", func(t *testing.T) {
		root := t.TempDir()
		sealedDocs(t, root, "sealed", false)

		if _, plan := planOf(t, root, "sealed"); len(plan.Warnings) != 0 {
			t.Errorf("a chain approved under fingerprints carries warnings: %v", plan.Warnings)
		}
	})

	t.Run("chain sealed from times that agree", func(t *testing.T) {
		root := t.TempDir()
		preFingerprintDocs(t, root, "old-era")
		if _, err := Apply(context.Background(), root, "old-era", testutil.NewFakeRunner(), nil); err != nil {
			t.Fatalf("Apply: %v", err)
		}

		if _, plan := planOf(t, root, "old-era"); len(plan.Warnings) != 0 {
			t.Errorf("a chain sealed from times that agree carries warnings: %v", plan.Warnings)
		}
	})
}

func TestSealedLinkWarningNamesBothHistories(t *testing.T) {
	cases := []struct {
		name     string
		set      string
		recorded string
		approved string
	}{
		{"documents sealed by v0.15.2", sealedByBackfill, designRecordsAt, requirementsApproveAt},
		{"documents approved again by v0.4.0", approvedAgain, approvedAgainDesignRecordsAt, approvedAgainRequirementsAt},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			copyLinkDocs(t, root, "greeting", tc.set)
			before := hashWaldenTree(t, root)

			_, plan := planOf(t, root, "greeting")

			// v0.15.2 plans both sets as complete: the seal binds them.
			if plan.Class != ClassComplete {
				t.Errorf("class = %q, want %q, the class the previous release gives", plan.Class, ClassComplete)
			}
			if len(plan.Warnings) != 1 {
				t.Fatalf("warnings = %v, want exactly one", plan.Warnings)
			}
			requireFragments(t, "the warning", plan.Warnings[0],
				"design.md", "requirements.md", tc.recorded, tc.approved,
				// The two histories that leave such a link.
				"requirements.md was approved again without a change",
				"a backfill by v0.10.0 to v0.15.2",
				// Where they are told apart, and the way to clear the warning.
				"the adoption guide",
				"the last one first", "approve them again", "the first one first")
			if after := hashWaldenTree(t, root); after != before {
				t.Error("the plan wrote under .walden")
			}
		})
	}

	t.Run("upstream not approved again", func(t *testing.T) {
		// The chain approved by v0.4.0 as it was before the second approval
		// of its requirements: they carry the time the design records.
		root := t.TempDir()
		copyLinkDocs(t, root, "greeting", approvedAgain)
		rewriteDoc(t, root, "greeting", "requirements.md",
			"approved_at: "+approvedAgainRequirementsAt, "approved_at: "+approvedAgainDesignRecordsAt)

		if _, plan := planOf(t, root, "greeting"); len(plan.Warnings) != 0 {
			t.Errorf("a chain approved under fingerprints whose upstream was not approved again carries warnings: %v", plan.Warnings)
		}
	})
}
