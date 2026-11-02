package workflow

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

const (
	emptyPackageLine  = "ok  \tlab/a\t0.243s [no tests to run]\n"
	testedPackageLine = "ok  \tlab/b\t0.243s\n"
	noFilesLine       = "?   \tlab/c\t[no test files]\n"
)

func proofOf(steps ...spec.VerificationStep) ExecutableTask {
	proof := spec.VerificationSpec{Steps: steps}
	return ExecutableTask{ID: "1.1", Title: "Step", Verification: proof.Display(), Proof: proof}
}

func stepOf(argv ...string) spec.VerificationStep {
	return spec.VerificationStep{Argv: argv}
}

func expectingStep(expect string, argv ...string) spec.VerificationStep {
	return spec.VerificationStep{Argv: argv, ExpectOutput: &expect}
}

func TestEmptyTestRunRecognition(t *testing.T) {
	for _, tc := range []struct {
		name   string
		output string
		empty  bool
	}{
		{"one empty package", emptyPackageLine, true},
		{"two empty packages", emptyPackageLine + "ok  \tlab/b\t(cached) [no tests to run]\n", true},
		{"cached result", "ok  \tlab/a\t(cached) [no tests to run]\n", true},
		{"verbose run", "testing: warning: no tests to run\nPASS\n" + emptyPackageLine, true},
		{"coverage note", "ok  \tlab/a\t0.194s\tcoverage: [no statements] [no tests to run]\n", true},
		{"carriage return", "ok  \tlab/a\t0.1s [no tests to run]\r\n", true},
		{"no final newline", strings.TrimSuffix(emptyPackageLine, "\n"), true},
		{"package without test files", noFilesLine, true},
		{"no test files beside an empty package", emptyPackageLine + noFilesLine, true},

		{"a package ran, listed first", testedPackageLine + emptyPackageLine, false},
		{"a package ran, listed last", emptyPackageLine + testedPackageLine, false},
		{"a package ran beside no test files", testedPackageLine + noFilesLine, false},

		{"failed package", "FAIL\nFAIL\tlab/f\t0.076s\n" + emptyPackageLine + "FAIL\n", false},
		{"bare FAIL after an empty package", emptyPackageLine + "FAIL\n", false},
		{"build failure", "FAIL\tlab/f [build failed]\n" + emptyPackageLine, false},

		{"no output", "", false},
		{"output of another command", "# lab/a\ncompiled\n", false},
		{"runner warning without a package line", "testing: warning: no tests to run\n", false},
		{"indented package line", "    " + emptyPackageLine, false},
		{"marker quoted in a sentence", "the log says [no tests to run] here\n", false},
		{"no test files quoted in a sentence", "the log says [no test files] here\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := emptyTestRun(tc.output); got != tc.empty {
				t.Fatalf("emptyTestRun(%q) = %v, want %v", tc.output, got, tc.empty)
			}
		})
	}
}

