package workflow

import (
	"context"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/testutil"
)

// A ledger written before profiles existed must keep deriving and verifying
// with unchanged outcomes: the profile layer is additive, never gating.
func TestLegacyLedgerUnchangedByProfileLayer(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	overrideProbeRunner(t, probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		return shell.Response{ExitCode: 0}, nil
	}))
	completeBoth(t, root)

	// Rewrite the ledger as a pre-binding CLI would have left it.
	ledger, err := evidence.Load(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("load ledger: %v", err)
	}
	for taskID, record := range ledger.Tasks {
		record.Profile = nil
		ledger.Tasks[taskID] = record
	}
	if err := evidence.Save(root, ledger); err != nil {
		t.Fatalf("save legacy ledger: %v", err)
	}

	// Derive: both verified, exactly as before the profile layer existed.
	_, entries, err := EvidenceReport(context.Background(), root, "todo-app-demo")
	if err != nil {
		t.Fatalf("EvidenceReport: %v", err)
	}
	for _, entry := range entries {
		if entry.State != evidence.StateVerified {
			t.Fatalf("legacy record derived %s, want verified", entry.State)
		}
	}

	// Default verify: nothing to re-prove, both skipped as verified.
	idle := testutil.NewFakeRunner()
	result, err := Verify(context.Background(), root, "todo-app-demo", false, false, idle)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if len(idle.Calls()) != 0 || len(result.Skipped) != 2 {
		t.Fatalf("legacy ledger changed verify outcomes: skipped=%v calls=%d", result.Skipped, len(idle.Calls()))
	}
}
