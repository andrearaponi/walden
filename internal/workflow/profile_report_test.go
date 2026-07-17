package workflow

import (
	"context"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
)

func TestEvidenceReportComputesDrift(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	writeProbes(t, root, "- go: [\"go\", \"version\"]\n")
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))

	overrideProbeRunner(t, probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		return shell.Response{Stdout: "go1.25.0\n", ExitCode: 0}, nil
	}))
	completeBoth(t, root)

	// The machine drifts.
	probeRunner = probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		return shell.Response{Stdout: "go1.24.0\n", ExitCode: 0}, nil
	})
	resetProfileCaptureForTests()

	_, entries, err := EvidenceReport(context.Background(), root, "todo-app-demo")
	if err != nil {
		t.Fatalf("EvidenceReport: %v", err)
	}
	for _, entry := range entries {
		if entry.ProfileLegacy {
			t.Fatalf("profiled record misreported as legacy: %+v", entry)
		}
		if len(entry.ProfileDrift) != 1 {
			t.Fatalf("expected one drift entry for %s, got %+v", entry.TaskID, entry.ProfileDrift)
		}
		drift := entry.ProfileDrift[0]
		if drift.Key != "go" || drift.Recorded != "go1.25.0" || drift.Current != "go1.24.0" {
			t.Fatalf("drift must carry both values: %+v", drift)
		}
		if entry.RecordedProfile["go"] != "go1.25.0" {
			t.Fatalf("recorded profile not exposed: %v", entry.RecordedProfile)
		}
	}
}

func TestEvidenceReportMarksLegacyRecords(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	overrideProbeRunner(t, probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		return shell.Response{ExitCode: 0}, nil
	}))
	completeBoth(t, root)

	// Strip the profiles, simulating a ledger written before this feature.
	ledger, err := evidence.Load(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("load ledger: %v", err)
	}
	for taskID, record := range ledger.Tasks {
		record.Profile = nil
		ledger.Tasks[taskID] = record
	}
	if err := evidence.Save(root, ledger); err != nil {
		t.Fatalf("save stripped ledger: %v", err)
	}

	_, entries, err := EvidenceReport(context.Background(), root, "todo-app-demo")
	if err != nil {
		t.Fatalf("EvidenceReport: %v", err)
	}
	for _, entry := range entries {
		if !entry.ProfileLegacy {
			t.Fatalf("legacy record not marked for %s", entry.TaskID)
		}
		if len(entry.ProfileDrift) != 0 {
			t.Fatalf("legacy record must not be compared: %+v", entry.ProfileDrift)
		}
	}
}