func TestExecuteProofReportsEmptySteps(t *testing.T) {
	first := stepOf("go", "test", "-run", "TestFirst", "./first")
	second := stepOf("go", "test", "-run", "TestSecond", "./second")

	run := func(t *testing.T, task ExecutableTask, responses ...testutil.Response) (proofRun, error) {
		t.Helper()
		runner := testutil.NewFakeRunner(responses...)
		result, err := executeProof(context.Background(), runner, task)
		if got := len(runner.Calls()); got != len(responses) {
			t.Fatalf("the proof ran %d commands, want the %d it declares", got, len(responses))
		}
		return result, err
	}

	t.Run("a selector matching nothing warns once", func(t *testing.T) {
		result, err := run(t, proofOf(first), testutil.Response{Stdout: emptyPackageLine})
		if err != nil {
			t.Fatalf("an empty run must pass: %v", err)
		}
		if len(result.warnings) != 1 {
			t.Fatalf("warnings = %q, want one", result.warnings)
		}
	})

	t.Run("a selector matching a test stays silent", func(t *testing.T) {
		result, err := run(t, proofOf(first), testutil.Response{Stdout: testedPackageLine})
		if err != nil {
			t.Fatalf("a passing run failed: %v", err)
		}
		if len(result.warnings) != 0 {
			t.Fatalf("warnings = %q, want none", result.warnings)
		}
	})

	t.Run("two empty steps warn twice in order", func(t *testing.T) {
		result, err := run(t, proofOf(first, second),
			testutil.Response{Stdout: emptyPackageLine}, testutil.Response{Stdout: emptyPackageLine})
		if err != nil {
			t.Fatalf("empty runs must pass: %v", err)
		}
		if len(result.warnings) != 2 {
			t.Fatalf("warnings = %q, want two", result.warnings)
		}
		if !strings.Contains(result.warnings[0], "TestFirst") || !strings.Contains(result.warnings[1], "TestSecond") {
			t.Fatalf("warnings out of order: %q", result.warnings)
		}
	})

	t.Run("a failing exit drops the warning of an earlier step", func(t *testing.T) {
		result, err := run(t, proofOf(first, second),
			testutil.Response{Stdout: emptyPackageLine}, testutil.Response{ExitCode: 1})
		if err == nil {
			t.Fatal("a failing step passed")
		}
		if len(result.warnings) != 0 {
			t.Fatalf("warnings = %q on a failed proof, want none", result.warnings)
		}
		if len(result.steps) != 2 {
			t.Fatalf("steps = %d, the failure must keep what ran", len(result.steps))
		}
	})

	t.Run("an output mismatch drops the warning of an earlier step", func(t *testing.T) {
		strong := expectingStep("--- PASS: TestSecond", "go", "test", "-v", "-run", "TestSecond", "./second")
		result, err := run(t, proofOf(first, strong),
			testutil.Response{Stdout: emptyPackageLine}, testutil.Response{Stdout: emptyPackageLine})
		if err == nil {
			t.Fatal("an unmet expectation passed")
		}
		if len(result.warnings) != 0 {
			t.Fatalf("warnings = %q on a failed proof, want none", result.warnings)
		}
	})
}

func TestEmptyRunWarningText(t *testing.T) {
	kept := stepOf("go", "test", "-run", "TestKept", "./kept")
	gone := stepOf("go", "test", "-run", "TestGone", "./gone")
	runner := testutil.NewFakeRunner(testutil.Response{Stdout: testedPackageLine}, testutil.Response{Stdout: emptyPackageLine})

	result, err := executeProof(context.Background(), runner, proofOf(kept, gone))
	if err != nil {
		t.Fatalf("the proof must pass: %v", err)
	}
	if len(result.warnings) != 1 {
		t.Fatalf("warnings = %q, want one", result.warnings)
	}

	warning := result.warnings[0]
	for _, want := range []string{"task 1.1", `["go","test","-run","TestGone","./gone"]`, "without running any test", "selector", "expect_output"} {
		if !strings.Contains(warning, want) {
			t.Errorf("warning %q does not hold %q", warning, want)
		}
	}
	if strings.Contains(warning, "TestKept") {
		t.Errorf("warning %q names the step that ran tests", warning)
	}
}

func TestEmptyRunDeclaredIntent(t *testing.T) {
	for _, tc := range []struct {
		name     string
		step     spec.VerificationStep
		output   string
		warnings int
	}{
		{"exact selector as an argument", stepOf("go", "test", "-run", "^$", "./..."), emptyPackageLine, 0},
		{"exact selector with an equals sign", stepOf("go", "test", "-run=^$", "./..."), emptyPackageLine, 0},
		{"exact selector in a shell string", stepOf("sh", "-c", "go test -run '^$' ./..."), emptyPackageLine, 0},
		{"exact selector with an equals sign in a shell string", stepOf("sh", "-c", `go test -run="^$" ./... | tail -1`), emptyPackageLine, 0},
		{"shell variable after the anchor", stepOf("sh", "-c", `for t in A B; do go test -run "^$t$" ./x; done`), emptyPackageLine, 1},
		{"shell variable with an equals sign", stepOf("sh", "-c", `go test -run="^$1$" ./x`), emptyPackageLine, 1},
		{"selector that names a test", stepOf("go", "test", "-run", "TestGone", "./..."), emptyPackageLine, 1},
		{"-run with no value", stepOf("go", "test", "-run"), emptyPackageLine, 1},

		{"expectation an empty run satisfies", expectingStep("ok", "go", "test", "-run", "TestGone", "./..."), emptyPackageLine, 1},
		{"verbose expectation an empty run satisfies", expectingStep("PASS", "go", "test", "-v", "-run", "TestGone", "./..."), "PASS\n" + emptyPackageLine, 1},
		{"expectation naming no tests to run", expectingStep("[no tests to run]", "go", "test", "-run", "TestRemoved", "./..."), emptyPackageLine, 0},
		{"expectation naming no test files", expectingStep("no test files", "go", "test", "./c"), noFilesLine, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := testutil.NewFakeRunner(testutil.Response{Stdout: tc.output})
			result, err := executeProof(context.Background(), runner, proofOf(tc.step))
			if err != nil {
				t.Fatalf("the step must pass: %v", err)
			}
			if len(result.warnings) != tc.warnings {
				t.Fatalf("warnings = %q, want %d", result.warnings, tc.warnings)
			}
		})
	}
}

