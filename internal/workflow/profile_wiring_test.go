package workflow

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/testutil"
)

func TestCompleteRecordsProfile(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	writeProbes(t, root, "- go: [\"go\", \"version\"]\n")
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	overrideProbeRunner(t, probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		return shell.Response{Stdout: "go version go1.25.0\n", ExitCode: 0}, nil
	}))

	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok", ExitCode: 0})
	if _, err := CompleteTask(context.Background(), root, "todo-app-demo", "1.1", runner); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}

	ledger, err := evidence.Load(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("load ledger: %v", err)
	}
	record := ledger.Tasks["1.1"]
	if record.Profile["go"] != "go version go1.25.0" {
		t.Fatalf("probe output not recorded: %v", record.Profile)
	}
	if record.Profile["platform"] == "" || record.Profile["walden"] == "" {
		t.Fatalf("reserved keys missing: %v", record.Profile)
	}
}

func TestVerifyRecordsProfile(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	overrideProbeRunner(t, probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		return shell.Response{Stdout: "node v22.1.0\n", ExitCode: 0}, nil
	}))
	completeBoth(t, root)

	writeProbes(t, root, "- node: [\"node\", \"--version\"]\n")
	resetProfileCaptureForTests()

	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	)
	if _, err := Verify(context.Background(), root, "todo-app-demo", true, false, runner); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	ledger, err := evidence.Load(root, "todo-app-demo")
	if err != nil {
		t.Fatalf("load ledger: %v", err)
	}
	for _, taskID := range []string{"1.1", "1.2"} {
		if ledger.Tasks[taskID].Profile["node"] != "node v22.1.0" {
			t.Fatalf("refreshed record %s lacks the probe output: %v", taskID, ledger.Tasks[taskID].Profile)
		}
	}
}

func TestProfileCapturedOncePerRun(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	writeProbes(t, root, "- go: [\"go\", \"version\"]\n")
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))

	var probeRuns atomic.Int64
	overrideProbeRunner(t, probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		probeRuns.Add(1)
		return shell.Response{Stdout: "go version go1.25.0\n", ExitCode: 0}, nil
	}))

	runner := testutil.NewFakeRunner(
		testutil.Response{Stdout: "ok", ExitCode: 0},
		testutil.Response{Stdout: "ok", ExitCode: 0},
	)
	if _, err := CompleteAllTasks(context.Background(), root, "todo-app-demo", runner); err != nil {
		t.Fatalf("CompleteAllTasks: %v", err)
	}

	if got := probeRuns.Load(); got != 1 {
		t.Fatalf("probe executed %d times across the batch, want exactly 1", got)
	}
}
