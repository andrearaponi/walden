package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrearaponi/walden/internal/evidence"
	"github.com/andrearaponi/walden/internal/shell"
	"github.com/andrearaponi/walden/internal/spec"
	"github.com/andrearaponi/walden/internal/testutil"
)

type integrityProofFunc func(context.Context, string, ...string) (shell.Response, error)

func (f integrityProofFunc) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	return f(ctx, name, args...)
}

type integrityCaptureRunner struct {
	inner      shell.Runner
	failNext   bool
	statuses   int
	failStatus int
}

func (r *integrityCaptureRunner) Run(ctx context.Context, name string, args ...string) (shell.Response, error) {
	for _, arg := range args {
		if arg == "status" {
			r.statuses++
			if r.failStatus > 0 && r.statuses == r.failStatus {
				return shell.Response{}, fmt.Errorf("injected boundary failure")
			}
		}
	}
	if r.failNext {
		r.failNext = false
		return shell.Response{}, fmt.Errorf("injected identity failure")
	}
	return r.inner.Run(ctx, name, args...)
}

func executionFact(t *testing.T, record evidence.Record, key string) any {
	t.Helper()
	data, _ := json.Marshal(record)
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	facts, ok := value["execution"].(map[string]any)
	if !ok {
		t.Fatalf("new record lacks execution facts: %s", data)
	}
	return facts[key]
}