// TestEmptyRunRealGoOutput reads what the installed go test prints, so a change
// of its line format fails here and not silently in a proof.
func TestEmptyRunRealGoOutput(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOWORK", "off")
	root := t.TempDir()
	for path, content := range map[string]string{
		"go.mod":             "module lab\n\ngo 1.25\n",
		"tested/t.go":        "package tested\n",
		"tested/t_test.go":   "package tested\n\nimport \"testing\"\n\nfunc TestReal(t *testing.T) {}\n",
		"other/o.go":         "package other\n",
		"other/o_test.go":    "package other\n\nimport \"testing\"\n\nfunc TestOther(t *testing.T) {}\n",
		"failing/f.go":       "package failing\n",
		"failing/f_test.go":  "package failing\n\nimport \"testing\"\n\nfunc TestBad(t *testing.T) { t.Fatal(\"boom\") }\n",
		"nofiles/nofiles.go": "package nofiles\n",
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)

	nope := []string{"go", "test", "-run", "TestNope", "./tested"}
	for _, tc := range []struct {
		name  string
		argv  []string
		shows string
		warns bool
	}{
		{"direct run", nope, "[no tests to run]", true},
		{"cached result", nope, "(cached)", true},
		{"verbose run", []string{"go", "test", "-v", "-run", "TestNope", "./tested"}, "PASS", true},
		{"coverage run", []string{"go", "test", "-cover", "-run", "TestNope", "./tested"}, "coverage:", true},
		{"pipe keeping the last line", []string{"sh", "-c", "go test -run TestNope ./tested 2>&1 | tail -1"}, "[no tests to run]", true},
		{"package without test files", []string{"go", "test", "./nofiles"}, "[no test files]", true},
		{"no test files beside an empty package", []string{"go", "test", "-run", "TestNope", "./tested", "./nofiles"}, "[no test files]", true},
		{"compile-only selector", []string{"go", "test", "-run", "^$", "./tested"}, "[no tests to run]", false},
		{"a test ran", []string{"go", "test", "-run", "TestReal", "./tested"}, "ok", false},
		{"one package ran among empty ones", []string{"go", "test", "-run", "TestReal", "./..."}, "[no tests to run]", false},
		{"failure hidden by a pipe", []string{"sh", "-c", "go test -run TestBad ./failing ./other 2>&1 | tail -4"}, "[no tests to run]", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := shell.NewExecRunner().Run(context.Background(), tc.argv[0], tc.argv[1:]...)
			var exitErr interface{ ExitCode() int }
			if err != nil && !errors.As(err, &exitErr) {
				t.Fatalf("run %v: %v", tc.argv, err)
			}
			if response.ExitCode != 0 {
				t.Fatalf("%v exited %d, want a passing step:\n%s%s", tc.argv, response.ExitCode, response.Stdout, response.Stderr)
			}
			output := response.Stdout + response.Stderr
			if !strings.Contains(output, tc.shows) {
				t.Fatalf("the output does not show %q, so this row tests nothing:\n%s", tc.shows, output)
			}
			step := proofCommand{Name: tc.argv[0], Args: tc.argv[1:], Display: "the step"}
			if got := emptyRunWarning("1.1", step, output) != ""; got != tc.warns {
				t.Fatalf("warns = %v, want %v, on the output:\n%s", got, tc.warns, output)
			}
		})
	}
}

// leafWarning spells out the warning of a writeLeafPlan leaf, so the tests pin
// the text and not the function that builds it.
func leafWarning(id int) string {
	return fmt.Sprintf(`task %d: command ["go","test","./pkg%d"] passed without running any test: check its selector and assert the test by name with -v and expect_output`, id, id)
}

