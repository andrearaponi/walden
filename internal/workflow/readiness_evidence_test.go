package workflow

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/testutil"
)

func TestReadinessEvidenceSilentWhenVerified(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	completeBoth(t, root)

	readiness, err := LoadExecutionReadiness(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("LoadExecutionReadiness: %v", err)
	}
	if len(readiness.EvidenceWarnings) != 0 {
		t.Fatalf("verified plan raised warnings: %v", readiness.EvidenceWarnings)
	}
}

func TestReadinessEvidenceWarnsOnStaleCode(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	completeBoth(t, root)

	// The code moves under the completed plan.
	overrideIdentityRunner(t, identityYielding("100644 blob bbb\tmain.go\n"))

	readiness, err := LoadExecutionReadiness(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("LoadExecutionReadiness: %v", err)
	}
	if len(readiness.EvidenceWarnings) != 1 {
		t.Fatalf("warnings = %v, want one evidence warning", readiness.EvidenceWarnings)
	}
	warning := readiness.EvidenceWarnings[0]
	if !strings.Contains(warning, "stale-code") || !strings.Contains(warning, "walden verify") {
		t.Fatalf("warning %q does not name stale-code and the remedy", warning)
	}
}

func TestReadinessEvidenceWarnsOnStaleSpecAfterReapproval(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	completeBoth(t, root)

	// The WDN-002 replay: requirements change and the chain is re-approved
	// with fresh fingerprints, but no proof ever reruns.
	writeFreshFeatureDoc(t, root, "todo-app-demo", "requirements.md", `---
status: approved
approved_at: 2026-03-22T09:00:00Z
last_modified: 2026-03-22T09:00:00Z
---

# Requirements Document

## R2 A brand new requirement
`)
	writeFreshFeatureDoc(t, root, "todo-app-demo", "design.md", `---
status: approved
approved_at: 2026-03-22T09:10:00Z
last_modified: 2026-03-22T09:10:00Z
source_requirements_approved_at: 2026-03-22T09:00:00Z
---

# Feature Design
`)

	readiness, err := LoadExecutionReadiness(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("LoadExecutionReadiness: %v", err)
	}
	if readiness.GateBlocked {
		t.Fatalf("fixture chain unexpectedly blocked: %v", readiness.Blockers)
	}
	if len(readiness.EvidenceWarnings) != 1 || !strings.Contains(readiness.EvidenceWarnings[0], "stale-spec") {
		t.Fatalf("warnings = %v, want a stale-spec warning", readiness.EvidenceWarnings)
	}
}

func TestReadinessEvidenceFlagsUnrecordedLegacyCompletions(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))

	// Complete without the ledger by checking the box the pre-ledger way:
	// run one completion, then delete the evidence document.
	completeBoth(t, root)
	if err := removeEvidence(root, "todo-app-demo"); err != nil {
		t.Fatalf("remove evidence: %v", err)
	}

	readiness, err := LoadExecutionReadiness(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("LoadExecutionReadiness: %v", err)
	}
	if len(readiness.EvidenceWarnings) != 1 || !strings.Contains(readiness.EvidenceWarnings[0], "unrecorded") {
		t.Fatalf("warnings = %v, want an unrecorded warning", readiness.EvidenceWarnings)
	}

	// verify --all upgrades legacy completions into recorded evidence.
	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	)
	if _, err := Verify(context.Background(), root, "todo-app-demo", true, false, runner); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	readiness, err = LoadExecutionReadiness(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("LoadExecutionReadiness after verify: %v", err)
	}
	if len(readiness.EvidenceWarnings) != 0 {
		t.Fatalf("warnings persisted after verify --all: %v", readiness.EvidenceWarnings)
	}
}

func removeEvidence(root, feature string) error {
	return os.Remove(evidence.DocumentPath(root, feature))
}

func TestReadinessEvidenceWarnsOnUnreadableLedger(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	completeBoth(t, root)

	path := evidence.DocumentPath(root, "todo-app-demo")
	if err := os.WriteFile(path, []byte("{truncated"), 0o644); err != nil {
		t.Fatalf("corrupt ledger: %v", err)
	}

	readiness, err := LoadExecutionReadiness(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("LoadExecutionReadiness: %v", err)
	}
	if len(readiness.EvidenceWarnings) != 1 || !strings.Contains(readiness.EvidenceWarnings[0], "unreadable") {
		t.Fatalf("corrupt ledger did not surface a warning: %v", readiness.EvidenceWarnings)
	}
}
