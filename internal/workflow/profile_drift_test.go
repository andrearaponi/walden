package workflow

import (
	"context"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/testutil"
)

func TestVerifyFailureNamesEnvironmentDrift(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	writeProbes(t, root, "- go: [\"go\", \"version\"]\n")
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))

	// Evidence recorded under go1.25.0.
	overrideProbeRunner(t, probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		return shell.Response{Stdout: "go1.25.0\n", ExitCode: 0}, nil
	}))
	completeBoth(t, root)

	// The machine drifts to go1.24.0 and the proof starts failing.
	probeRunner = probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		return shell.Response{Stdout: "go1.24.0\n", ExitCode: 0}, nil
	})
	resetProfileCaptureForTests()

	runner := testutil.NewFakeRunner(
		testutil.Response{Stderr: "compile error", ExitCode: 1},
		testutil.Response{Stderr: "compile error", ExitCode: 1},
	)
	result, err := Verify(context.Background(), root, "todo-app-demo", true, false, runner)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if len(result.Failed) != 2 {
		t.Fatalf("expected both tasks to fail, got %v", result.Failed)
	}
	for _, outcome := range result.Outcomes {
		if !strings.Contains(outcome.Failure, `environment drift: go: recorded "go1.25.0" → current "go1.24.0"`) {
			t.Fatalf("failure lacks the drift diagnosis: %q", outcome.Failure)
		}
	}
}

func TestVerifyFailureWithoutDriftStaysBare(t *testing.T) {
	root := t.TempDir()
	writeVerifyFixture(t, root)
	writeProbes(t, root, "- go: [\"go\", \"version\"]\n")
	overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
	overrideProbeRunner(t, probeFunc(func(ctx context.Context, name string, args ...string) (shell.Response, error) {
		return shell.Response{Stdout: "go1.25.0\n", ExitCode: 0}, nil
	}))
	completeBoth(t, root)

	runner := testutil.NewFakeRunner(
		testutil.Response{Stderr: "assertion blew up", ExitCode: 1},
		testutil.Response{Stderr: "assertion blew up", ExitCode: 1},
	)
	result, err := Verify(context.Background(), root, "todo-app-demo", true, false, runner)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	for _, outcome := range result.Outcomes {
		if strings.Contains(outcome.Failure, "environment drift") {
			t.Fatalf("stable environment must not decorate the failure: %q", outcome.Failure)
		}
	}
}