const identityWarning = "code identity unavailable (git not usable here); evidence recorded without it"

func TestCompleteTaskWarnsOnEmptyRun(t *testing.T) {
	complete := func(t *testing.T, response testutil.Response) (string, TaskCompletionResult) {
		t.Helper()
		root := t.TempDir()
		writeLeafPlan(t, root, 1)
		result, err := CompleteTask(context.Background(), root, verifyTaskFeature, "1", testutil.NewFakeRunner(response))
		if err != nil {
			t.Fatalf("CompleteTask: %v", err)
		}
		return root, result
	}

	t.Run("completes the task and reports the warning", func(t *testing.T) {
		overrideIdentityRunner(t, identityYielding(identityBefore))
		root, result := complete(t, testutil.Response{Stdout: emptyPackageLine})

		if !slices.Equal(result.Warnings, []string{leafWarning(1)}) {
			t.Fatalf("warnings = %q, want the warning of task 1 alone", result.Warnings)
		}
		if !slices.Equal(result.CompletedTasks, []string{"1"}) {
			t.Fatalf("completed = %v, want task 1", result.CompletedTasks)
		}
		plan, err := os.ReadFile(filepath.Join(root, ".walden", "specs", verifyTaskFeature, "tasks.md"))
		if err != nil {
			t.Fatalf("read plan: %v", err)
		}
		if !strings.Contains(string(plan), "- [x] 1. Task 1") {
			t.Fatalf("checkbox did not move:\n%s", plan)
		}
	})

	t.Run("a proof that ran tests warns nothing and keeps the slice", func(t *testing.T) {
		overrideIdentityRunner(t, identityYielding(identityBefore))
		_, result := complete(t, testutil.Response{Stdout: testedPackageLine})

		if result.Warnings == nil || len(result.Warnings) != 0 {
			t.Fatalf("warnings = %#v, want an empty non-nil slice", result.Warnings)
		}
	})

	t.Run("the identity warning follows", func(t *testing.T) {
		overrideIdentityRunner(t, &scriptedIdentityRunner{fail: true})
		_, result := complete(t, testutil.Response{Stdout: emptyPackageLine})

		if want := []string{leafWarning(1), identityWarning}; !slices.Equal(result.Warnings, want) {
			t.Fatalf("warnings = %q, want %q", result.Warnings, want)
		}
	})
}

func TestEmptyRunRecordMatchesPassingProof(t *testing.T) {
	overrideIdentityRunner(t, identityYielding(identityBefore))

	record := func(t *testing.T, stdout string) (evidence.Record, []byte) {
		t.Helper()
		root := t.TempDir()
		writeLeafPlan(t, root, 1)
		if _, err := CompleteTask(context.Background(), root, verifyTaskFeature, "1", testutil.NewFakeRunner(testutil.Response{Stdout: stdout})); err != nil {
			t.Fatalf("CompleteTask: %v", err)
		}
		raw, err := os.ReadFile(evidence.DocumentPath(root, verifyTaskFeature))
		if err != nil {
			t.Fatalf("read ledger: %v", err)
		}
		return loadLeafLedger(t, root).Tasks["1"], raw
	}
	setAside := func(record evidence.Record) evidence.Record {
		record.VerifiedAt = ""
		steps := slices.Clone(record.Steps)
		for i := range steps {
			steps[i].OutputDigest = ""
		}
		record.Steps = steps
		return record
	}

	warned, warnedRaw := record(t, emptyPackageLine)
	tested, testedRaw := record(t, testedPackageLine)

	if len(warned.Steps) != 1 {
		t.Fatalf("warned record steps = %+v, want one", warned.Steps)
	}
	if warned.Steps[0].OutputDigest != evidence.DigestOutput(emptyPackageLine) {
		t.Fatalf("output digest = %q, want the digest of the proof output", warned.Steps[0].OutputDigest)
	}
	if !reflect.DeepEqual(setAside(warned), setAside(tested)) {
		t.Fatalf("records differ:\nwarned: %+v\ntested: %+v", setAside(warned), setAside(tested))
	}
	for name, raw := range map[string][]byte{"warned": warnedRaw, "tested": testedRaw} {
		for _, word := range []string{"warning", "without running any test"} {
			if bytes.Contains(raw, []byte(word)) {
				t.Fatalf("%s ledger holds %q:\n%s", name, word, raw)
			}
		}
	}
}