func TestEvidenceIntegrityVerifyPurity(t *testing.T) {
	for _, restore := range []bool{false, true} {
		for _, check := range []bool{false, true} {
			t.Run(fmt.Sprintf("contamination restore=%t check=%t", restore, check), func(t *testing.T) {
				root := t.TempDir()
				writeVerifyFixture(t, root)
				overrideIdentityRunner(t, &dynamicIdentityRunner{root: root})
				completeBoth(t, root)
				before, _ := os.ReadFile(evidence.DocumentPath(root, "todo-app-demo"))
				calls := 0
				path := filepath.Join(root, "generated.txt")
				runner := integrityProofFunc(func(_ context.Context, _ string, _ ...string) (shell.Response, error) {
					calls++
					if calls == 1 {
						if err := os.WriteFile(path, []byte("contaminated"), 0o644); err != nil {
							t.Fatal(err)
						}
					} else {
						if _, err := os.Stat(path); err != nil {
							t.Fatal("successor did not observe the mutated tree")
						}
						if restore {
							if err := os.Remove(path); err != nil {
								t.Fatal(err)
							}
						}
					}
					return shell.Response{Stdout: "ok"}, nil
				})
				result, err := Verify(context.Background(), root, "todo-app-demo", true, check, runner)
				if err != nil {
					t.Fatal(err)
				}
				if calls != 2 || len(result.Failed) != 2 || result.Outcomes[1].Passed || result.Outcomes[1].State == "verified" {
					t.Fatalf("contaminated successor earned a pass: %+v, calls=%d", result, calls)
				}
				if !strings.Contains(result.Outcomes[0].Failure, "generated.txt") || !strings.Contains(result.Outcomes[1].Failure, "contaminat") {
					t.Fatalf("missing policy/path diagnosis: %+v", result.Outcomes)
				}
				if !restore {
					if _, err := os.Stat(path); err != nil {
						t.Fatal("verify rolled back source")
					}
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
				after, _ := os.ReadFile(evidence.DocumentPath(root, "todo-app-demo"))
				if check {
					if string(before) != string(after) {
						t.Fatal("check mode persisted evidence")
					}
					return
				}
				ledger, err := evidence.Load(root, "todo-app-demo")
				if err != nil {
					t.Fatal(err)
				}
				if ledger.Tasks["1.2"].CodeIdentity == ledger.Tasks["1.1"].CodeIdentity {
					t.Fatal("successor was falsely anchored to the initial tree")
				}
				if ledger.Tasks["1.2"].Result != evidence.ResultFailed || executionFact(t, ledger.Tasks["1.2"], "origin") != "verify" || executionFact(t, ledger.Tasks["1.2"], "assertion_result") != "passed" {
					t.Fatal("policy failure overwrote assertion facts or was not persisted")
				}
				retry := testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}, testutil.Response{ExitCode: 1, Stderr: "fails on restored code"})
				again, err := Verify(context.Background(), root, "todo-app-demo", false, false, retry)
				if err != nil {
					t.Fatal(err)
				}
				if len(retry.Calls()) != 2 || len(again.Failed) != 1 || again.Failed[0] != "1.2" {
					t.Fatalf("restoration revived unearned evidence: %+v", again)
				}
			})
		}
	}
	t.Run("capture failure remains sticky", func(t *testing.T) {
		root := t.TempDir()
		writeVerifyFixture(t, root)
		capture := &integrityCaptureRunner{inner: &dynamicIdentityRunner{root: root}}
		overrideIdentityRunner(t, capture)
		completeBoth(t, root)
		calls := 0
		runner := integrityProofFunc(func(_ context.Context, _ string, _ ...string) (shell.Response, error) {
			calls++
			if calls == 1 {
				capture.failNext = true
			}
			return shell.Response{Stdout: "ok"}, nil
		})
		result, err := Verify(context.Background(), root, "todo-app-demo", true, false, runner)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Failed) != 2 || calls != 2 {
			t.Fatalf("missing capture allowed success: %+v", result)
		}
	})
	t.Run("missing second pre-capture", func(t *testing.T) {
		root := t.TempDir()
		writeVerifyFixture(t, root)
		capture := &integrityCaptureRunner{inner: &dynamicIdentityRunner{root: root}}
		overrideIdentityRunner(t, capture)
		completeBoth(t, root)
		capture.statuses, capture.failStatus = 0, 3
		runner := testutil.NewFakeRunner(testutil.Response{Stdout: "ok"}, testutil.Response{Stdout: "ok"})
		result, err := Verify(context.Background(), root, "todo-app-demo", true, false, runner)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Failed) != 1 || result.Failed[0] != "1.2" || len(runner.Calls()) != 2 {
			t.Fatalf("missing pre-capture was accepted: %+v", result)
		}
	})
	t.Run("timeout and mutation preserve both diagnoses", func(t *testing.T) {
		root := t.TempDir()
		writeVerifyFixture(t, root)
		feature, err := spec.LoadFeature(root, "todo-app-demo")
		if err != nil {
			t.Fatal(err)
		}
		body := strings.Replace(feature.Tasks.Body, "[\"go\", \"test\", \"./internal/spec\"]", "[\"go\", \"test\", \"./internal/spec\"]\n        timeout: 10ms", 1)
		writeFreshFeatureDoc(t, root, "todo-app-demo", "tasks.md", approvedTasksContent(body, feature.Tasks.ApprovedAt, feature.Design.ApprovedAt, feature.Design.ApprovedFingerprint))
		overrideIdentityRunner(t, &dynamicIdentityRunner{root: root})
		completeBoth(t, root)
		calls := 0
		runner := integrityProofFunc(func(ctx context.Context, _ string, _ ...string) (shell.Response, error) {
			calls++
			if calls == 1 {
				if err := os.WriteFile(filepath.Join(root, "generated.txt"), []byte("changed"), 0o644); err != nil {
					t.Fatal(err)
				}
				<-ctx.Done()
			}
			return shell.Response{Stdout: "ok"}, nil
		})
		result, err := Verify(context.Background(), root, "todo-app-demo", true, false, runner)
		if err != nil {
			t.Fatal(err)
		}
		if calls != 2 || len(result.Failed) != 2 || !strings.Contains(result.Outcomes[0].Failure, "timeout") || !strings.Contains(result.Outcomes[0].Failure, "generated.txt") {
			t.Fatalf("timeout/purity distinction lost: %+v", result)
		}
	})
	t.Run("pure prefix and completion facts", func(t *testing.T) {
		root := t.TempDir()
		writeVerifyFixture(t, root)
		overrideIdentityRunner(t, &dynamicIdentityRunner{root: root})
		completeBoth(t, root)
		ledger, err := evidence.Load(root, "todo-app-demo")
		if err != nil {
			t.Fatal(err)
		}
		if executionFact(t, ledger.Tasks["1.1"], "origin") != "complete" || executionFact(t, ledger.Tasks["1.1"], "integrity") != "post-state" {
			t.Fatal("completion claimed a verify guarantee")
		}
		calls := 0
		runner := integrityProofFunc(func(_ context.Context, _ string, _ ...string) (shell.Response, error) {
			calls++
			if calls == 2 {
				if err := os.WriteFile(filepath.Join(root, "generated.txt"), []byte("changed"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			return shell.Response{Stdout: "ok"}, nil
		})
		result, err := Verify(context.Background(), root, "todo-app-demo", true, false, runner)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Outcomes[0].Passed || result.Outcomes[0].State != "stale-code" || len(result.Failed) != 1 {
			t.Fatalf("pure prefix reassigned or reported fresh for later tree: %+v", result)
		}
		ledger, _ = evidence.Load(root, "todo-app-demo")
		if ledger.Tasks["1.1"].Result != evidence.ResultPassed {
			t.Fatal("pure earlier proof retroactively failed")
		}
	})
	t.Run("unavailable preflight executes nothing", func(t *testing.T) {
		root := t.TempDir()
		writeVerifyFixture(t, root)
		overrideIdentityRunner(t, identityYielding("100644 blob aaa\tmain.go\n"))
		completeBoth(t, root)
		overrideIdentityRunner(t, &scriptedIdentityRunner{fail: true})
		runner := testutil.NewFakeRunner()
		if _, err := Verify(context.Background(), root, "todo-app-demo", true, false, runner); err == nil || len(runner.Calls()) != 0 {
			t.Fatalf("unavailable preflight executed proofs or claimed success: %v", err)
		}
	})
}
