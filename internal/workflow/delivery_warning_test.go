package workflow

import (
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
)

// TestEvidenceWarningsLeaveDeliveredOut: the warning of task status is about
// completed tasks that are not verified, and a delivered task is not one of
// them.
func TestEvidenceWarningsLeaveDeliveredOut(t *testing.T) {
	root, _ := deliveryRepo(t)
	completeDelivery(t, root, "1")
	completeDelivery(t, root, "2")
	if record := deliveryRecord(t, root, "1"); record.Attestation == nil {
		t.Fatal("the fixture must hold a delivered task")
	}

	// The delivered task is the only one that is not verified.
	readiness, err := LoadExecutionReadiness(root, deliveryFeature)
	if err != nil {
		t.Fatalf("load readiness: %v", err)
	}
	if len(readiness.EvidenceWarnings) != 0 {
		t.Errorf("a feature whose only task that is not verified is delivered is warned: %v", readiness.EvidenceWarnings)
	}

	// After a code change the regression task is stale: the warning names
	// it, and it alone.
	factsFile(t, root, "notes.txt", "superseded\n")
	factsGit(t, root, "add", "-A")
	factsGit(t, root, "commit", "-qm", "the notes move on")
	readiness, err = LoadExecutionReadiness(root, deliveryFeature)
	if err != nil {
		t.Fatalf("load readiness: %v", err)
	}
	if len(readiness.EvidenceWarnings) != 1 {
		t.Fatalf("warnings = %v, want one", readiness.EvidenceWarnings)
	}
	warning := readiness.EvidenceWarnings[0]
	for _, want := range []string{"1 completed task(s) not verified", "2 (" + evidence.StateStaleCode + ")"} {
		if !strings.Contains(warning, want) {
			t.Errorf("the warning %q does not hold %q", warning, want)
		}
	}
	if strings.Contains(warning, evidence.StateDelivered) || strings.Contains(warning, "1 (") {
		t.Errorf("the warning names the delivered task: %q", warning)
	}
}