// gateClosingRunner answers one proof and then changes the approved
// requirements, so the next readiness check of the batch finds the gate closed.
type gateClosingRunner struct {
	root  string
	inner shell.Runner
}

func (r gateClosingRunner) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	response, err := r.inner.Run(ctx, name, args...)
	path := filepath.Join(r.root, ".walden", "specs", verifyTaskFeature, "requirements.md")
	file, openErr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if openErr != nil {
		return response, openErr
	}
	defer file.Close()
	if _, writeErr := file.WriteString("\nA change after approval.\n"); writeErr != nil {
		return response, writeErr
	}
	return response, err
}

func TestCompleteAllKeepsEmptyRunWarnings(t *testing.T) {
	empty := testutil.Response{Stdout: emptyPackageLine}
	batch := func(t *testing.T, count int, responses ...testutil.Response) (BatchCompletionResult, error) {
		t.Helper()
		root := t.TempDir()
		writeLeafPlan(t, root, count)
		return CompleteAllTasks(context.Background(), root, verifyTaskFeature, testutil.NewFakeRunner(responses...))
	}

	t.Run("every empty task in plan order", func(t *testing.T) {
		overrideIdentityRunner(t, identityYielding(identityBefore))
		result, err := batch(t, 3, empty, testutil.Response{Stdout: testedPackageLine}, empty)
		if err != nil {
			t.Fatalf("CompleteAllTasks: %v", err)
		}
		if want := []string{leafWarning(1), leafWarning(3)}; !slices.Equal(result.Warnings, want) {
			t.Fatalf("warnings = %q, want %q", result.Warnings, want)
		}
	})

	t.Run("a later failure keeps them", func(t *testing.T) {
		overrideIdentityRunner(t, identityYielding(identityBefore))
		result, err := batch(t, 2, empty, testutil.Response{ExitCode: 1})
		if err == nil {
			t.Fatal("expected the second proof to fail the batch")
		}
		if result.FailedTask != "2" {
			t.Fatalf("failed task = %q, want 2", result.FailedTask)
		}
		if want := []string{leafWarning(1)}; !slices.Equal(result.Warnings, want) {
			t.Fatalf("warnings = %q, want %q", result.Warnings, want)
		}
	})

	t.Run("a closed gate keeps them", func(t *testing.T) {
		overrideIdentityRunner(t, identityYielding(identityBefore))
		root := t.TempDir()
		writeLeafPlan(t, root, 2)
		runner := gateClosingRunner{root: root, inner: testutil.NewFakeRunner(empty)}

		result, err := CompleteAllTasks(context.Background(), root, verifyTaskFeature, runner)
		if err == nil {
			t.Fatal("expected the closed gate to stop the batch")
		}
		if result.FailedTask != "" {
			t.Fatalf("failed task = %q, want the gate to stop the batch, not a proof", result.FailedTask)
		}
		if want := []string{leafWarning(1)}; !slices.Equal(result.Warnings, want) {
			t.Fatalf("warnings = %q, want %q", result.Warnings, want)
		}
	})

	t.Run("the identity warning it used to drop", func(t *testing.T) {
		overrideIdentityRunner(t, &scriptedIdentityRunner{fail: true})
		result, err := batch(t, 1, empty)
		if err != nil {
			t.Fatalf("CompleteAllTasks: %v", err)
		}
		if want := []string{leafWarning(1), identityWarning}; !slices.Equal(result.Warnings, want) {
			t.Fatalf("warnings = %q, want %q", result.Warnings, want)
		}
	})

	t.Run("nothing to report stays nil", func(t *testing.T) {
		overrideIdentityRunner(t, identityYielding(identityBefore))
		result, err := batch(t, 2, testutil.Response{Stdout: testedPackageLine}, testutil.Response{Stdout: testedPackageLine})
		if err != nil {
			t.Fatalf("CompleteAllTasks: %v", err)
		}
		if result.Warnings != nil {
			t.Fatalf("warnings = %#v, want nil", result.Warnings)
		}
	})
}

