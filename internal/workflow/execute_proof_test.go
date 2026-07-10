package workflow

import (
	"context"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

func expectOutputTask(expect string) ExecutableTask {
	step := spec.VerificationStep{Argv: []string{"go", "test", "-run", "TestX", "./pkg/"}}
	if expect != "" {
		step.ExpectOutput = &expect
	}
	proof := spec.VerificationSpec{Steps: []spec.VerificationStep{step}}
	return ExecutableTask{
		ID:           "1.1",
		Title:        "Step",
		Verification: proof.Display(),
		Proof:        proof,
	}
}

func TestExecuteProofCapturesStepOutcomes(t *testing.T) {
	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok  \tpkg\t0.1s\n", ExitCode: 0})

	results, display, err := executeProof(context.Background(), runner, expectOutputTask(""))
	if err != nil {
		t.Fatalf("executeProof returned error: %v", err)
	}
	if display == "" {
		t.Fatal("proof display is empty")
	}
	if len(results) != 1 {
		t.Fatalf("captured %d step outcomes, want 1", len(results))
	}

	step := results[0]
	if strings.Join(step.Command, " ") != "go test -run TestX ./pkg/" {
		t.Fatalf("recorded command = %v", step.Command)
	}
	if step.ExpectedExit != 0 || step.ActualExit != 0 {
		t.Fatalf("recorded exits = %d/%d, want 0/0", step.ExpectedExit, step.ActualExit)
	}
	if step.OutputDigest != evidence.DigestOutput("ok  \tpkg\t0.1s\n") {
		t.Fatalf("output digest mismatch: %s", step.OutputDigest)
	}
}

func TestExecuteProofPassesWhenOutputContainsExpectation(t *testing.T) {
	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok  \tpkg\t0.1s\n", ExitCode: 0})

	if _, _, err := executeProof(context.Background(), runner, expectOutputTask("ok  ")); err != nil {
		t.Fatalf("expectation unexpectedly failed: %v", err)
	}
}

func TestExecuteProofFailsVacuousRuns(t *testing.T) {
	// `go test -run NoMatch` exits 0 while printing "no tests to run" — the
	// vacuous pass expect_output exists to catch.
	runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok  \tpkg\t0.1s [no tests to run]\n", ExitCode: 0})

	_, _, err := executeProof(context.Background(), runner, expectOutputTask("--- PASS: TestX"))
	if err == nil {
		t.Fatal("vacuous proof passed despite the output expectation")
	}
	if !strings.Contains(err.Error(), "--- PASS: TestX") {
		t.Fatalf("error %q does not name the expected content", err)
	}
}

func TestExecuteProofStopsAtExitMismatchBeforeOutputCheck(t *testing.T) {
	runner := testutil.NewFakeRunner(testutil.Response{Stderr: "build failed", ExitCode: 2})

	results, _, err := executeProof(context.Background(), runner, expectOutputTask("ok"))
	if err == nil {
		t.Fatal("exit mismatch accepted")
	}
	if !strings.Contains(err.Error(), "exited with code 2") {
		t.Fatalf("error %q does not report the exit mismatch", err)
	}
	if len(results) != 1 || results[0].ActualExit != 2 {
		t.Fatalf("failing step outcome not captured: %+v", results)
	}
}