func TestVerifyReportsEmptyRunWarnings(t *testing.T) {
	empty := testutil.Response{Stdout: emptyPackageLine}
	bothLeaves := []string{leafWarning(1), leafWarning(2)}

	assertVerified := func(t *testing.T, result VerifyResult, count int) {
		t.Helper()
		if len(result.Failed) != 0 {
			t.Fatalf("failed = %v, want none", result.Failed)
		}
		if len(result.Outcomes) != count {
			t.Fatalf("outcomes = %+v, want %d", result.Outcomes, count)
		}
		for _, outcome := range result.Outcomes {
			if !outcome.Passed || outcome.State != evidence.StateVerified {
				t.Fatalf("outcome = %+v, want passed and verified", outcome)
			}
		}
	}

	t.Run("plain run", func(t *testing.T) {
		root := staleLeaves(t, 2)
		result, err := Verify(context.Background(), root, verifyTaskFeature, false, false, testutil.NewFakeRunner(empty, empty))
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if !slices.Equal(result.Warnings, bothLeaves) {
			t.Fatalf("warnings = %q, want %q", result.Warnings, bothLeaves)
		}
		assertVerified(t, result, 2)
	})

	t.Run("all", func(t *testing.T) {
		root := completedLeaves(t, 2)
		result, err := Verify(context.Background(), root, verifyTaskFeature, true, false, testutil.NewFakeRunner(empty, empty))
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if !slices.Equal(result.Warnings, bothLeaves) {
			t.Fatalf("warnings = %q, want %q", result.Warnings, bothLeaves)
		}
		assertVerified(t, result, 2)
	})

	t.Run("one task", func(t *testing.T) {
		root := completedLeaves(t, 2)
		result, err := VerifyTask(context.Background(), root, verifyTaskFeature, "2", false, testutil.NewFakeRunner(empty))
		if err != nil {
			t.Fatalf("VerifyTask: %v", err)
		}
		if want := []string{leafWarning(2)}; !slices.Equal(result.Warnings, want) {
			t.Fatalf("warnings = %q, want %q", result.Warnings, want)
		}
		assertVerified(t, result, 1)
	})

	t.Run("check", func(t *testing.T) {
		root := staleLeaves(t, 2)
		before, err := os.ReadFile(evidence.DocumentPath(root, verifyTaskFeature))
		if err != nil {
			t.Fatalf("read ledger: %v", err)
		}
		result, err := Verify(context.Background(), root, verifyTaskFeature, false, true, testutil.NewFakeRunner(empty, empty))
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if !slices.Equal(result.Warnings, bothLeaves) {
			t.Fatalf("warnings = %q, want %q", result.Warnings, bothLeaves)
		}
		after, err := os.ReadFile(evidence.DocumentPath(root, verifyTaskFeature))
		if err != nil {
			t.Fatalf("read ledger: %v", err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("check changed the ledger:\n%s\nwas:\n%s", after, before)
		}
	})

	t.Run("a task skipped as verified gives none", func(t *testing.T) {
		root := completedLeaves(t, 2)
		runner := testutil.NewFakeRunner(empty, empty)
		result, err := Verify(context.Background(), root, verifyTaskFeature, false, false, runner)
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if calls := runner.Calls(); len(calls) != 0 {
			t.Fatalf("proofs run = %d, want none", len(calls))
		}
		if !slices.Equal(result.Skipped, []string{"1", "2"}) || len(result.Warnings) != 0 {
			t.Fatalf("skipped = %v, warnings = %q, want both skipped and no warning", result.Skipped, result.Warnings)
		}
	})

	t.Run("a failed proof gives none of its own", func(t *testing.T) {
		root := staleLeaves(t, 2)
		failing := testutil.Response{Stdout: emptyPackageLine, ExitCode: 1}
		result, err := Verify(context.Background(), root, verifyTaskFeature, false, false, testutil.NewFakeRunner(empty, failing))
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if want := []string{leafWarning(1)}; !slices.Equal(result.Warnings, want) {
			t.Fatalf("warnings = %q, want %q", result.Warnings, want)
		}
		if !slices.Equal(result.Failed, []string{"2"}) {
			t.Fatalf("failed = %v, want task 2", result.Failed)
		}
	})

	t.Run("a proof that ran tests gives none", func(t *testing.T) {
		root := staleLeaves(t, 2)
		tested := testutil.Response{Stdout: testedPackageLine}
		result, err := Verify(context.Background(), root, verifyTaskFeature, false, false, testutil.NewFakeRunner(tested, tested))
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if len(result.Warnings) != 0 {
			t.Fatalf("warnings = %q, want none", result.Warnings)
		}
	})
}
